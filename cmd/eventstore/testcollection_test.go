package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	publicapi "github.com/phemantras/oz-zirndorf-event-store/internal/adapter/publicapi/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// The test collection of Story 3.4: 39 events at 19 locations. At
// testCollectionNow the 15 events up to 2026-09-30 are over, the other 24
// are still to come.
const (
	testCollectionPath           = "../../testdata/zirndorf_events.v1.json"
	testCollectionEvents         = 39
	testCollectionLocations      = 19
	testCollectionArchivedEvents = 15
)

// testCollectionFirstActiveDay is the day of testCollectionNow: archived
// events start before it, active ones on or after it.
const testCollectionFirstActiveDay = "2026-10-01"

// testCollectionNow is the fixed clock of the import: 2026-10-01 12:00
// Europe/Berlin.
var testCollectionNow = fixedClock(time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC))

// TestTestCollectionImportsCompletelyAndRepeatably is the acceptance test
// of SM-2: into an empty store the test collection imports completely, its
// past events appear in the archive at once, and a second import of the
// same file changes nothing.
func TestTestCollectionImportsCompletelyAndRepeatably(t *testing.T) {
	pool := connectAndMigrate(t, testDatabaseURL(t))
	emptyStore(t, pool)
	t.Cleanup(func() { emptyStore(t, pool) })
	data, err := os.ReadFile(testCollectionPath)
	if err != nil {
		t.Fatalf("read test collection: %v", err)
	}
	locations := postgres.NewLocationRepo(pool)
	events := core.NewEventService(postgres.NewTxRunner(pool), postgres.NewEventRepo(pool), locations)
	imports := core.NewImportService(events)

	first := previewAndCommit(t, imports, data, core.ImportClassNew)

	if got := first.CountOf(core.ImportOutcomeCreated); got != testCollectionEvents {
		t.Errorf("first import created %d events, want %d; results %+v", got, testCollectionEvents, first.Results)
	}
	if first.CreatedLocations != testCollectionLocations {
		t.Errorf("first import created %d locations, want %d", first.CreatedLocations, testCollectionLocations)
	}
	public := publicapi.NewHandler(publicapi.Config{Logger: slog.New(slog.DiscardHandler), Events: events, Clock: testCollectionNow})
	archived := publicStartDates(t, public, publicArchivePath)
	active := publicStartDates(t, public, publicAllEventsPath)
	if len(archived) != testCollectionArchivedEvents || len(active) != testCollectionEvents-testCollectionArchivedEvents {
		t.Errorf("GET %s lists %d events, GET %s %d, want %d and %d",
			publicArchivePath, len(archived), publicAllEventsPath, len(active),
			testCollectionArchivedEvents, testCollectionEvents-testCollectionArchivedEvents)
	}
	for _, startDate := range archived {
		if startDate >= testCollectionFirstActiveDay {
			t.Errorf("GET %s lists an event starting %s, want only events before %s", publicArchivePath, startDate, testCollectionFirstActiveDay)
		}
	}

	second := previewAndCommit(t, imports, data, core.ImportClassUnchanged)

	if got := second.CountOf(core.ImportOutcomeUnchanged); got != testCollectionEvents || second.CreatedLocations != 0 {
		t.Errorf("second import: %d unchanged, %d locations created, want %d and 0; results %+v",
			got, second.CreatedLocations, testCollectionEvents, second.Results)
	}
	var storedEvents, storedLocations int
	err = pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM events), (SELECT count(*) FROM locations)").
		Scan(&storedEvents, &storedLocations)
	if err != nil {
		t.Fatalf("count events and locations: %v", err)
	}
	if storedEvents != testCollectionEvents || storedLocations != testCollectionLocations {
		t.Errorf("store holds %d events at %d locations after the second import, want %d at %d",
			storedEvents, storedLocations, testCollectionEvents, testCollectionLocations)
	}
}

// emptyStore deletes every event with its timetable and every location.
// Unlike the other tests of the package, which remove only their own
// rows, the test collection needs an empty store: the import classifies
// against every stored event and location.
func emptyStore(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE timetable_entries, events, locations"); err != nil {
		t.Fatalf("truncate timetable entries, events and locations: %v", err)
	}
}

// previewAndCommit previews data, expecting every entry to be of
// wantClass, and commits it with the decisions the import form sends for
// that preview, as a person who decides nothing does.
func previewAndCommit(t *testing.T, imports *core.ImportService, data []byte, wantClass core.ImportClass) core.ImportSummary {
	t.Helper()
	ctx := context.Background()
	preview, err := imports.PreviewImport(ctx, data)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	decisions := make([]core.ImportDecision, 0, len(preview.Entries))
	for _, entry := range preview.Entries {
		if entry.Class != wantClass {
			t.Errorf("preview: entry %d %q is %s with problems %+v, want %s", entry.Position, entry.Title, entry.Class, entry.Problems, wantClass)
		}
		decisions = append(decisions, core.ImportDecision{
			Position: entry.Position, Class: entry.Class, TargetID: entry.TargetID,
			NewLocation: entry.NewLocation, CandidateIDs: entry.StoredCandidateIDs(),
		})
	}
	summary, err := imports.CommitImport(ctx, data, decisions)
	if err != nil {
		t.Fatalf("CommitImport: %v", err)
	}
	return summary
}

// publicStartDates returns the start dates of the events that GET path
// lists.
func publicStartDates(t *testing.T, public http.Handler, path string) []string {
	t.Helper()
	recorder := httptest.NewRecorder()
	public.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, body %s", path, recorder.Code, recorder.Body)
	}
	var list struct {
		Data []struct {
			StartDate string `json:"startDate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode GET %s: %v", path, err)
	}
	startDates := make([]string, 0, len(list.Data))
	for _, event := range list.Data {
		startDates = append(startDates, event.StartDate)
	}
	return startDates
}
