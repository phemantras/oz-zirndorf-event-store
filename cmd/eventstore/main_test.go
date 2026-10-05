package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
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
	databaseURL := testDatabaseURL(t)
	running := startRun(t, databaseURL, slog.New(slog.DiscardHandler))

	assertStatus(t, running.baseURL+adminLoginPath, http.StatusOK)
	assertStatus(t, running.baseURL+publicEventTypesPath, http.StatusOK)
	assertStatus(t, running.baseURL+publicEventsPath, http.StatusOK)
	assertListsServedWithSession(t, running.baseURL)

	running.stop(t)
}

// TestRunLogsEventsWhoseRecomputationFailsAndStartsAnyway stores an event
// that today's rules reject (all day with a start time) and checks that the
// start logs its ID and still serves (ENT-5).
func TestRunLogsEventsWhoseRecomputationFailsAndStartsAnyway(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	brokenID := insertBrokenEvent(t, databaseURL)
	logs := &syncBuffer{}

	running := startRun(t, databaseURL, newLogger(logs))
	// The admin must share the event use cases with the recomputation, or
	// the review mark would be lost.
	status, body := logIn(t, running.baseURL).get(t, adminEventsPath)
	running.stop(t)

	if !strings.Contains(logs.String(), logMsgRecomputeFailed) || !strings.Contains(logs.String(), brokenID) {
		t.Errorf("log %q does not name the failed event %s", logs.String(), brokenID)
	}
	if status != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want %d", adminEventsPath, status, http.StatusOK)
	}
	link := `<a href="` + adminEventsPath + "/" + brokenID + `">` + brokenEventTitle + `</a>`
	if row := tableRowWith(body, link); !strings.Contains(row, "prüfen") {
		t.Errorf("row of the broken event %q is not marked prüfen; body: %s", row, body)
	}
}

// tableRowWith returns the table row of page that contains text, or "".
func tableRowWith(page, text string) string {
	for _, row := range strings.Split(page, "<tr>") {
		if before, _, found := strings.Cut(row, "</tr>"); found && strings.Contains(before, text) {
			return before
		}
	}
	return ""
}

// brokenEventTitle is the title of the event insertBrokenEvent stores.
const brokenEventTitle = "Kaputt bei Startprüfung"

// brokenLocationNameKey is the name key of the location insertBrokenEvent
// stores; leftovers of an aborted run are removed before inserting.
const brokenLocationNameKey = "startprüfung"

