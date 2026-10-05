package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// Log messages and keys of the recomputation at startup.
const (
	logMsgRecomputeFailed = "recomputing derived values failed, event keeps its stored values"
	logMsgRecomputed      = "derived values recomputed"
	logKeyEventID         = "eventId"
	logKeyFailed          = "failed"
)

// derivedRecomputer is the core use case that recomputes derived values.
type derivedRecomputer interface {
	RecomputeDerived(ctx context.Context) ([]core.RecomputeFailure, error)
}

// recomputeDerived recomputes the derived values of all events before the
// HTTP server starts (AD-16). An event that fails is logged with its ID and
// does not stop the start (ENT-5); events that cannot be read or stored do.
func recomputeDerived(ctx context.Context, recomputer derivedRecomputer, logger *slog.Logger) error {
	failures, err := recomputer.RecomputeDerived(ctx)
	if err != nil {
		return fmt.Errorf("recompute derived values: %w", err)
	}
	for _, failure := range failures {
		logger.Error(logMsgRecomputeFailed, logKeyEventID, failure.EventID, "error", failure.Err)
	}
	logger.Info(logMsgRecomputed, logKeyFailed, len(failures))
	return nil
}
