package v1

import (
	"context"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// server answers the operations of the spec from the core queries; it only
// maps core values to the generated types.
type server struct {
	events EventLister
	clock  core.Clock
}

// ListEventTypes returns every event type with its German label.
func (server) ListEventTypes(context.Context, ListEventTypesRequestObject) (ListEventTypesResponseObject, error) {
	var entries []EventTypeEntry
	for _, eventType := range core.ListEventTypes() {
		entries = append(entries, EventTypeEntry{Code: EventType(eventType.Code), Label: eventType.Label})
	}
	return ListEventTypes200JSONResponse{Data: entries}, nil
}
