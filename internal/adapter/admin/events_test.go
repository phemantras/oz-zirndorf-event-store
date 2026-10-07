package admin

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// Now lets the test clock serve as core.Clock for the event list.
func (c *fakeClock) Now() time.Time { return c.current }

// memoryEventRepo is an in-memory core.EventRepo, so the admin tests run
// against the real core use cases and rules.
type memoryEventRepo struct {
	events  map[string]core.Event
	created int
	// writeErr is what create and update return instead of writing,
	// simulating the foreign key that refuses a location deleted meanwhile.
	writeErr error
}

func newMemoryEventRepo() *memoryEventRepo {
	return &memoryEventRepo{events: map[string]core.Event{}}
}

func (r *memoryEventRepo) List(context.Context) ([]core.Event, error) {
	all := make([]core.Event, 0, len(r.events))
	for _, event := range r.events {
		all = append(all, event)
	}
	return all, nil
}

// ListOverlapping is part of core.EventRepo; the admin never lists by
// period, so it answers like List.
func (r *memoryEventRepo) ListOverlapping(ctx context.Context, _ core.Overlap) ([]core.Event, error) {
	return r.List(ctx)
}

func (r *memoryEventRepo) Get(_ context.Context, id string) (core.Event, error) {
	event, ok := r.events[strings.ToLower(id)]
	if !ok {
		return core.Event{}, core.ErrNotFound
	}
	return event, nil
}

func (r *memoryEventRepo) Create(_ context.Context, event core.Event) (core.Event, error) {
	if r.writeErr != nil {
		return core.Event{}, r.writeErr
	}
	r.created++
	event.ID = fmt.Sprintf("0192f0b1-0000-7000-9000-%012d", r.created)
	r.events[event.ID] = event
	return event, nil
}

// Update keeps the stored import key, like the database does.
func (r *memoryEventRepo) Update(_ context.Context, event core.Event) (core.Event, error) {
	if r.writeErr != nil {
		return core.Event{}, r.writeErr
	}
	event.ImportKey = r.events[event.ID].ImportKey
	r.events[event.ID] = event
	return event, nil
}

func (r *memoryEventRepo) FindByDuplicateKey(_ context.Context, key core.DuplicateKey) ([]core.Event, error) {
	var matches []core.Event
	for _, event := range r.events {
		if event.TitleKey == key.TitleKey && event.Times.StartDate == key.StartDate && event.LocationID == key.LocationID {
			matches = append(matches, event)
		}
	}
	return matches, nil
}

func (r *memoryEventRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.events[id]; !ok {
		return core.ErrNotFound
	}
	delete(r.events, id)
	return nil
}

func (r *memoryEventRepo) CountByLocation(_ context.Context, locationID string) (int, error) {
	count := 0
	for _, event := range r.events {
		if event.LocationID == locationID {
			count++
		}
	}
	return count, nil
}

func (r *memoryEventRepo) UpdateDerived(_ context.Context, id string, derived core.Derived) error {
	event := r.events[id]
	event.Period, event.TitleKey = derived.Period, derived.TitleKey
	r.events[id] = event
	return nil
}

func (r *memoryEventRepo) SetImportKey(_ context.Context, id, importKey string) error {
	event, ok := r.events[id]
	if !ok {
		return core.ErrNotFound
	}
	event.ImportKey = importKey
	r.events[id] = event
	return nil
}

// MarkArchived is part of core.EventRepo; the admin never marks events
// archived, so nothing is marked.
func (r *memoryEventRepo) MarkArchived(context.Context, time.Time) (int, error) {
	return 0, nil
}

// failingEvents answers every use case with err.
type failingEvents struct{ err error }

func (f failingEvents) SaveEvent(context.Context, string, core.EventInput, core.DuplicatePolicy) (core.Event, error) {
	return core.Event{}, f.err
}

func (f failingEvents) GetEvent(context.Context, string) (core.Event, error) {
	return core.Event{}, f.err
}

func (f failingEvents) ListEvents(context.Context, core.Clock) ([]core.EventListEntry, error) {
	return nil, f.err
}

func (f failingEvents) DeleteEvent(context.Context, string) error {
	return f.err
}

func (f failingEvents) RemoveImportKey(context.Context, string) error {
	return f.err
}

const (
	marketTitle = "Kirchweihmarkt"
	// unknownEventID is a well-formed id that no test stores.
	unknownEventID = "0192f0b1-0000-7000-9000-0000000000ff"
	// msgEventNotFoundText is the heading of the event 404 page.
	msgEventNotFoundText = "Event nicht gefunden."
)

