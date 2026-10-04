package core

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
)

// EventType classifies an event for filters and map icons.
type EventType string

// Event type codes. The OpenAPI spec (api/v1/openapi.yaml) defines them in
// the schema EventType; these constants mirror it, and a test of the public
// API compares both (AD-9).
const (
	EventTypeFestival EventType = "festival"
	EventTypeMarket   EventType = "market"
	EventTypeCulture  EventType = "culture"
	EventTypePolitics EventType = "politics"
	EventTypeClub     EventType = "club"
	EventTypeSports   EventType = "sports"
	EventTypeOther    EventType = "other"
)

// EventTypes returns every event type code in display order.
func EventTypes() []EventType {
	return []EventType{
		EventTypeFestival, EventTypeMarket, EventTypeCulture, EventTypePolitics,
		EventTypeClub, EventTypeSports, EventTypeOther,
	}
}

// EventTypeEntry is an event type code with its German label.
type EventTypeEntry struct {
	Code  EventType
	Label string
}

// eventTypeLabels are the German names of the event type codes (FR-5), the
// only source for the public API and the admin.
var eventTypeLabels = map[EventType]string{
	EventTypeFestival: "Fest/Kirchweih",
	EventTypeMarket:   "Markt",
	EventTypeCulture:  "Kultur/Bühne",
	EventTypePolitics: "Politik/Sitzung",
	EventTypeClub:     "Verein/Treff",
	EventTypeSports:   "Sport",
	EventTypeOther:    "Sonstiges",
}

// ListEventTypes returns every event type with its German label, in the
// order of EventTypes.
func ListEventTypes() []EventTypeEntry {
	var entries []EventTypeEntry
	for _, eventType := range EventTypes() {
		entries = append(entries, EventTypeEntry{Code: eventType, Label: eventTypeLabels[eventType]})
	}
	return entries
}

// Field names of an event besides its times (see EventFieldStartDate and
// following), used in FieldError. The OpenAPI spec (api/v1/openapi.yaml)
// will define them with the event schemas (Story 2.2); these constants must
// then match it (AD-9).
const (
	EventFieldTitle             = "title"
	EventFieldType              = "type"
	EventFieldLocationID        = "locationId"
	EventFieldSourceDescription = "source.description"
	EventFieldSourceURL         = "source.url"
	EventFieldNote              = "note"
)

// Link schemes a source URL may use.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// The date input has the form YYYY-MM-DD: three parts split by a hyphen.
const (
	dateSeparator = "-"
	dateParts     = 3
	yearPart      = 0
	monthPart     = 1
	dayPart       = 2
	yearDigits    = 4
	monthDigits   = 2
	dayDigits     = 2
)

// The time input has the form HH:MM: two parts split by a colon.
const (
	timeSeparator = ":"
	timeParts     = 2
	hourPart      = 0
	minutePart    = 1
	hourDigits    = 2
	minuteDigits  = 2
)

// Digits are read in base 10, counting from the ASCII zero.
const (
	decimalBase = 10
	asciiZero   = '0'
)

// EventSource says where the information about an event comes from. The
// description is required, the link optional (ENT-10).
type EventSource struct {
	Description string
	// URL is empty or an http(s) link with host.
	URL string
}

// Event is a dated happening at a location. It refers to its location by ID
// and never copies the address or coordinates.
type Event struct {
	ID    string
	Title string
	// TitleKey is NormalizeKey(Title), stored with the event to find
	// duplicates (AD-11).
	TitleKey   string
	Type       EventType
	LocationID string
	Times      EventTimes
	Source     EventSource
	// Note is optional; empty means none.
	Note string
	// Period is derived from Times by EffectivePeriod and stored with the
	// event (AD-16).
	Period Period
	// Timetable is sorted chronologically and lies within Period; it never
	// changes Period (AD-15).
	Timetable []TimetableEntry
}

// EventInput is an event as entered by a person or an import. Dates arrive
// as YYYY-MM-DD and times as HH:MM; an empty time means unknown, never
// 00:00.
type EventInput struct {
	Title      string
	Type       string
	LocationID string
	StartDate  string
	StartTime  string
	EndDate    string
	EndTime    string
	AllDay     bool
	Source     EventSource
	Note       string
	// Timetable replaces the whole stored timetable on save (AD-15).
	Timetable []TimetableEntryInput
}

