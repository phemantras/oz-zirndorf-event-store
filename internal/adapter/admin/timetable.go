package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// timetableEntryPath answers with one empty timetable entry, which htmx
// appends to the entries of the event form (FR-4).
const timetableEntryPath = eventsPath + "/timetable-entry"

// Form fields of the timetable entries. Every entry sends the four fields,
// so the n-th value of each list belongs to the n-th entry.
const (
	timetableFormPrefix      = core.EventFieldTimetable + "."
	timetableFormDescription = timetableFormPrefix + core.TimetableFieldDescription
	timetableFormDate        = timetableFormPrefix + core.TimetableFieldDate
	timetableFormStartTime   = timetableFormPrefix + core.TimetableFieldStartTime
	timetableFormEndTime     = timetableFormPrefix + core.TimetableFieldEndTime
	timetableEntryFragment   = "timetableEntry"
)

// German texts of the timetable.
const (
	msgOutsideEvent       = "Der Programmpunkt liegt außerhalb des Event-Zeitraums."
	msgTimetableProblems  = "Bitte die markierten Programmpunkte prüfen."
	msgEntryEndsNextDay   = "Liegt die End-Uhrzeit vor der Beginn-Uhrzeit, endet der Programmpunkt am Folgetag."
	msgEntryTimeFormat    = " ist keine gültige Uhrzeit (Format HH:MM)."
	msgEntryStartTimeName = "Die Beginn-Uhrzeit"
	msgEntryEndTimeName   = "Die End-Uhrzeit"
)

// errUnevenTimetable means the timetable lists of a form differ in length,
// which no form of the admin sends.
var errUnevenTimetable = errors.New("timetable fields differ in number")

// timetableFieldMessages are the German messages for the problems the core
// reports for a field within a timetable entry.
var timetableFieldMessages = map[core.FieldError]string{
	{Field: core.TimetableFieldDescription, Problem: core.ProblemMissing}:       "Bitte eine Beschreibung angeben.",
	{Field: core.TimetableFieldDate, Problem: core.ProblemMissing}:              "Bitte ein Datum angeben.",
	{Field: core.TimetableFieldDate, Problem: core.ProblemInvalidFormat}:        "Das Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
	{Field: core.TimetableFieldDate, Problem: core.ProblemOutsideEvent}:         msgOutsideEvent,
	{Field: core.TimetableFieldStartTime, Problem: core.ProblemInvalidFormat}:   msgEntryStartTimeName + msgEntryTimeFormat,
	{Field: core.TimetableFieldStartTime, Problem: core.ProblemNonexistentTime}: msgNonexistentTime,
	{Field: core.TimetableFieldStartTime, Problem: core.ProblemOutsideEvent}:    msgOutsideEvent,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemInvalidFormat}:     msgEntryEndTimeName + msgEntryTimeFormat,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemNonexistentTime}:   msgNonexistentTime,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemNotAfterStart}:     msgEndNotAfterStart,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemOutsideEvent}:      msgOutsideEvent,
}

// timetableEntryView is the data of one entry on the event form: the
// values as entered and the messages per field within the entry.
type timetableEntryView struct {
	Values core.TimetableEntryInput
	Errors map[string]string
	// UnknownTimeHint and EndsNextDayHint are shown at the time fields.
	UnknownTimeHint string
	EndsNextDayHint string
}

func (h *handler) showTimetableEntry(w http.ResponseWriter, _ *http.Request) {
	h.renderFragment(w, timetableEntryTemplate, timetableEntryFragment, http.StatusOK, newTimetableEntryView(core.TimetableEntryInput{}, nil))
}

func newTimetableEntryView(values core.TimetableEntryInput, messages map[string]string) timetableEntryView {
	return timetableEntryView{Values: values, Errors: messages, UnknownTimeHint: msgUnknownTime, EndsNextDayHint: msgEntryEndsNextDay}
}

// timetableFromForm reads the entries in the order of the form. An entry
// whose four fields are all blank is dropped, so an added but unused entry
// is no error. Lists of different length are an error.
func timetableFromForm(form url.Values) ([]core.TimetableEntryInput, error) {
	descriptions, dates := form[timetableFormDescription], form[timetableFormDate]
	startTimes, endTimes := form[timetableFormStartTime], form[timetableFormEndTime]
	count := len(descriptions)
	if len(dates) != count || len(startTimes) != count || len(endTimes) != count {
		return nil, errUnevenTimetable
	}
	var entries []core.TimetableEntryInput
	for i := range count {
		entry := core.TimetableEntryInput{
			Description: descriptions[i], Date: dates[i], StartTime: startTimes[i], EndTime: endTimes[i],
		}
		if !isBlankEntry(entry) {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func isBlankEntry(entry core.TimetableEntryInput) bool {
	return strings.TrimSpace(entry.Description+entry.Date+entry.StartTime+entry.EndTime) == ""
}

// splitTimetableProblems separates the problems of timetable entries from
// those of the other event fields and returns the German messages of each
// entry by its index.
func splitTimetableProblems(fields []core.FieldError) ([]core.FieldError, map[int]map[string]string) {
	var eventFields []core.FieldError
	byEntry := map[int]map[string]string{}
	for _, field := range fields {
		index, name, ok := core.SplitTimetableField(field.Field)
		if !ok {
			eventFields = append(eventFields, field)
			continue
		}
		if byEntry[index] == nil {
			byEntry[index] = map[string]string{}
		}
		byEntry[index][name] = fieldErrorMessage(core.FieldError{Field: name, Problem: field.Problem, Limit: field.Limit}, timetableFieldMessages)
	}
	return eventFields, byEntry
}

// timetableViews returns the entries of the form with their messages.
func timetableViews(entries []core.TimetableEntryInput, errorsByEntry map[int]map[string]string) []timetableEntryView {
	views := make([]timetableEntryView, 0, len(entries))
	for i, entry := range entries {
		views = append(views, newTimetableEntryView(entry, errorsByEntry[i]))
	}
	return views
}
