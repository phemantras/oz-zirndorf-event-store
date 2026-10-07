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
	SaveEvent(ctx context.Context, id string, in core.EventInput, policy core.DuplicatePolicy) (core.Event, error)
	GetEvent(ctx context.Context, id string) (core.Event, error)
	ListEvents(ctx context.Context, clock core.Clock) ([]core.EventListEntry, error)
	DeleteEvent(ctx context.Context, id string) error
	RemoveImportKey(ctx context.Context, id string) error
}

// Routes of the event pages.
const (
	eventsPath       = adminPathPrefix + "/events"
	newEventPath     = eventsPath + "/new"
	eventIDParam     = "id"
	eventPathPattern = eventsPath + "/{" + eventIDParam + "}"
	// eventDeletePathPattern deletes the event; only POST, never GET.
	eventDeletePathPattern = eventPathPattern + deletePathSuffix
	// importKeyRemovePathSuffix turns the path of an event into the path
	// that removes its import key.
	importKeyRemovePathSuffix = "/import-key/remove"
	// eventImportKeyRemovePathPattern removes the import key of the event;
	// only POST, never GET.
	eventImportKeyRemovePathPattern = eventPathPattern + importKeyRemovePathSuffix
	// maxEventFormBytes bounds the event form body. It leaves room for a
	// form of mostly ASCII text that uses every limit of ENT-24,
	// URL-encoded, among them 100 timetable entries of 500 characters
	// each; the same at full length in umlauts, nine bytes each when
	// encoded, exceeds it.
	maxEventFormBytes = 256 << 10
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

// German texts of deleting an event.
const (
	msgEventGone = "Dieses Event ist nicht mehr vorhanden."
	// msgConfirmDeleteEvent is the question before deleting, with the title.
	msgConfirmDeleteEvent = "Event „%s“ wirklich löschen? Der Ablaufplan wird mit gelöscht."
)

// German texts of the import key of an event.
const (
	msgImportKeyLabel  = "Import-Schlüssel"
	msgImportKeyHint   = "Ein erneuter Import mit diesem Schlüssel aktualisiert dieses Event, auch im Archiv. Ohne Schlüssel behandelt der Import es wie jedes andere Event."
	msgImportKeyButton = "Import-Schlüssel entfernen"
	// msgConfirmRemoveImportKey is the question before removing, with the
	// key.
	msgConfirmRemoveImportKey = "Import-Schlüssel „%s“ wirklich entfernen? Ein späterer Import mit diesem Schlüssel aktualisiert dieses Event dann nicht mehr."
)

// logMsgEventImportKeyRemoved is logged with the requested id after the
// import key of an event was removed, so the removal can be traced.
const logMsgEventImportKeyRemoved = "admin event import key removed"

// logMsgEventsFailed is logged when an event page fails for a reason the
// admin cannot show as a field message.
const logMsgEventsFailed = "admin event request failed"

// logMsgEventDeleted is logged with the requested id after a delete, so an
// accidental delete can be traced.
const logMsgEventDeleted = "admin event deleted"

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
	// Timetable are the entries in the order of Values.Timetable, with
	// their messages; TimetableError is set when any entry has one.
	Timetable      []timetableEntryView
	TimetableError string
	// DuplicateWarning is set when the input may duplicate stored events.
	DuplicateWarning *duplicateWarning
	// ImportKey is set for an existing event with import key only.
	ImportKey *importKeyForm
	// Delete is set for an existing event only.
	Delete *deleteForm
}

// importKeyForm shows the import key of an event read-only, with its own
// form after the edit form to remove it after a confirmation question.
type importKeyForm struct {
	Label   string
	Key     string
	Hint    string
	Action  string
	Confirm string
	Button  string
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
		h.failEventRequest(w, r, err)
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
		h.failEventRequest(w, r, err)
		return
	}
	h.renderEventForm(w, r, http.StatusOK, eventForm{
		id: event.ID, storedTitle: event.Title, importKey: event.ImportKey, values: core.EventInputOf(event),
	})
}