// marketForm is a complete, valid event form at location locationID; the
// test clock stands on 2 October 2026, so the market is active.
func marketForm(locationID string) url.Values {
	return url.Values{
		core.EventFieldTitle:             {marketTitle},
		core.EventFieldType:              {string(core.EventTypeMarket)},
		core.EventFieldLocationID:        {locationID},
		core.EventFieldStartDate:         {"2026-10-16"},
		core.EventFieldStartTime:         {"19:00"},
		core.EventFieldEndDate:           {"2026-10-19"},
		core.EventFieldEndTime:           {""},
		core.EventFieldSourceDescription: {"Amtsblatt 41/2026"},
		core.EventFieldSourceURL:         {"https://www.zirndorf.de/amtsblatt"},
		core.EventFieldNote:              {"Mit Fahrgeschäften"},
	}
}

func eventPath(id string) string {
	return eventsPath + "/" + id
}

// seedEvent stores an event through the core use case and returns it.
func (ts *testServer) seedEvent(t *testing.T, form url.Values) core.Event {
	t.Helper()
	before := ts.events.created
	rec := ts.post(eventsPath, form)
	assertRedirect(t, rec, eventsPath)
	if ts.events.created != before+1 {
		t.Fatal("seeded event not stored")
	}
	return ts.events.events[fmt.Sprintf("0192f0b1-0000-7000-9000-%012d", ts.events.created)]
}

func TestCreateEventStoresItWithPeriodAndRedirectsToList(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)

	event := ts.seedEvent(t, marketForm(hall.ID))

	if event.Title != marketTitle || event.LocationID != hall.ID || event.Source.Description != "Amtsblatt 41/2026" {
		t.Errorf("stored = %+v, want the market at the hall", event)
	}
	wantStart := time.Date(2026, time.October, 16, 17, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.October, 19, 22, 0, 0, 0, time.UTC)
	if !event.Period.Start.Equal(wantStart) || !event.Period.End.Equal(wantEnd) {
		t.Errorf("period = [%v, %v), want [%v, %v)", event.Period.Start, event.Period.End, wantStart, wantEnd)
	}
}

func TestEventListShowsEveryColumnAndStatus(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	past := marketForm(hall.ID)
	past.Set(core.EventFieldTitle, "Flohmarkt")
	past.Set(core.EventFieldType, string(core.EventTypeOther))
	past.Set(core.EventFieldStartDate, "2026-09-12")
	past.Set(core.EventFieldStartTime, "")
	past.Set(core.EventFieldEndDate, "")
	flea := ts.seedEvent(t, past)
	allDay := marketForm(hall.ID)
	allDay.Set(core.EventFieldTitle, "Bürgerfest")
	allDay.Set(core.EventFieldType, string(core.EventTypeFestival))
	allDay.Set(core.EventFieldStartTime, "")
	allDay.Set(core.EventFieldAllDay, allDayChecked)
	festival := ts.seedEvent(t, allDay)

	rec := ts.get(eventsPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, `href="`+newEventPath+`"`)
	assertBodyLacks(t, rec, "prüfen")
	rows := map[string][]string{
		market.ID: {`<a href="` + eventPath(market.ID) + `">` + marketTitle + `</a>`,
			"Markt", hallName, "16.10.2026 19:00", "19.10.2026", statusActive},
		flea.ID: {`<a href="` + eventPath(flea.ID) + `">Flohmarkt</a>`,
			"Sonstiges", hallName, "12.09.2026", "", statusArchived},
		festival.ID: {`<a href="` + eventPath(festival.ID) + `">Bürgerfest</a>`,
			"Fest/Kirchweih", hallName, "16.10.2026 (ganztägig)", "19.10.2026 (ganztägig)", statusActive},
	}
	for id, want := range rows {
		if got := tableRowCells(rec, eventPath(id)); !slices.Equal(got, want) {
			t.Errorf("row of %s = %q, want %q", id, got, want)
		}
	}
	body := html.UnescapeString(rec.Body.String())
	if active, archived := strings.Index(body, marketTitle), strings.Index(body, "Flohmarkt"); active < 0 || active > archived {
		t.Errorf("positions = %d, %d, want the active market before the archived flea market", active, archived)
	}
}

