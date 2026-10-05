package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/cleanup"
	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	publicapi "github.com/phemantras/oz-zirndorf-event-store/internal/adapter/publicapi/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// fixedClock is a core.Clock that always shows the same instant.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// publicListPaths are the public lists whose answers the cleanup must not
// change.
var publicListPaths = []string{publicAllEventsPath, publicArchivePath, publicEventTypesPath}

// publicAllEventsPath lists every active event, whatever its start.
const publicAllEventsPath = "/v1/events?from=1900-01-01"

// publicArchivePath lists every past event.
const publicArchivePath = "/v1/archive/events"

// TestCleanupLeavesThePublicAnswersUnchanged stores a past, a running and a
// future event and compares the public lists before and after marking
// past events archived at the same fixed clock: they must be byte-equal.
func TestCleanupLeavesThePublicAnswersUnchanged(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	pool, locationID := insertCleanupLocation(t, databaseURL)
	ctx := context.Background()
	locations := postgres.NewLocationRepo(pool)
	events := core.NewEventService(postgres.NewTxRunner(pool), postgres.NewEventRepo(pool), locations)
	var pastID string
	for _, in := range []core.EventInput{
		{Title: "Adventsbasar", StartDate: "2026-12-20"},
		{Title: "Krippenspiel", StartDate: "2026-12-24", StartTime: "10:00", EndDate: "2026-12-24", EndTime: "14:00"},
		{Title: "Neujahrskonzert", StartDate: "2027-01-10", StartTime: "17:00"},
	} {
		in.Type, in.LocationID, in.Source = string(core.EventTypeOther), locationID, core.EventSource{Description: "Test"}
		saved, err := events.SaveEvent(ctx, "", in, core.AllowDuplicates)
		if err != nil {
			t.Fatalf("SaveEvent %q: %v", in.Title, err)
		}
		if pastID == "" {
			pastID = saved.ID
		}
	}
	clock := fixedClock(time.Date(2026, time.December, 24, 11, 0, 0, 0, time.UTC)) // 12:00 Europe/Berlin
	public := publicapi.NewHandler(publicapi.Config{Logger: slog.New(slog.DiscardHandler), Events: events, Clock: clock})

	before := publicAnswers(t, public)
	if !strings.Contains(before[publicArchivePath], "Adventsbasar") || !strings.Contains(before[publicAllEventsPath], "Neujahrskonzert") {
		t.Fatalf("lists before the cleanup lack the stored events: %v", before)
	}
	marked, err := events.MarkArchived(ctx, clock)
	if err != nil || marked < 1 {
		t.Fatalf("MarkArchived = %d, %v, want at least the past event marked", marked, err)
	}
	var pastMarked bool
	if err := pool.QueryRow(ctx, "SELECT archived_at IS NOT NULL FROM events WHERE id = $1", pastID).Scan(&pastMarked); err != nil || !pastMarked {
		t.Fatalf("past fixture event marked = %v, %v, want marked", pastMarked, err)
	}
	after := publicAnswers(t, public)

	for _, path := range publicListPaths {
		if before[path] != after[path] {
			t.Errorf("GET %s changed by the cleanup:\nbefore %s\nafter  %s", path, before[path], after[path])
		}
	}
}

// publicAnswers returns the body of every public list path, failing on a
// status other than 200.
func publicAnswers(t *testing.T, public http.Handler) map[string]string {
	t.Helper()
	answers := make(map[string]string, len(publicListPaths))
	for _, path := range publicListPaths {
		recorder := httptest.NewRecorder()
		public.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		body, err := io.ReadAll(recorder.Result().Body)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, body %s", path, recorder.Code, body)
		}
		answers[path] = string(body)
	}
	return answers
}

// failingArchiver fails every run and counts the runs.
type failingArchiver struct{ runs atomic.Int32 }

func (a *failingArchiver) MarkArchived(context.Context, core.Clock) (int, error) {
	a.runs.Add(1)
	return 0, errors.New("database down")
}

// TestStartCleanupRunsOnceThenRepeatsDespiteFailures checks without a
// database that a failing first run does not stop the start and that the
// job keeps running every interval until the context ends.
func TestStartCleanupRunsOnceThenRepeatsDespiteFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	archiver := &failingArchiver{}
	job := cleanup.Job{Archiver: archiver, Clock: fixedClock(time.Now()), Logger: slog.New(slog.DiscardHandler)}

	startCleanup(ctx, job, time.Millisecond)

	if runs := archiver.runs.Load(); runs < 1 {
		t.Fatalf("runs after startCleanup = %d, want the first run done", runs)
	}
	deadline := time.Now().Add(shutdownTimeout)
	for archiver.runs.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("runs = %d, want more than one before the context ends", archiver.runs.Load())
		}
		time.Sleep(time.Millisecond)
	}
}
