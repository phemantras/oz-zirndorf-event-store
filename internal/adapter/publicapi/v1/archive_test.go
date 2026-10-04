package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

func serveArchive(t *testing.T, events EventLister, query string) *httptest.ResponseRecorder {
	t.Helper()
	var logs bytes.Buffer
	return serve(newEventsHandler(events, &logs), http.MethodGet, archivePath+query)
}

// coreEventsWithPast is coreEvents plus events that ended before, at and
// after the clock of the examples.
func coreEventsWithPast(t *testing.T) (*core.EventService, *overlapRepo) {
	t.Helper()
	events, repo := coreEvents(t)
	repo.events = append(repo.events,
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000321", "Sommerfest", core.EventTypeFestival,
			core.EventTimes{StartDate: core.LocalDate{Year: 2026, Month: time.June, Day: 20}}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000322", "Adventsbasar", core.EventTypeMarket,
			core.EventTimes{StartDate: december(23)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000323", "Frühschoppen", core.EventTypeClub,
			core.EventTimes{StartDate: december(24), StartTime: localTime(10, 0), EndDate: december(24), EndTime: localTime(11, 59)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000324", "Mittagsläuten", core.EventTypeCulture,
			core.EventTimes{StartDate: december(24), StartTime: localTime(11, 0), EndDate: december(24), EndTime: localTime(12, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000325", "Bescherung", core.EventTypeOther,
			core.EventTimes{StartDate: december(24), StartTime: localTime(11, 30), EndDate: december(24), EndTime: localTime(12, 1)}),
	)
	return events, repo
}

func TestListArchivedEventsPassesTheParametersAndTheClockToTheCore(t *testing.T) {
	lister := &recordingLister{}

	rec := serveArchive(t, lister, "?from=2026-06-01&to=2026-12-24T11:00%2B01:00&type=market&type=market")

	if rec.Code != http.StatusOK || lister.archivedCalls != 1 || lister.calls != 0 {
		t.Fatalf("status = %d after %d archive and %d active calls, want %d after one archive call",
			rec.Code, lister.archivedCalls, lister.calls, http.StatusOK)
	}
	from, to := lister.filter.From, lister.filter.To
	if from == nil || *from != "2026-06-01" || to == nil || *to != "2026-12-24T11:00+01:00" {
		t.Errorf("from, to = %v, %v, want 2026-06-01 and 2026-12-24T11:00+01:00", from, to)
	}
	if want := []string{"market", "market"}; !slices.Equal(lister.filter.Types, want) {
		t.Errorf("types = %v, want %v", lister.filter.Types, want)
	}
	if lister.clock != core.Clock(christmasNoon) {
		t.Errorf("clock = %v, want the configured clock", lister.clock)
	}
	if lister.context == nil {
		t.Error("context = nil, want the request context")
	}
}

func TestListArchivedEventsWithoutParametersPassesAnEmptyFilter(t *testing.T) {
	lister := &recordingLister{}

	rec := serveArchive(t, lister, "")

	if lister.filter.From != nil || lister.filter.To != nil || lister.filter.Types != nil {
		t.Errorf("filter = %+v, want none", lister.filter)
	}
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `{"data":[]}` {
		t.Errorf("status %d, body = %s, want an empty list", rec.Code, got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	assertAllowsAnyOrigin(t, rec.Header())
}

func TestListArchivedEventsAnswersTheExamplesOfTheStory(t *testing.T) {
	past := []string{"Mittagsläuten", "Frühschoppen", "Adventsbasar", "Sommerfest"}
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		// Frühschoppen ended 11:59, Mittagsläuten ends at 12:00, now;
		// Bescherung runs until 12:01.
		{"without parameters", "", past},
		{"only to", "?to=2026-06-30", []string{"Sommerfest"}},
		{"only from", "?from=2026-12-01", []string{"Mittagsläuten", "Frühschoppen", "Adventsbasar"}},
		{"future", "?from=2027-01-01&to=2027-01-31", []string{}},
		{"types", "?type=market&type=market", []string{"Adventsbasar"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, _ := coreEventsWithPast(t)
			if got := listedTitles(t, serveArchive(t, events, test.query)); !slices.Equal(got, test.want) {
				t.Errorf("titles = %v, want %v", got, test.want)
			}
		})
	}
}

func TestListArchivedEventsRejectsInvalidParametersWithoutAskingTheRepository(t *testing.T) {
	tests := []struct {
		name  string
		query string
		names []string
	}{
		{"german date", "?from=24.12.2026", []string{"from", "%2B"}},
		{"instant without offset", "?to=2026-06-30T18:00", []string{"to"}},
		{"unknown type", "?type=foo", []string{"type", "market"}},
		{"empty type", "?type=", []string{"type"}},
		{"from given empty", "?from=", []string{"from"}},
		{"empty period", "?from=2026-12-28&to=2026-12-27", []string{"from", "to"}},
		{"only from after now", "?from=2027-01-01", []string{"from", "/v1/events"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, repo := coreEventsWithPast(t)

			detail := assertProblem(t, serveArchive(t, events, test.query), http.StatusBadRequest)

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

func TestListArchivedEventsFailureIsAnInternalServerErrorAndLogged(t *testing.T) {
	var logs bytes.Buffer
	handler := newEventsHandler(&recordingLister{err: errAnswerFailed}, &logs)

	detail := assertProblem(t, serve(handler, http.MethodGet, archivePath), http.StatusInternalServerError)

	if strings.Contains(detail, errAnswerFailed.Error()) {
		t.Errorf("detail %q leaks the internal error", detail)
	}
	if !strings.Contains(logs.String(), errAnswerFailed.Error()) {
		t.Errorf("log %q does not contain the error", logs.String())
	}
}

func TestListArchivedEventsDeliversTheReadFormMarkedArchived(t *testing.T) {
	events, _ := coreEventsWithPast(t)
	rec := serveArchive(t, events, "")

	listed := decodeEvents(t, rec)
	if len(listed) == 0 {
		t.Fatal("archive is empty, want past events")
	}
	for _, event := range listed {
		assertKeys(t, "event", event, "title", "type", "location", "startDate", "startTime", "endDate", "endTime", "allDay",
			"startPrecision", "endPrecision", "source", "note", "timetable", "effectiveStart", "effectiveEnd", "archived")
		if event["archived"] != true {
			t.Errorf("%v archived = %v, want true", event["title"], event["archived"])
		}
	}
	var body any = map[string]any{"data": anySlice(listed)}
	for _, key := range keysAtEveryLevel(body) {
		if slices.Contains([]string{"id", "locationId", "importKey", "archivedAt", "titleKey", "nameKey"}, key) {
			t.Errorf("response contains the internal key %q", key)
		}
	}
	if strings.Contains(rec.Body.String(), hallID) {
		t.Errorf("response contains the location ID %s", hallID)
	}
}

func anySlice(objects []map[string]any) []any {
	values := make([]any, 0, len(objects))
	for _, object := range objects {
		values = append(values, object)
	}
	return values
}

func TestEveryEventIsInExactlyOneOfEventsAndArchive(t *testing.T) {
	events, repo := coreEventsWithPast(t)

	active := listedTitles(t, serveEvents(t, events, "?from=1900-01-01"))
	archived := listedTitles(t, serveArchive(t, events, ""))

	for _, event := range repo.events {
		inActive, inArchive := slices.Contains(active, event.Title), slices.Contains(archived, event.Title)
		if inActive == inArchive {
			t.Errorf("%s in /v1/events = %v, in the archive = %v, want exactly one", event.Title, inActive, inArchive)
		}
	}
}