// tableRowCells returns the cell contents of the table row that links to
// path, as the browser shows them.
func tableRowCells(rec *httptest.ResponseRecorder, path string) []string {
	for _, row := range strings.Split(html.UnescapeString(rec.Body.String()), "<tr>") {
		row, _, found := strings.Cut(row, "</tr>")
		if !found || !strings.Contains(row, `href="`+path+`"`) {
			continue
		}
		var cells []string
		for _, cell := range strings.Split(row, "<td>")[1:] {
			content, _, _ := strings.Cut(cell, "</td>")
			cells = append(cells, content)
		}
		return cells
	}
	return nil
}

func TestEventListWithoutEventsLinksNewForm(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(eventsPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "Noch keine Events angelegt.", `href="`+newEventPath+`"`)
}

func TestEventListMarksEventsWhoseRecomputationFailed(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	broken := ts.events.events[market.ID]
	broken.Times.AllDay = true
	ts.events.events[market.ID] = broken
	if _, err := ts.eventService.RecomputeDerived(context.Background()); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}

	assertBodyContains(t, ts.get(eventsPath), "prüfen")

	assertRedirect(t, ts.post(eventPath(market.ID), marketForm(hall.ID)), eventsPath)
	assertBodyLacks(t, ts.get(eventsPath), "prüfen")
}

func TestCorrectingArchivedEventIntoTheFutureShowsItAsActive(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := marketForm(hall.ID)
	form.Set(core.EventFieldStartDate, "2025-10-17")
	form.Set(core.EventFieldEndDate, "2025-10-20")
	market := ts.seedEvent(t, form)
	assertBodyContains(t, ts.get(eventsPath), "archiviert")

	rec := ts.post(eventPath(market.ID), marketForm(hall.ID))

	assertRedirect(t, rec, eventsPath)
	list := ts.get(eventsPath)
	assertBodyContains(t, list, "aktiv")
	assertBodyLacks(t, list, "archiviert")
}

func TestNewEventFormOffersTypesLocationsAndHints(t *testing.T) {
	ts := newTestServer(t)
	ts.seed(t, "Zirndorfer Ölmühle")
	park := ts.seed(t, "Bibertpark")

	rec := ts.get(newEventPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`action="`+eventsPath+`"`,
		`<option value="festival">Fest/Kirchweih</option>`,
		`<option value="market">Markt</option>`,
		`<option value="culture">Kultur/Bühne</option>`,
		`<option value="politics">Politik/Sitzung</option>`,
		`<option value="club">Verein/Treff</option>`,
		`<option value="sports">Sport</option>`,
		`<option value="other">Sonstiges</option>`,
		`<option value="`+park.ID+`">Bibertpark</option>`,
		`name="startDate" type="date"`, `name="endDate" type="date"`,
		`name="startTime" type="time"`, `name="endTime" type="time"`,
		`name="allDay" type="checkbox" value="true"`,
		`name="source.description"`, `name="source.url"`, `name="note"`,
	)
	body := html.UnescapeString(rec.Body.String())
	if got := strings.Count(body, msgNoPersonalData); got != 3 {
		t.Errorf("privacy hint appears %d times, want 3 (title, source and note)", got)
	}
	if got := strings.Count(body, msgUnknownTime); got != 2 {
		t.Errorf("unknown time hint appears %d times, want 2 (start and end time)", got)
	}
	if got := strings.Count(body, `<option value="`); got != len(core.EventTypes())+2+2 {
		t.Errorf("form has %d options, want seven types, two locations and two prompts", got)
	}
	if park, mill := strings.Index(body, "Bibertpark"), strings.Index(body, "Zirndorfer Ölmühle"); park < 0 || park > mill {
		t.Errorf("positions = %d, %d, want locations in core order", park, mill)
	}
	assertBodyLacks(t, rec, msgNoLocations)
}

func TestNewEventFormWithoutLocationsLinksNewLocation(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newEventPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, msgNoLocations, `href="`+newLocationPath+`"`)
}

func TestCreateEventWithMissingFieldsShowsGermanMessagesAndKeepsInput(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := marketForm(hall.ID)
	form.Set(core.EventFieldTitle, "  ")
	form.Set(core.EventFieldType, "")
	form.Set(core.EventFieldLocationID, "")
	form.Set(core.EventFieldStartDate, "")
	form.Set(core.EventFieldSourceDescription, " ")

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec,
		"Bitte einen Titel angeben.", "Bitte einen Event-Typ auswählen.", "Bitte einen Ort auswählen.",
		"Bitte ein Beginn-Datum angeben.", "Bitte eine Quelle angeben.",
		`name="title" type="text" value="  "`, `value="19:00"`, `value="2026-10-19"`,
		`value="https://www.zirndorf.de/amtsblatt"`, "Mit Fahrgeschäften",
	)
	if len(ts.events.events) != 0 {
		t.Error("an invalid event was stored")
	}
}

