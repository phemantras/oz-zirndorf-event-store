package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// EventRepo is the storage port for events. Only EventService calls
// Create, Update and UpdatePeriod; adapters never get the repository to
// write past the core (AD-6). Events come with their timetable in no
// particular order; the core sorts it.
type EventRepo interface {
	// List returns all events in no particular order.
	List(ctx context.Context) ([]Event, error)
	// Get returns the event with id, or ErrNotFound, also for an id that is
	// not a valid UUID.
	Get(ctx context.Context, id string) (Event, error)
	// Create stores a new event with its timetable and returns it with the
	// generated IDs.
	Create(ctx context.Context, event Event) (Event, error)
	// Update replaces the event with event.ID and its whole timetable, or
	// yields ErrNotFound.
	Update(ctx context.Context, event Event) (Event, error)
	// UpdatePeriod replaces only the stored effective period of the event
	// with id, or yields ErrNotFound.
	UpdatePeriod(ctx context.Context, id string, period Period) error
}

// Repos are the repositories of one transaction.
type Repos struct {
	Events    EventRepo
	Locations LocationRepo
}

// TxRunner is the transaction port. A use case that writes more than one
// row runs its transaction-bound part through it (AD-6, AD-15).
type TxRunner interface {
	// InTx runs fn in one transaction; an error from fn rolls back.
	InTx(ctx context.Context, fn func(Repos) error) error
}

// EventListEntry is one event of the admin list with what the list shows
// beside it.
type EventListEntry struct {
	Event    Event
	Location Location
	// Archived is Event.Period.IsOver at the time of the list (AD-16).
	Archived bool
	// NeedsReview marks an event whose derived values could not be
	// recomputed at startup; it stays set until the event is saved.
	NeedsReview bool
}

// RecomputeFailure names an event whose derived values could not be
// recomputed and why. The event keeps its stored values.
type RecomputeFailure struct {
	EventID string
	Err     error
}

// EventService holds the event use cases. It keeps the IDs of events that
// need review in memory only; they are recomputed at every start.
type EventService struct {
	tx        TxRunner
	events    EventRepo
	locations LocationRepo

	mu       sync.Mutex
	inReview map[string]struct{}
}

// NewEventService returns the event use cases: saving runs in transactions
// of tx, reading and recomputing use events and locations directly.
func NewEventService(tx TxRunner, events EventRepo, locations LocationRepo) *EventService {
	return &EventService{tx: tx, events: events, locations: locations, inReview: map[string]struct{}{}}
}

// SaveEvent creates an event when id is empty and otherwise updates the
// event with id, keeping its ID. Event and timetable are written in one
// transaction, the timetable replacing the stored one (AD-15). It returns
// ErrNotFound for an unknown id and *ValidationError listing every invalid
// field, including an unknown location. A successful save clears the
// review mark of the event.
func (s *EventService) SaveEvent(ctx context.Context, id string, in EventInput) (Event, error) {
	var saved Event
	err := s.tx.InTx(ctx, func(repos Repos) error {
		var err error
		saved, err = saveEvent(ctx, repos, id, in)
		return err
	})
	if err != nil {
		return Event{}, err
	}
	s.clearReview(saved.ID)
	return saved, nil
}

// saveEvent is the transaction-bound part of SaveEvent.
func saveEvent(ctx context.Context, repos Repos, id string, in EventInput) (Event, error) {
	storedID, err := storedEventID(ctx, repos.Events, id)
	if err != nil {
		return Event{}, err
	}
	event, problems := newEvent(in)
	locationID, locationProblems, err := storedLocationID(ctx, repos.Locations, event.LocationID)
	if err != nil {
		return Event{}, err
	}
	problems = append(problems, locationProblems...)
	if problems != nil {
		return Event{}, &ValidationError{Fields: problems}
	}

	event.ID, event.LocationID = storedID, locationID
	saved, err := writeEvent(ctx, repos.Events, event)
	if err != nil {
		return Event{}, fmt.Errorf("save event: %w", err)
	}
	saved.Timetable = sortedTimetable(saved.Timetable)
	return saved, nil
}

// storedEventID returns the ID of the event to update as the repository
// spells it. An empty id stays empty: a new event is created.
func storedEventID(ctx context.Context, events EventRepo, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	current, err := events.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get event to update: %w", err)
	}
	return current.ID, nil
}

