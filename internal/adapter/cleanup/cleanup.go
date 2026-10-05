package cleanup

import (
	"context"
	"log/slog"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// DailyInterval is the time between two runs after the one at startup.
const DailyInterval = 24 * time.Hour

// Log messages and keys of the cleanup.
const (
	logMsgArchived      = "past events marked as archived"
	logMsgArchiveFailed = "marking past events as archived failed, the next run retries"
	logKeyMarked        = "marked"
	logKeyError         = "error"
)

// Archiver is the core use case that marks past events as archived.
type Archiver interface {
	MarkArchived(ctx context.Context, clock core.Clock) (int, error)
}

// Job marks past events as archived through Archiver at the time of Clock
// and logs the outcome to Logger.
type Job struct {
	Archiver Archiver
	Clock    core.Clock
	Logger   *slog.Logger
}

// RunOnce marks past events once and logs how many it marked. A failure is
// only logged, so the program keeps running and the next run retries; a
// failure because ctx ended, as at shutdown, is not logged.
func (j Job) RunOnce(ctx context.Context) {
	marked, err := j.Archiver.MarkArchived(ctx, j.Clock)
	if err != nil {
		if ctx.Err() == nil {
			j.Logger.Error(logMsgArchiveFailed, logKeyError, err)
		}
		return
	}
	j.Logger.Info(logMsgArchived, logKeyMarked, marked)
}

// RunDaily runs RunOnce every interval, the first time one interval from
// now, until ctx ends.
func (j Job) RunDaily(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.RunOnce(ctx)
		}
	}
}
