package cleanup_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/cleanup"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// shortInterval lets RunDaily tick several times within a test.
const shortInterval = time.Millisecond

// waitTimeout bounds how long a test waits for the job.
const waitTimeout = 5 * time.Second

var errDatabaseDown = errors.New("database down")

// fixedClock is a core.Clock that always shows the same instant.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// christmasNoon is 2026-12-24 12:00 Europe/Berlin.
var christmasNoon = fixedClock(time.Date(2026, time.December, 24, 11, 0, 0, 0, time.UTC))

// fakeArchiver records the clocks it was called with and answers with
// marked and err; every call is also sent on calls when it is set.
type fakeArchiver struct {
	marked int
	err    error
	// cancel, when set, ends the job's context during the call, like a
	// shutdown in the middle of a run.
	cancel context.CancelFunc

	mu     sync.Mutex
	clocks []core.Clock
	calls  chan struct{}
}

func (a *fakeArchiver) MarkArchived(ctx context.Context, clock core.Clock) (int, error) {
	a.mu.Lock()
	a.clocks = append(a.clocks, clock)
	a.mu.Unlock()
	if a.calls != nil {
		select {
		case a.calls <- struct{}{}:
		default:
		}
	}
	if a.cancel != nil {
		a.cancel()
		return 0, ctx.Err()
	}
	return a.marked, a.err
}

func (a *fakeArchiver) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.clocks)
}

// syncBuffer is a bytes.Buffer that the job goroutine may write while the
// test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// logEntries parses the JSON lines of logs.
func logEntries(t *testing.T, logs *syncBuffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func newJob(archiver *fakeArchiver, logs *syncBuffer) cleanup.Job {
	return cleanup.Job{Archiver: archiver, Clock: christmasNoon, Logger: slog.New(slog.NewJSONHandler(logs, nil))}
}

func TestRunOnceLogsHowManyEventsWereMarked(t *testing.T) {
	archiver, logs := &fakeArchiver{marked: 2}, &syncBuffer{}

	newJob(archiver, logs).RunOnce(context.Background())

	entries := logEntries(t, logs)
	if len(entries) != 1 || entries[0]["level"] != "INFO" || entries[0]["marked"] != float64(2) {
		t.Errorf("log = %v, want one info entry with marked=2", entries)
	}
	if archiver.callCount() != 1 || archiver.clocks[0] != core.Clock(christmasNoon) {
		t.Errorf("archiver called with %v, want once with the job's clock", archiver.clocks)
	}
}

func TestRunOnceLogsAFailureAndReturns(t *testing.T) {
	archiver, logs := &fakeArchiver{err: errDatabaseDown}, &syncBuffer{}

	newJob(archiver, logs).RunOnce(context.Background())

	entries := logEntries(t, logs)
	if len(entries) != 1 || entries[0]["level"] != "ERROR" || entries[0]["error"] != errDatabaseDown.Error() {
		t.Errorf("log = %v, want one error entry naming %v", entries, errDatabaseDown)
	}
}

func TestRunOnceDoesNotLogAFailureCausedByShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	archiver, logs := &fakeArchiver{cancel: cancel}, &syncBuffer{}

	newJob(archiver, logs).RunOnce(ctx)

	if entries := logEntries(t, logs); len(entries) != 0 {
		t.Errorf("log = %v, want nothing after shutdown", entries)
	}
}

func TestRunDailyRunsRepeatedlyEvenAfterFailuresUntilTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	archiver := &fakeArchiver{err: errDatabaseDown, calls: make(chan struct{}, 1)}
	done := make(chan struct{})
	go func() {
		newJob(archiver, &syncBuffer{}).RunDaily(ctx, shortInterval)
		close(done)
	}()

	const wantRuns = 3
	for range wantRuns {
		select {
		case <-archiver.calls:
		case <-time.After(waitTimeout):
			t.Fatalf("job ran %d times, want %d", archiver.callCount(), wantRuns)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(waitTimeout):
		t.Fatal("RunDaily did not end with its context")
	}
}

func TestRunDailyDoesNotRunBeforeTheFirstInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	archiver, logs := &fakeArchiver{}, &syncBuffer{}

	newJob(archiver, logs).RunDaily(ctx, time.Hour)

	if archiver.callCount() != 0 || logs.String() != "" {
		t.Errorf("job ran %d times and logged %q, want neither before the first interval", archiver.callCount(), logs.String())
	}
}

func TestDailyIntervalIs24Hours(t *testing.T) {
	if cleanup.DailyInterval != 24*time.Hour {
		t.Errorf("DailyInterval = %v, want 24h", cleanup.DailyInterval)
	}
}
