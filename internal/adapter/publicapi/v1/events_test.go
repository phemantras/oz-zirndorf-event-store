package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

var berlin = mustLoadBerlin()

func mustLoadBerlin() *time.Location {
	zone, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return zone
}

// fixedClock is a core.Clock that always shows the same instant.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

// christmasNoon is the clock of the examples: 2026-12-24 12:00
// Europe/Berlin.
var christmasNoon = fixedClock(time.Date(2026, time.December, 24, 12, 0, 0, 0, berlin))

// recordingLister stands in for the core queries: it records what it was
// asked and answers with listed or err.
type recordingLister struct {
	listed []core.ListedEvent
	err    error

	calls         int
	archivedCalls int
	filter        core.EventFilter
	clock         core.Clock
	context       context.Context
}

func (l *recordingLister) ListActiveEvents(ctx context.Context, clock core.Clock, filter core.EventFilter) ([]core.ListedEvent, error) {
	l.calls++
	l.context, l.clock, l.filter = ctx, clock, filter
	return l.listed, l.err
}

func (l *recordingLister) ListArchivedEvents(ctx context.Context, clock core.Clock, filter core.EventFilter) ([]core.ListedEvent, error) {
	l.archivedCalls++
	l.context, l.clock, l.filter = ctx, clock, filter
	return l.listed, l.err
}

func newEventsHandler(events EventLister, logs *bytes.Buffer) http.Handler {
	return NewHandler(Config{Logger: slog.New(slog.NewJSONHandler(logs, nil)), Events: events, Clock: christmasNoon})
}

func serveEvents(t *testing.T, events EventLister, query string) *httptest.ResponseRecorder {
	t.Helper()
	var logs bytes.Buffer
	return serve(newEventsHandler(events, &logs), http.MethodGet, eventsPath+query)
}

// overlapRepo is the part of core.EventRepo the event query uses: it
// answers ListOverlapping by the predicate of AD-16 and counts the calls.
type overlapRepo struct {
	core.EventRepo
	events []core.Event
	calls  int
}

func (r *overlapRepo) ListOverlapping(_ context.Context, overlap core.Overlap) ([]core.Event, error) {
	r.calls++
	var matches []core.Event
	for _, event := range r.events {
		if (overlap.Hi == nil || event.Period.Start.Before(*overlap.Hi)) && (overlap.Lo == nil || event.Period.End.After(*overlap.Lo)) {
			matches = append(matches, event)
		}
	}
	return matches, nil
}

// locationList is the part of core.LocationRepo the event query uses.
type locationList struct {
	core.LocationRepo
	locations []core.Location
}

func (l locationList) List(context.Context) ([]core.Location, error) { return l.locations, nil }

const hallID = "0192f0b1-0000-7000-8000-000000000001"

func hall() core.Location {
	return core.Location{
		ID: hallID, Name: "Paul-Metz-Halle", NameKey: "paul-metz-halle",
		Street: "Volkhardtstraße 2", PostalCode: "90513", City: "Zirndorf",
		Latitude: 49.4424, Longitude: 10.9539, Precision: core.PrecisionBuilding, Note: "Eingang über den Hof",
	}
}

func localTime(hour, minute int) *core.LocalTime { return &core.LocalTime{Hour: hour, Minute: minute} }

func december(day int) core.LocalDate {
	return core.LocalDate{Year: 2026, Month: time.December, Day: day}
}

// storedEvent returns an event at the hall as the repository holds it.
func storedEvent(t *testing.T, id, title string, eventType core.EventType, times core.EventTimes) core.Event {
	t.Helper()
	period, err := times.EffectivePeriod()
	if err != nil {
		t.Fatalf("EffectivePeriod: %v", err)
	}
	return core.Event{
		ID: id, Title: title, TitleKey: strings.ToLower(title), Type: eventType, LocationID: hallID,
		Times: times, Source: core.EventSource{Description: "Amtsblatt"}, Period: period,
	}
}

