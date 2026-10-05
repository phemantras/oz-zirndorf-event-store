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
