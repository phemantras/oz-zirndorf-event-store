package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// German texts the delete tests expect, written out so a changed constant
// cannot pass unnoticed.
const (
	eventGoneText          = "Dieses Event ist nicht mehr vorhanden."
	locationGoneText       = "Dieser Ort ist nicht mehr vorhanden."
	locationInUseThreeText = "Dieser Ort kann nicht gelöscht werden, weil noch 3 Events auf ihn verweisen."
	locationInUseOneText   = "Dieser Ort kann nicht gelöscht werden, weil noch 1 Event auf ihn verweist."
	locationStillUsedText  = "Dieser Ort wird noch von Events verwendet und kann nicht gelöscht werden."
	deleteButton           = `<button type="submit">Löschen</button>`
)

func eventDeletePath(id string) string {
	return eventPath(id) + "/delete"
}

func locationDeletePath(id string) string {
	return locationPath(id) + "/delete"
}

// assertHTMXRedirect checks that an htmx request is sent to location with
// HX-Redirect instead of a redirect that htmx would follow into the page.
func assertHTMXRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()
	if got := rec.Header().Get(htmxRedirectHeader); got != location {
		t.Errorf("%s = %q, want %q", htmxRedirectHeader, got, location)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q, want none", got)
	}
	if rec.Code >= http.StatusMultipleChoices {
		t.Errorf("status = %d, want a success", rec.Code)
	}
}

// assertDeleteFormAfterEditForm checks that the delete form with its
// confirmation is a form of its own after the edit form and its buttons,
// so Enter in the edit form never deletes.
func assertDeleteFormAfterEditForm(t *testing.T, rec *httptest.ResponseRecorder, deleteForm string) {
	t.Helper()
	assertBodyContains(t, rec, deleteForm+"\n  "+deleteButton+"\n</form>")
	body := rec.Body.String()
	editFormEnd := strings.Index(body, "</form>")
	saveButton := strings.Index(body, "Speichern</button>")
	if deleteAt := strings.Index(body, deleteButton); deleteAt < editFormEnd || deleteAt < saveButton {
		t.Errorf("delete button at %d, edit form ends at %d, save button at %d; want it after both", deleteAt, editFormEnd, saveButton)
	}
}

// assertDeleteLogged checks for one log line with message and the id, so
// an accidental delete can be traced.
func assertDeleteLogged(t *testing.T, ts *testServer, message, id string) {
	t.Helper()
	want := `"msg":"` + message + `","id":"` + id + `"`
	if count := strings.Count(ts.logs.String(), want); count != 1 {
		t.Errorf("log has %d lines with %s, want 1; log: %s", count, want, ts.logs.String())
	}
}

func TestDeleteEventRemovesItWithItsTimetableAndRedirectsToList(t *testing.T) {
	forms := map[string]func(string) url.Values{
		"active with timetable": func(locationID string) url.Values {
			return festForm(locationID, [4]string{"Bieranstich", "2026-10-16", "18:00", ""})
		},
		"archived": archivedMarketForm,
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			hall := ts.seed(t, hallName)
			event := ts.seedEvent(t, form(hall.ID))

			rec := ts.post(eventDeletePath(event.ID), url.Values{})

			assertRedirect(t, rec, eventsPath)
			if len(ts.events.events) != 0 {
				t.Errorf("events = %v, want none", ts.events.events)
			}
			assertDeleteLogged(t, ts, logMsgEventDeleted, event.ID)
		})
	}
}

func TestDeleteEventByHTMXAnswersWithHXRedirectToList(t *testing.T) {
	ts := newTestServer(t)
	event := ts.seedEvent(t, marketForm(ts.seed(t, hallName).ID))

	rec := ts.htmxPost(eventDeletePath(strings.ToUpper(event.ID)), url.Values{})

	assertHTMXRedirect(t, rec, eventsPath)
	if len(ts.events.events) != 0 {
		t.Errorf("events = %v, want none", ts.events.events)
	}
}

func TestDeletingAGoneEventShowsItIsNoLongerAvailable(t *testing.T) {
	ts := newTestServer(t)
	event := ts.seedEvent(t, marketForm(ts.seed(t, hallName).ID))
	assertRedirect(t, ts.post(eventDeletePath(event.ID), url.Values{}), eventsPath)

	responses := map[string]*httptest.ResponseRecorder{
		"second tab":   ts.post(eventDeletePath(event.ID), url.Values{}),
		"double click": ts.htmxPost(eventDeletePath(event.ID), url.Values{}),
		"malformed id": ts.post(eventDeletePath("kaputt"), url.Values{}),
	}
	for name, rec := range responses {
		t.Run(name, func(t *testing.T) {
			assertStatusCode(t, rec, http.StatusNotFound)
			assertBodyContains(t, rec, "<h1>"+eventGoneText+"</h1>", `href="`+eventsPath+`">Zurück zur Event-Liste`)
		})
	}
}

