package admin

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// confirmationOf is what confirming the warning shown for form sends.
func confirmationOf(form url.Values) string {
	return core.DuplicateConfirmationOf(core.EventInput{
		Title:      form.Get(core.EventFieldTitle),
		StartDate:  form.Get(core.EventFieldStartDate),
		LocationID: form.Get(core.EventFieldLocationID),
	})
}

// confirmDuplicateButton is the submit button that saves form despite the
// warning.
func confirmDuplicateButton(form url.Values) string {
	return `<button type="submit" name="duplicates" value="` + confirmationOf(form) + `">Trotzdem speichern</button>`
}

// archivedMarketForm is the market of September, archived on the test
// clock's 2 October 2026.
func archivedMarketForm(locationID string) url.Values {
	form := marketForm(locationID)
	form.Set(core.EventFieldStartDate, "2026-09-12")
	form.Set(core.EventFieldEndDate, "")
	return form
}

func TestSuspectedDuplicateShowsWarningWithLinksAndKeepsInput(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	archived := ts.seedEvent(t, archivedMarketForm(hall.ID))
	twin := archivedMarketForm(hall.ID)
	twin.Set(core.EventFieldTitle, " KIRCHWEIHMARKT ")
	twin.Set(core.EventFieldStartTime, "20:00")

	rec := ts.post(eventsPath, twin)

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec,
		msgDuplicateSuspect,
		`<a href="`+eventPath(archived.ID)+`" target="_blank" rel="noopener">`+marketTitle+`</a> (12.09.2026 19:00, archiviert)`,
		confirmDuplicateButton(twin),
		`hx-on:click="document.querySelectorAll('.duplicate-warning').forEach(e => e.remove())"`,
		`name="title" type="text" value=" KIRCHWEIHMARKT "`, `value="20:00"`, "Mit Fahrgeschäften",
	)
	if len(ts.events.events) != 1 {
		t.Errorf("%d events stored, want only the archived one", len(ts.events.events))
	}
}

func TestConfirmingTheWarningSavesTheDuplicate(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	twin := marketForm(hall.ID)
	twin.Set("duplicates", confirmationOf(twin))

	rec := ts.post(eventsPath, twin)

	assertRedirect(t, rec, eventsPath)
	if len(ts.events.events) != 2 {
		t.Errorf("%d events stored, want the market twice", len(ts.events.events))
	}
}

func TestEnterInTheFormNeverConfirmsTheDuplicate(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))

	body := html.UnescapeString(ts.post(eventsPath, marketForm(hall.ID)).Body.String())

	save, confirm := strings.Index(body, `<button type="submit">Speichern</button>`), strings.Index(body, confirmDuplicateButton(marketForm(hall.ID)))
	if save < 0 || confirm < save {
		t.Errorf("positions save = %d, confirm = %d, want the plain save button first, so Enter saves without confirming", save, confirm)
	}
}

func TestEditingAnEventIntoAnothersTwinWarns(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	other := marketForm(hall.ID)
	other.Set(core.EventFieldTitle, "Konzert")
	concert := ts.seedEvent(t, other)

	rec := ts.post(eventPath(concert.ID), marketForm(hall.ID))

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, `action="`+eventPath(concert.ID)+`"`, `href="`+eventPath(market.ID)+`"`, "(16.10.2026 19:00)")
	assertBodyLacks(t, rec, `href="`+eventPath(concert.ID)+`" target`)
	if stored := ts.events.events[concert.ID]; stored.Title != "Konzert" {
		t.Errorf("concert title = %q, want unchanged", stored.Title)
	}
}

func TestFieldProblemsShowNoDuplicateWarning(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	twin := marketForm(hall.ID)
	twin.Set(core.EventFieldSourceDescription, "")

	rec := ts.post(eventsPath, twin)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, "Bitte eine Quelle angeben.")
	assertBodyLacks(t, rec, msgDuplicateSuspect, "Trotzdem speichern")
}

