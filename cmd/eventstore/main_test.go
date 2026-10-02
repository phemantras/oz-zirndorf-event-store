package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
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
	err := run(context.Background(), slog.New(slog.DiscardHandler), envFrom(map[string]string{
		envDatabaseURL: "postgres://%zz",
		envPort:        "8080",
	}))
	if err == nil {
		t.Fatal("run returned no error for a malformed database url")
	}
}

func TestRunErrorDoesNotLeakPasswordFromUnparsableURL(t *testing.T) {
	err := run(context.Background(), slog.New(slog.DiscardHandler), envFrom(map[string]string{
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
	err := run(context.Background(), slog.New(slog.DiscardHandler), envFrom(map[string]string{
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
		done <- run(ctx, slog.New(slog.DiscardHandler), envFrom(map[string]string{
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

	cancel()
	if err := <-done; err != nil {
		t.Errorf("run returned %v, want nil after shutdown", err)
	}
}

func TestServeAnswersUntilContextIsCancelled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := newServer(&fakePinger{}, slog.New(slog.DiscardHandler))
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
	server := newServer(&fakePinger{}, slog.New(slog.DiscardHandler))

	if err := serve(context.Background(), server, listener, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("serve returned no error for a closed listener")
	}
}