func TestDeleteEventClearsTheReviewMark(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	concert := marketForm(hall.ID)
	concert.Set(core.EventFieldTitle, "Konzert")
	ts.seedEvent(t, concert)
	broken := ts.events.events[market.ID]
	broken.Times.AllDay = true
	ts.events.events[market.ID] = broken
	if _, err := ts.eventService.RecomputeDerived(context.Background()); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}
	assertBodyContains(t, ts.get(eventsPath), "prüfen")

	assertRedirect(t, ts.post(eventDeletePath(market.ID), url.Values{}), eventsPath)

	assertBodyLacks(t, ts.get(eventsPath), "prüfen")
}

func TestEditEventFormOffersDeleteWithConfirmation(t *testing.T) {
	ts := newTestServer(t)
	event := ts.seedEvent(t, marketForm(ts.seed(t, hallName).ID))

	rec := ts.get(eventPath(event.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertDeleteFormAfterEditForm(t, rec, `<form method="post" action="`+eventDeletePath(event.ID)+`" hx-post="`+eventDeletePath(event.ID)+`"`+
		` hx-confirm="Event „`+marketTitle+`“ wirklich löschen? Der Ablaufplan wird mit gelöscht." hx-target="body">`)
}

func TestNewEventFormOffersNoDelete(t *testing.T) {
	ts := newTestServer(t)
	ts.seed(t, hallName)

	rec := ts.get(newEventPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyLacks(t, rec, "/delete", deleteButton, "hx-confirm")
}

func TestDeleteLocationRemovesAnUnusedLocationAndRedirectsToList(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	park := ts.seed(t, "Bibertpark")

	assertRedirect(t, ts.post(locationDeletePath(hall.ID), url.Values{}), locationsPath)
	assertHTMXRedirect(t, ts.htmxPost(locationDeletePath(strings.ToUpper(park.ID)), hallForm()), locationsPath)

	if len(ts.locations.locations) != 0 {
		t.Errorf("locations = %v, want none", ts.locations.locations)
	}
	assertDeleteLogged(t, ts, logMsgLocationDeleted, hall.ID)
	assertDeleteLogged(t, ts, logMsgLocationDeleted, strings.ToUpper(park.ID))
}

func TestDeleteLocationInUseNamesTheNumberOfEventsAndKeepsTheLocation(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	first := archivedMarketForm(hall.ID)
	ts.seedEvent(t, first)
	second := archivedMarketForm(hall.ID)
	second.Set(core.EventFieldTitle, "Flohmarkt")
	ts.seedEvent(t, second)

	rec := ts.post(locationDeletePath(hall.ID), url.Values{})

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec,
		`<p class="error" role="alert">`+locationInUseThreeText+`</p>`,
		`action="`+locationPath(hall.ID)+`"`,
		`value="`+hallName+`"`, `value="`+hallStreet+`"`, `value="49.4424"`,
		deleteButton,
	)
	if _, ok := ts.locations.locations[hall.ID]; !ok {
		t.Error("a location in use was deleted")
	}
}

func TestDeleteLocationUsedByOneEventSaysEventInSingular(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, archivedMarketForm(hall.ID))

	rec := ts.post(locationDeletePath(hall.ID), url.Values{})

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, locationInUseOneText)
}

func TestDeleteLocationInUseByHTMXKeepsTheUnsavedInput(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	edited := hallForm()
	edited.Set(core.LocationFieldName, "Paul-Metz-Halle (umbenannt)")
	edited.Set(core.LocationFieldLatitude, "49,5")

	rec := ts.htmxPost(locationDeletePath(hall.ID), edited)

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, locationInUseOneText, `value="Paul-Metz-Halle (umbenannt)"`, `value="49,5"`, locationConfirm(hallName))
}

func TestDeleteLocationRefusedByTheDatabaseSaysItIsStillUsed(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.locations.deleteErr = core.ErrConflict

	rec := ts.post(locationDeletePath(hall.ID), url.Values{})

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, `<p class="error" role="alert">`+locationStillUsedText+`</p>`, `value="`+hallName+`"`)
}

