package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// adminImportCommitPath commits a checked import file.
const adminImportCommitPath = "/admin/import/commit"

// doubleCommitLocationName is the new location of doubleCommitFile, used by
// no other test.
const doubleCommitLocationName = "Importprobe Doppelklick"

// doubleCommitFile has one event that brings doubleCommitLocationName.
const doubleCommitFile = `{"formatVersion":1,"events":[{
	"title":"Konzert zur Importprobe","type":"culture","startDate":"2026-10-17","importKey":"importprobe-doppelklick",
	"location":{"name":"` + doubleCommitLocationName + `","address":{"street":"Burgweg 1","postalCode":"90513","city":"Zirndorf"},
		"latitude":49.4501,"longitude":10.9376,"precision":"building"},
	"source":{"description":"Plakat"}}]}`

// doubleCommitFields are the fields the preview sends for doubleCommitFile:
// the entry new and bringing a new location.
var doubleCommitFields = [][2]string{
	{"position", "1"}, {"class-1", string(core.ImportClassNew)}, {"target-1", ""}, {"newLocation-1", "true"},
}

// Summary lines of the two commits: one creates the event and its location,
// the other finds both and reports the entry as stale.
const (
	wantCreatedOnce = "neu angelegt: 1"
	wantStaleOnce   = "veraltet: 1"
)

// commitBody returns the commit form for file with fields, and its
// Content-Type.
func commitBody(t *testing.T, file string, fields [][2]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range append([][2]string{{"content", file}}, fields...) {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatalf("write field %s: %v", field[0], err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, writer.FormDataContentType()
}

// commitTwiceAtOnce sends the commit form for file with fields twice at the
// same time, as a double click does, and returns both answers joined.
func commitTwiceAtOnce(t *testing.T, session adminSession, file string, fields [][2]string) string {
	t.Helper()
	const commits = 2
	pages := make([]string, commits)
	errs := make([]error, commits)
	var wg sync.WaitGroup
	for index := range commits {
		body, contentType := commitBody(t, file, fields)
		req, err := http.NewRequest(http.MethodPost, session.baseURL+adminImportCommitPath, body)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.AddCookie(session.cookie)
		wg.Go(func() { pages[index], errs[index] = commitPage(session.client, req) })
	}
	wg.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("commit %d: %v", index+1, err)
		}
	}
	return strings.Join(pages, "\n")
}

// countEventsAt returns how many events the location with nameKey has.
func countEventsAt(t *testing.T, pool *pgxpool.Pool, nameKey string) int {
	t.Helper()
	var events int
	err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM events e JOIN locations l ON l.id = e.location_id WHERE l.name_key = $1", nameKey).Scan(&events)
	if err != nil {
		t.Fatalf("count events: %v", err)
	}
	return events
}

// TestRunCommitsAnImportSentTwiceAtOnceOnlyOnce sends the same commit form
// twice at the same time, as a double click does: the write lock lets the
// second commit wait and classify again, so neither the event nor its
// location is stored twice.
func TestRunCommitsAnImportSentTwiceAtOnceOnlyOnce(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	pool := connectAndMigrate(t, databaseURL)
	nameKey := core.NormalizeKey(doubleCommitLocationName)
	removeLocationWithEvents(t, pool, nameKey)
	t.Cleanup(func() { removeLocationWithEvents(t, pool, nameKey) })
	running := startRun(t, databaseURL, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { running.stop(t) })
	session := logIn(t, running.baseURL)

	joined := commitTwiceAtOnce(t, session, doubleCommitFile, doubleCommitFields)

	if strings.Count(joined, wantCreatedOnce) != 1 || strings.Count(joined, wantStaleOnce) != 1 {
		t.Errorf("commits answered %q, want one creating and one stale", joined)
	}
	var locations int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM locations WHERE name_key = $1", nameKey).Scan(&locations); err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if events := countEventsAt(t, pool, nameKey); events != 1 || locations != 1 {
		t.Errorf("stored %d events at %d locations, want one each", events, locations)
	}
}

// probeLocationName is the location of the stored events of the import
// probes below, used by no other test.
const probeLocationName = "Importprobe Bestand"

// probeStartDate is the day of every probe event, far enough ahead that
// no cleanup archives it.
const probeStartDate = "2099-10-17"

// probeEvent is a stored event of an import probe; an empty start time or
// import key is stored as NULL.
type probeEvent struct {
	title     string
	allDay    bool
	startTime string
	importKey string
}

