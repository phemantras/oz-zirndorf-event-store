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

// fakeRecomputer returns fixed recompute results.
type fakeRecomputer struct {
	failures []core.RecomputeFailure
	err      error
}

func (f fakeRecomputer) RecomputeDerived(context.Context) ([]core.RecomputeFailure, error) {
	return f.failures, f.err
}

func TestRecomputeDerivedLogsEveryFailedEventWithItsIDAndContinues(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	recomputer := fakeRecomputer{failures: []core.RecomputeFailure{
		{EventID: "0192f0b1-0000-7000-8000-000000000101", Err: errors.New("start time conflicts with all day")},
		{EventID: "0192f0b1-0000-7000-8000-000000000102", Err: errors.New("database hiccup")},
	}}

	if err := recomputeDerived(context.Background(), recomputer, logger); err != nil {
		t.Fatalf("recomputeDerived: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	var failed []string
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
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

func TestRecomputeDerivedFailsWhenEventsCannotBeRead(t *testing.T) {
	errDown := errors.New("database down")

	err := recomputeDerived(context.Background(), fakeRecomputer{err: errDown}, slog.New(slog.DiscardHandler))

	if !errors.Is(err, errDown) {
		t.Errorf("err = %v, want %v", err, errDown)
	}
}
