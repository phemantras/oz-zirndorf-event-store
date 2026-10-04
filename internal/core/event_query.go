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
// (api/v1/openapi.yaml) defines them as query parameters of /events; these
// constants mirror it (AD-9).
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

// Overlap is the normalized, half-open filter period [Lo, Hi). A nil Hi
// means the period is open-ended. An event matches when its effective
// start is before Hi and its effective end after Lo (AD-16).
type Overlap struct {
	Lo time.Time
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
	events, err := s.events.ListOverlapping(ctx, overlap)
	if err != nil {
		return nil, fmt.Errorf("list overlapping events: %w", err)
	}
	locations, err := s.locationsByID(ctx)
	if err != nil {
		return nil, err
	}

	listed := make([]ListedEvent, 0, len(events))
	for _, event := range events {
		if !types.allows(event.Type) {
			continue
		}
		location, ok := locations[event.LocationID]
		if !ok {
			return nil, fmt.Errorf("location %s of event %s: %w", event.LocationID, event.ID, ErrNotFound)
		}
		listed = append(listed, listedEventOf(event, location, frozenClock(now)))
	}
	slices.SortFunc(listed, compareByStartThenID)
	return listed, nil
}

// normalizeActiveFilter turns the filter into the overlap of active events
// at now and the allowed types. Lo is never before now, so only events
// with an effective end after now match.
func normalizeActiveFilter(filter EventFilter, now time.Time) (Overlap, eventTypeSet, error) {
	today := dateOf(now.In(berlin)).String()
	fromText, toText := today, today
	if filter.From != nil {
		fromText = *filter.From
	}
	if filter.To != nil {
		toText = *filter.To
	}
	// Only from without to is open-ended; without both, to is today.
	hasTo := filter.To != nil || filter.From == nil

	var problems []FieldError
	report := func(field string, problem FieldProblem) {
		problems = append(problems, FieldError{Field: field, Problem: problem})
	}
	from, ok := parseFilterBound(fromText)
	if !ok {
		report(FilterFieldFrom, ProblemInvalidFormat)
	}
	var to *filterBound
	if hasTo {
		bound, ok := parseFilterBound(toText)
		if !ok {
			report(FilterFieldTo, ProblemInvalidFormat)
		}
		to = &bound
	}
	types, problem := parseEventTypeFilter(filter.Types)
	if problem != "" {
		report(FilterFieldType, problem)
	}
	if problems != nil {
		return Overlap{}, nil, &ValidationError{Fields: problems}
	}

	overlap := Overlap{Lo: from.start}
	if to != nil {
		if problem := emptyPeriodProblem(from, *to, filter.From == nil); problem != "" {
			return Overlap{}, nil, &ValidationError{Fields: []FieldError{{Field: FilterFieldTo, Problem: problem}}}
		}
		overlap.Hi = &to.end
	}
	if now.After(overlap.Lo) {
		overlap.Lo = now
	}
	return overlap, types, nil
}

// emptyPeriodProblem reports a period [from.start, to.end) that contains
// no instant. When from was not given, from is today, and such a to lies
// before today.
func emptyPeriodProblem(from, to filterBound, fromIsToday bool) FieldProblem {
	if from.start.Before(to.end) {
		return ""
	}
	if fromIsToday {
		return ProblemBeforeToday
	}
	return ProblemEmptyPeriod
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
