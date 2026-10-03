package admin

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// memoryTx runs the transaction-bound part of a use case directly on the
// in-memory repositories.
type memoryTx struct{ repos core.Repos }

func (tx memoryTx) InTx(_ context.Context, fn func(core.Repos) error) error {
	return fn(tx.repos)
}

// festForm is the Kirchweih from 16 October 2026, 18:00, to the end of 17
// October with the given timetable entries, each as description, date,
// start and end time.
func festForm(locationID string, entries ...[4]string) url.Values {
	form := marketForm(locationID)
	form.Set(core.EventFieldStartTime, "18:00")
	form.Set(core.EventFieldEndDate, "2026-10-17")
	for _, entry := range entries {
		form.Add(timetableFormDescription, entry[0])
		form.Add(timetableFormDate, entry[1])
		form.Add(timetableFormStartTime, entry[2])
		form.Add(timetableFormEndTime, entry[3])
	}
	return form
}

var (
	disco       = [4]string{"Disco", "2026-10-16", "22:00", "01:00"}
	bieranstich = [4]string{"Bieranstich", "2026-10-16", "18:00", ""}
	markttag    = [4]string{"Markttag", "2026-10-17", "", ""}
	blankEntry  = [4]string{" ", "", "", " "}
)

func storedDescriptions(event core.Event) []string {
	var descriptions []string
	for _, entry := range event.Timetable {
		descriptions = append(descriptions, entry.Description)
	}
	return descriptions
}

func TestEventFormOffersAddingTimetableEntriesWithHtmx(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newEventPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"Ablaufplan (optional)", `<div id="timetable-entries">`,
		`hx-get="`+timetableEntryPath+`" hx-target="#timetable-entries" hx-swap="beforeend"`,
		"Programmpunkt hinzufügen",
	)
	assertBodyLacks(t, rec, `name="`+timetableFormDescription+`"`, msgTimetableProblems)
}

func TestTimetableEntryFragmentIsAnEmptyEntryWithRemoveButton(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.htmxGet(timetableEntryPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	assertBodyContains(t, rec,
		`<fieldset class="timetable-entry">`,
		`<label>Beschreibung`, `name="`+timetableFormDescription+`" type="text" value=""`,
		`<label>Datum`, `name="`+timetableFormDate+`" type="date" value=""`,
		`<label>Beginn-Uhrzeit (optional)`, `name="`+timetableFormStartTime+`" type="time" value=""`,
		`<label>End-Uhrzeit (optional)`, `name="`+timetableFormEndTime+`" type="time" value=""`,
		`hx-on:click="this.closest('fieldset').remove()"`, "Programmpunkt entfernen",
	)
	body := html.UnescapeString(rec.Body.String())
	for _, hint := range []string{msgUnknownTime, msgEntryEndsNextDay} {
		if !strings.Contains(body, hint) {
			t.Errorf("fragment lacks hint %q", hint)
		}
	}
	assertBodyLacks(t, rec, `id="`)
}

func TestTimetableEntryFragmentRequiresSession(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, timetableEntryPath, nil)
	req.Header.Set(htmxRequestHeader, htmxRequestTrue)

	rec := ts.do(req)

	assertStatusCode(t, rec, http.StatusUnauthorized)
	if got := rec.Header().Get(htmxRedirectHeader); got != loginPath {
		t.Errorf("HX-Redirect = %q, want %q", got, loginPath)
	}
}

func TestSavingEventStoresTimetableSortedWithoutBlankEntries(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)

	event := ts.seedEvent(t, festForm(hall.ID, disco, blankEntry, markttag, bieranstich))

	if got, want := storedDescriptions(event), []string{"Bieranstich", "Disco", "Markttag"}; !slices.Equal(got, want) {
		t.Errorf("stored timetable = %v, want %v", got, want)
	}
	if end := event.Timetable[1].EndTime; end == nil || *end != (core.LocalTime{Hour: 1}) {
		t.Errorf("disco end = %v, want 01:00", end)
	}
}