// Canonicalize returns the input with every text normalized by
// normalizeText: NFC-composed and trimmed, so a text of only whitespace is
// empty, i.e. missing. The timetable is sorted like a stored one.
func (in EventInput) Canonicalize() EventInput {
	canonical := in.normalized()
	slices.SortStableFunc(canonical.Timetable, compareTimetableInputs)
	return canonical
}

// normalized returns the input with every text normalized, keeping the
// timetable in the order entered, so problems name entries by that order.
func (in EventInput) normalized() EventInput {
	var timetable []TimetableEntryInput
	for _, entry := range in.Timetable {
		timetable = append(timetable, entry.normalized())
	}
	return EventInput{
		Title:      normalizeText(in.Title),
		Type:       normalizeText(in.Type),
		LocationID: normalizeText(in.LocationID),
		StartDate:  normalizeText(in.StartDate),
		StartTime:  normalizeText(in.StartTime),
		EndDate:    normalizeText(in.EndDate),
		EndTime:    normalizeText(in.EndTime),
		AllDay:     in.AllDay,
		Source: EventSource{
			Description: normalizeText(in.Source.Description),
			URL:         normalizeText(in.Source.URL),
		},
		Note:      normalizeText(in.Note),
		Timetable: timetable,
	}
}

// EventInputOf returns the input that describes a stored event, so an edit
// form shows exactly what is stored.
func EventInputOf(event Event) EventInput {
	var timetable []TimetableEntryInput
	for _, entry := range event.Timetable {
		timetable = append(timetable, TimetableEntryInput{
			Description: entry.Description,
			Date:        dateText(entry.Date),
			StartTime:   timeText(entry.StartTime),
			EndTime:     timeText(entry.EndTime),
		})
	}
	return EventInput{
		Title:      event.Title,
		Type:       string(event.Type),
		LocationID: event.LocationID,
		StartDate:  dateText(event.Times.StartDate),
		StartTime:  timeText(event.Times.StartTime),
		EndDate:    dateText(event.Times.EndDate),
		EndTime:    timeText(event.Times.EndTime),
		AllDay:     event.Times.AllDay,
		Source:     event.Source,
		Note:       event.Note,
		Timetable:  timetable,
	}
}

func dateText(date LocalDate) string {
	if date.IsZero() {
		return ""
	}
	return date.String()
}

func timeText(timeOfDay *LocalTime) string {
	if timeOfDay == nil {
		return ""
	}
	return timeOfDay.String()
}

// newEvent canonicalizes and validates input and returns the event without
// ID, with its effective period and its timetable sorted. It returns every
// rejected field, timetable entries by the index they were entered with;
// whether the location exists is checked by the caller.
func newEvent(in EventInput) (Event, []FieldError) {
	in = in.normalized()
	var problems []FieldError
	report := func(field string, problem FieldProblem) {
		problems = append(problems, FieldError{Field: field, Problem: problem})
	}

	event := Event{
		Title:      in.Title,
		TitleKey:   NormalizeKey(in.Title),
		LocationID: in.LocationID,
		Source:     in.Source,
		Note:       in.Note,
	}
	if event.Title == "" {
		report(EventFieldTitle, ProblemMissing)
	}
	var problem FieldProblem
	if event.Type, problem = parseEventType(in.Type); problem != "" {
		report(EventFieldType, problem)
	}
	if event.LocationID == "" {
		report(EventFieldLocationID, ProblemMissing)
	}
	var timeProblems []FieldError
	event.Times, event.Period, timeProblems = parseEventTimes(in)
	problems = append(problems, timeProblems...)
	if event.Source.Description == "" {
		report(EventFieldSourceDescription, ProblemMissing)
	}
	if event.Source.URL != "" && !isHTTPLink(event.Source.URL) {
		report(EventFieldSourceURL, ProblemInvalidFormat)
	}
	// Only a valid period bounds the timetable; otherwise the entries are
	// checked for their own form only.
	var bounds *Period
	if timeProblems == nil {
		bounds = &event.Period
	}
	var timetableProblems []FieldError
	event.Timetable, timetableProblems = parseTimetable(in.Timetable, bounds)
	problems = append(problems, timetableProblems...)
	return event, problems
}