func TestDeletingAGoneLocationShowsItIsNoLongerAvailable(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	assertRedirect(t, ts.post(locationDeletePath(hall.ID), url.Values{}), locationsPath)

	responses := map[string]*httptest.ResponseRecorder{
		"second tab":   ts.post(locationDeletePath(hall.ID), url.Values{}),
		"double click": ts.htmxPost(locationDeletePath(hall.ID), hallForm()),
		"malformed id": ts.post(locationDeletePath("kaputt"), url.Values{}),
	}
	for name, rec := range responses {
		t.Run(name, func(t *testing.T) {
			assertStatusCode(t, rec, http.StatusNotFound)
			assertBodyContains(t, rec, "<h1>"+locationGoneText+"</h1>", `href="`+locationsPath+`">Zurück zur Ortsliste`)
		})
	}
}

func TestEditLocationFormOffersDeleteWithConfirmation(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)

	rec := ts.get(locationPath(hall.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, `<form id="`+locationFormID+`" method="post" action="`+locationPath(hall.ID)+`" novalidate>`)
	assertDeleteFormAfterEditForm(t, rec, `<form method="post" action="`+locationDeletePath(hall.ID)+`" hx-post="`+locationDeletePath(hall.ID)+`"`+
		` hx-include="#`+locationFormID+`" hx-confirm="Ort „`+hallName+`“ wirklich löschen?" hx-target="body">`)
}

func TestNewLocationFormOffersNoDelete(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newLocationPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyLacks(t, rec, "/delete", deleteButton, "hx-confirm")
}

// lookupFailingLocations reports a location in use but cannot load it
// again for the form.
type lookupFailingLocations struct {
	LocationUseCases
	getErr error
}

func (f lookupFailingLocations) DeleteLocation(context.Context, string) error {
	return &core.LocationInUseError{EventCount: 1}
}

func (f lookupFailingLocations) GetLocation(context.Context, string) (core.Location, error) {
	return core.Location{}, f.getErr
}

func TestDeleteLocationInUseThatCannotBeLoadedAgain(t *testing.T) {
	t.Run("gone meanwhile", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.locations = lookupFailingLocations{getErr: core.ErrNotFound}

		rec := ts.post(locationDeletePath(unknownLocationID), url.Values{})

		assertStatusCode(t, rec, http.StatusNotFound)
		assertBodyContains(t, rec, locationGoneText)
	})
	t.Run("storage down", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.locations = lookupFailingLocations{getErr: errStorageDown}

		rec := ts.post(locationDeletePath(unknownLocationID), url.Values{})

		assertStatusCode(t, rec, http.StatusInternalServerError)
		if !strings.Contains(ts.logs.String(), logMsgLocationsFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
			t.Errorf("log %q does not record the failure", ts.logs.String())
		}
	})
}

func TestDeleteFailuresAnswerServerErrorAndLog(t *testing.T) {
	t.Run("event", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.events = failingEvents{err: errStorageDown}

		assertServerErrorLogged(t, ts, ts.post(eventDeletePath(unknownEventID), url.Values{}))
	})
	t.Run("location", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.locations = failingLocations{err: errStorageDown}

		rec := ts.post(locationDeletePath(unknownLocationID), url.Values{})

		assertStatusCode(t, rec, http.StatusInternalServerError)
		if !strings.Contains(ts.logs.String(), logMsgLocationsFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
			t.Errorf("log %q does not record the failure", ts.logs.String())
		}
	})
}

func TestOversizedLocationDeleteFormIsRejected(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := hallForm()
	form.Set(core.LocationFieldNote, strings.Repeat("x", maxLocationFormBytes))

	rec := ts.post(locationDeletePath(hall.ID), form)

	assertStatusCode(t, rec, http.StatusBadRequest)
	if _, ok := ts.locations.locations[hall.ID]; !ok {
		t.Error("an oversized request deleted the location")
	}
}

func TestDeleteRoutesRequireSessionAndSameOrigin(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	event := ts.seedEvent(t, marketForm(hall.ID))
	unused := ts.seed(t, "Bibertpark")
	paths := []string{eventDeletePath(event.ID), locationDeletePath(unused.ID)}

	for _, path := range paths {
		assertRedirect(t, ts.do(httptest.NewRequest(http.MethodPost, path, nil)), loginPath)

		crossSite := httptest.NewRequest(http.MethodPost, path, nil)
		crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
		assertStatusCode(t, ts.do(withCookie(crossSite, ts.validCookie())), http.StatusForbidden)

		if get := ts.get(path); get.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want %d", path, get.Code, http.StatusMethodNotAllowed)
		}
	}
	if len(ts.events.events) != 1 || len(ts.locations.locations) != 2 {
		t.Errorf("events = %d, locations = %d, want nothing deleted", len(ts.events.events), len(ts.locations.locations))
	}
}

// eventConfirm is the delete question of the event titled title.
func eventConfirm(title string) string {
	return `hx-confirm="Event „` + title + `“ wirklich löschen? Der Ablaufplan wird mit gelöscht."`
}

