package core

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// EventFieldTimetable is the field name of an event's timetable. The
// fields of one entry are named by TimetableField. The OpenAPI spec
// (api/v1/openapi.yaml) defines them in the schemas Event, EventInput and
// TimetableEntry; these constants mirror it (AD-9).
const EventFieldTimetable = "timetable"

// Field names within one timetable entry, used with TimetableField.
const (
	TimetableFieldDescription = "description"
	TimetableFieldDate        = "date"
	TimetableFieldStartTime   = "startTime"
	TimetableFieldEndTime     = "endTime"
)

// A timetable field path has the form timetable[2].date.
const (
	timetableFieldFormat = "%s[%d].%s"
	timetableIndexOpen   = "["
	timetableIndexClose  = "]."
	firstTimetableIndex  = 0
)

// TimetableEntry is one item of an event's timetable, a value object of the
// event (AD-15). Its times follow the time model: nil means unknown, never
// 00:00. An end time before the start time ends on the following day
// (ENT-3).
type TimetableEntry struct {
	ID          string
	Description string
	Date        LocalDate
	StartTime   *LocalTime
	EndTime     *LocalTime
}

// TimetableEntryInput is a timetable entry as entered: the date as
// YYYY-MM-DD, the times as HH:MM or empty for unknown.
type TimetableEntryInput struct {
	Description string
	Date        string
	StartTime   string
	EndTime     string
}

// TimetableField returns the field name of field within the timetable entry
// at index, such as timetable[2].date. The index counts the entries in the
// order they were entered.
func TimetableField(index int, field string) string {
	return fmt.Sprintf(timetableFieldFormat, EventFieldTimetable, index, field)
}

// SplitTimetableField is the inverse of TimetableField: it returns the
// entry index and the field within the entry, and false for any other
// field name.
func SplitTimetableField(name string) (index int, field string, ok bool) {
	rest, found := strings.CutPrefix(name, EventFieldTimetable+timetableIndexOpen)
	if !found {
		return 0, "", false
	}
	indexText, field, found := strings.Cut(rest, timetableIndexClose)
	if !found || field == "" {
		return 0, "", false
	}
	index, err := strconv.Atoi(indexText)
	if err != nil || index < firstTimetableIndex {
		return 0, "", false
	}
	return index, field, true
}

// normalized returns the entry with every text normalized by
// normalizeText.
func (in TimetableEntryInput) normalized() TimetableEntryInput {
	return TimetableEntryInput{
		Description: normalizeText(in.Description),
		Date:        normalizeText(in.Date),
		StartTime:   normalizeText(in.StartTime),
		EndTime:     normalizeText(in.EndTime),
	}
}

// compareTimetableInputs orders entered entries like compareTimetableEntries.
// Dates and times compare as text, which matches their chronological order
// in the forms YYYY-MM-DD and HH:MM; an empty time sorts first.
func compareTimetableInputs(a, b TimetableEntryInput) int {
	return cmp.Or(
		strings.Compare(a.Date, b.Date),
		strings.Compare(a.StartTime, b.StartTime),
		strings.Compare(a.EndTime, b.EndTime),
		strings.Compare(a.Description, b.Description),
	)
}

// sortedTimetable returns a sorted copy of entries: by date, entries
// without start time first, then by start time, end time, description and
// ID.
func sortedTimetable(entries []TimetableEntry) []TimetableEntry {
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, compareTimetableEntries)
	return sorted
}

func compareTimetableEntries(a, b TimetableEntry) int {
	return cmp.Or(
		compareLocalDates(a.Date, b.Date),
		compareLocalTimes(a.StartTime, b.StartTime),
		compareLocalTimes(a.EndTime, b.EndTime),
		strings.Compare(a.Description, b.Description),
		strings.Compare(a.ID, b.ID),
	)
}

func compareLocalDates(a, b LocalDate) int {
	return cmp.Or(cmp.Compare(a.Year, b.Year), cmp.Compare(a.Month, b.Month), cmp.Compare(a.Day, b.Day))
}

// compareLocalTimes orders an unknown (nil) time before every known one.
func compareLocalTimes(a, b *LocalTime) int {
	if a == nil || b == nil {
		return cmp.Compare(boolRank(a != nil), boolRank(b != nil))
	}
	return cmp.Or(cmp.Compare(a.Hour, b.Hour), cmp.Compare(a.Minute, b.Minute))
}

// boolRank orders false before true.
func boolRank(known bool) int {
	if known {
		return 1
	}
	return 0
}

// timetableEntryFields names the fields of the entry at one index.
type timetableEntryFields struct {
	description string
	date        string
	startTime   string
	endTime     string
}

func timetableEntryFieldsAt(index int) timetableEntryFields {
	return timetableEntryFields{
		description: TimetableField(index, TimetableFieldDescription),
		date:        TimetableField(index, TimetableFieldDate),
		startTime:   TimetableField(index, TimetableFieldStartTime),
		endTime:     TimetableField(index, TimetableFieldEndTime),
	}
}