func TestCreateEventShowsFieldMessagesForInvalidInput(t *testing.T) {
	tests := map[string]struct {
		change   func(form url.Values)
		messages []string
	}{
		"unknown type": {
			func(form url.Values) { form.Set(core.EventFieldType, "concert") },
			[]string{"Bitte einen der angebotenen Event-Typen auswählen."},
		},
		"unknown location": {
			func(form url.Values) { form.Set(core.EventFieldLocationID, unknownLocationID) },
			[]string{"Den gewählten Ort gibt es nicht mehr. Bitte einen anderen Ort auswählen."},
		},
		"ftp link": {
			func(form url.Values) { form.Set(core.EventFieldSourceURL, "ftp://x") },
			[]string{msgSourceURLFormat},
		},
		"link without scheme": {
			func(form url.Values) { form.Set(core.EventFieldSourceURL, "amtsblatt.de") },
			[]string{msgSourceURLFormat},
		},
		"German date and spoken time": {
			func(form url.Values) {
				form.Set(core.EventFieldStartDate, "16.10.2026")
				form.Set(core.EventFieldStartTime, "7 Uhr")
			},
			[]string{
				"Das Beginn-Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
				"Die Beginn-Uhrzeit ist keine gültige Uhrzeit (Format HH:MM).",
			},
		},
		"malformed end": {
			func(form url.Values) {
				form.Set(core.EventFieldEndDate, "19.10.")
				form.Set(core.EventFieldEndTime, "spät")
			},
			[]string{
				"Das End-Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
				"Die End-Uhrzeit ist keine gültige Uhrzeit (Format HH:MM).",
			},
		},
		"end before start": {
			func(form url.Values) {
				form.Set(core.EventFieldEndDate, "2026-10-16")
				form.Set(core.EventFieldEndTime, "18:00")
			},
			[]string{msgEndNotAfterStart},
		},
		"end date before start date": {
			func(form url.Values) { form.Set(core.EventFieldEndDate, "2026-10-15") },
			[]string{msgEndNotAfterStart},
		},
		"all day with times": {
			func(form url.Values) {
				form.Set(core.EventFieldAllDay, allDayChecked)
				form.Set(core.EventFieldEndTime, "22:00")
			},
			[]string{"Ein ganztägiges Event hat keine Beginn-Uhrzeit.", "Ein ganztägiges Event hat keine End-Uhrzeit."},
		},
		"end time without end date": {
			func(form url.Values) {
				form.Set(core.EventFieldEndDate, "")
				form.Set(core.EventFieldEndTime, "22:00")
			},
			[]string{"Bitte zur End-Uhrzeit auch ein End-Datum angeben."},
		},
		"spring gap": {
			func(form url.Values) {
				form.Set(core.EventFieldStartDate, "2027-03-28")
				form.Set(core.EventFieldStartTime, "02:30")
				form.Set(core.EventFieldEndDate, "2027-03-28")
				form.Set(core.EventFieldEndTime, "02:45")
			},
			[]string{msgNonexistentTime},
		},
		"end in the repeated hour": {
			func(form url.Values) {
				form.Set(core.EventFieldStartDate, "2026-10-25")
				form.Set(core.EventFieldStartTime, "02:45")
				form.Set(core.EventFieldEndDate, "2026-10-25")
				form.Set(core.EventFieldEndTime, "02:15")
			},
			[]string{"Das Ende muss nach dem Beginn liegen. In der doppelten Stunde der Zeitumstellung gilt die erste, die Sommerzeit."},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			hall := ts.seed(t, hallName)
			form := marketForm(hall.ID)
			tt.change(form)

			rec := ts.post(eventsPath, form)

			assertStatusCode(t, rec, http.StatusUnprocessableEntity)
			assertBodyContains(t, rec, tt.messages...)
			if len(ts.events.events) != 0 {
				t.Error("an invalid event was stored")
			}
		})
	}
}

func TestEventFormKeepsSelectionsAndAllDayAfterError(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := marketForm(hall.ID)
	form.Set(core.EventFieldTitle, "")
	form.Set(core.EventFieldType, string(core.EventTypeSports))
	form.Set(core.EventFieldAllDay, allDayChecked)

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec,
		`<option value="sports" selected>Sport</option>`,
		`<option value="`+hall.ID+`" selected>`+hallName+`</option>`,
		`value="true" checked`,
	)
}

