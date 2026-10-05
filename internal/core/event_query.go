package core

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"time"
)

// Parameter names of the event filter, used in FieldError. The OpenAPI spec
// (api/v1/openapi.yaml) defines them as query parameters of /events and
// /archive/events; these constants mirror it (AD-9).
const (
	FilterFieldFrom = "from"
	FilterFieldTo   = "to"
	FilterFieldType = "type"
)

// filterInstantPattern is the shape of an instant in from or to:
// YYYY-MM-DDTHH:MM, optional seconds with an optional fraction, then Z or
// an offset ±HH:MM. The ranges of the parts are checked by time.Parse.
var filterInstantPattern = regexp.MustCompile(
	`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$`)

// Layouts of an instant in from or to, without and with seconds;
// time.Parse accepts a fraction after the seconds by itself.
const (
	instantLayoutMinutes = "2006-01-02T15:04Z07:00"
	instantLayoutSeconds = "2006-01-02T15:04:05Z07:00"
	// instantMinutesLength is the length of YYYY-MM-DDTHH:MM; a colon
	// right after it starts the seconds.
	instantMinutesLength = len("2006-01-02T15:04")
	secondsSeparator     = ':'
)

// filterResolution is the unit an instant in to is cut to; to then covers
// that whole unit, so it is inclusive.
const filterResolution = time.Minute

// EventFilter is the filter of a list of events as the caller gave it:
// From and To as a date YYYY-MM-DD or an instant with offset, nil when not
// given (a given empty text is invalid); Types as event type codes, empty
// for every type.
type EventFilter struct {
	From  *string
	To    *string
	Types []string
}

// Overlap is the normalized, half-open filter period [Lo, Hi). A nil Lo
// means the period is open at the start, a nil Hi that it is open-ended.
// An event matches when its effective start is before Hi (if any) and its
// effective end after Lo (if any) (AD-16).
type Overlap struct {
	Lo *time.Time
	Hi *time.Time
}

// ListedEvent is an event of a public list, completed with its location
// and what the core derives from it. Its period is in Europe/Berlin and
// its timetable sorted chronologically.
type ListedEvent struct {
	Event
	Location Location
	// StartPrecision and EndPrecision are derived from Times; EndPrecision
	// is empty when the event has no end date.
	StartPrecision TimePrecision
	EndPrecision   TimePrecision
	// Archived is Period.IsOver at the time of the query (AD-16).
	Archived bool
}

// filterBound is the value of from or to as the instants where it starts
// and where it ends: a date covers its whole day, an instant its minute.
type filterBound struct {
	start time.Time
	end   time.Time
}

// eventTypeSet holds the event types a filter allows; an empty set allows
// every type.
type eventTypeSet map[EventType]bool

func (s eventTypeSet) allows(eventType EventType) bool {
	return len(s) == 0 || s[eventType]
}

// ListActiveEvents returns the active events whose effective period
// overlaps the filter period, of any of the filter types, sorted by
// effective start, equal starts by ID. Without from and to the period is
// today in Europe/Berlin; without from it starts today, without to it is
// open-ended. A date covers its whole day, an instant in to its whole
// minute. The clock is read once. An invalid filter yields
// *ValidationError naming from, to or type, and the repository is not
// asked.
func (s *EventService) ListActiveEvents(ctx context.Context, clock Clock, filter EventFilter) ([]ListedEvent, error) {
	now := clock.Now()
	overlap, types, err := normalizeActiveFilter(filter, now)
	if err != nil {
		return nil, err
	}
	// Lo is never before now, so every event found is active.
	listed, err := s.listMatchingEvents(ctx, eventQuery{overlap: overlap, types: types, now: now, keep: everyEvent})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(listed, compareByStartThenID)
	return listed, nil
}

// ListArchivedEvents returns the past events, those whose effective end is
// not after now, whose effective period overlaps the filter period, of any
// of the filter types, sorted by effective start descending, equal starts
// by ID. Without from the period is open at the start, without to it ends
// now. Dates and instants count as in ListActiveEvents. The clock is read
// once. An invalid filter yields *ValidationError naming from, to or type,
// and the repository is not asked.
func (s *EventService) ListArchivedEvents(ctx context.Context, clock Clock, filter EventFilter) ([]ListedEvent, error) {
	now := clock.Now()
	overlap, types, err := normalizeArchiveFilter(filter, now)
	if err != nil {
		return nil, err
	}
	// Being over is no overlap with a period, so the core leaves out the
	// running events by the rule that also sets Archived (AD-16).
	listed, err := s.listMatchingEvents(ctx, eventQuery{overlap: overlap, types: types, now: now, keep: archivedEvent})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(listed, compareByStartDescendingThenID)
	return listed, nil
}