// parseTimetable reads every entry, reports problems with the index of the
// entry as entered and returns the entries sorted. When period is not nil,
// each entry must also lie within it; a nil period means the event's own
// times are invalid, so the bounds are unknown.
func parseTimetable(inputs []TimetableEntryInput, period *Period) ([]TimetableEntry, []FieldError) {
	var entries []TimetableEntry
	var problems []FieldError
	for index, in := range inputs {
		entry, entryProblems := parseTimetableEntry(in, timetableEntryFieldsAt(index), period)
		entries = append(entries, entry)
		problems = append(problems, entryProblems...)
	}
	return sortedTimetable(entries), problems
}

// parseTimetableEntry reads one entry and checks its times: first the
// form of each field, then the instants and the bounds.
func parseTimetableEntry(in TimetableEntryInput, fields timetableEntryFields, period *Period) (TimetableEntry, []FieldError) {
	var problems []FieldError
	entry := TimetableEntry{Description: in.Description}
	if entry.Description == "" {
		problems = appendProblem(problems, fields.description, ProblemMissing)
	}
	var problem FieldProblem
	entry.Date, problem = parseLocalDate(in.Date)
	if in.Date == "" {
		problem = ProblemMissing
	}
	problems = appendProblem(problems, fields.date, problem)
	entry.StartTime, problem = parseLocalTime(in.StartTime)
	problems = appendProblem(problems, fields.startTime, problem)
	entry.EndTime, problem = parseLocalTime(in.EndTime)
	problems = appendProblem(problems, fields.endTime, problem)
	if problems != nil {
		return entry, problems
	}
	if entry.StartTime == nil && entry.EndTime == nil {
		return entry, dayEntryProblems(entry, fields, period)
	}
	return entry, timedEntryProblems(entry, fields, period)
}

// dayEntryProblems checks an entry without times: its day [00:00, 00:00 of
// the next day) must overlap the event period (ENT-20).
func dayEntryProblems(entry TimetableEntry, fields timetableEntryFields, period *Period) []FieldError {
	dayStart, startProblem := timetableInstant(entry.Date, nil)
	dayEnd, endProblem := timetableInstant(entry.Date.NextDay(), nil)
	if problem := cmp.Or(startProblem, endProblem); problem != "" {
		return []FieldError{{Field: fields.date, Problem: problem}}
	}
	if period != nil && (!dayStart.Before(period.End) || !dayEnd.After(period.Start)) {
		return []FieldError{{Field: fields.date, Problem: ProblemOutsideEvent}}
	}
	return nil
}

// timedEntryProblems checks an entry with a start or end time. The start
// must lie in [effectiveStart, effectiveEnd) and the end in
// (effectiveStart, effectiveEnd], so an entry may end with the event
// (AD-15, ENT-20). An end time before the start time is on the next day
// (ENT-3); an end equal to the start is rejected.
func timedEntryProblems(entry TimetableEntry, fields timetableEntryFields, period *Period) []FieldError {
	var problems []FieldError
	var start, end time.Time
	var problem FieldProblem
	if entry.StartTime != nil {
		start, problem = timetableInstant(entry.Date, entry.StartTime)
		problems = appendProblem(problems, fields.startTime, problem)
	}
	if entry.EndTime != nil {
		end, problem = timetableInstant(entry.endDate(), entry.EndTime)
		problems = appendProblem(problems, fields.endTime, problem)
	}
	if problems != nil {
		return problems
	}
	if entry.StartTime != nil && entry.EndTime != nil && !end.After(start) {
		return []FieldError{{Field: fields.endTime, Problem: ProblemNotAfterStart}}
	}
	if period == nil {
		return nil
	}
	if entry.StartTime != nil && (start.Before(period.Start) || !start.Before(period.End)) {
		return []FieldError{{Field: fields.startTime, Problem: ProblemOutsideEvent}}
	}
	if entry.EndTime != nil && (!end.After(period.Start) || end.After(period.End)) {
		return []FieldError{{Field: fields.endTime, Problem: ProblemOutsideEvent}}
	}
	return nil
}

// endDate is the day the entry ends: the next day when the end time lies
// before the start time (ENT-3), otherwise the entry's date.
func (e TimetableEntry) endDate() LocalDate {
	if e.StartTime != nil && compareLocalTimes(e.EndTime, e.StartTime) < 0 {
		return e.Date.NextDay()
	}
	return e.Date
}

func appendProblem(problems []FieldError, field string, problem FieldProblem) []FieldError {
	if problem == "" {
		return problems
	}
	return append(problems, FieldError{Field: field, Problem: problem})
}

// timetableInstant converts with ToInstant and turns its errors into the
// problem of the converted field.
func timetableInstant(date LocalDate, timeOfDay *LocalTime) (time.Time, FieldProblem) {
	instant, err := ToInstant(date, timeOfDay)
	switch {
	case err == nil:
		return instant, ""
	case errors.Is(err, ErrNonexistentLocalTime):
		return time.Time{}, ProblemNonexistentTime
	default:
		return time.Time{}, ProblemInvalidFormat
	}
}
