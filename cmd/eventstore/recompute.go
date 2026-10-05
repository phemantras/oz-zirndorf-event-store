package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// Log messages and keys of the recomputation at startup.
const (
	logMsgRecomputeFailed         = "recomputing derived values failed, event keeps its stored values"
	logMsgLocationRecomputeFailed = "recomputing name keys failed, colliding locations keep their stored name keys"
	logMsgRecomputed              = "derived values recomputed"
	logKeyEventID                 = "eventId"
	logKeyLocationIDs             = "locationIds"
	logKeyFailedEvents            = "failedEvents"
	logKeyFailedLocationGroups    = "failedLocationGroups"
)

// nameKeyRecomputer is the core use case that recomputes the name keys of
// locations.
type nameKeyRecomputer interface {
	RecomputeNameKeys(ctx context.Context) ([]core.LocationRecomputeFailure, error)
}

// derivedRecomputer is the core use case that recomputes the derived values
// of events.
type derivedRecomputer interface {
	RecomputeDerived(ctx context.Context) ([]core.RecomputeFailure, error)
}

// recomputeDerived recomputes the name keys of all locations and then the
// derived values of all events before the HTTP server starts (AD-16). A
// group of colliding locations is logged with all its IDs, an event that
// fails with its ID; neither stops the start (ENT-5). Locations or events
// that cannot be read or stored do.
func recomputeDerived(ctx context.Context, locations nameKeyRecomputer, events derivedRecomputer, logger *slog.Logger) error {
	locationFailures, err := locations.RecomputeNameKeys(ctx)
	if err != nil {
		return fmt.Errorf("recompute name keys: %w", err)
	}
	for _, failure := range locationFailures {
		logger.Error(logMsgLocationRecomputeFailed, logKeyLocationIDs, failure.LocationIDs, "error", failure.Err)
	}

	eventFailures, err := events.RecomputeDerived(ctx)
	if err != nil {
		return fmt.Errorf("recompute derived values: %w", err)
	}
	for _, failure := range eventFailures {
		logger.Error(logMsgRecomputeFailed, logKeyEventID, failure.EventID, "error", failure.Err)
	}
	logger.Info(logMsgRecomputed, logKeyFailedEvents, len(eventFailures), logKeyFailedLocationGroups, len(locationFailures))
	return nil
}