func TestEditEventFormShowsStoredValues(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))

	rec := ts.get(eventPath(strings.ToUpper(market.ID)))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"Event bearbeiten", `action="`+eventPath(market.ID)+`"`, `value="`+marketTitle+`"`,
		`<option value="market" selected>Markt</option>`, `<option value="`+hall.ID+`" selected>`+hallName+`</option>`,
		`name="startDate" type="date" value="2026-10-16"`, `name="startTime" type="time" value="19:00"`,
		`name="endDate" type="date" value="2026-10-19"`, `name="endTime" type="time" value=""`,
		`value="Amtsblatt 41/2026"`, `value="https://www.zirndorf.de/amtsblatt"`, "Mit Fahrgeschäften",
	)
	assertBodyLacks(t, rec, "checked")
}

func TestEditingEventKeepsID(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	form := marketForm(hall.ID)
	form.Set(core.EventFieldTitle, "Kirchweih")

	rec := ts.post(eventPath(market.ID), form)

	assertRedirect(t, rec, eventsPath)
	if stored := ts.events.events[market.ID]; stored.Title != "Kirchweih" || len(ts.events.events) != 1 {
		t.Errorf("stored = %+v of %d, want the new title under id %s", stored, len(ts.events.events), market.ID)
	}
}

func TestUnknownOrMalformedEventIDShowsGermanNotFoundPage(t *testing.T) {
	for _, id := range []string{unknownEventID, "kaputt"} {
		t.Run(id, func(t *testing.T) {
			ts := newTestServer(t)
			hall := ts.seed(t, hallName)

			for _, rec := range []*httptest.ResponseRecorder{ts.get(eventPath(id)), ts.post(eventPath(id), marketForm(hall.ID))} {
				assertStatusCode(t, rec, http.StatusNotFound)
				assertBodyContains(t, rec, msgEventNotFoundText, `href="`+eventsPath+`">Zurück zur Event-Liste`)
			}
			if len(ts.events.events) != 0 {
				t.Error("an event was stored for an unknown id")
			}
		})
	}
}

func TestEventPagesRequireSession(t *testing.T) {
	ts := newTestServer(t)
	form := marketForm(unknownLocationID).Encode()
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, eventsPath, nil),
		httptest.NewRequest(http.MethodGet, newEventPath, nil),
		httptest.NewRequest(http.MethodGet, eventPath(unknownEventID), nil),
		httptest.NewRequest(http.MethodPost, eventsPath, strings.NewReader(form)),
		httptest.NewRequest(http.MethodPost, eventPath(unknownEventID), strings.NewReader(form)),
	}
	for _, req := range requests {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		assertRedirect(t, ts.do(req), loginPath)
	}
	if len(ts.events.events) != 0 {
		t.Error("an event was stored without session")
	}
}

func TestEventPagesAnswerFailuresWithServerErrorAndLog(t *testing.T) {
	form := marketForm(unknownLocationID)
	failingEventsRequests := map[string]func(*testServer) *httptest.ResponseRecorder{
		"list":   func(ts *testServer) *httptest.ResponseRecorder { return ts.get(eventsPath) },
		"edit":   func(ts *testServer) *httptest.ResponseRecorder { return ts.get(eventPath(unknownEventID)) },
		"create": func(ts *testServer) *httptest.ResponseRecorder { return ts.post(eventsPath, form) },
		"update": func(ts *testServer) *httptest.ResponseRecorder { return ts.post(eventPath(unknownEventID), form) },
	}
	for name, request := range failingEventsRequests {
		t.Run("events "+name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.handler.events = failingEvents{err: errStorageDown}
			assertServerErrorLogged(t, ts, request(ts))
		})
	}

	locationOptionsRequests := map[string]func(*testServer, core.Event) *httptest.ResponseRecorder{
		"new form":  func(ts *testServer, _ core.Event) *httptest.ResponseRecorder { return ts.get(newEventPath) },
		"edit form": func(ts *testServer, e core.Event) *httptest.ResponseRecorder { return ts.get(eventPath(e.ID)) },
		"form on error": func(ts *testServer, _ core.Event) *httptest.ResponseRecorder {
			return ts.post(eventsPath, url.Values{})
		},
	}
	for name, request := range locationOptionsRequests {
		t.Run("locations for "+name, func(t *testing.T) {
			ts := newTestServer(t)
			hall := ts.seed(t, hallName)
			market := ts.seedEvent(t, marketForm(hall.ID))
			ts.handler.locations = failingLocations{err: errStorageDown}
			assertServerErrorLogged(t, ts, request(ts, market))
		})
	}
}