func TestEditingEventShowsStoredEntriesSorted(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	event := ts.seedEvent(t, festForm(hall.ID, markttag, disco, bieranstich))

	rec := ts.get(eventPath(event.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`value="Bieranstich"`, `value="Disco"`, `value="Markttag"`,
		`name="`+timetableFormStartTime+`" type="time" value="22:00"`,
		`name="`+timetableFormEndTime+`" type="time" value="01:00"`,
	)
	body := rec.Body.String()
	first, second, third := strings.Index(body, `value="Bieranstich"`), strings.Index(body, `value="Disco"`), strings.Index(body, `value="Markttag"`)
	if first > second || second > third {
		t.Errorf("positions = %d, %d, %d, want chronological order", first, second, third)
	}
}

func TestEditingEventWithFewerEntriesReplacesTheTimetable(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	event := ts.seedEvent(t, festForm(hall.ID, markttag, disco, bieranstich))

	rec := ts.post(eventPath(event.ID), festForm(hall.ID, disco))

	assertRedirect(t, rec, eventsPath)
	if got := storedDescriptions(ts.events.events[event.ID]); !slices.Equal(got, []string{"Disco"}) {
		t.Errorf("stored timetable = %v, want only Disco", got)
	}
}

func TestInvalidTimetableEntriesShowMessagesAtTheEntryAndKeepInput(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := festForm(hall.ID,
		bieranstich,
		blankEntry,
		[4]string{"", "2026-10-18", "", ""},
		[4]string{"Kehraus", "2026-10-18", "", ""},
		[4]string{"Gleich", "2026-10-16", "20:00", "20:00"},
	)

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	if len(ts.events.events) != 0 {
		t.Error("an event with an invalid timetable was stored")
	}
	entries := timetableEntriesOf(rec)
	if len(entries) != 4 {
		t.Fatalf("form shows %d entries, want the four non-blank ones", len(entries))
	}
	wantMessages := [][]string{
		nil,
		{"Bitte eine Beschreibung angeben."},
		{msgOutsideEvent},
		{msgEndNotAfterStart},
	}
	for i, want := range wantMessages {
		if got := fieldErrorsIn(entries[i]); !slices.Equal(got, want) {
			t.Errorf("entry %d messages = %q, want %q", i, got, want)
		}
	}
	if !strings.Contains(entries[2], `value="Kehraus"`) || !strings.Contains(entries[3], `value="20:00"`) {
		t.Error("entered values were lost")
	}
	assertBodyContains(t, rec, msgTimetableProblems)
}

func TestEventFieldErrorsAloneShowNoTimetableBanner(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := festForm(hall.ID, bieranstich)
	form.Set(core.EventFieldTitle, "")

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyLacks(t, rec, msgTimetableProblems)
}

func TestShorteningEventBelowItsEntriesNamesTheEntries(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	event := ts.seedEvent(t, festForm(hall.ID, bieranstich, disco, markttag))
	form := festForm(hall.ID, bieranstich, disco, markttag)
	form.Set(core.EventFieldEndDate, "2026-10-16")
	form.Set(core.EventFieldEndTime, "23:00")

	rec := ts.post(eventPath(event.ID), form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	entries := timetableEntriesOf(rec)
	for i, want := range [][]string{nil, {msgOutsideEvent}, {msgOutsideEvent}} {
		if got := fieldErrorsIn(entries[i]); !slices.Equal(got, want) {
			t.Errorf("entry %d messages = %q, want %q", i, got, want)
		}
	}
	if got := storedDescriptions(ts.events.events[event.ID]); len(got) != 3 {
		t.Errorf("stored timetable = %v, want the three original entries", got)
	}
}

func TestTimetableFieldMessagesCoverEveryProblemTheCoreReports(t *testing.T) {
	tests := map[string]struct {
		entry [4]string
		want  []string
	}{
		"missing date":       {[4]string{"Markt", "", "", ""}, []string{"Bitte ein Datum angeben."}},
		"malformed date":     {[4]string{"Markt", "16.10.2026", "", ""}, []string{"Das Datum ist kein gültiges Datum (Format JJJJ-MM-TT)."}},
		"malformed times":    {[4]string{"Markt", "2026-10-16", "7 Uhr", "spät"}, []string{"Die Beginn-Uhrzeit ist keine gültige Uhrzeit (Format HH:MM).", "Die End-Uhrzeit ist keine gültige Uhrzeit (Format HH:MM)."}},
		"start before event": {[4]string{"Aufbau", "2026-10-16", "17:00", ""}, []string{msgOutsideEvent}},
		"end after event":    {[4]string{"Disco", "2026-10-17", "22:00", "01:00"}, []string{msgOutsideEvent}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			hall := ts.seed(t, hallName)

			rec := ts.post(eventsPath, festForm(hall.ID, tt.entry))

			assertStatusCode(t, rec, http.StatusUnprocessableEntity)
			if got := fieldErrorsIn(timetableEntriesOf(rec)[0]); !slices.Equal(got, tt.want) {
				t.Errorf("messages = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTimetableTimesInTheSpringGapShowTheDaylightSavingMessage(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := marketForm(hall.ID)
	form.Set(core.EventFieldStartDate, "2026-03-28")
	form.Set(core.EventFieldStartTime, "")
	form.Set(core.EventFieldEndDate, "2026-03-29")
	for _, entry := range [][4]string{{"Nachtwanderung", "2026-03-29", "02:30", ""}, {"Disco", "2026-03-28", "23:00", "02:30"}} {
		form.Add(timetableFormDescription, entry[0])
		form.Add(timetableFormDate, entry[1])
		form.Add(timetableFormStartTime, entry[2])
		form.Add(timetableFormEndTime, entry[3])
	}

	rec := ts.post(eventsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	for i, entry := range timetableEntriesOf(rec) {
		if got := fieldErrorsIn(entry); !slices.Equal(got, []string{msgNonexistentTime}) {
			t.Errorf("entry %d messages = %q, want the daylight saving message", i, got)
		}
	}
}

func TestUnexpectedTimetableProblemFallsBackToGenericMessage(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.events = failingEvents{err: &core.ValidationError{Fields: []core.FieldError{
		{Field: core.TimetableField(0, core.TimetableFieldDescription), Problem: core.ProblemOutOfRange},
	}}}

	rec := ts.post(eventsPath, festForm(unknownLocationID, bieranstich))

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	if got := fieldErrorsIn(timetableEntriesOf(rec)[0]); !slices.Equal(got, []string{msgFieldInvalid}) {
		t.Errorf("messages = %q, want the generic message", got)
	}
}

func TestEventFormWithUnevenTimetableFieldsIsABadRequest(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	for _, field := range []string{timetableFormDescription, timetableFormDate, timetableFormStartTime, timetableFormEndTime} {
		form := festForm(hall.ID, bieranstich)
		form.Add(field, "zusätzlich")

		rec := ts.post(eventsPath, form)

		assertStatusCode(t, rec, http.StatusBadRequest)
	}
	if len(ts.events.events) != 0 {
		t.Error("an event with uneven timetable fields was stored")
	}
}

func TestEventFormAcceptsALargeTimetable(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := marketForm(hall.ID)
	form.Set(core.EventFieldNote, strings.Repeat("x", 4000))
	const entries = 60
	for range entries {
		form.Add(timetableFormDescription, strings.Repeat("Programmpunkt ", 10))
		form.Add(timetableFormDate, "2026-10-17")
		form.Add(timetableFormStartTime, "10:00")
		form.Add(timetableFormEndTime, "11:00")
	}

	event := ts.seedEvent(t, form)

	if len(event.Timetable) != entries {
		t.Errorf("stored %d entries, want %d", len(event.Timetable), entries)
	}
}

// timetableEntriesOf returns the markup of each timetable entry of a form
// response.
func timetableEntriesOf(rec *httptest.ResponseRecorder) []string {
	parts := strings.Split(html.UnescapeString(rec.Body.String()), `<fieldset class="timetable-entry">`)[1:]
	for i, part := range parts {
		parts[i], _, _ = strings.Cut(part, "</fieldset>")
	}
	return parts
}

// fieldErrorsIn returns the field messages within markup in order.
func fieldErrorsIn(markup string) []string {
	var messages []string
	for _, part := range strings.Split(markup, `<p class="field-error">`)[1:] {
		message, _, _ := strings.Cut(part, "</p>")
		messages = append(messages, message)
	}
	return messages
}
