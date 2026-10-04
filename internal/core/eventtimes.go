package core

import (
	"errors"
	"time"
)

// TimePrecision says how exactly the start or end of an event is known. It
// is derived from EventTimes and never stored (AD-3).
type TimePrecision string

// Time precision codes. The OpenAPI spec (api/v1/openapi.yaml) defines them
// in the schema TimePrecision; these constants mirror it, and a test of the
// public API compares both (AD-9). An end
// without date has no precision, the empty TimePrecision.
const (
	TimePrecisionExact    TimePrecision = "exact"
	TimePrecisionDateOnly TimePrecision = "dateOnly"
	TimePrecisionAllDay   TimePrecision = "allDay"
)

// TimePrecisions returns every time precision code, from most to least
// exact.
func TimePrecisions() []TimePrecision {
	return []TimePrecision{TimePrecisionExact, TimePrecisionDateOnly, TimePrecisionAllDay}
}

// Field names of an event's times, used in FieldError. The OpenAPI spec
// (api/v1/openapi.yaml) defines them in the schemas Event and EventInput;
// these constants mirror it (AD-9).
const (
	EventFieldStartDate = "startDate"
	EventFieldStartTime = "startTime"
	EventFieldEndDate   = "endDate"
	EventFieldEndTime   = "endTime"
	EventFieldAllDay    = "allDay"
)

// EventTimes holds the times of an event exactly as they are known. Dates
// are values whose zero value means missing; times are pointers whose nil
// means unknown, never 00:00. AllDay applies to start and end together.
type EventTimes struct {
	StartDate LocalDate
	StartTime *LocalTime
	EndDate   LocalDate
	EndTime   *LocalTime
	AllDay    bool
}

// dateTimeFields names the fields of one date with its time.
type dateTimeFields struct {
	date string
	time string
}

var (
	startFields = dateTimeFields{date: EventFieldStartDate, time: EventFieldStartTime}
	endFields   = dateTimeFields{date: EventFieldEndDate, time: EventFieldEndTime}
)

// StartPrecision derives how exactly the start is known.
func (t EventTimes) StartPrecision() TimePrecision {
	return t.precisionOf(t.StartTime)
}

// EndPrecision derives how exactly the end is known; it is empty when there
// is no end date.
func (t EventTimes) EndPrecision() TimePrecision {
	if t.EndDate.IsZero() {
		return ""
	}
	return t.precisionOf(t.EndTime)
}

func (t EventTimes) precisionOf(timeOfDay *LocalTime) TimePrecision {
	switch {
	case t.AllDay:
		return TimePrecisionAllDay
	case timeOfDay != nil:
		return TimePrecisionExact
	default:
		return TimePrecisionDateOnly
	}
}

// EffectivePeriod validates the times and returns the half-open period
// [start, end) (AD-4). A start without time begins at 00:00 of its day; an
// end without time, or a missing end, ends at 00:00 of the day after the
// end or start day. It reports every rejected field at once as
// *ValidationError: first structural problems, then format and
// daylight-saving-gap problems, and only then whether the end is after the
// start.
func (t EventTimes) EffectivePeriod() (Period, error) {
	if problems := t.structuralProblems(); problems != nil {
		return Period{}, &ValidationError{Fields: problems}
	}
	start, problems := instantOf(t.StartDate, t.StartTime, startFields)
	end, endProblems := t.effectiveEnd()
	problems = append(problems, endProblems...)
	if problems != nil {
		return Period{}, &ValidationError{Fields: problems}
	}
	if !end.After(start) {
		return Period{}, &ValidationError{Fields: []FieldError{t.notAfterStartProblem(start, end)}}
	}
	return Period{Start: start, End: end}, nil
}

// structuralProblems reports missing fields and times combined with AllDay.
func (t EventTimes) structuralProblems() []FieldError {
	var problems []FieldError
	report := func(field string, problem FieldProblem) {
		problems = append(problems, FieldError{Field: field, Problem: problem})
	}
	if t.StartDate.IsZero() {
		report(EventFieldStartDate, ProblemMissing)
	}
	if t.AllDay && t.StartTime != nil {
		report(EventFieldStartTime, ProblemConflictsWithAllDay)
	}
	if t.AllDay && t.EndTime != nil {
		report(EventFieldEndTime, ProblemConflictsWithAllDay)
	}
	if t.EndTime != nil && t.EndDate.IsZero() {
		report(EventFieldEndDate, ProblemMissing)
	}
	return problems
}

// effectiveEnd returns the end instant: the end time if known, else 00:00
// of the day after the end day, else of the day after the start day.
func (t EventTimes) effectiveEnd() (time.Time, []FieldError) {
	switch {
	case t.EndTime != nil:
		return instantOf(t.EndDate, t.EndTime, endFields)
	case !t.EndDate.IsZero():
		return startOfDayAfter(t.EndDate, endFields)
	case t.StartDate.IsValid():
		return startOfDayAfter(t.StartDate, startFields)
	default:
		// The start already reports the invalid start date.
		return time.Time{}, nil
	}
}

// startOfDayAfter returns 00:00 of the calendar day after date, never
// date plus 24 hours.
func startOfDayAfter(date LocalDate, fields dateTimeFields) (time.Time, []FieldError) {
	if !date.IsValid() {
		return time.Time{}, []FieldError{{Field: fields.date, Problem: ProblemInvalidFormat}}
	}
	return instantOf(date.NextDay(), nil, fields)
}

// instantOf converts with ToInstant and turns its errors into field
// problems.
func instantOf(date LocalDate, timeOfDay *LocalTime, fields dateTimeFields) (time.Time, []FieldError) {
	instant, err := ToInstant(date, timeOfDay)
	var problems []FieldError
	if errors.Is(err, ErrInvalidLocalDate) {
		problems = append(problems, FieldError{Field: fields.date, Problem: ProblemInvalidFormat})
	}
	if errors.Is(err, ErrInvalidLocalTime) {
		problems = append(problems, FieldError{Field: fields.time, Problem: ProblemInvalidFormat})
	}
	if errors.Is(err, ErrNonexistentLocalTime) {
		problems = append(problems, FieldError{Field: fields.time, Problem: ProblemNonexistentTime})
	}
	return instant, problems
}

// notAfterStartProblem names the end field that is not after the start and
// says whether the repeated autumn hour is the cause: the end lies in its
// first pass, and its second pass would have been after the start.
func (t EventTimes) notAfterStartProblem(start, end time.Time) FieldError {
	field := EventFieldEndDate
	if t.EndTime != nil {
		field = EventFieldEndTime
	}
	problem := ProblemNotAfterStart
	if inRepeatedHour(end) && end.Add(repeatedHourLength).After(start) {
		problem = ProblemNotAfterStartRepeatedHour
	}
	return FieldError{Field: field, Problem: problem}
}