func TestEventPageAnswersAnExpiredDeadlineWithServerErrorAndLogsAWarning(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.events = failingEvents{err: errRequestTimedOut}

	rec := ts.get(eventsPath)

	assertStatusCode(t, rec, http.StatusInternalServerError)
	assertLoggedAsWarningOnly(t, ts, logMsgEventsFailed)
}

func assertServerErrorLogged(t *testing.T, ts *testServer, rec *httptest.ResponseRecorder) {
	t.Helper()
	assertStatusCode(t, rec, http.StatusInternalServerError)
	if !strings.Contains(ts.logs.String(), logMsgEventsFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
		t.Errorf("log %q does not record the failure", ts.logs.String())
	}
	assertLoggedAt(t, ts, slog.LevelError, logMsgEventsFailed)
}

func TestUnexpectedEventFieldProblemFallsBackToGenericMessage(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.events = failingEvents{err: &core.ValidationError{Fields: []core.FieldError{
		{Field: core.EventFieldNote, Problem: core.ProblemOutOfRange},
	}}}

	rec := ts.post(eventsPath, marketForm(unknownLocationID))

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, msgFieldInvalid)
}

func TestOversizedEventFormIsRejected(t *testing.T) {
	ts := newTestServer(t)
	form := marketForm(unknownLocationID)
	form.Set(core.EventFieldNote, strings.Repeat("x", maxEventFormBytes))

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusBadRequest)
	if len(ts.events.events) != 0 {
		t.Error("an oversized form was stored")
	}
}

func TestHomeLinksEventsAndLocations(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(homePath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, `href="`+eventsPath+`">Events</a>`, `href="`+locationsPath+`">Orte</a>`)
	assertBodyLacks(t, rec, "Events folgen.")
}

// TestLocationDeletedWhileSavingShowsTheLocationMessage covers the race in
// which the location disappears between the core's check and the write.
func TestLocationDeletedWhileSavingShowsTheLocationMessage(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	ts.events.writeErr = fmt.Errorf("write event: %w", core.ErrConflict)
	created := marketForm(hall.ID)
	created.Set(core.EventFieldTitle, "Konzert")
	edited := marketForm(hall.ID)
	edited.Set(core.EventFieldNote, "Neue Notiz")

	for path, form := range map[string]url.Values{eventsPath: created, eventPath(market.ID): edited} {
		t.Run(path, func(t *testing.T) {
			rec := ts.post(path, form)

			assertStatusCode(t, rec, http.StatusUnprocessableEntity)
			assertBodyContains(t, rec, "Den gewählten Ort gibt es nicht mehr. Bitte einen anderen Ort auswählen.", form.Get(core.EventFieldNote))
		})
	}
}

func TestEventTextsOverTheirLimitsShowTheLimitAndKeepInput(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := marketForm(hall.ID)
	form.Set(core.EventFieldTitle, strings.Repeat("T", core.MaxTitleLength+1))
	form.Set(core.EventFieldSourceDescription, strings.Repeat("Q", core.MaxSourceDescriptionLength+1))
	form.Set(core.EventFieldSourceURL, "https://zirndorf.de/"+strings.Repeat("u", core.MaxSourceURLLength))
	form.Set(core.EventFieldNote, strings.Repeat("N", core.MaxNoteLength+1))

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, "Höchstens 200 Zeichen.", "Höchstens 500 Zeichen.", "Höchstens 2000 Zeichen.",
		form.Get(core.EventFieldTitle), form.Get(core.EventFieldNote), form.Get(core.EventFieldSourceURL),
		form.Get(core.EventFieldSourceDescription))
	// source.url and note share the message; both must show it.
	if got := strings.Count(html.UnescapeString(rec.Body.String()), "Höchstens 2000 Zeichen."); got != 2 {
		t.Errorf("message for 2000 characters shown %d times, want 2 (source URL and note)", got)
	}
	if len(ts.events.events) != 0 {
		t.Error("an event over the limits was stored")
	}
}

func TestEventFormLimitIs256KiB(t *testing.T) {
	if maxEventFormBytes != 256*1024 {
		t.Errorf("maxEventFormBytes = %d, want 256 KiB", maxEventFormBytes)
	}
}
