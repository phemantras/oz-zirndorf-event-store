package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// EventUseCases are the core use cases behind the event pages. The admin
// never gets the repository, so every write goes through the core (AD-6).
type EventUseCases interface {
	SaveEvent(ctx context.Context, id string, in core.EventInput) (core.Event, error)
	GetEvent(ctx context.Context, id string) (core.Event, error)
	ListEvents(ctx context.Context, clock core.Clock) ([]core.EventListEntry, error)
}

// Routes of the event pages.
const (
	eventsPath       = adminPathPrefix + "/events"
	newEventPath     = eventsPath + "/new"
	eventIDParam     = "id"
	eventPathPattern = eventsPath + "/{" + eventIDParam + "}"
	// maxEventFormBytes bounds the event form body; a real form with a long
	// note stays far below.
	maxEventFormBytes = 16 << 10
)

// allDayChecked is the value the all-day checkbox sends when checked.
const allDayChecked = "true"

// Display forms of dates and times in the event list.
const (
	displayDateFormat = "%02d.%02d.%04d"
	displayTimeFormat = "%s %02d:%02d"
	displayAllDay     = "%s (ganztägig)"
)

// German texts of the event pages.
const (
	headingNewEvent     = "Neues Event"
	headingEditEvent    = "Event bearbeiten"
	msgEventNotFound    = "Event nicht gefunden."
	backToEventList     = "Zurück zur Event-Liste"
	msgUnknownTime      = "Leer lassen, wenn die Uhrzeit nicht bekannt ist: leer bedeutet „unbekannt“."
	msgNoLocations      = "Noch keine Orte angelegt."
	statusActive        = "aktiv"
	statusArchived      = "archiviert"
	msgSourceURLFormat  = "Der Link muss mit http:// oder https:// beginnen und eine Adresse enthalten."
	msgEndNotAfterStart = "Das Ende muss nach dem Beginn liegen."
	msgNonexistentTime  = "Diese Uhrzeit gibt es wegen der Umstellung auf Sommerzeit nicht."
	msgRepeatedHour     = "Das Ende muss nach dem Beginn liegen. In der doppelten Stunde der Zeitumstellung gilt die erste, die Sommerzeit."
)

// logMsgEventsFailed is logged when an event page fails for a reason the
// admin cannot show as a field message.
const logMsgEventsFailed = "admin event request failed"

// eventTypeLabels are the German names of the event type codes (FR-5).
var eventTypeLabels = map[core.EventType]string{
	core.EventTypeFestival: "Fest/Kirchweih",
	core.EventTypeMarket:   "Markt",
	core.EventTypeCulture:  "Kultur/Bühne",
	core.EventTypePolitics: "Politik/Sitzung",
	core.EventTypeClub:     "Verein/Treff",
	core.EventTypeSports:   "Sport",
	core.EventTypeOther:    "Sonstiges",
}

// eventFieldMessages are the German messages for the field problems the
// core reports for an event.
var eventFieldMessages = map[core.FieldError]string{
	{Field: core.EventFieldTitle, Problem: core.ProblemMissing}:                     "Bitte einen Titel angeben.",
	{Field: core.EventFieldType, Problem: core.ProblemMissing}:                      "Bitte einen Event-Typ auswählen.",
	{Field: core.EventFieldType, Problem: core.ProblemUnknownCode}:                  "Bitte einen der angebotenen Event-Typen auswählen.",
	{Field: core.EventFieldLocationID, Problem: core.ProblemMissing}:                "Bitte einen Ort auswählen.",
	{Field: core.EventFieldLocationID, Problem: core.ProblemNotFound}:               "Den gewählten Ort gibt es nicht mehr. Bitte einen anderen Ort auswählen.",
	{Field: core.EventFieldStartDate, Problem: core.ProblemMissing}:                 "Bitte ein Beginn-Datum angeben.",
	{Field: core.EventFieldStartDate, Problem: core.ProblemInvalidFormat}:           "Das Beginn-Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
	{Field: core.EventFieldStartTime, Problem: core.ProblemInvalidFormat}:           "Die Beginn-Uhrzeit ist keine gültige Uhrzeit (Format HH:MM).",
	{Field: core.EventFieldStartTime, Problem: core.ProblemConflictsWithAllDay}:     "Ein ganztägiges Event hat keine Beginn-Uhrzeit.",
	{Field: core.EventFieldStartTime, Problem: core.ProblemNonexistentTime}:         msgNonexistentTime,
	{Field: core.EventFieldEndDate, Problem: core.ProblemMissing}:                   "Bitte zur End-Uhrzeit auch ein End-Datum angeben.",
	{Field: core.EventFieldEndDate, Problem: core.ProblemInvalidFormat}:             "Das End-Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
	{Field: core.EventFieldEndDate, Problem: core.ProblemNotAfterStart}:             msgEndNotAfterStart,
	{Field: core.EventFieldEndTime, Problem: core.ProblemInvalidFormat}:             "Die End-Uhrzeit ist keine gültige Uhrzeit (Format HH:MM).",
	{Field: core.EventFieldEndTime, Problem: core.ProblemConflictsWithAllDay}:       "Ein ganztägiges Event hat keine End-Uhrzeit.",
	{Field: core.EventFieldEndTime, Problem: core.ProblemNonexistentTime}:           msgNonexistentTime,
	{Field: core.EventFieldEndTime, Problem: core.ProblemNotAfterStart}:             msgEndNotAfterStart,
	{Field: core.EventFieldEndTime, Problem: core.ProblemNotAfterStartRepeatedHour}: msgRepeatedHour,
	{Field: core.EventFieldSourceDescription, Problem: core.ProblemMissing}:         "Bitte eine Quelle angeben.",
	{Field: core.EventFieldSourceURL, Problem: core.ProblemInvalidFormat}:           msgSourceURLFormat,
}