// coreEvents returns the real event query on stored events around the
// clock of the examples, and the repository to count its calls.
func coreEvents(t *testing.T) (*core.EventService, *overlapRepo) {
	t.Helper()
	repo := &overlapRepo{events: []core.Event{
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000301", "Weihnachtsmarkt", core.EventTypeMarket,
			core.EventTimes{StartDate: core.LocalDate{Year: 2026, Month: time.November, Day: 27}, EndDate: december(24)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000302", "Krippenspiel", core.EventTypeClub,
			core.EventTimes{StartDate: december(24), StartTime: localTime(10, 0), EndDate: december(24), EndTime: localTime(14, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000303", "Christmette", core.EventTypeCulture,
			core.EventTimes{StartDate: december(24), StartTime: localTime(18, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000304", "Silvesterlauf", core.EventTypeSports,
			core.EventTimes{StartDate: december(31), StartTime: localTime(14, 0)}),
	}}
	return core.NewEventService(nil, repo, locationList{locations: []core.Location{hall()}}), repo
}

// listedTitles decodes a 200 response and returns the titles in order.
func listedTitles(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var list EventList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	titles := []string{}
	for _, event := range list.Data {
		titles = append(titles, event.Title)
	}
	return titles
}

func TestListEventsPassesTheParametersAndTheClockToTheCore(t *testing.T) {
	lister := &recordingLister{}

	rec := serveEvents(t, lister, "?from=2026-11-29&to=2026-12-24T18:00%2B01:00&type=market&type=club&type=market")

	if rec.Code != http.StatusOK || lister.calls != 1 {
		t.Fatalf("status = %d after %d calls, want %d after one", rec.Code, lister.calls, http.StatusOK)
	}
	from, to := lister.filter.From, lister.filter.To
	if from == nil || *from != "2026-11-29" || to == nil || *to != "2026-12-24T18:00+01:00" {
		t.Errorf("from, to = %v, %v, want 2026-11-29 and 2026-12-24T18:00+01:00", from, to)
	}
	if want := []string{"market", "club", "market"}; !slices.Equal(lister.filter.Types, want) {
		t.Errorf("types = %v, want %v", lister.filter.Types, want)
	}
	if lister.clock != core.Clock(christmasNoon) || lister.context == nil {
		t.Errorf("clock = %v, want the configured clock", lister.clock)
	}
}

func TestListEventsWithoutParametersPassesAnEmptyFilter(t *testing.T) {
	lister := &recordingLister{}

	rec := serveEvents(t, lister, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if lister.filter.From != nil || lister.filter.To != nil || lister.filter.Types != nil {
		t.Errorf("filter = %+v, want none", lister.filter)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"data":[]}` {
		t.Errorf("body = %s, want an empty list", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
}

func TestListEventsAnswersTheExamplesOfTheStory(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"today", "", []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}},
		{"date range", "?from=2026-11-29&to=2026-12-24", []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}},
		{"to is inclusive", "?to=2026-12-24T18:00%2B01:00", []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}},
		{"instant in from and date in to", "?from=2026-12-24T18:00%2B01:00&to=2026-12-24", []string{"Weihnachtsmarkt", "Christmette"}},
		{"only from", "?from=2026-12-30", []string{"Silvesterlauf"}},
		{"types", "?type=market&type=club&type=market", []string{"Weihnachtsmarkt", "Krippenspiel"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, _ := coreEvents(t)
			if got := listedTitles(t, serveEvents(t, events, test.query)); !slices.Equal(got, test.want) {
				t.Errorf("titles = %v, want %v", got, test.want)
			}
		})
	}
}

func TestListEventsRejectsInvalidParametersWithoutAskingTheRepository(t *testing.T) {
	tests := []struct {
		name  string
		query string
		names []string
	}{
		{"german date", "?from=24.12.2026", []string{"from", "%2B"}},
		{"unencoded plus in the offset", "?from=2026-12-24T18:00+01:00", []string{"from", "%2B"}},
		{"from given empty", "?from=", []string{"from"}},
		{"to given empty", "?to=", []string{"to"}},
		{"instant without offset", "?to=2026-12-24T18:00", []string{"to"}},
		{"unknown type", "?type=foo", []string{"type", "market"}},
		{"empty type", "?type=", []string{"type"}},
		{"empty period", "?from=2026-12-28&to=2026-12-27", []string{"from", "to"}},
		{"only to before today", "?to=2026-12-23", []string{"to", "/v1/archive/events"}},
		{"every parameter", "?from=morgen&to=bald&type=foo", []string{"from", "to", "type"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, repo := coreEvents(t)

			rec := serveEvents(t, events, test.query)

			detail := assertProblem(t, rec, http.StatusBadRequest)
			for _, name := range test.names {
				if !strings.Contains(detail, name) {
					t.Errorf("detail %q does not name %s", detail, name)
				}
			}
			if repo.calls != 0 {
				t.Errorf("repository was asked %d times", repo.calls)
			}
		})
	}
}

// The details of a period that only the other list can hold are the
// beforeToday and afterNow examples of api/v1/openapi.yaml word for word.
func TestPeriodOfTheOtherListPointsThereAsTheSpecExampleSays(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"beforeToday", eventsPath + "?to=2026-12-23",
			"Parameter to lies before today, but /v1/events lists only active events; use /v1/archive/events for past events."},
		{"afterNow", archivePath + "?from=2027-01-01",
			"Parameter from lies at or after now, but /v1/archive/events lists only past events; use /v1/events for active and future events."},
		{"unknownType", eventsPath + "?type=fair",
			"Parameter type must be an event type code: festival, market, culture, politics, club, sports, other."},
		{"emptyPeriod", eventsPath + "?from=2026-12-24&to=2026-12-23",
			"Parameters from and to give an empty period; to must not lie before from."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, _ := coreEvents(t)
			var logs bytes.Buffer

			detail := assertProblem(t, serve(newEventsHandler(events, &logs), http.MethodGet, test.path), http.StatusBadRequest)

			if detail != test.want {
				t.Errorf("detail = %q, want %q", detail, test.want)
			}
		})
	}
}

func TestListEventsRejectsAParameterGivenTwice(t *testing.T) {
	lister := &recordingLister{}

	detail := assertProblem(t, serveEvents(t, lister, "?from=2026-12-24&from=2026-12-25"), http.StatusBadRequest)

	if !strings.Contains(detail, "from") {
		t.Errorf("detail %q does not name from", detail)
	}
	if lister.calls != 0 {
		t.Errorf("core was asked %d times", lister.calls)
	}
}

func TestListEventsNamesTheFieldOfAnUnforeseenProblem(t *testing.T) {
	lister := &recordingLister{err: &core.ValidationError{Fields: []core.FieldError{{Field: "from", Problem: core.ProblemOutOfRange}}}}

	detail := assertProblem(t, serveEvents(t, lister, ""), http.StatusBadRequest)

	if !strings.Contains(detail, "from") {
		t.Errorf("detail %q does not name from", detail)
	}
}

func TestInvalidParameterWithoutNameIsABadRequest(t *testing.T) {
	rec := httptest.NewRecorder()

	responder{logger: slog.New(slog.DiscardHandler)}.invalidParameter(rec, httptest.NewRequest(http.MethodGet, eventsPath, nil), errAnswerFailed)

	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if rec.Code != http.StatusBadRequest || problem.Status != http.StatusBadRequest || problem.Detail == "" {
		t.Errorf("status %d, problem %+v, want a bad request with detail", rec.Code, problem)
	}
	if strings.Contains(problem.Detail, errAnswerFailed.Error()) {
		t.Errorf("detail %q leaks the error", problem.Detail)
	}
}

func TestListEventsFailureIsAnInternalServerErrorAndLogged(t *testing.T) {
	var logs bytes.Buffer
	handler := newEventsHandler(&recordingLister{err: errAnswerFailed}, &logs)

	detail := assertProblem(t, serve(handler, http.MethodGet, eventsPath), http.StatusInternalServerError)

	if strings.Contains(detail, errAnswerFailed.Error()) {
		t.Errorf("detail %q leaks the internal error", detail)
	}
	if !strings.Contains(logs.String(), errAnswerFailed.Error()) {
		t.Errorf("log %q does not contain the error", logs.String())
	}
	assertLogEntry(t, &logs, logEntry{Level: "ERROR", Msg: "public api request failed"})
}

// logEntry is the level and message of a JSON log line.
type logEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// assertLogEntry checks that logs holds one JSON entry with want's level
// and message.
func assertLogEntry(t *testing.T, logs *bytes.Buffer, want logEntry) {
	t.Helper()
	var entry logEntry
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode log %q: %v", logs.String(), err)
	}
	if entry != want {
		t.Errorf("log entry = %+v, want %+v", entry, want)
	}
}

func TestListEventsCancelledByTheClientIsLoggedAsInfo(t *testing.T) {
	var logs bytes.Buffer
	cancelled := fmt.Errorf("list events: %w", context.Canceled)
	handler := newEventsHandler(&recordingLister{err: cancelled}, &logs)

	assertProblem(t, serve(handler, http.MethodGet, eventsPath), http.StatusInternalServerError)

	assertLogEntry(t, &logs, logEntry{Level: "INFO", Msg: "public api request cancelled by client"})
	if strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("log %q reports a cancelled request as an error", logs.String())
	}
}

// TestListsPastTheRequestDeadlineAreServiceUnavailableAndLoggedAsWarning
// lets the lister fail because the request deadline ran out, as with a slow
// database or an exhausted pool: both lists answer 503 and log a warning.
func TestListsPastTheRequestDeadlineAreServiceUnavailableAndLoggedAsWarning(t *testing.T) {
	for _, path := range []string{eventsPath, archivePath} {
		t.Run(path, func(t *testing.T) {
			var logs bytes.Buffer
			timedOut := fmt.Errorf("list events: %w", context.DeadlineExceeded)
			handler := newEventsHandler(&recordingLister{err: timedOut}, &logs)

			detail := assertProblem(t, serve(handler, http.MethodGet, path), http.StatusServiceUnavailable)

			if want := "The server is overloaded and could not answer in time; try again later."; detail != want {
				t.Errorf("detail = %q, want %q", detail, want)
			}
			assertLogEntry(t, &logs, logEntry{Level: "WARN", Msg: "public api request timed out"})
		})
	}
}

// fullListedEvent uses every field of the read form.
func fullListedEvent() core.ListedEvent {
	return core.ListedEvent{
		Event: core.Event{
			ID: "0192f0b1-0000-7000-8000-000000000101", Title: "Weihnachtskonzert", TitleKey: "weihnachtskonzert",
			Type: core.EventTypeCulture, LocationID: hallID,
			Times:  core.EventTimes{StartDate: december(24), StartTime: localTime(19, 30), EndDate: december(24), EndTime: localTime(21, 0)},
			Source: core.EventSource{Description: "Plakat", URL: "https://www.zirndorf.de/konzert"},
			Note:   "Eintritt frei",
			Period: core.Period{
				Start: time.Date(2026, time.December, 24, 19, 30, 0, 0, berlin),
				End:   time.Date(2026, time.December, 24, 21, 0, 0, 0, berlin),
			},
			Timetable: []core.TimetableEntry{
				{ID: "0192f0b1-0000-7000-8000-000000000201", Description: "Einlass", Date: december(24), StartTime: localTime(19, 0)},
				{ID: "0192f0b1-0000-7000-8000-000000000202", Description: "Zugabe", Date: december(24), StartTime: localTime(20, 45), EndTime: localTime(21, 0)},
			},
		},
		Location:       hall(),
		StartPrecision: core.TimePrecisionExact,
		EndPrecision:   core.TimePrecisionExact,
	}
}

// minimalListedEvent leaves every optional value empty, at a location
// entered before addresses were split into parts.
func minimalListedEvent() core.ListedEvent {
	legacy := core.Location{ID: "0192f0b1-0000-7000-8000-000000000002", Name: "Bibertpark", Latitude: 49.44, Longitude: 10.95, Precision: core.PrecisionArea}
	return core.ListedEvent{
		Event: core.Event{
			ID: "0192f0b1-0000-7000-8000-000000000102", Title: "Feuerschale", Type: core.EventTypeOther, LocationID: legacy.ID,
			Times:  core.EventTimes{StartDate: december(24), AllDay: true},
			Source: core.EventSource{Description: "Aushang"},
			Period: core.Period{
				Start: time.Date(2026, time.December, 24, 0, 0, 0, 0, berlin),
				End:   time.Date(2026, time.December, 25, 0, 0, 0, 0, berlin),
			},
		},
		Location:       legacy,
		StartPrecision: core.TimePrecisionAllDay,
	}
}

// decodeEvents returns the events of a 200 response as generic JSON.
func decodeEvents(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string][]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body["data"]
}

func assertKeys(t *testing.T, level string, object any, want ...string) {
	t.Helper()
	fields, ok := object.(map[string]any)
	if !ok {
		t.Fatalf("%s = %v, want an object", level, object)
	}
	got := slices.Sorted(maps.Keys(fields))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s keys = %v, want %v", level, got, want)
	}
}

func TestListEventsDeliversExactlyTheReadForm(t *testing.T) {
	events := decodeEvents(t, serveEvents(t, &recordingLister{listed: []core.ListedEvent{fullListedEvent(), minimalListedEvent()}}, ""))

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	for _, event := range events {
		assertKeys(t, "event", event, "title", "type", "location", "startDate", "startTime", "endDate", "endTime", "allDay",
			"startPrecision", "endPrecision", "source", "note", "timetable", "effectiveStart", "effectiveEnd")
		location := event["location"].(map[string]any)
		assertKeys(t, "location", location, "name", "address", "latitude", "longitude", "precision", "note")
		assertKeys(t, "address", location["address"], "street", "postalCode", "city")
		assertKeys(t, "source", event["source"], "description", "url")
		for _, entry := range event["timetable"].([]any) {
			assertKeys(t, "timetable entry", entry, "description", "date", "startTime", "endTime")
		}
	}
}

func TestListEventsGivesOutNoInternalValues(t *testing.T) {
	rec := serveEvents(t, &recordingLister{listed: []core.ListedEvent{fullListedEvent(), minimalListedEvent()}}, "")

	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	forbidden := []string{"id", "locationId", "importKey", "archivedAt", "titleKey", "nameKey", "title_key", "name_key"}
	for _, key := range keysAtEveryLevel(body) {
		if slices.Contains(forbidden, key) {
			t.Errorf("response contains the internal key %q", key)
		}
	}
	for _, value := range []string{hallID, fullListedEvent().ID, fullListedEvent().Timetable[0].ID} {
		if strings.Contains(rec.Body.String(), value) {
			t.Errorf("response contains the ID %s", value)
		}
	}
}

// keysAtEveryLevel returns the keys of every object within value.
func keysAtEveryLevel(value any) []string {
	var keys []string
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			keys = append(keys, key)
			keys = append(keys, keysAtEveryLevel(nested)...)
		}
	case []any:
		for _, nested := range typed {
			keys = append(keys, keysAtEveryLevel(nested)...)
		}
	}
	return keys
}

func TestListEventsFormatsDatesTimesAndInstants(t *testing.T) {
	rec := serveEvents(t, &recordingLister{listed: []core.ListedEvent{fullListedEvent()}}, "")

	want := `{"data":[{"allDay":false,` +
		`"effectiveEnd":"2026-12-24T21:00:00+01:00","effectiveStart":"2026-12-24T19:30:00+01:00",` +
		`"endDate":"2026-12-24","endPrecision":"exact","endTime":"21:00",` +
		`"location":{"address":{"city":"Zirndorf","postalCode":"90513","street":"Volkhardtstraße 2"},` +
		`"latitude":49.4424,"longitude":10.9539,"name":"Paul-Metz-Halle","note":"Eingang über den Hof","precision":"building"},` +
		`"note":"Eintritt frei","source":{"description":"Plakat","url":"https://www.zirndorf.de/konzert"},` +
		`"startDate":"2026-12-24","startPrecision":"exact","startTime":"19:30",` +
		`"timetable":[{"date":"2026-12-24","description":"Einlass","endTime":null,"startTime":"19:00"},` +
		`{"date":"2026-12-24","description":"Zugabe","endTime":"21:00","startTime":"20:45"}],` +
		`"title":"Weihnachtskonzert","type":"culture"}]}`
	assertSameJSON(t, rec.Body.Bytes(), want)
}

func TestListEventsDeliversEmptyValuesAsNull(t *testing.T) {
	rec := serveEvents(t, &recordingLister{listed: []core.ListedEvent{minimalListedEvent()}}, "")

	want := `{"data":[{"allDay":true,` +
		`"effectiveEnd":"2026-12-25T00:00:00+01:00","effectiveStart":"2026-12-24T00:00:00+01:00",` +
		`"endDate":null,"endPrecision":null,"endTime":null,` +
		`"location":{"address":{"city":null,"postalCode":null,"street":null},` +
		`"latitude":49.44,"longitude":10.95,"name":"Bibertpark","note":null,"precision":"area"},` +
		`"note":null,"source":{"description":"Aushang","url":null},` +
		`"startDate":"2026-12-24","startPrecision":"allDay","startTime":null,` +
		`"timetable":[],"title":"Feuerschale","type":"other"}]}`
	assertSameJSON(t, rec.Body.Bytes(), want)
}

func TestListEventsDeliversAnArchivedEventWithoutAnArchivedKey(t *testing.T) {
	archived := minimalListedEvent()
	archived.Archived = true

	events := decodeEvents(t, serveEvents(t, &recordingLister{listed: []core.ListedEvent{archived}}, ""))

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if _, found := events[0]["archived"]; found {
		t.Errorf("event %v contains the key archived", events[0])
	}
}

// assertSameJSON compares two JSON documents independent of key order.
func assertSameJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("decode want: %v", err)
	}
	gotText, _ := json.Marshal(gotValue)
	wantText, _ := json.Marshal(wantValue)
	if !bytes.Equal(gotText, wantText) {
		t.Errorf("body =\n%s\nwant\n%s", gotText, wantText)
	}
}

func TestListEventsKeepsTheOrderOfTheCore(t *testing.T) {
	later, earlier := fullListedEvent(), minimalListedEvent()

	events := decodeEvents(t, serveEvents(t, &recordingLister{listed: []core.ListedEvent{later, earlier}}, ""))

	if len(events) != 2 || events[0]["title"] != later.Title || events[1]["title"] != earlier.Title {
		t.Errorf("events = %v, want the order of the core", events)
	}
}
