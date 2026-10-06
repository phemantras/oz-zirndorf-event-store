package admin

import (
	"net/http"
	"slices"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// marketImportKey is the import key the classification test gives the
// stored market.
const marketImportKey = "kirchweihmarkt-2026"

// classifiedImportFile has, against the stored market with
// marketImportKey: an update of it with a differing postal code of the
// hall, the same market without key, and two entries that bring the same
// new location with different streets.
const classifiedImportFile = `{
  "formatVersion": 1,
  "events": [
    {"title": "Kirchweihmarkt", "type": "festival", "importKey": "` + marketImportKey + `",
     "location": {"name": "Paul-Metz-Halle", "address": {"postalCode": "90522"}},
     "startDate": "2026-10-16", "startTime": "19:00", "endDate": "2026-10-19",
     "source": {"description": "Amtsblatt 41/2026", "url": "https://www.zirndorf.de/amtsblatt"},
     "timetable": [{"description": "Musik", "date": "2026-10-17"}]},
    {"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
     "startDate": "2026-10-16", "source": {"description": "Plakat"}},
    {"title": "Konzert an der Veste", "type": "culture", "startDate": "2026-10-17",
     "location": {"name": "Alte Veste", "address": {"street": "Burgweg 1", "postalCode": "90513", "city": "Zirndorf"},
                  "latitude": 49.4501, "longitude": 10.9376, "precision": "building"},
     "source": {"description": "Plakat"}},
    {"title": "Lesung an der Veste", "type": "culture", "startDate": "2026-10-18",
     "location": {"name": "Alte Veste", "address": {"street": "Burgweg 3", "postalCode": "90513", "city": "Zirndorf"},
                  "latitude": 49.4501, "longitude": 10.9376, "precision": "building"},
     "source": {"description": "Plakat"}}
  ]
}`

func TestImportShowsClassTargetChangesCandidatesAndHints(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	market.ImportKey = marketImportKey
	ts.events.events[market.ID] = market

	rec := ts.postImport(t, importFileField, classifiedImportFile)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"<li>neu: 2</li>", "<li>Aktualisierung: 1</li>", "<li>Duplikatverdacht: 1</li>",
		`<tr id="import-entry-2">`, `href="`+eventURL(market.ID)+`"`, `href="#import-entry-1"`,
		"<td>Alte Veste</td><td>3, 4</td>",
	)
	wantRows := map[string][]string{
		"1": {
			"1", "Kirchweihmarkt", "Aktualisierung",
			"Ziel:", "vorhandenes Event",
			"Änderungen:",
			"Event-Typ:", "Markt", "→", "Fest/Kirchweih",
			"Notiz:", "Mit Fahrgeschäften", "→", msgImportEmptyValue,
			"Ablaufplan:", msgImportEmptyValue, "→", "2026-10-17 Musik",
			"Hinweise:",
			"Angabe „PLZ“ weicht vom vorhandenen Ort ab; der Ort bleibt unverändert.",
			msgImportDuplicateHint, "Position 2: Kirchweihmarkt (16.10.2026)",
		},
		"2": {
			"2", "Kirchweihmarkt", "Duplikatverdacht",
			"Mögliche Duplikate:", "Kirchweihmarkt (16.10.2026)", "Position 1: Kirchweihmarkt (16.10.2026)",
		},
		"4": {
			"4", "Lesung an der Veste", "neu",
			"Hinweise:", "Angabe „Straße und Hausnummer“ weicht von der ersten Angabe dieses neuen Orts ab; es gilt die erste.",
		},
	}
	for position, want := range wantRows {
		if got := importRowTexts(rec, position); !slices.Equal(got, want) {
			t.Errorf("row %s = %q, want %q", position, got, want)
		}
	}
	if len(ts.events.events) != 1 || len(ts.locations.locations) != 1 || ts.events.events[market.ID].Type != core.EventTypeMarket {
		t.Error("checking an import changed the store")
	}
}

func TestImportShowsChangedValuesInGerman(t *testing.T) {
	tests := []struct {
		field, value, want string
	}{
		{field: core.EventFieldAllDay, value: "true", want: "ja"},
		{field: core.EventFieldAllDay, value: "false", want: "nein"},
		{field: core.EventFieldType, value: string(core.EventTypeSports), want: "Sport"},
		{field: core.EventFieldType, value: "unbekannt", want: "unbekannt"},
		{field: core.EventFieldNote, value: "", want: msgImportEmptyValue},
		{field: core.EventFieldTitle, value: "Kirchweih", want: "Kirchweih"},
	}
	for _, tt := range tests {
		if got := importChangeValue(tt.field, tt.value); got != tt.want {
			t.Errorf("importChangeValue(%q, %q) = %q, want %q", tt.field, tt.value, got, tt.want)
		}
	}
}