// eventQuery is what listMatchingEvents asks for: the events overlapping
// overlap, of the allowed types, completed at now and kept by keep.
type eventQuery struct {
	overlap Overlap
	types   eventTypeSet
	now     time.Time
	keep    func(ListedEvent) bool
}

// everyEvent keeps every listed event.
func everyEvent(ListedEvent) bool { return true }

// archivedEvent keeps the listed events that are over.
func archivedEvent(event ListedEvent) bool { return event.Archived }

// listMatchingEvents asks the repository for the overlapping events and
// returns those of the allowed types that query.keep keeps, completed with
// their locations, in no particular order.
func (s *EventService) listMatchingEvents(ctx context.Context, query eventQuery) ([]ListedEvent, error) {
	events, err := s.events.ListOverlapping(ctx, query.overlap)
	if err != nil {
		return nil, fmt.Errorf("list overlapping events: %w", err)
	}
	locations, err := s.locationsByID(ctx)
	if err != nil {
		return nil, err
	}

	listed := make([]ListedEvent, 0, len(events))
	for _, event := range events {
		if !query.types.allows(event.Type) {
			continue
		}
		location, ok := locations[event.LocationID]
		if !ok {
			return nil, fmt.Errorf("location %s of event %s: %w", event.LocationID, event.ID, ErrNotFound)
		}
		if completed := listedEventOf(event, location, frozenClock(query.now)); query.keep(completed) {
			listed = append(listed, completed)
		}
	}
	return listed, nil
}

// normalizeActiveFilter turns the filter into the overlap of active events
// at now and the allowed types. Lo is never before now, so only events
// with an effective end after now match.
func normalizeActiveFilter(filter EventFilter, now time.Time) (Overlap, eventTypeSet, error) {
	today := dateOf(now.In(berlin)).String()
	fromText, toText := filter.From, filter.To
	defaulted := noBoundDefaulted
	if filter.From == nil {
		fromText, defaulted = &today, fromDefaultedToToday
		// Only from without to is open-ended; without both, to is today.
		if filter.To == nil {
			toText = &today
		}
	}
	parsed, err := parseFilter(fromText, toText, filter.Types)
	if err != nil {
		return Overlap{}, nil, err
	}

	var overlap Overlap
	if parsed.to != nil {
		if problem := emptyPeriodProblem(parsed.from.start, parsed.to.end, defaulted); problem != nil {
			return Overlap{}, nil, &ValidationError{Fields: []FieldError{*problem}}
		}
		overlap.Hi = &parsed.to.end
	}
	lo := parsed.from.start
	if now.After(lo) {
		lo = now
	}
	overlap.Lo = &lo
	return overlap, parsed.types, nil
}

// normalizeArchiveFilter turns the filter into the overlap the archive
// asks for at now and the allowed types: without from Lo is open, without
// to Hi is now.
func normalizeArchiveFilter(filter EventFilter, now time.Time) (Overlap, eventTypeSet, error) {
	parsed, err := parseFilter(filter.From, filter.To, filter.Types)
	if err != nil {
		return Overlap{}, nil, err
	}

	hi, defaulted := now, toDefaultedToNow
	if parsed.to != nil {
		hi, defaulted = parsed.to.end, noBoundDefaulted
	}
	overlap := Overlap{Hi: &hi}
	if parsed.from != nil {
		if problem := emptyPeriodProblem(parsed.from.start, hi, defaulted); problem != nil {
			return Overlap{}, nil, &ValidationError{Fields: []FieldError{*problem}}
		}
		overlap.Lo = &parsed.from.start
	}
	return overlap, parsed.types, nil
}

// parsedFilter is a filter whose parameters are all valid: from and to as
// bounds, nil when open, and the allowed types.
type parsedFilter struct {
	from  *filterBound
	to    *filterBound
	types eventTypeSet
}

// parseFilter reads from and to, each nil when open, and the type codes.
// It reports every invalid parameter, in the order from, to, type, as
// *ValidationError.
func parseFilter(fromText, toText *string, typeTexts []string) (parsedFilter, error) {
	var problems []FieldError
	report := func(field string, problem FieldProblem) {
		problems = append(problems, FieldError{Field: field, Problem: problem})
	}
	from, ok := parseOptionalFilterBound(fromText)
	if !ok {
		report(FilterFieldFrom, ProblemInvalidFormat)
	}
	to, ok := parseOptionalFilterBound(toText)
	if !ok {
		report(FilterFieldTo, ProblemInvalidFormat)
	}
	types, problem := parseEventTypeFilter(typeTexts)
	if problem != "" {
		report(FilterFieldType, problem)
	}
	if problems != nil {
		return parsedFilter{}, &ValidationError{Fields: problems}
	}
	return parsedFilter{from: from, to: to, types: types}, nil
}

