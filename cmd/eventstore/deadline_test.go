package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	publicapi "github.com/phemantras/oz-zirndorf-event-store/internal/adapter/publicapi/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

func TestRequestTimeoutEndsRequestsBeforeTheWriteTimeout(t *testing.T) {
	const wantRequestTimeout = 20 * time.Second
	if requestTimeout != wantRequestTimeout {
		t.Errorf("requestTimeout = %v, want %v", requestTimeout, wantRequestTimeout)
	}
	if requestTimeout >= writeTimeout {
		t.Errorf("requestTimeout %v must lie below writeTimeout %v, or the answer cannot be written", requestTimeout, writeTimeout)
	}
}

// deadlineRecorder is a handler that records the deadline of the request
// context it serves.
type deadlineRecorder struct {
	deadline    time.Time
	hasDeadline bool
}

func (d *deadlineRecorder) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	d.deadline, d.hasDeadline = r.Context().Deadline()
}

// deadlinePinger is a pinger that records the deadline of its context.
type deadlinePinger struct {
	deadlineRecorder
}

func (p *deadlinePinger) Ping(ctx context.Context) error {
	p.deadline, p.hasDeadline = ctx.Deadline()
	return nil
}

// assertDeadlineBetween fails unless recorded holds a deadline from
// earliest to latest.
func assertDeadlineBetween(t *testing.T, name string, recorded *deadlineRecorder, earliest, latest time.Time) {
	t.Helper()
	if !recorded.hasDeadline {
		t.Fatalf("%s: request context has no deadline", name)
	}
	if recorded.deadline.Before(earliest) || recorded.deadline.After(latest) {
		t.Errorf("%s: deadline %v lies outside %v to %v", name, recorded.deadline, earliest, latest)
	}
}

func TestWithRequestDeadlineGivesTheRequestContextTheTimeout(t *testing.T) {
	const timeout = time.Second
	recorder := &deadlineRecorder{}
	start := time.Now()

	withRequestDeadline(recorder, timeout).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assertDeadlineBetween(t, "handler", recorder, start.Add(timeout), time.Now().Add(timeout))
}

func TestServerGivesEveryRequestADeadline(t *testing.T) {
	pinger, adminHandler, publicHandler := &deadlinePinger{}, &deadlineRecorder{}, &deadlineRecorder{}
	server := newServer(pinger, slog.New(slog.DiscardHandler), routeHandlers{admin: adminHandler, public: publicHandler})
	start := time.Now()

	for _, path := range []string{healthPath, adminLoginPath, publicEventsPath} {
		server.Handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	served := time.Now()
	// The health check keeps its own, shorter ping timeout.
	assertDeadlineBetween(t, "health", &pinger.deadlineRecorder, start, served.Add(healthPingTimeout))
	assertDeadlineBetween(t, "admin", adminHandler, start.Add(requestTimeout), served.Add(requestTimeout))
	assertDeadlineBetween(t, "public", publicHandler, start.Add(requestTimeout), served.Add(requestTimeout))
}

// hangingPinger is a database whose ping answers only when its context
// ends.
type hangingPinger struct{}

func (hangingPinger) Ping(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestServerAnswersHealthWithUnavailableAtThePingTimeoutWhenTheDatabaseHangs(t *testing.T) {
	server := newServer(hangingPinger{}, slog.New(slog.DiscardHandler), routeHandlers{admin: http.NotFoundHandler(), public: http.NotFoundHandler()})
	rec := httptest.NewRecorder()
	start := time.Now()

	server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, healthPath, nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if elapsed := time.Since(start); elapsed < healthPingTimeout || elapsed > healthPingTimeout+answerReserve {
		t.Errorf("answer took %v, want %v plus at most %v", elapsed, healthPingTimeout, answerReserve)
	}
}

// Timing of the tests against a slow database: a short request deadline
// and the reserve within which the 503 must follow it.
const (
	shortRequestTimeout = 200 * time.Millisecond
	answerReserve       = 2 * time.Second
)

// migratedPool returns a pool on the test database with all migrations
// applied, closed after the test.
func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return pool
}

// publicAPIWithShortDeadline returns the public API on pool, wired to the
// core as run does, behind a request deadline of shortRequestTimeout.
func publicAPIWithShortDeadline(pool *pgxpool.Pool, logs *bytes.Buffer) http.Handler {
	locations := postgres.NewLocationRepo(pool)
	events := core.NewEventService(postgres.NewTxRunner(pool), postgres.NewEventRepo(pool), locations)
	public := publicapi.NewHandler(publicapi.Config{Logger: newLogger(logs), Events: events, Clock: systemClock{}})
	return withRequestDeadline(public, shortRequestTimeout)
}

// assertServiceUnavailableInTime requests the active events and expects a
// 503 problem shortly after the deadline, logged as warning only.
func assertServiceUnavailableInTime(t *testing.T, handler http.Handler, logs *bytes.Buffer) {
	t.Helper()
	rec := httptest.NewRecorder()
	start := time.Now()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, publicEventsPath, nil))

	if elapsed := time.Since(start); elapsed > shortRequestTimeout+answerReserve {
		t.Errorf("answer took %v, want at most %v", elapsed, shortRequestTimeout+answerReserve)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var problem publicapi.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem %q: %v", rec.Body.String(), err)
	}
	if problem.Status != http.StatusServiceUnavailable || problem.Detail == "" {
		t.Errorf("problem = %+v, want status %d with a detail", problem, http.StatusServiceUnavailable)
	}
	assertLoggedAsTimeoutWarningOnly(t, logs)
}

// logLine is the level and message of a JSON log line.
type logLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// assertLoggedAsTimeoutWarningOnly checks that logs record the expired
// request as a warning of the public API and hold no error.
func assertLoggedAsTimeoutWarningOnly(t *testing.T, logs *bytes.Buffer) {
	t.Helper()
	want := logLine{Level: slog.LevelWarn.String(), Msg: "public api request timed out"}
	found := false
	for _, raw := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var line logLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("decode log line %q: %v", raw, err)
		}
		found = found || line == want
		if line.Level == slog.LevelError.String() {
			t.Errorf("log line %q reports an expired deadline as an error", raw)
		}
	}
	if !found {
		t.Errorf("log %q has no warning %q", logs.String(), want.Msg)
	}
}

// TestSlowQueryEndsWithServiceUnavailableAtTheDeadline locks the events
// table, so the query of the public API waits until its deadline ends it.
func TestSlowQueryEndsWithServiceUnavailableAtTheDeadline(t *testing.T) {
	ctx := context.Background()
	pool := migratedPool(t)
	locker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer func() { _ = locker.Rollback(ctx) }()
	if _, err := locker.Exec(ctx, "LOCK TABLE events IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatalf("lock events: %v", err)
	}
	var logs bytes.Buffer

	assertServiceUnavailableInTime(t, publicAPIWithShortDeadline(pool, &logs), &logs)
}

// TestExhaustedPoolEndsWithServiceUnavailableAtTheDeadline holds every
// connection of the pool, so the public API waits for one until its
// deadline ends it.
func TestExhaustedPoolEndsWithServiceUnavailableAtTheDeadline(t *testing.T) {
	ctx := context.Background()
	pool := migratedPool(t)
	for range pool.Config().MaxConns {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		t.Cleanup(conn.Release)
	}
	var logs bytes.Buffer

	assertServiceUnavailableInTime(t, publicAPIWithShortDeadline(pool, &logs), &logs)
}