// eventListPage is the data of the event list.
type eventListPage struct {
	Events []eventRow
}

type eventRow struct {
	Title    string
	URL      string
	Type     string
	Location string
	Start    string
	// End is empty when the event has no end date.
	End    string
	Status string
	// NeedsReview marks an event whose derived values could not be
	// recomputed at startup.
	NeedsReview bool
}

// eventFormPage is the data of the form for a new or an existing event.
// Values hold the input exactly as entered, so nothing is lost on an
// error.
type eventFormPage struct {
	Heading        string
	Action         string
	Values         core.EventInput
	Errors         map[string]string
	Types          []selectOption
	LocationChoice locationChoice
	// NewLocation is the closed inline input for a new location.
	NewLocation newLocationArea
	// AllDayValue is what the checkbox sends when checked.
	AllDayValue string
	// PrivacyHint is shown under title, source and note (NFR-4).
	PrivacyHint string
	// UnknownTimeHint is shown at both time fields.
	UnknownTimeHint string
}

// locationChoice is the data of the location select on the event form.
// The inline input for a new location replaces it out of band.
type locationChoice struct {
	Locations []selectOption
	// Error is the message for the location field.
	Error       string
	NoLocations string
	// OutOfBand marks the choice for an htmx out-of-band swap.
	OutOfBand bool
}

func (h *handler) showEvents(w http.ResponseWriter, r *http.Request) {
	entries, err := h.events.ListEvents(r.Context(), h.clock)
	if err != nil {
		h.failEventRequest(w, err)
		return
	}
	rows := make([]eventRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, eventRowOf(entry))
	}
	h.render(w, eventsTemplate, http.StatusOK, eventListPage{Events: rows})
}

func (h *handler) showNewEvent(w http.ResponseWriter, r *http.Request) {
	h.renderEventForm(w, r, http.StatusOK, eventForm{values: core.EventInput{}})
}

func (h *handler) showEvent(w http.ResponseWriter, r *http.Request) {
	event, err := h.events.GetEvent(r.Context(), r.PathValue(eventIDParam))
	if errors.Is(err, core.ErrNotFound) {
		h.renderEventNotFound(w)
		return
	}
	if err != nil {
		h.failEventRequest(w, err)
		return
	}
	h.renderEventForm(w, r, http.StatusOK, eventForm{id: event.ID, values: core.EventInputOf(event)})
}

func (h *handler) createEvent(w http.ResponseWriter, r *http.Request) {
	h.saveEvent(w, r, "")
}

func (h *handler) updateEvent(w http.ResponseWriter, r *http.Request) {
	h.saveEvent(w, r, r.PathValue(eventIDParam))
}

// saveEvent hands the form to the core and translates its typed errors:
// field messages (422), unknown event (404).
func (h *handler) saveEvent(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEventFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	values := eventInputFromForm(r.PostForm)
	_, err := h.events.SaveEvent(r.Context(), id, values)

	var validation *core.ValidationError
	switch {
	case err == nil:
		http.Redirect(w, r, eventsPath, http.StatusSeeOther)
	case errors.As(err, &validation):
		form := eventForm{id: id, values: values, errors: fieldErrorMessages(validation.Fields, eventFieldMessages)}
		h.renderEventForm(w, r, http.StatusUnprocessableEntity, form)
	case errors.Is(err, core.ErrNotFound):
		h.renderEventNotFound(w)
	default:
		h.failEventRequest(w, err)
	}
}

