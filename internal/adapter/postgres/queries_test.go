package postgres_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// queryNamePrefix starts the line that names a sqlc query.
const queryNamePrefix = "-- name: "

// archiveMarkColumn is the column only the cleanup sets and only saving
// and recomputing clear; no read query may use it (AD-16).
const archiveMarkColumn = "archived_at"

// TestOnlyMarkingAndUpdatingUseTheArchiveMark reads the queries without a
// database: lists, the API and the admin decide "archived" by the effective
// period, never by archived_at.
func TestOnlyMarkingAndUpdatingUseTheArchiveMark(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("queries", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("glob queries: %v, %d files", err, len(files))
	}
	var using []string
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, query := range strings.Split(string(content), queryNamePrefix)[1:] {
			if strings.Contains(query, archiveMarkColumn) {
				name, _, _ := strings.Cut(query, " ")
				using = append(using, name)
			}
		}
	}
	slices.Sort(using)
	if want := []string{"MarkEventsArchived", "UpdateEvent", "UpdateEventDerived"}; !slices.Equal(using, want) {
		t.Errorf("queries using %s = %v, want %v", archiveMarkColumn, using, want)
	}
}

// importKeyColumn is the column only the import commit sets (Story 3.3).
const importKeyColumn = "import_key"

// adminEventWrites are the queries behind creating and updating an event
// in the admin.
var adminEventWrites = []string{"CreateEvent", "UpdateEvent"}

// returningClause starts the part of a writing query that only reads.
const returningClause = "RETURNING"

// TestAdminEventWritesLeaveTheImportKeyAlone reads the queries without a
// database: CreateEvent and UpdateEvent may return import_key but never
// write it, so saving in the admin keeps it.
func TestAdminEventWritesLeaveTheImportKeyAlone(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("queries", "events.sql"))
	if err != nil {
		t.Fatalf("read event queries: %v", err)
	}
	found := map[string]bool{}
	for _, query := range strings.Split(string(content), queryNamePrefix)[1:] {
		name, _, _ := strings.Cut(query, " ")
		if !slices.Contains(adminEventWrites, name) {
			continue
		}
		found[name] = true
		writing, reading, _ := strings.Cut(query, returningClause)
		if strings.Contains(writing, importKeyColumn) {
			t.Errorf("query %s writes %s", name, importKeyColumn)
		}
		if !strings.Contains(reading, importKeyColumn) {
			t.Errorf("query %s does not return %s", name, importKeyColumn)
		}
	}
	for _, name := range adminEventWrites {
		if !found[name] {
			t.Errorf("query %s not found", name)
		}
	}
}