func (h *handler) createEvent(w http.ResponseWriter, r *http.Request) {
	h.saveEvent(w, r, "")
}

func (h *handler) updateEvent(w http.ResponseWriter, r *http.Request) {
	h.saveEvent(w, r, r.PathValue(eventIDParam))
}

// saveEvent hands the form to the core and translates its typed errors:
// field messages, also per timetable entry (422), suspected duplicates
// (409), unknown event (404). A body that is too large or has uneven
// timetable fields is a bad request.
func (h *handler) saveEvent(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEventFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	values, err := eventInputFromForm(r.PostForm)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	_, err = h.events.SaveEvent(r.Context(), id, values, duplicatePolicyOf(r.PostForm, values))

	var validation *core.ValidationError
	var suspect *core.DuplicateSuspectError
	switch {
	case err == nil:
		http.Redirect(w, r, eventsPath, http.StatusSeeOther)
	case errors.As(err, &validation):
		eventFields, timetableErrors := splitTimetableProblems(validation.Fields)
		form := eventForm{
			id:              id,
			values:          values,
			errors:          fieldErrorMessages(eventFields, eventFieldMessages),
			timetableErrors: timetableErrors,
		}
		h.renderEventFormAgain(w, r, http.StatusUnprocessableEntity, form)
	case errors.As(err, &suspect):
		h.renderEventFormAgain(w, r, http.StatusConflict, eventForm{id: id, values: values, duplicates: suspect.Candidates})
	case errors.Is(err, core.ErrNotFound):
		h.renderEventNotFound(w)
	default:
		h.failEventRequest(w, r, err)
	}
}

// deleteEvent hands the deletion to the core and translates its typed
// errors: an event that is no longer there is a German 404 page, never a
// server error.
func (h *handler) deleteEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue(eventIDParam)
	err := h.events.DeleteEvent(r.Context(), id)
	switch {
	case err == nil:
		h.logger.Info(logMsgEventDeleted, logKeyID, id)
		redirectAfterDelete(w, r, eventsPath)
	case errors.Is(err, core.ErrNotFound):
		h.render(w, notFoundTemplate, http.StatusNotFound, notFoundPage{
			Message: msgEventGone, BackURL: eventsPath, BackLabel: backToEventList,
		})
	default:
		h.failEventRequest(w, r, err)
	}
}

// removeImportKey hands the removal of an event's import key to the core
// and sends the browser back to the event: an event that is no longer there
// is a German 404 page, never a server error.
func (h *handler) removeImportKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue(eventIDParam)
	err := h.events.RemoveImportKey(r.Context(), id)
	switch {
	case err == nil:
		h.logger.Info(logMsgEventImportKeyRemoved, logKeyID, id)
		redirectTo(w, r, eventURL(id), http.StatusNoContent)
	case errors.Is(err, core.ErrNotFound):
		h.render(w, notFoundTemplate, http.StatusNotFound, notFoundPage{
			Message: msgEventGone, BackURL: eventsPath, BackLabel: backToEventList,
		})
	default:
		h.failEventRequest(w, r, err)
	}
}

// eventForm is what an event form shows: the event's id, stored title and
// stored import key (empty for a new one), the values, the messages per
// field and per timetable entry and the events the values may duplicate.
type eventForm struct {
	id              string
	storedTitle     string
	importKey       string
	values          core.EventInput
	errors          map[string]string
	timetableErrors map[int]map[string]string
	duplicates      []core.Event
}

