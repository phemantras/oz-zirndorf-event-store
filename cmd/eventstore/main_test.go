package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// unreachableDatabaseURL points at a port where no server listens, so the
// migration step fails fast.
const unreachableDatabaseURL = "postgres://eventstore:secret@127.0.0.1:1/eventstore?connect_timeout=1"

func TestNewLoggerWritesJSON(t *testing.T) {
	var out bytes.Buffer
	newLogger(&out).Info("hello", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("log line %q is not JSON: %v", out.String(), err)
	}
	if entry["msg"] != "hello" || entry["key"] != "value" {
		t.Errorf("entry = %v, want msg=hello key=value", entry)
	}
}

func TestRunFailsWithoutRequiredVariables(t *testing.T) {
	err := run(context.Background(), slog.New(slog.DiscardHandler), envFrom(map[string]string{}))
	var missing *missingVariablesError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want *missingVariablesError", err)
	}
}

func TestRunFailsWithMalformedDatabaseURL(t *testing.T) {
	err := run(context.Background(), slog.New(slog.DiscardHandler), validEnv(map[string]string{
		envDatabaseURL: "postgres://%zz",
		envPort:        "8080",
	}))
	if err == nil {
		t.Fatal("run returned no error for a malformed database url")
	}
}

func TestRunErrorDoesNotLeakPasswordFromUnparsableURL(t *testing.T) {
	err := run(context.Background(), slog.New(slog.DiscardHandler), validEnv(map[string]string{
		envDatabaseURL: "postgres://eventstore:secret@localhost:5432/eventstore?sslmode=bogus",
		envPort:        "8080",
	}))
	if err == nil {
		t.Fatal("run returned no error for an invalid sslmode")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q leaks the database password", err)
	}
}

func TestRunFailsWhenMigrationFails(t *testing.T) {
	port := freePort(t)
	err := run(context.Background(), slog.New(slog.DiscardHandler), validEnv(map[string]string{
		envDatabaseURL: unreachableDatabaseURL,
		envPort:        port,
	}))
	if err == nil || !strings.Contains(err.Error(), "migrate") {
		t.Fatalf("err = %v, want a migration error", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q leaks the database password", err)
	}
	if conn, dialErr := net.Dial("tcp", "127.0.0.1:"+port); dialErr == nil {
		_ = conn.Close()
		t.Error("an HTTP listener exists although the migration failed")
	}
}

// testDatabaseURLVariable names the environment variable that points the
// integration tests at a real PostgreSQL 18 database.
const testDatabaseURLVariable = "EVENTSTORE_TEST_DATABASE_URL"

// ciVariable is set by CI runners; there a missing test database is an error,
// not a reason to skip.
const ciVariable = "CI"

const healthPollInterval = 50 * time.Millisecond

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	return port
}

func TestRunMigratesThenServesHealthUntilCancelled(t *testing.T) {
	databaseURL := os.Getenv(testDatabaseURLVariable)
	if databaseURL == "" {
		if os.Getenv(ciVariable) != "" {
			t.Fatalf("%s must be set in CI", testDatabaseURLVariable)
		}
		t.Skipf("%s not set, skipping PostgreSQL integration test", testDatabaseURLVariable)
	}
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, slog.New(slog.DiscardHandler), validEnv(map[string]string{
			envDatabaseURL: databaseURL,
			envPort:        port,
		}))
	}()

	url := "http://127.0.0.1:" + port + healthPath
	deadline := time.Now().Add(shutdownTimeout)
	for {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			break
		}
		select {
		case err := <-done:
			t.Fatalf("run returned before serving: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not answer: %v", err)
		}
		time.Sleep(healthPollInterval)
	}

	baseURL := "http://127.0.0.1:" + port
	assertStatus(t, baseURL+adminLoginPath, http.StatusOK)
	assertLocationsServedWithSession(t, baseURL)

	cancel()
	if err := <-done; err != nil {
		t.Errorf("run returned %v, want nil after shutdown", err)
	}
}

// assertLocationsServedWithSession logs in at baseURL and checks that the
// location list, wired to PostgreSQL, answers with the session.
func assertLocationsServedWithSession(t *testing.T, baseURL string) {
	t.Helper()
	// The session cookie is Secure, so a cookie jar would drop it over plain
	// HTTP; the test carries it by hand and stops at the login redirect.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	form := url.Values{"username": {validAdminUser}, "password": {validAdminPassword}}
	login, err := client.PostForm(baseURL+adminLoginPath, form)
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	_ = login.Body.Close()
	cookies := login.Cookies()
	if login.StatusCode != http.StatusSeeOther || len(cookies) != 1 {
		t.Fatalf("login: status = %d with %d cookies, want %d with the session cookie",
			login.StatusCode, len(cookies), http.StatusSeeOther)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+adminLocationsPath, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.AddCookie(cookies[0])
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", adminLocationsPath, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET %s with session: status = %d, want %d", adminLocationsPath, resp.StatusCode, http.StatusOK)
	}
}

func TestServeAnswersUntilContextIsCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	logger := slog.New(slog.DiscardHandler)
	server := newServer(&fakePinger{}, logger, newAdminHandler(validConfig(t), logger, emptyLocations{}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener, slog.New(slog.DiscardHandler)) }()

	resp, err := http.Get("http://" + listener.Addr().String() + healthPath)
	if err != nil {
		t.Fatalf("GET healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	assertStatus(t, "http://"+listener.Addr().String()+adminLoginPath, http.StatusOK)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve returned %v, want nil after shutdown", err)
		}
	case <-time.After(shutdownTimeout):
		t.Fatal("serve did not return after context cancellation")
	}
}

func TestServeReturnsErrorWhenListenerFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_ = listener.Close()
	server := newServer(&fakePinger{}, slog.New(slog.DiscardHandler), http.NotFoundHandler())

	if err := serve(context.Background(), server, listener, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("serve returned no error for a closed listener")
	}
}

// adminLoginPath is where the admin login form is served.
const adminLoginPath = "/admin/login"

// adminLocationsPath lists the locations; it needs a session.
const adminLocationsPath = "/admin/locations"

// validAdminPassword is the password behind validPasswordHash.
const validAdminPassword = "richtig-und-lang"

// emptyLocations stands in for the location use cases where no database is
// available: there are no locations.
type emptyLocations struct{}

func (emptyLocations) SaveLocation(context.Context, string, core.LocationInput) (core.Location, error) {
	return core.Location{}, core.ErrNotFound
}

func (emptyLocations) GetLocation(context.Context, string) (core.Location, error) {
	return core.Location{}, core.ErrNotFound
}

func (emptyLocations) ListLocations(context.Context) ([]core.Location, error) {
	return nil, nil
}

func validConfig(t *testing.T) config {
	t.Helper()
	cfg, err := loadConfig(validEnv(nil))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	return cfg
}

func assertStatus(t *testing.T, url string, want int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		t.Errorf("GET %s: status = %d, want %d", url, resp.StatusCode, want)
	}
}

func TestRunFailsWithWeakSessionSecretWithoutLeakingIt(t *testing.T) {
	const shortSecret = "only-thirty-one-bytes-long-abcd"
	err := run(context.Background(), slog.New(slog.DiscardHandler), validEnv(map[string]string{
		envSessionSecret: shortSecret,
	}))
	if !errors.Is(err, errSessionSecretTooShort) {
		t.Fatalf("err = %v, want errSessionSecretTooShort", err)
	}
	if strings.Contains(err.Error(), shortSecret) {
		t.Errorf("error %q leaks the session secret", err)
	}
}

func TestNewAdminHandlerLogsInWithConfiguredCredentials(t *testing.T) {
	handler := newAdminHandler(validConfig(t), slog.New(slog.DiscardHandler), emptyLocations{})
	form := url.Values{"username": {validAdminUser}, "password": {validAdminPassword}}
	login := httptest.NewRequest(http.MethodPost, adminLoginPath, strings.NewReader(form.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, login)

	if loginRec.Code != http.StatusSeeOther || loginRec.Header().Get("Location") != adminPath {
		t.Fatalf("login: status = %d, Location = %q, want %d to %s",
			loginRec.Code, loginRec.Header().Get("Location"), http.StatusSeeOther, adminPath)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login set %d cookies, want 1 session cookie", len(cookies))
	}
	home := httptest.NewRequest(http.MethodGet, adminPath, nil)
	home.AddCookie(cookies[0])
	homeRec := httptest.NewRecorder()
	handler.ServeHTTP(homeRec, home)
	if homeRec.Code != http.StatusOK {
		t.Errorf("home with session cookie: status = %d, want %d", homeRec.Code, http.StatusOK)
	}
	locations := httptest.NewRequest(http.MethodGet, adminLocationsPath, nil)
	locations.AddCookie(cookies[0])
	locationsRec := httptest.NewRecorder()
	handler.ServeHTTP(locationsRec, locations)
	if locationsRec.Code != http.StatusOK {
		t.Errorf("locations with session cookie: status = %d, want %d", locationsRec.Code, http.StatusOK)
	}
}