// insertBrokenEvent migrates the test database and inserts a location with
// an event that the core rejects, removing leftovers before and both
// afterwards.
func insertBrokenEvent(t *testing.T, databaseURL string) string {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	removeBrokenEvent(t, pool)
	t.Cleanup(func() { removeBrokenEvent(t, pool) })
	var locationID, eventID string
	err = pool.QueryRow(ctx,
		`INSERT INTO locations (name, name_key, street, postal_code, city, latitude, longitude, precision)
		 VALUES ('Startprüfung', $1, 'Marktplatz', '90513', 'Zirndorf', 49.44, 10.95, 'area')
		 RETURNING id::text`, brokenLocationNameKey).Scan(&locationID)
	if err != nil {
		t.Fatalf("insert location: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO events (title, type, location_id, start_date, start_time, all_day, source_description,
		                     effective_start, effective_end)
		 VALUES ($2, 'other', $1, '2026-10-16', '19:00', true, 'Test',
		         '2026-10-16 17:00+00', '2026-10-16 22:00+00')
		 RETURNING id::text`, locationID, brokenEventTitle).Scan(&eventID)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	return eventID
}

// removeBrokenEvent deletes the location of insertBrokenEvent and its
// events, also leftovers of an aborted earlier run.
func removeBrokenEvent(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		"DELETE FROM events WHERE location_id IN (SELECT id FROM locations WHERE name_key = $1)", brokenLocationNameKey)
	if err != nil {
		t.Errorf("delete events of the broken location: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM locations WHERE name_key = $1", brokenLocationNameKey); err != nil {
		t.Errorf("delete broken location: %v", err)
	}
}

// syncBuffer is a bytes.Buffer that the server goroutine may write while
// the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// testDatabaseURL returns the PostgreSQL test database or skips the test
// outside CI when there is none.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := os.Getenv(testDatabaseURLVariable)
	if databaseURL == "" {
		if os.Getenv(ciVariable) != "" {
			t.Fatalf("%s must be set in CI", testDatabaseURLVariable)
		}
		t.Skipf("%s not set, skipping PostgreSQL integration test", testDatabaseURLVariable)
	}
	return databaseURL
}

// runningService is a run in the background that answers on baseURL.
type runningService struct {
	baseURL string
	cancel  context.CancelFunc
	done    chan error
}

// startRun starts run against databaseURL on a free port and waits until
// the health check answers with 200.
func startRun(t *testing.T, databaseURL string, logger *slog.Logger) runningService {
	t.Helper()
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	running := runningService{baseURL: "http://127.0.0.1:" + port, cancel: cancel, done: make(chan error, 1)}
	go func() {
		running.done <- run(ctx, logger, validEnv(map[string]string{
			envDatabaseURL: databaseURL,
			envPort:        port,
		}))
	}()

	deadline := time.Now().Add(shutdownTimeout)
	for {
		resp, err := http.Get(running.baseURL + healthPath)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			return running
		}
		select {
		case err := <-running.done:
			t.Fatalf("run returned before serving: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not answer: %v", err)
		}
		time.Sleep(healthPollInterval)
	}
}

// stop cancels the run and expects a clean shutdown.
func (r runningService) stop(t *testing.T) {
	t.Helper()
	r.cancel()
	if err := <-r.done; err != nil {
		t.Errorf("run returned %v, want nil after shutdown", err)
	}
}

// assertListsServedWithSession logs in at baseURL and checks that the
// location and event lists, wired to PostgreSQL, answer with the session.
func assertListsServedWithSession(t *testing.T, baseURL string) {
	t.Helper()
	session := logIn(t, baseURL)
	for _, path := range []string{adminLocationsPath, adminEventsPath} {
		if status, _ := session.get(t, path); status != http.StatusOK {
			t.Errorf("GET %s with session: status = %d, want %d", path, status, http.StatusOK)
		}
	}
}

// adminSession is a logged-in admin at baseURL.
type adminSession struct {
	baseURL string
	client  *http.Client
	cookie  *http.Cookie
}

// logIn logs in at baseURL with the configured credentials.
func logIn(t *testing.T, baseURL string) adminSession {
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
	return adminSession{baseURL: baseURL, client: client, cookie: cookies[0]}
}

// get requests path with the session and returns status and body.
func (s adminSession) get(t *testing.T, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.AddCookie(s.cookie)
	resp, err := s.client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return resp.StatusCode, string(body)
}

func TestServeAnswersUntilContextIsCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	logger := slog.New(slog.DiscardHandler)
	server := newServer(&fakePinger{}, logger, newRouteHandlers(validConfig(t), logger, emptyUseCases()))
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
	assertStatus(t, "http://"+listener.Addr().String()+publicEventTypesPath, http.StatusOK)
	assertStatus(t, "http://"+listener.Addr().String()+publicEventsPath, http.StatusOK)
	assertAdminHasNoCORS(t, "http://"+listener.Addr().String()+adminLoginPath)

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
	server := newServer(&fakePinger{}, slog.New(slog.DiscardHandler), routeHandlers{admin: http.NotFoundHandler(), public: http.NotFoundHandler()})

	if err := serve(context.Background(), server, listener, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("serve returned no error for a closed listener")
	}
}

// publicEventTypesPath lists the event types of the public API.
const publicEventTypesPath = "/v1/event-types"

// publicEventsPath lists today's active events in the public API.
const publicEventsPath = "/v1/events"

// assertAdminHasNoCORS fails if the admin answers with a CORS header, which
// only the public API may send (NFR-1).
func assertAdminHasNoCORS(t *testing.T, url string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("GET %s Access-Control-Allow-Origin = %q, want none", url, got)
	}
}

// adminLoginPath is where the admin login form is served.
const adminLoginPath = "/admin/login"

// adminLocationsPath lists the locations; it needs a session.
const adminLocationsPath = "/admin/locations"

// adminEventsPath lists the events; it needs a session.
const adminEventsPath = "/admin/events"

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

func (emptyLocations) DeleteLocation(context.Context, string) error {
	return core.ErrNotFound
}

// emptyEvents stands in for the event use cases where no database is
// available: there are no events.
type emptyEvents struct{}

func (emptyEvents) SaveEvent(context.Context, string, core.EventInput, core.DuplicatePolicy) (core.Event, error) {
	return core.Event{}, core.ErrNotFound
}

func (emptyEvents) GetEvent(context.Context, string) (core.Event, error) {
	return core.Event{}, core.ErrNotFound
}

func (emptyEvents) ListEvents(context.Context, core.Clock) ([]core.EventListEntry, error) {
	return nil, nil
}

func (emptyEvents) DeleteEvent(context.Context, string) error {
	return core.ErrNotFound
}

func (emptyEvents) ListActiveEvents(context.Context, core.Clock, core.EventFilter) ([]core.ListedEvent, error) {
	return nil, nil
}

func (emptyEvents) ListArchivedEvents(context.Context, core.Clock, core.EventFilter) ([]core.ListedEvent, error) {
	return nil, nil
}

func emptyUseCases() useCases {
	return useCases{locations: emptyLocations{}, events: emptyEvents{}}
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
	handler := newAdminHandler(validConfig(t), slog.New(slog.DiscardHandler), emptyUseCases())
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
	for _, path := range []string{adminLocationsPath, adminEventsPath} {
		list := httptest.NewRequest(http.MethodGet, path, nil)
		list.AddCookie(cookies[0])
		listRec := httptest.NewRecorder()
		handler.ServeHTTP(listRec, list)
		if listRec.Code != http.StatusOK {
			t.Errorf("%s with session cookie: status = %d, want %d", path, listRec.Code, http.StatusOK)
		}
	}
}
