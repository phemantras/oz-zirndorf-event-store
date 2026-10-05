package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	publicapi "github.com/phemantras/oz-zirndorf-event-store/internal/adapter/publicapi/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// christmasMarketQuery is the period query of SM-1: the Advent season up
// to Christmas Eve 2026.
const christmasMarketQuery = "/v1/events?from=2026-11-29&to=2026-12-24"

// christmasMarketLocationName is the location of the Weihnachtsmarkt
// fixture.
const christmasMarketLocationName = "Marktplatz Zirndorf"

// Titles of the Weihnachtsmarkt fixture: like in the real data, one event
// per contiguous block, so the market does not count as running on the
// weekdays between its weekends.
const (
	firstAdventWeekendTitle  = "Zirndorfer Weihnachtsmarkt, 1. Adventswochenende"
	fourthAdventWeekendTitle = "Zirndorfer Weihnachtsmarkt, 4. Adventswochenende"
)

// wantFirstAdventWeekend is the complete answer for the first weekend:
// period, location with coordinates, both time precisions, the location
// precision and the source.
const wantFirstAdventWeekend = `{
	"title": "Zirndorfer Weihnachtsmarkt, 1. Adventswochenende",
	"type": "market",
	"location": {
		"name": "Marktplatz Zirndorf",
		"address": {"street": "Marktplatz", "postalCode": "90513", "city": "Zirndorf"},
		"latitude": 49.4427,
		"longitude": 10.9545,
		"precision": "street",
		"note": null
	},
	"startDate": "2026-11-27",
	"startTime": "17:00",
	"endDate": "2026-11-29",
	"endTime": null,
	"allDay": false,
	"startPrecision": "exact",
	"endPrecision": "dateOnly",
	"source": {
		"description": "Amtsblatt der Stadt Zirndorf, November 2026",
		"url": "https://www.zirndorf.de/amtsblatt"
	},
	"note": null,
	"timetable": [
		{"description": "Eröffnung durch den Bürgermeister, Stadtkapelle", "date": "2026-11-27", "startTime": "17:00", "endTime": "21:00"},
		{"description": "Besuch des Christkinds", "date": "2026-11-28", "startTime": "14:00", "endTime": "21:00"}
	],
	"effectiveStart": "2026-11-27T17:00:00+01:00",
	"effectiveEnd": "2026-11-30T00:00:00+01:00",
	"archived": false
}`

// TestChristmasMarketIsFoundByPeriod is the acceptance test of SM-1: with
// two of the four Advent weekends of the Weihnachtsmarkt 2026 stored and
// the clock before Advent, the period query lists them with period,
// location, time and location precision and source.
func TestChristmasMarketIsFoundByPeriod(t *testing.T) {
	pool := connectAndMigrate(t, testDatabaseURL(t))
	nameKey := core.NormalizeKey(christmasMarketLocationName)
	removeLocationWithEvents(t, pool, nameKey)
	t.Cleanup(func() { removeLocationWithEvents(t, pool, nameKey) })
	ctx := context.Background()
	tx, locationRepo := postgres.NewTxRunner(pool), postgres.NewLocationRepo(pool)
	location, err := core.NewLocationService(tx, locationRepo).SaveLocation(ctx, "", core.LocationInput{
		Name: christmasMarketLocationName, Street: "Marktplatz", PostalCode: "90513", City: "Zirndorf",
		Latitude: "49.4427", Longitude: "10.9545", Precision: string(core.PrecisionStreet),
	})
	if err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}
	events := core.NewEventService(tx, postgres.NewEventRepo(pool), locationRepo)
	source := core.EventSource{Description: "Amtsblatt der Stadt Zirndorf, November 2026", URL: "https://www.zirndorf.de/amtsblatt"}
	for _, in := range []core.EventInput{
		{
			Title: firstAdventWeekendTitle, StartDate: "2026-11-27", StartTime: "17:00", EndDate: "2026-11-29",
			Timetable: []core.TimetableEntryInput{
				{Description: "Eröffnung durch den Bürgermeister, Stadtkapelle", Date: "2026-11-27", StartTime: "17:00", EndTime: "21:00"},
				{Description: "Besuch des Christkinds", Date: "2026-11-28", StartTime: "14:00", EndTime: "21:00"},
			},
		},
		{Title: fourthAdventWeekendTitle, StartDate: "2026-12-18", StartTime: "17:00", EndDate: "2026-12-20"},
	} {
		in.Type, in.LocationID, in.Source = string(core.EventTypeMarket), location.ID, source
		if _, err := events.SaveEvent(ctx, "", in, core.AllowDuplicates); err != nil {
			t.Fatalf("SaveEvent %q: %v", in.Title, err)
		}
	}
	clock := fixedClock(time.Date(2026, time.November, 20, 11, 0, 0, 0, time.UTC)) // 12:00 Europe/Berlin
	public := publicapi.NewHandler(publicapi.Config{Logger: slog.New(slog.DiscardHandler), Events: events, Clock: clock})

	recorder := httptest.NewRecorder()
	public.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, christmasMarketQuery, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, body %s", christmasMarketQuery, recorder.Code, recorder.Body)
	}
	listed := listedEventsByTitle(t, recorder.Body.Bytes())
	if _, found := listed[fourthAdventWeekendTitle]; !found {
		t.Errorf("GET %s lacks %q: %s", christmasMarketQuery, fourthAdventWeekendTitle, recorder.Body)
	}
	var want any
	if err := json.Unmarshal([]byte(wantFirstAdventWeekend), &want); err != nil {
		t.Fatalf("decode expected event: %v", err)
	}
	if got := listed[firstAdventWeekendTitle]; !reflect.DeepEqual(got, want) {
		t.Errorf("GET %s lists the first Advent weekend as\n%v\nwant\n%v", christmasMarketQuery, got, want)
	}
}

// connectAndMigrate connects to the test database, closes the pool after
// the test and brings the schema up to date.
func connectAndMigrate(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return pool
}

// listedEventsByTitle decodes an event list and returns its events as
// generic JSON values by title, so a comparison sees every field.
func listedEventsByTitle(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var list struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode event list %s: %v", body, err)
	}
	byTitle := make(map[string]any, len(list.Data))
	for _, event := range list.Data {
		title, _ := event["title"].(string)
		byTitle[title] = event
	}
	return byTitle
}