// renderEventFormAgain renders the form after a refused save. The delete
// question of an existing event names its stored title, never the unsaved
// one, and the import key is never part of the form, so the stored event
// is loaded again.
func (h *handler) renderEventFormAgain(w http.ResponseWriter, r *http.Request, status int, form eventForm) {
	if form.id != "" {
		stored, err := h.events.GetEvent(r.Context(), form.id)
		if errors.Is(err, core.ErrNotFound) {
			h.renderEventNotFound(w)
			return
		}
		if err != nil {
			h.failEventRequest(w, r, fmt.Errorf("load event refused for saving: %w", err))
			return
		}
		form.storedTitle, form.importKey = stored.Title, stored.ImportKey
	}
	h.renderEventForm(w, r, status, form)
}

// renderEventForm renders the form with the locations to choose from.
func (h *handler) renderEventForm(w http.ResponseWriter, r *http.Request, status int, form eventForm) {
	locations, err := h.locations.ListLocations(r.Context())
	if err != nil {
		h.failEventRequest(w, r, err)
		return
	}
	page := eventFormPage{
		Heading:          headingNewEvent,
		Action:           eventsPath,
		Values:           form.values,
		Errors:           form.errors,
		LocationChoice:   locationChoiceOf(locations, form.values.LocationID, form.errors[core.EventFieldLocationID]),
		AllDayValue:      allDayChecked,
		PrivacyHint:      msgNoPersonalData,
		UnknownTimeHint:  msgUnknownTime,
		Timetable:        timetableViews(form.values.Timetable, form.timetableErrors),
		DuplicateWarning: duplicateWarningOf(form.duplicates, form.values, h.clock),
	}
	switch {
	case form.errors[core.EventFieldTimetable] != "":
		page.TimetableError = form.errors[core.EventFieldTimetable]
	case len(form.timetableErrors) > 0:
		page.TimetableError = msgTimetableProblems
	}
	if form.id != "" {
		page.Heading = headingEditEvent
		page.Action = eventURL(form.id)
		page.Delete = &deleteForm{
			Action:  eventURL(form.id) + deletePathSuffix,
			Confirm: fmt.Sprintf(msgConfirmDeleteEvent, form.storedTitle),
		}
		page.ImportKey = importKeyFormOf(form.id, form.importKey)
	}
	for _, eventType := range core.ListEventTypes() {
		page.Types = append(page.Types, selectOption{
			Value:    string(eventType.Code),
			Label:    eventType.Label,
			Selected: string(eventType.Code) == form.values.Type,
		})
	}
	h.render(w, eventFormTemplate, status, page)
}

// importKeyFormOf returns the import key area of the event with id, nil
// when it has no import key.
func importKeyFormOf(id, importKey string) *importKeyForm {
	if importKey == "" {
		return nil
	}
	return &importKeyForm{
		Label:   msgImportKeyLabel,
		Key:     importKey,
		Hint:    msgImportKeyHint,
		Action:  eventURL(id) + importKeyRemovePathSuffix,
		Confirm: fmt.Sprintf(msgConfirmRemoveImportKey, importKey),
		Button:  msgImportKeyButton,
	}
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

func (h *handler) failEventRequest(w http.ResponseWriter, r *http.Request, err error) {
	h.failRequest(w, r, err, failedArea{logMsg: logMsgEventsFailed, backURL: eventsPath, backLabel: backToEventList})
}

// eventInputFromForm reads the event fields exactly as entered, the
// timetable without blank entries.
func eventInputFromForm(form url.Values) (core.EventInput, error) {
	timetable, err := timetableFromForm(form)
	if err != nil {
		return core.EventInput{}, err
	}
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
		Note:      form.Get(core.EventFieldNote),
		Timetable: timetable,
	}, nil
}

// eventTypeLabels maps each event type code to its German label from the
// core, which is the only source of the labels.
var eventTypeLabels = eventTypeLabelsByCode(core.ListEventTypes())

func eventTypeLabelsByCode(entries []core.EventTypeEntry) map[core.EventType]string {
	labels := make(map[core.EventType]string, len(entries))
	for _, entry := range entries {
		labels[entry.Code] = entry.Label
	}
	return labels
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