// storedLocationID returns the location ID as the repository spells it,
// or reports locationId notFound when no such location exists. A missing
// location ID is already reported by newEvent.
func storedLocationID(ctx context.Context, locations LocationRepo, id string) (string, []FieldError, error) {
	if id == "" {
		return "", nil, nil
	}
	location, err := locations.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return "", []FieldError{{Field: EventFieldLocationID, Problem: ProblemNotFound}}, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("check event location: %w", err)
	}
	return location.ID, nil, nil
}

// writeEvent creates event when it has no ID and updates it otherwise.
func writeEvent(ctx context.Context, events EventRepo, event Event) (Event, error) {
	if event.ID == "" {
		return events.Create(ctx, event)
	}
	return events.Update(ctx, event)
}

// GetEvent returns the event with id and its timetable sorted, or
// ErrNotFound.
func (s *EventService) GetEvent(ctx context.Context, id string) (Event, error) {
	event, err := s.events.Get(ctx, id)
	if err != nil {
		return Event{}, fmt.Errorf("get event: %w", err)
	}
	event.Timetable = sortedTimetable(event.Timetable)
	return event, nil
}

// ListEvents returns all events, each with its timetable sorted, its
// location, archive status at the time of clock and review mark: active
// events first, earliest start first, then archived events, latest start
// first; equal starts are ordered by ID.
func (s *EventService) ListEvents(ctx context.Context, clock Clock) ([]EventListEntry, error) {
	events, err := s.events.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	locations, err := s.locationsByID(ctx)
	if err != nil {
		return nil, err
	}

	// One instant for the whole list, so no event changes status midway.
	now := frozenClock(clock.Now())
	entries := make([]EventListEntry, 0, len(events))
	for _, event := range events {
		location, ok := locations[event.LocationID]
		if !ok {
			return nil, fmt.Errorf("location %s of event %s: %w", event.LocationID, event.ID, ErrNotFound)
		}
		event.Timetable = sortedTimetable(event.Timetable)
		entries = append(entries, EventListEntry{
			Event:       event,
			Location:    location,
			Archived:    event.Period.IsOver(now),
			NeedsReview: s.needsReview(event.ID),
		})
	}
	slices.SortFunc(entries, compareListEntries)
	return entries, nil
}

func (s *EventService) locationsByID(ctx context.Context) (map[string]Location, error) {
	locations, err := s.locations.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations of events: %w", err)
	}
	byID := make(map[string]Location, len(locations))
	for _, location := range locations {
		byID[location.ID] = location
	}
	return byID, nil
}

// compareListEntries orders active before archived events; active ones by
// ascending, archived ones by descending start; ties by ID.
func compareListEntries(a, b EventListEntry) int {
	if a.Archived != b.Archived {
		if a.Archived {
			return 1
		}
		return -1
	}
	byStart := a.Event.Period.Start.Compare(b.Event.Period.Start)
	if a.Archived {
		byStart = -byStart
	}
	return cmp.Or(byStart, cmp.Compare(a.Event.ID, b.Event.ID))
}

// RecomputeDerived recomputes the effective period of every event and
// stores it where it changed (AD-16). An event that fails keeps its stored
// values, is marked for review and is returned as failure; only a failing
// list is an error.
func (s *EventService) RecomputeDerived(ctx context.Context) ([]RecomputeFailure, error) {
	events, err := s.events.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events to recompute: %w", err)
	}
	var failures []RecomputeFailure
	for _, event := range events {
		if err := s.recomputePeriod(ctx, event); err != nil {
			s.markForReview(event.ID)
			failures = append(failures, RecomputeFailure{EventID: event.ID, Err: err})
		}
	}
	return failures, nil
}

// recomputePeriod writes the event's period if the current rules derive
// another one from its times.
func (s *EventService) recomputePeriod(ctx context.Context, event Event) error {
	period, err := event.Times.EffectivePeriod()
	if err != nil {
		return fmt.Errorf("recompute effective period: %w", err)
	}
	if period.Start.Equal(event.Period.Start) && period.End.Equal(event.Period.End) {
		return nil
	}
	if err := s.events.UpdatePeriod(ctx, event.ID, period); err != nil {
		return fmt.Errorf("store effective period: %w", err)
	}
	return nil
}

func (s *EventService) markForReview(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inReview[id] = struct{}{}
}

func (s *EventService) clearReview(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.inReview, id)
}

func (s *EventService) needsReview(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.inReview[id]
	return ok
}

// frozenClock is a Clock that always shows the same instant.
type frozenClock time.Time

func (c frozenClock) Now() time.Time { return time.Time(c) }