// eventForm is what an event form shows: the event's id (empty for a new
// one), the values and the messages per field.
type eventForm struct {
	id     string
	values core.EventInput
	errors map[string]string
}

// renderEventForm renders the form with the locations to choose from.
func (h *handler) renderEventForm(w http.ResponseWriter, r *http.Request, status int, form eventForm) {
	locations, err := h.locations.ListLocations(r.Context())
	if err != nil {
		h.failEventRequest(w, err)
		return
	}
	page := eventFormPage{
		Heading:         headingNewEvent,
		Action:          eventsPath,
		Values:          form.values,
		Errors:          form.errors,
		LocationChoice:  locationChoiceOf(locations, form.values.LocationID, form.errors[core.EventFieldLocationID]),
		AllDayValue:     allDayChecked,
		PrivacyHint:     msgNoPersonalData,
		UnknownTimeHint: msgUnknownTime,
	}
	if form.id != "" {
		page.Heading = headingEditEvent
		page.Action = eventURL(form.id)
	}
	for _, eventType := range core.EventTypes() {
		page.Types = append(page.Types, selectOption{
			Value:    string(eventType),
			Label:    eventTypeLabels[eventType],
			Selected: string(eventType) == form.values.Type,
		})
	}
	h.render(w, eventFormTemplate, status, page)
}

// locationChoiceOf returns the location select with selectedID selected and
// errorMessage at the field.
func locationChoiceOf(locations []core.Location, selectedID, errorMessage string) locationChoice {
	choice := locationChoice{Error: errorMessage, NoLocations: msgNoLocations}
	for _, location := range locations {
		choice.Locations = append(choice.Locations, selectOption{
			Value:    location.ID,
			Label:    location.Name,
			Selected: location.ID == selectedID,
		})
	}
	return choice
}

func (h *handler) renderEventNotFound(w http.ResponseWriter) {
	h.render(w, notFoundTemplate, http.StatusNotFound, notFoundPage{
		Message: msgEventNotFound, BackURL: eventsPath, BackLabel: backToEventList,
	})
}

func (h *handler) failEventRequest(w http.ResponseWriter, err error) {
	h.logger.Error(logMsgEventsFailed, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// eventInputFromForm reads the event fields exactly as entered.
func eventInputFromForm(form url.Values) core.EventInput {
	return core.EventInput{
		Title:      form.Get(core.EventFieldTitle),
		Type:       form.Get(core.EventFieldType),
		LocationID: form.Get(core.EventFieldLocationID),
		StartDate:  form.Get(core.EventFieldStartDate),
		StartTime:  form.Get(core.EventFieldStartTime),
		EndDate:    form.Get(core.EventFieldEndDate),
		EndTime:    form.Get(core.EventFieldEndTime),
		AllDay:     form.Get(core.EventFieldAllDay) == allDayChecked,
		Source: core.EventSource{
			Description: form.Get(core.EventFieldSourceDescription),
			URL:         form.Get(core.EventFieldSourceURL),
		},
		Note: form.Get(core.EventFieldNote),
	}
}

func eventRowOf(entry core.EventListEntry) eventRow {
	times := entry.Event.Times
	row := eventRow{
		Title:       entry.Event.Title,
		URL:         eventURL(entry.Event.ID),
		Type:        eventTypeLabels[entry.Event.Type],
		Location:    entry.Location.Name,
		Start:       formatMoment(times.StartDate, times.StartTime, times.AllDay),
		Status:      statusActive,
		NeedsReview: entry.NeedsReview,
	}
	if !times.EndDate.IsZero() {
		row.End = formatMoment(times.EndDate, times.EndTime, times.AllDay)
	}
	if entry.Archived {
		row.Status = statusArchived
	}
	return row
}

// formatMoment shows a date as 16.10.2026, with a known time as
// 16.10.2026 19:00 and for an all-day event as 16.10.2026 (ganztägig).
func formatMoment(date core.LocalDate, timeOfDay *core.LocalTime, allDay bool) string {
	text := fmt.Sprintf(displayDateFormat, date.Day, int(date.Month), date.Year)
	if timeOfDay != nil {
		text = fmt.Sprintf(displayTimeFormat, text, timeOfDay.Hour, timeOfDay.Minute)
	}
	if allDay {
		text = fmt.Sprintf(displayAllDay, text)
	}
	return text
}

func eventURL(id string) string {
	return eventsPath + "/" + url.PathEscape(id)
}
