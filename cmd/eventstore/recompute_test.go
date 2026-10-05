package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// fakeRecomputer returns fixed recompute results and records the order in
// which the use cases ran.
type fakeRecomputer struct {
	failures         []core.RecomputeFailure
	err              error
	locationFailures []core.LocationRecomputeFailure
	locationErr      error
	calls            *[]string
}

func (f fakeRecomputer) RecomputeDerived(context.Context) ([]core.RecomputeFailure, error) {
	f.record("events")
	return f.failures, f.err
}

func (f fakeRecomputer) RecomputeNameKeys(context.Context) ([]core.LocationRecomputeFailure, error) {
	f.record("locations")
	return f.locationFailures, f.locationErr
}

func (f fakeRecomputer) record(call string) {
	if f.calls != nil {
		*f.calls = append(*f.calls, call)
	}
}

// logEntries parses every JSON log line.
func logEntries(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestRecomputeDerivedLogsEveryFailedEventWithItsIDAndContinues(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	recomputer := fakeRecomputer{failures: []core.RecomputeFailure{
		{EventID: "0192f0b1-0000-7000-8000-000000000101", Err: errors.New("start time conflicts with all day")},
		{EventID: "0192f0b1-0000-7000-8000-000000000102", Err: errors.New("database hiccup")},
	}}

	if err := recomputeDerived(context.Background(), recomputer, recomputer, logger); err != nil {
		t.Fatalf("recomputeDerived: %v", err)
	}

	var failed []string
	for _, entry := range logEntries(t, &logs) {
		if entry["msg"] == logMsgRecomputeFailed {
			failed = append(failed, entry[logKeyEventID].(string)+" "+entry["error"].(string))
		}
	}
	want := []string{
		"0192f0b1-0000-7000-8000-000000000101 start time conflicts with all day",
		"0192f0b1-0000-7000-8000-000000000102 database hiccup",
	}
	if strings.Join(failed, "|") != strings.Join(want, "|") {
		t.Errorf("logged failures = %v, want %v", failed, want)
	}
	if !strings.Contains(logs.String(), logMsgRecomputed) {
		t.Errorf("log %q lacks the summary", logs.String())
	}
}

func TestRecomputeDerivedLogsEveryCollidingLocationGroupAndContinues(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	var calls []string
	recomputer := fakeRecomputer{
		locationFailures: []core.LocationRecomputeFailure{
			{LocationIDs: []string{"id-a", "id-b", "id-c"}, Err: errors.New(`collide on name key "festzelt"`)},
			{LocationIDs: []string{"id-d", "id-e"}, Err: errors.New(`collide on name key "markt"`)},
		},
		failures: []core.RecomputeFailure{{EventID: "id-event", Err: errors.New("timetable entry outside")}},
		calls:    &calls,
	}

	if err := recomputeDerived(context.Background(), recomputer, recomputer, logger); err != nil {
		t.Fatalf("recomputeDerived: %v", err)
	}

	if strings.Join(calls, ",") != "locations,events" {
		t.Errorf("calls = %v, want locations before events", calls)
	}
	var groups []string
	var summary map[string]any
	for _, entry := range logEntries(t, &logs) {
		switch entry["msg"] {
		case logMsgLocationRecomputeFailed:
			var ids []string
			for _, id := range entry[logKeyLocationIDs].([]any) {
				ids = append(ids, id.(string))
			}
			groups = append(groups, strings.Join(ids, ",")+" "+entry["error"].(string))
		case logMsgRecomputed:
			summary = entry
		}
	}
	want := []string{`id-a,id-b,id-c collide on name key "festzelt"`, `id-d,id-e collide on name key "markt"`}
	if strings.Join(groups, "|") != strings.Join(want, "|") {
		t.Errorf("logged groups = %v, want %v", groups, want)
	}
	if summary[logKeyFailedEvents] != 1.0 || summary[logKeyFailedLocationGroups] != 2.0 {
		t.Errorf("summary = %v, want 1 failed event and 2 failed location groups", summary)
	}
}

func TestRecomputeDerivedFailsWhenEventsCannotBeRead(t *testing.T) {
	errDown := errors.New("database down")
	recomputer := fakeRecomputer{err: errDown}

	err := recomputeDerived(context.Background(), recomputer, recomputer, slog.New(slog.DiscardHandler))

	if !errors.Is(err, errDown) {
		t.Errorf("err = %v, want %v", err, errDown)
	}
}

func TestRecomputeDerivedFailsBeforeTheEventsWhenLocationsCannotBeRead(t *testing.T) {
	errDown := errors.New("database down")
	var calls []string
	recomputer := fakeRecomputer{locationErr: errDown, calls: &calls}

	err := recomputeDerived(context.Background(), recomputer, recomputer, slog.New(slog.DiscardHandler))

	if !errors.Is(err, errDown) {
		t.Errorf("err = %v, want %v", err, errDown)
	}
	if strings.Join(calls, ",") != "locations" {
		t.Errorf("calls = %v, want only locations", calls)
	}
}