// parseEventType accepts exactly one of the event type codes.
func parseEventType(text string) (EventType, FieldProblem) {
	eventType := EventType(text)
	if eventType == "" {
		return "", ProblemMissing
	}
	if !slices.Contains(EventTypes(), eventType) {
		return "", ProblemUnknownCode
	}
	return eventType, ""
}

// parseEventTimes reads the date and time texts and derives the effective
// period. Only when every text has the right form are the rules of
// EffectivePeriod checked, so a malformed date is not reported as missing.
func parseEventTimes(in EventInput) (EventTimes, Period, []FieldError) {
	var problems []FieldError
	report := func(field string, problem FieldProblem) {
		if problem != "" {
			problems = append(problems, FieldError{Field: field, Problem: problem})
		}
	}
	times := EventTimes{AllDay: in.AllDay}
	var problem FieldProblem
	times.StartDate, problem = parseLocalDate(in.StartDate)
	report(EventFieldStartDate, problem)
	times.StartTime, problem = parseLocalTime(in.StartTime)
	report(EventFieldStartTime, problem)
	times.EndDate, problem = parseLocalDate(in.EndDate)
	report(EventFieldEndDate, problem)
	times.EndTime, problem = parseLocalTime(in.EndTime)
	report(EventFieldEndTime, problem)
	if problems != nil {
		return EventTimes{}, Period{}, problems
	}

	period, err := times.EffectivePeriod()
	var validation *ValidationError
	if errors.As(err, &validation) {
		return EventTimes{}, Period{}, validation.Fields
	}
	return times, period, nil
}

// parseLocalDate reads YYYY-MM-DD. Empty text is a missing date without
// problem; any other form, or a day that does not exist, is invalidFormat.
func parseLocalDate(text string) (LocalDate, FieldProblem) {
	if text == "" {
		return LocalDate{}, ""
	}
	parts := strings.Split(text, dateSeparator)
	if len(parts) != dateParts {
		return LocalDate{}, ProblemInvalidFormat
	}
	year, yearOK := fixedDigits(parts[yearPart], yearDigits)
	month, monthOK := fixedDigits(parts[monthPart], monthDigits)
	day, dayOK := fixedDigits(parts[dayPart], dayDigits)
	date := LocalDate{Year: year, Month: time.Month(month), Day: day}
	if !yearOK || !monthOK || !dayOK || !date.IsValid() {
		return LocalDate{}, ProblemInvalidFormat
	}
	return date, ""
}

// parseLocalTime reads HH:MM. Empty text is an unknown time (nil) without
// problem; any other form, or a time outside 00:00 to 23:59, is
// invalidFormat.
func parseLocalTime(text string) (*LocalTime, FieldProblem) {
	if text == "" {
		return nil, ""
	}
	parts := strings.Split(text, timeSeparator)
	if len(parts) != timeParts {
		return nil, ProblemInvalidFormat
	}
	hour, hourOK := fixedDigits(parts[hourPart], hourDigits)
	minute, minuteOK := fixedDigits(parts[minutePart], minuteDigits)
	timeOfDay := LocalTime{Hour: hour, Minute: minute}
	if !hourOK || !minuteOK || !timeOfDay.IsValid() {
		return nil, ProblemInvalidFormat
	}
	return &timeOfDay, ""
}

// fixedDigits returns the value of text if it consists of exactly length
// ASCII digits; signs, spaces and other digits are rejected.
func fixedDigits(text string, length int) (int, bool) {
	if len(text) != length || strings.ContainsFunc(text, isNotASCIIDigit) {
		return 0, false
	}
	value := 0
	for _, digit := range text {
		value = value*decimalBase + int(digit-asciiZero)
	}
	return value, true
}

// isHTTPLink reports whether text is an absolute http or https URL with a
// host.
func isHTTPLink(text string) bool {
	link, err := url.Parse(text)
	if err != nil {
		return false
	}
	return (link.Scheme == schemeHTTP || link.Scheme == schemeHTTPS) && link.Host != ""
}