// insertProbeEvent migrates the test database and stores event at
// probeLocationName on probeStartDate, removing the location with its
// events before and afterwards. It returns the pool and the ID of the
// event.
func insertProbeEvent(t *testing.T, databaseURL string, event probeEvent) (*pgxpool.Pool, string) {
	t.Helper()
	pool := connectAndMigrate(t, databaseURL)
	nameKey := core.NormalizeKey(probeLocationName)
	removeLocationWithEvents(t, pool, nameKey)
	t.Cleanup(func() { removeLocationWithEvents(t, pool, nameKey) })
	ctx := context.Background()
	var locationID, eventID string
	err := pool.QueryRow(ctx,
		`INSERT INTO locations (name, name_key, street, postal_code, city, latitude, longitude, precision)
		 VALUES ($1, $2, 'Marktplatz', '90513', 'Zirndorf', 49.44, 10.95, 'area')
		 RETURNING id::text`, probeLocationName, nameKey).Scan(&locationID)
	if err != nil {
		t.Fatalf("insert location: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO events (title, title_key, type, location_id, start_date, start_time, all_day, source_description,
		                     effective_start, effective_end, import_key)
		 VALUES ($1, $2, 'culture', $3, $4::date, NULLIF($5, '')::time, $6, 'Plakat',
		         $4::date - interval '2 hours', $4::date + interval '22 hours', NULLIF($7, ''))
		 RETURNING id::text`,
		event.title, core.NormalizeKey(event.title), locationID, probeStartDate, event.startTime, event.allDay, event.importKey,
	).Scan(&eventID)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	return pool, eventID
}

// probeFile returns an import file of one event at probeLocationName on
// probeStartDate at 19:00 with title and, unless empty, importKey.
func probeFile(title, importKey string) string {
	key := ""
	if importKey != "" {
		key = `"importKey":"` + importKey + `",`
	}
	return `{"formatVersion":1,"events":[{` + key + `
	"title":"` + title + `","type":"culture","startDate":"` + probeStartDate + `","startTime":"19:00",
	"location":{"name":"` + probeLocationName + `"},"source":{"description":"Plakat"}}]}`
}

// duplicateProbeTitle is the title of the stored event and of the entry of
// the duplicate probe.
const duplicateProbeTitle = "Lesung zur Importprobe"

// TestRunCommitsADuplicateChosenAsNewOnlyOnce sends the commit form of a
// duplicate suspect decided as new twice at once: the second commit finds
// the event the first created as a candidate the preview did not show and
// reports the entry as stale.
func TestRunCommitsADuplicateChosenAsNewOnlyOnce(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	pool, storedID := insertProbeEvent(t, databaseURL, probeEvent{title: duplicateProbeTitle, allDay: true})
	running := startRun(t, databaseURL, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { running.stop(t) })
	session := logIn(t, running.baseURL)
	fields := [][2]string{
		{"position", "1"}, {"class-1", string(core.ImportClassDuplicateSuspect)}, {"target-1", ""},
		{"candidates-1", storedID}, {"choice-1", string(core.ImportChoiceCreate)},
	}

	joined := commitTwiceAtOnce(t, session, probeFile(duplicateProbeTitle, ""), fields)

	if strings.Count(joined, wantCreatedOnce) != 1 || strings.Count(joined, wantStaleOnce) != 1 {
		t.Errorf("commits answered %q, want one creating and one stale", joined)
	}
	if events := countEventsAt(t, pool, core.NormalizeKey(probeLocationName)); events != 2 {
		t.Errorf("stored %d events at the location, want the stored one and one created", events)
	}
}

// Import key and title of the repair probe. The stored event is all-day
// with a start time, which the core rejects when it recomputes at the
// start.
const (
	repairProbeKey   = "importprobe-reparatur"
	repairProbeTitle = "Konzert zur Reparaturprobe"
)

// repairProbePublicPath lists the public events from the day of the probe.
const repairProbePublicPath = "/v1/events?from=" + probeStartDate

// wantUpdatedOnce is the summary line of a commit that updated one event.
const wantUpdatedOnce = "aktualisiert: 1"

// TestRunCommitThatRepairsAMarkedEventClearsTheMark stores an event the
// start marks for review. An import that updates it by its import key
// clears the mark in the admin, which shares the event use cases with the
// import, and the public API serves the event again.
func TestRunCommitThatRepairsAMarkedEventClearsTheMark(t *testing.T) {
	databaseURL := testDatabaseURL(t)
	_, brokenID := insertProbeEvent(t, databaseURL, probeEvent{
		title: repairProbeTitle, allDay: true, startTime: "19:00", importKey: repairProbeKey,
	})
	running := startRun(t, databaseURL, slog.New(slog.DiscardHandler))
	t.Cleanup(func() { running.stop(t) })
	session := logIn(t, running.baseURL)
	link := `<a href="` + adminEventsPath + "/" + brokenID + `">` + repairProbeTitle + `</a>`
	if _, list := session.get(t, adminEventsPath); !strings.Contains(tableRowWith(list, link), reviewMark) {
		t.Fatalf("the broken event is not marked %s after the start; body: %s", reviewMark, list)
	}
	fields := [][2]string{{"position", "1"}, {"class-1", string(core.ImportClassUpdate)}, {"target-1", brokenID}}
	body, contentType := commitBody(t, probeFile(repairProbeTitle, repairProbeKey), fields)

	status, page := session.upload(t, adminImportCommitPath, contentType, body)

	if status != http.StatusOK || !strings.Contains(page, wantUpdatedOnce) {
		t.Fatalf("commit: status = %d, page %s, want the event updated", status, page)
	}
	if _, list := session.get(t, adminEventsPath); strings.Contains(tableRowWith(list, link), reviewMark) {
		t.Errorf("the repaired event is still marked %s; body: %s", reviewMark, list)
	}
	if list := publicBody(t, running.baseURL+repairProbePublicPath); !strings.Contains(list, repairProbeTitle) {
		t.Errorf("GET %s does not serve the repaired event: %s", repairProbePublicPath, list)
	}
}

// reviewMark is how the admin lists mark an entry for review.
const reviewMark = "prüfen"

// commitPage sends req and returns the body of a 200 answer.
func commitPage(client *http.Client, req *http.Request) (string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", &unexpectedStatusError{status: resp.StatusCode, body: string(body)}
	}
	return string(body), nil
}

// unexpectedStatusError reports an answer other than 200.
type unexpectedStatusError struct {
	status int
	body   string
}

func (e *unexpectedStatusError) Error() string {
	return http.StatusText(e.status) + ": " + e.body
}