// locationConfirm is the delete question of the location named name.
func locationConfirm(name string) string {
	return `hx-confirm="Ort „` + name + `“ wirklich löschen?"`
}

func TestDeleteQuestionNamesTheStoredEventAfterAnError(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	concertForm := marketForm(hall.ID)
	concertForm.Set(core.EventFieldTitle, "Konzert")
	concert := ts.seedEvent(t, concertForm)
	untitled := marketForm(hall.ID)
	untitled.Set(core.EventFieldTitle, "")

	responses := map[string]struct {
		rec    *httptest.ResponseRecorder
		status int
	}{
		"field problem": {ts.post(eventPath(concert.ID), untitled), http.StatusUnprocessableEntity},
		"duplicate":     {ts.post(eventPath(concert.ID), marketForm(hall.ID)), http.StatusConflict},
	}
	for name, response := range responses {
		t.Run(name, func(t *testing.T) {
			assertStatusCode(t, response.rec, response.status)
			assertBodyContains(t, response.rec, eventConfirm("Konzert"))
		})
	}
}

// storedEventFailingEvents refuses every save as invalid and cannot load
// the stored event again for the form.
type storedEventFailingEvents struct {
	EventUseCases
	getErr error
}

func (f storedEventFailingEvents) SaveEvent(context.Context, string, core.EventInput, core.DuplicatePolicy) (core.Event, error) {
	return core.Event{}, &core.ValidationError{Fields: []core.FieldError{{Field: core.EventFieldTitle, Problem: core.ProblemMissing}}}
}

func (f storedEventFailingEvents) GetEvent(context.Context, string) (core.Event, error) {
	return core.Event{}, f.getErr
}

func TestEventFormAfterAnErrorWhenTheStoredEventCannotBeLoaded(t *testing.T) {
	t.Run("gone meanwhile", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.events = storedEventFailingEvents{getErr: core.ErrNotFound}

		rec := ts.post(eventPath(unknownEventID), marketForm(unknownLocationID))

		assertStatusCode(t, rec, http.StatusNotFound)
		assertBodyContains(t, rec, msgEventNotFoundText)
	})
	t.Run("storage down", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.events = storedEventFailingEvents{getErr: errStorageDown}

		assertServerErrorLogged(t, ts, ts.post(eventPath(unknownEventID), marketForm(unknownLocationID)))
	})
}

func TestDeleteQuestionNamesTheStoredLocationAfterAnError(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seed(t, "Bibertpark")
	unnamed := hallForm()
	unnamed.Set(core.LocationFieldName, "")
	taken := hallForm()
	taken.Set(core.LocationFieldName, "Bibertpark")

	responses := map[string]struct {
		rec    *httptest.ResponseRecorder
		status int
	}{
		"field problem": {ts.post(locationPath(hall.ID), unnamed), http.StatusUnprocessableEntity},
		"name conflict": {ts.post(locationPath(hall.ID), taken), http.StatusConflict},
	}
	for name, response := range responses {
		t.Run(name, func(t *testing.T) {
			assertStatusCode(t, response.rec, response.status)
			assertBodyContains(t, response.rec, locationConfirm(hallName))
		})
	}
}

// storedLocationFailingLocations refuses every save as invalid and cannot
// load the stored location again for the form.
type storedLocationFailingLocations struct {
	LocationUseCases
	getErr error
}

func (f storedLocationFailingLocations) SaveLocation(context.Context, string, core.LocationInput) (core.Location, error) {
	return core.Location{}, &core.ValidationError{Fields: []core.FieldError{{Field: core.LocationFieldName, Problem: core.ProblemMissing}}}
}

func (f storedLocationFailingLocations) GetLocation(context.Context, string) (core.Location, error) {
	return core.Location{}, f.getErr
}

func TestLocationFormAfterAnErrorWhenTheStoredLocationCannotBeLoaded(t *testing.T) {
	t.Run("gone meanwhile", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.locations = storedLocationFailingLocations{getErr: core.ErrNotFound}

		rec := ts.post(locationPath(unknownLocationID), hallForm())

		assertStatusCode(t, rec, http.StatusNotFound)
		assertBodyContains(t, rec, msgLocationNotFound)
	})
	t.Run("storage down", func(t *testing.T) {
		ts := newTestServer(t)
		ts.handler.locations = storedLocationFailingLocations{getErr: errStorageDown}

		rec := ts.post(locationPath(unknownLocationID), hallForm())

		assertStatusCode(t, rec, http.StatusInternalServerError)
		if !strings.Contains(ts.logs.String(), logMsgLocationsFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
			t.Errorf("log %q does not record the failure", ts.logs.String())
		}
	})
}