func TestSameTitleElsewhereOrOnAnotherDaySavesWithoutWarning(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	park := ts.seed(t, "Bibertpark")
	ts.seedEvent(t, marketForm(hall.ID))
	nextDay := marketForm(hall.ID)
	nextDay.Set(core.EventFieldStartDate, "2026-10-17")

	for _, form := range []url.Values{marketForm(park.ID), nextDay} {
		rec := ts.post(eventsPath, form)
		assertRedirect(t, rec, eventsPath)
		assertBodyLacks(t, rec, msgDuplicateSuspect)
	}
	if len(ts.events.events) != 3 {
		t.Errorf("%d events stored, want all three", len(ts.events.events))
	}
}

func TestWarningListsEveryCandidateEarliestFirst(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	late := archivedMarketForm(hall.ID)
	late.Set(core.EventFieldStartTime, "21:00")
	early := archivedMarketForm(hall.ID)
	early.Set(core.EventFieldStartTime, "10:00")
	early.Set(duplicatesField, confirmationOf(early))
	lateEvent := ts.seedEvent(t, late)
	earlyEvent := ts.seedEvent(t, early)
	active := marketForm(hall.ID)
	active.Set(core.EventFieldStartDate, "2026-09-12")
	active.Set(core.EventFieldEndDate, "2026-10-19")

	rec := ts.post(eventsPath, active)

	assertStatusCode(t, rec, http.StatusConflict)
	body := html.UnescapeString(rec.Body.String())
	first := `<a href="` + eventPath(earlyEvent.ID) + `" target="_blank" rel="noopener">` + marketTitle + `</a> (12.09.2026 10:00, archiviert)`
	second := `<a href="` + eventPath(lateEvent.ID) + `" target="_blank" rel="noopener">` + marketTitle + `</a> (12.09.2026 21:00, archiviert)`
	if i, j := strings.Index(body, first), strings.Index(body, second); i < 0 || j < i {
		t.Errorf("positions = %d, %d, want both candidates, earliest first", i, j)
	}
}

func TestEventFormsWithoutSuspectShowNoWarning(t *testing.T) {
	ts := newTestServer(t)

	assertBodyLacks(t, ts.get(newEventPath), msgDuplicateSuspect, "Trotzdem speichern", "duplicate-warning")
}

func TestConfirmationLapsesWhenTheDuplicateKeyChanges(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	fleaForm := marketForm(hall.ID)
	fleaForm.Set(core.EventFieldTitle, "Flohmarkt")
	flea := ts.seedEvent(t, fleaForm)
	// Confirmed as twin of the market, then retitled into the flea market.
	retitled := marketForm(hall.ID)
	retitled.Set(duplicatesField, confirmationOf(marketForm(hall.ID)))
	retitled.Set(core.EventFieldTitle, "Flohmarkt")

	rec := ts.post(eventsPath, retitled)

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, msgDuplicateSuspect, `href="`+eventPath(flea.ID)+`"`, confirmDuplicateButton(retitled))
	if len(ts.events.events) != 2 {
		t.Errorf("%d events stored, want only the two seeded", len(ts.events.events))
	}
}

func TestConfirmationOfAnotherKeySavesWhenTheNewKeyHasNoTwin(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	nextDay := marketForm(hall.ID)
	nextDay.Set(duplicatesField, confirmationOf(marketForm(hall.ID)))
	nextDay.Set(core.EventFieldStartDate, "2026-10-17")

	assertRedirect(t, ts.post(eventsPath, nextDay), eventsPath)
}

func TestFormerAllowValueConfirmsNothing(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	ts.seedEvent(t, marketForm(hall.ID))
	twin := marketForm(hall.ID)
	twin.Set(duplicatesField, "allow")

	rec := ts.post(eventsPath, twin)

	assertStatusCode(t, rec, http.StatusConflict)
	if len(ts.events.events) != 1 {
		t.Errorf("%d events stored, want only the seeded market", len(ts.events.events))
	}
}
