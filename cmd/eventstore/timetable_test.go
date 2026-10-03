package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
)

// timetableLocationNameKey is the name key of the location the timetable
// test stores; leftovers of an aborted run are removed before inserting.
const timetableLocationNameKey = "ablaufplanprüfung"

// TestRunSavesTimetableThroughTheWiredTransaction posts an event with a
// timetable to the admin of a running service and checks that event and
// entries reached PostgreSQL, so the transaction runner is wired.
func TestRunSavesTimetableThroughTheWiredTransaction(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	pool, locationID := insertTimetableLocation(t, databaseURL)
	running := startRun(t, databaseURL, slog.New(slog.DiscardHandler))

	form := url.Values{
		"title": {"Kirchweih"}, "type": {"festival"}, "locationId": {locationID},
		"startDate": {"2026-10-16"}, "startTime": {"18:00"}, "endDate": {"2026-10-17"},
		"source.description":    {"Amtsblatt"},
		"timetable.description": {"Disco", "Bieranstich"},
		"timetable.date":        {"2026-10-16", "2026-10-16"},
		"timetable.startTime":   {"22:00", "18:00"},
		"timetable.endTime":     {"01:00", ""},
	}
	status := logIn(t, running.baseURL).post(t, adminEventsPath, form)
	running.stop(t)

	if status != http.StatusSeeOther {
		t.Fatalf("POST %s: status = %d, want %d", adminEventsPath, status, http.StatusSeeOther)
	}
	var entries int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM timetable_entries
		 WHERE event_id IN (SELECT id FROM events WHERE location_id = $1)`, locationID).Scan(&entries)
	if err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if entries != 2 {
		t.Errorf("stored %d timetable entries, want 2", entries)
	}
}

// insertTimetableLocation migrates the test database and inserts a
// location, removing it with its events before and afterwards.
func insertTimetableLocation(t *testing.T, databaseURL string) (*pgxpool.Pool, string) {
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
	removeTimetableLocation(t, pool)
	t.Cleanup(func() { removeTimetableLocation(t, pool) })
	var locationID string
	err = pool.QueryRow(ctx,
		`INSERT INTO locations (name, name_key, street, postal_code, city, latitude, longitude, precision)
		 VALUES ('Ablaufplanprüfung', $1, 'Marktplatz', '90513', 'Zirndorf', 49.44, 10.95, 'area')
		 RETURNING id::text`, timetableLocationNameKey).Scan(&locationID)
	if err != nil {
		t.Fatalf("insert location: %v", err)
	}
	return pool, locationID
}

// removeTimetableLocation deletes the location of insertTimetableLocation
// and its events; their entries go with them.
func removeTimetableLocation(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		"DELETE FROM events WHERE location_id IN (SELECT id FROM locations WHERE name_key = $1)", timetableLocationNameKey)
	if err != nil {
		t.Errorf("delete events of the timetable location: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM locations WHERE name_key = $1", timetableLocationNameKey); err != nil {
		t.Errorf("delete timetable location: %v", err)
	}
}

// post sends form to path with the session and returns the status.
func (s adminSession) post(t *testing.T, path string, form url.Values) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(s.cookie)
	resp, err := s.client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}
