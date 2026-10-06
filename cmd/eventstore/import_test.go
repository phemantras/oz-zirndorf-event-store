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

// Summary lines of the two commits: one creates the event and its location,
// the other finds both and reports the entry as stale.
const (
	wantCreatedOnce = "neu angelegt: 1"
	wantStaleOnce   = "veraltet: 1"
)

// commitBody returns the commit form for doubleCommitFile as the preview
// sends it: the entry new and bringing a new location.
func commitBody(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := [][2]string{
		{"content", doubleCommitFile}, {"position", "1"},
		{"class-1", string(core.ImportClassNew)}, {"target-1", ""}, {"newLocation-1", "true"},
	}
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			t.Fatalf("write field %s: %v", field[0], err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, writer.FormDataContentType()
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

	const commits = 2
	pages := make([]string, commits)
	errs := make([]error, commits)
	var wg sync.WaitGroup
	for index := range commits {
		body, contentType := commitBody(t)
		req, err := http.NewRequest(http.MethodPost, running.baseURL+adminImportCommitPath, body)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.AddCookie(session.cookie)
		wg.Go(func() { pages[index], errs[index] = commitPage(session.client, req) })
	}
	wg.Wait()

	joined := strings.Join(pages, "\n")
	for index, err := range errs {
		if err != nil {
			t.Fatalf("commit %d: %v", index+1, err)
		}
	}
	if strings.Count(joined, wantCreatedOnce) != 1 || strings.Count(joined, wantStaleOnce) != 1 {
		t.Errorf("commits answered %q, want one creating and one stale", joined)
	}
	var events, locations int
	ctx := context.Background()
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events e JOIN locations l ON l.id = e.location_id WHERE l.name_key = $1", nameKey).Scan(&events); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM locations WHERE name_key = $1", nameKey).Scan(&locations); err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if events != 1 || locations != 1 {
		t.Errorf("stored %d events at %d locations, want one each", events, locations)
	}
}

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
