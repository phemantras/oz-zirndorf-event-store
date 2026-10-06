package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// German texts the import key tests expect, written out so a changed
// constant cannot pass unnoticed.
const (
	importKeyLabelText  = "Import-Schlüssel"
	importKeyButtonText = `<button type="submit">Import-Schlüssel entfernen</button>`
)

func importKeyRemovePath(id string) string {
	return eventPath(id) + "/import-key/remove"
}

// importKeyConfirm is the question before removing key.
func importKeyConfirm(key string) string {
	return `hx-confirm="Import-Schlüssel „` + key + `“ wirklich entfernen? Ein späterer Import mit diesem Schlüssel aktualisiert dieses Event dann nicht mehr."`
}

// importKeyFormTag is the opening tag of the form that removes the import key
// of the event with id.
func importKeyFormTag(id string) string {
	path := importKeyRemovePath(id)
	return `<form method="post" action="` + path + `" hx-post="` + path + `" ` + importKeyConfirm(marketImportKey) + ` hx-target="body">`
}

func TestEditEventShowsTheImportKeyReadOnlyWithAButtonToRemoveIt(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.get(eventPath(market.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		importKeyLabelText+": <code>"+marketImportKey+"</code>",
		importKeyFormTag(market.ID)+"\n    "+importKeyButtonText+"\n  </form>",
	)
	assertBodyLacks(t, rec, `name="importKey"`)
	body := rec.Body.String()
	editFormEnd, removeAt, deleteAt := strings.Index(body, "</form>"), strings.Index(body, importKeyButtonText), strings.Index(body, deleteButton)
	if removeAt < editFormEnd || removeAt > deleteAt {
		t.Errorf("remove button at %d, edit form ends at %d, delete button at %d; want it between both", removeAt, editFormEnd, deleteAt)
	}
}

func TestEditEventWithoutImportKeyShowsNoImportKey(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedEvent(t, marketForm(ts.seed(t, hallName).ID))

	rec := ts.get(eventPath(market.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyLacks(t, rec, importKeyLabelText, "/import-key/remove")
}

func TestEventFormAfterARefusedSaveStillShowsTheImportKey(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)
	untitled := marketForm(market.LocationID)
	untitled.Set(core.EventFieldTitle, "")

	rec := ts.post(eventPath(market.ID), untitled)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, importKeyLabelText+": <code>"+marketImportKey+"</code>", importKeyFormTag(market.ID))
	if got := ts.events.events[market.ID].ImportKey; got != marketImportKey {
		t.Errorf("import key = %q, want it kept", got)
	}
}

func TestEventFormOfASuspectedDuplicateStillShowsTheImportKey(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)
	concert := marketForm(market.LocationID)
	concert.Set(core.EventFieldTitle, "Konzert")
	ts.seedEvent(t, concert)

	rec := ts.post(eventPath(market.ID), concert)

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, importKeyLabelText+": <code>"+marketImportKey+"</code>", importKeyFormTag(market.ID))
}

func TestRemoveImportKeyKeepsEveryOtherFieldAndRedirectsToTheEvent(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.post(importKeyRemovePath(market.ID), url.Values{})

	assertRedirect(t, rec, eventPath(market.ID))
	want := market
	want.ImportKey = ""
	if got := ts.events.events[market.ID]; !reflect.DeepEqual(got, want) {
		t.Errorf("market = %+v, want %+v", got, want)
	}
	assertDeleteLogged(t, ts, logMsgEventImportKeyRemoved, market.ID)
	assertBodyLacks(t, ts.get(eventPath(market.ID)), importKeyLabelText)
}

func TestRemoveImportKeyByHTMXAnswersWithHXRedirectToTheEvent(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.htmxPost(importKeyRemovePath(market.ID), url.Values{})

	assertHTMXRedirect(t, rec, eventPath(market.ID))
	if got := ts.events.events[market.ID].ImportKey; got != "" {
		t.Errorf("import key = %q, want it removed", got)
	}
}

func TestImportAfterRemovingTheImportKeyDoesNotUpdateTheEvent(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)
	assertRedirect(t, ts.post(importKeyRemovePath(market.ID), url.Values{}), eventPath(market.ID))

	rec := ts.postImport(t, importFileField, classifiedImportFile)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, `name="class-1" value="duplicateSuspect"`, `name="target-1" value=""`)
	assertBodyLacks(t, rec, `name="class-1" value="update"`)
}

func TestRemovingTheImportKeyOfAGoneEventShowsItIsNoLongerAvailable(t *testing.T) {
	responses := map[string]func(*testServer) *httptest.ResponseRecorder{
		"unknown": func(ts *testServer) *httptest.ResponseRecorder {
			return ts.post(importKeyRemovePath(unknownEventID), url.Values{})
		},
		"malformed id": func(ts *testServer) *httptest.ResponseRecorder {
			return ts.post(importKeyRemovePath("kaputt"), url.Values{})
		},
		"by htmx": func(ts *testServer) *httptest.ResponseRecorder {
			return ts.htmxPost(importKeyRemovePath(unknownEventID), url.Values{})
		},
	}
	for name, request := range responses {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)

			rec := request(ts)

			assertStatusCode(t, rec, http.StatusNotFound)
			assertBodyContains(t, rec, "<h1>"+eventGoneText+"</h1>", `href="`+eventsPath+`">Zurück zur Event-Liste`)
		})
	}
}

func TestRemoveImportKeyFailureAnswersServerErrorAndLogs(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.events = failingEvents{err: errStorageDown}

	assertServerErrorLogged(t, ts, ts.post(importKeyRemovePath(unknownEventID), url.Values{}))
}

func TestRemoveImportKeyRequiresPostSessionAndSameOrigin(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)
	path := importKeyRemovePath(market.ID)

	assertStatusCode(t, ts.get(path), http.StatusMethodNotAllowed)
	assertRedirect(t, ts.do(httptest.NewRequest(http.MethodPost, path, nil)), loginPath)
	crossSite := httptest.NewRequest(http.MethodPost, path, nil)
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	assertStatusCode(t, ts.do(withCookie(crossSite, ts.validCookie())), http.StatusForbidden)
	if got := ts.events.events[market.ID].ImportKey; got != marketImportKey {
		t.Errorf("import key = %q, want it kept", got)
	}
}