// parseOptionalFilterBound reads a bound that may be left open (nil).
func parseOptionalFilterBound(text *string) (*filterBound, bool) {
	if text == nil {
		return nil, true
	}
	bound, ok := parseFilterBound(*text)
	return &bound, ok
}

// defaultedBound names the bound of a filter period that was not given
// and took its default.
type defaultedBound int

const (
	noBoundDefaulted defaultedBound = iota
	// fromDefaultedToToday means from was not given and is today (active
	// events).
	fromDefaultedToToday
	// toDefaultedToNow means to was not given and is now (archive).
	toDefaultedToNow
)

// emptyPeriodProblem reports a period [lo, hi) that contains no instant,
// or returns nil. When from was not given and is today, such a to lies
// before today; when to was not given and is now, such a from lies at or
// after now.
func emptyPeriodProblem(lo, hi time.Time, defaulted defaultedBound) *FieldError {
	switch {
	case lo.Before(hi):
		return nil
	case defaulted == fromDefaultedToToday:
		return &FieldError{Field: FilterFieldTo, Problem: ProblemBeforeToday}
	case defaulted == toDefaultedToNow:
		return &FieldError{Field: FilterFieldFrom, Problem: ProblemAfterNow}
	default:
		return &FieldError{Field: FilterFieldTo, Problem: ProblemEmptyPeriod}
	}
}

// parseFilterBound reads a date YYYY-MM-DD or an instant with offset.
func parseFilterBound(text string) (filterBound, bool) {
	if bound, ok := parseFilterDate(text); ok {
		return bound, true
	}
	return parseFilterInstant(text)
}

// parseFilterDate reads a date as its whole day in Europe/Berlin, from
// 00:00 to 00:00 of the next day, both through ToInstant.
func parseFilterDate(text string) (filterBound, bool) {
	date, problem := parseLocalDate(text)
	if problem != "" || date.IsZero() {
		return filterBound{}, false
	}
	start, startErr := ToInstant(date, nil)
	end, endErr := ToInstant(date.NextDay(), nil)
	return filterBound{start: start, end: end}, startErr == nil && endErr == nil
}

// parseFilterInstant reads an instant with offset as the minute it lies
// in: from the instant itself to the start of the next minute.
func parseFilterInstant(text string) (filterBound, bool) {
	if !filterInstantPattern.MatchString(text) {
		return filterBound{}, false
	}
	layout := instantLayoutMinutes
	if text[instantMinutesLength] == secondsSeparator {
		layout = instantLayoutSeconds
	}
	instant, err := time.Parse(layout, text)
	if err != nil {
		return filterBound{}, false
	}
	return filterBound{start: instant, end: instant.Truncate(filterResolution).Add(filterResolution)}, true
}

// parseEventTypeFilter reads the type codes of a filter; a code given
// twice counts once. It returns the problem of the first invalid code.
func parseEventTypeFilter(texts []string) (eventTypeSet, FieldProblem) {
	types := make(eventTypeSet, len(texts))
	for _, text := range texts {
		eventType, problem := parseEventType(text)
		if problem != "" {
			return nil, problem
		}
		types[eventType] = true
	}
	return types, ""
}

// listedEventOf completes event with its location, sorted timetable,
// period in Europe/Berlin, precisions and archive status at now.
func listedEventOf(event Event, location Location, now Clock) ListedEvent {
	event.Timetable = sortedTimetable(event.Timetable)
	event.Period = Period{Start: event.Period.Start.In(berlin), End: event.Period.End.In(berlin)}
	return ListedEvent{
		Event:          event,
		Location:       location,
		StartPrecision: event.Times.StartPrecision(),
		EndPrecision:   event.Times.EndPrecision(),
		Archived:       event.Period.IsOver(now),
	}
}

// compareByStartThenID orders by ascending effective start, equal starts
// by ID, so the order is stable across requests.
func compareByStartThenID(a, b ListedEvent) int {
	return cmp.Or(a.Period.Start.Compare(b.Period.Start), cmp.Compare(a.ID, b.ID))
}

// compareByStartDescendingThenID orders by descending effective start,
// equal starts by ascending ID, so the order is stable across requests.
func compareByStartDescendingThenID(a, b ListedEvent) int {
	return cmp.Or(b.Period.Start.Compare(a.Period.Start), cmp.Compare(a.ID, b.ID))
}
