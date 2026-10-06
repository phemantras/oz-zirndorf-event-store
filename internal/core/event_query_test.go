package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// christmasEve is the day of the clock the examples of Story 2.3 use.
var christmasEve = LocalDate{2026, time.December, 24}

func december(day int) LocalDate { return LocalDate{2026, time.December, day} }

// christmasNoon is the fixed clock of the examples: 2026-12-24 12:00
// Europe/Berlin.
func christmasNoon(t *testing.T) Clock {
	t.Helper()
	return clockAt(t, christmasEve, 12, 0)
}

func newQueryService(repo *fakeEventRepo, locations ...Location) *EventService {
	return NewEventService(&fakeTx{}, repo, newFakeLocationRepo(locations...))
}

// listActiveTitles runs ListActiveEvents and returns the titles in order.
func listActiveTitles(t *testing.T, repo *fakeEventRepo, clock Clock, filter EventFilter) []string {
	t.Helper()
	return listActiveTitlesOf(t, newQueryService(repo, hall(), park()), clock, filter)
}

// listActiveTitlesOf runs ListActiveEvents of service and returns the
// titles in order.
func listActiveTitlesOf(t *testing.T, service *EventService, clock Clock, filter EventFilter) []string {
	t.Helper()
	listed, err := service.ListActiveEvents(context.Background(), clock, filter)
	if err != nil {
		t.Fatalf("ListActiveEvents(%+v): %v", filter, err)
	}
	return titlesOf(listed)
}

// titlesOf returns the titles of listed in order.
func titlesOf(listed []ListedEvent) []string {
	var titles []string
	for _, event := range listed {
		titles = append(titles, event.Title)
	}
	return titles
}

// IDs of the fixture events the review tests mark. Weihnachtskonzert is
// active and Sommerfest over at every clock of the partition test.
const (
	weihnachtsmarktID   = "0192f0b1-0000-7000-8000-000000000301"
	weihnachtskonzertID = "0192f0b1-0000-7000-8000-000000000304"
	sommerfestID        = "0192f0b1-0000-7000-8000-000000000321"
	nikolausmarktID     = "0192f0b1-0000-7000-8000-000000000322"
)

// christmasEvents are stored events around the clock of the examples.
func christmasEvents(t *testing.T) []Event {
	t.Helper()
	return []Event{
		storedEvent(t, weihnachtsmarktID, "Weihnachtsmarkt",
			EventTimes{StartDate: LocalDate{2026, time.November, 27}, EndDate: christmasEve}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000302", "Krippenspiel",
			EventTimes{StartDate: christmasEve, StartTime: localTime(10, 0), EndDate: christmasEve, EndTime: localTime(14, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000303", "Christmette",
			EventTimes{StartDate: christmasEve, StartTime: localTime(18, 0)}),
		storedEvent(t, weihnachtskonzertID, "Weihnachtskonzert",
			EventTimes{StartDate: december(25), StartTime: localTime(17, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000305", "Adventsbasar",
			EventTimes{StartDate: december(23)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000306", "Silvesterlauf",
			EventTimes{StartDate: december(31), StartTime: localTime(14, 0)}),
	}
}

func TestListActiveEventsWithoutFilterListsTodaysActiveEvents(t *testing.T) {
	repo := newFakeEventRepo(christmasEvents(t)...)

	got := listActiveTitles(t, repo, christmasNoon(t), EventFilter{})

	want := []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}
	if !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
	assertOverlap(t, repo, berlinInstant(t, christmasEve, 12, 0), ptr(berlinInstant(t, december(25), 0, 0)))
}

func TestListActiveEventsDropsAnEventFromTheMinuteItEnds(t *testing.T) {
	for _, test := range []struct {
		hour, minute int
		want         []string
	}{
		{13, 59, []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}},
		{14, 0, []string{"Weihnachtsmarkt", "Christmette"}},
	} {
		repo := newFakeEventRepo(christmasEvents(t)...)
		got := listActiveTitles(t, repo, clockAt(t, christmasEve, test.hour, test.minute), EventFilter{})
		if !slices.Equal(got, test.want) {
			t.Errorf("at %02d:%02d titles = %v, want %v", test.hour, test.minute, got, test.want)
		}
	}
}

func TestListActiveEventsNormalizesThePeriodFilter(t *testing.T) {
	noon := berlinInstant(t, christmasEve, 12, 0)
	tests := []struct {
		name   string
		filter EventFilter
		lo     time.Time
		hi     *time.Time
	}{
		{"date range starts now at the earliest", EventFilter{From: ptr("2026-11-29"), To: ptr("2026-12-24")},
			noon, ptr(berlinInstant(t, december(25), 0, 0))},
		{"instant in to counts its whole minute", EventFilter{To: ptr("2026-12-24T18:00:59.999+01:00")},
			noon, ptr(berlinInstant(t, christmasEve, 18, 1))},
		{"instant in to in another offset", EventFilter{To: ptr("2026-12-24T17:00Z")},
			noon, ptr(berlinInstant(t, christmasEve, 18, 1))},
		{"instant in from and date in to", EventFilter{From: ptr("2026-12-24T18:00+01:00"), To: ptr("2026-12-24")},
			berlinInstant(t, christmasEve, 18, 0), ptr(berlinInstant(t, december(25), 0, 0))},
		{"only from is open-ended", EventFilter{From: ptr("2026-12-30")},
			berlinInstant(t, december(30), 0, 0), nil},
		{"from with seconds", EventFilter{From: ptr("2026-12-30T08:15:30+01:00")},
			berlinInstant(t, december(30), 8, 15).Add(30 * time.Second), nil},
		{"to earlier today is no error", EventFilter{To: ptr("2026-12-24T09:00+01:00")},
			noon, ptr(berlinInstant(t, christmasEve, 9, 1))},
		{"from and to in the past are no error", EventFilter{From: ptr("2026-12-01"), To: ptr("2026-12-02")},
			noon, ptr(berlinInstant(t, december(3), 0, 0))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeEventRepo()
			if _, err := newQueryService(repo).ListActiveEvents(context.Background(), christmasNoon(t), test.filter); err != nil {
				t.Fatalf("ListActiveEvents: %v", err)
			}
			assertOverlap(t, repo, test.lo, test.hi)
		})
	}
}

func TestListActiveEventsMatchesByOverlapOnly(t *testing.T) {
	tests := []struct {
		name   string
		filter EventFilter
		want   []string
	}{
		{"date range", EventFilter{From: ptr("2026-11-29"), To: ptr("2026-12-24")}, []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}},
		{"event starting exactly at to", EventFilter{To: ptr("2026-12-24T18:00+01:00")}, []string{"Weihnachtsmarkt", "Krippenspiel", "Christmette"}},
		{"to one minute before the start", EventFilter{To: ptr("2026-12-24T17:59+01:00")}, []string{"Weihnachtsmarkt", "Krippenspiel"}},
		{"from an instant to a date", EventFilter{From: ptr("2026-12-24T18:00+01:00"), To: ptr("2026-12-24")}, []string{"Weihnachtsmarkt", "Christmette"}},
		{"only from", EventFilter{From: ptr("2026-12-30")}, []string{"Silvesterlauf"}},
		// Only events that still run overlap a past period and are active.
		{"from and to in the past", EventFilter{From: ptr("2026-12-01"), To: ptr("2026-12-02")}, []string{"Weihnachtsmarkt"}},
		{"from and to in the past before every running event", EventFilter{From: ptr("2026-11-01"), To: ptr("2026-11-02")}, nil},
		{"to earlier today", EventFilter{To: ptr("2026-12-24T09:00+01:00")}, []string{"Weihnachtsmarkt"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := listActiveTitles(t, newFakeEventRepo(christmasEvents(t)...), christmasNoon(t), test.filter)
			if !slices.Equal(got, test.want) {
				t.Errorf("titles = %v, want %v", got, test.want)
			}
		})
	}
}

func TestListActiveEventsFiltersByAnyOfTheTypes(t *testing.T) {
	events := christmasEvents(t)
	events[1].Type = EventTypeClub
	events[2].Type = EventTypeCulture
	repo := newFakeEventRepo(events...)

	got := listActiveTitles(t, repo, christmasNoon(t), EventFilter{Types: []string{"market", "club", "market"}})

	want := []string{"Weihnachtsmarkt", "Krippenspiel"}
	if !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestListActiveEventsRejectsInvalidFiltersWithoutAskingTheRepository(t *testing.T) {
	tests := []struct {
		name   string
		filter EventFilter
		want   []FieldError
	}{
		{"german date", EventFilter{From: ptr("24.12.2026")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"nonexistent date", EventFilter{From: ptr("2026-02-30")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"instant without offset", EventFilter{To: ptr("2026-12-24T18:00")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemInvalidFormat}}},
		{"offset without colon", EventFilter{To: ptr("2026-12-24T18:00+0100")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemInvalidFormat}}},
		{"hour with one digit", EventFilter{From: ptr("2026-12-24T8:00+01:00")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"hour out of range", EventFilter{From: ptr("2026-12-24T24:00+01:00")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"nonexistent day in an instant", EventFilter{From: ptr("2026-02-30T10:00Z")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"lower-case separator", EventFilter{From: ptr("2026-12-24t18:00z")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"from given empty", EventFilter{From: ptr("")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"to given empty", EventFilter{To: ptr("")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemInvalidFormat}}},
		{"from and to given empty", EventFilter{From: ptr(""), To: ptr("")},
			[]FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}, {Field: FilterFieldTo, Problem: ProblemInvalidFormat}}},
		{"unknown type", EventFilter{Types: []string{"market", "foo"}}, []FieldError{{Field: FilterFieldType, Problem: ProblemUnknownCode}}},
		{"empty type", EventFilter{Types: []string{""}}, []FieldError{{Field: FilterFieldType, Problem: ProblemMissing}}},
		{"every parameter", EventFilter{From: ptr("morgen"), To: ptr("übermorgen"), Types: []string{"foo"}}, []FieldError{
			{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}, {Field: FilterFieldTo, Problem: ProblemInvalidFormat}, {Field: FilterFieldType, Problem: ProblemUnknownCode},
		}},
		{"empty period", EventFilter{From: ptr("2026-12-28"), To: ptr("2026-12-27")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemEmptyPeriod}}},
		{"empty period of instants", EventFilter{From: ptr("2026-12-28T18:01+01:00"), To: ptr("2026-12-28T18:00+01:00")},
			[]FieldError{{Field: FilterFieldTo, Problem: ProblemEmptyPeriod}}},
		{"only to before today", EventFilter{To: ptr("2026-12-23")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemBeforeToday}}},
		{"only to just before today", EventFilter{To: ptr("2026-12-23T23:59+01:00")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemBeforeToday}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeEventRepo(christmasEvents(t)...)

			_, err := newQueryService(repo, hall()).ListActiveEvents(context.Background(), christmasNoon(t), test.filter)

			var validation *ValidationError
			if !errors.As(err, &validation) || !errors.Is(err, ErrValidation) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if !slices.Equal(validation.Fields, test.want) {
				t.Errorf("fields = %v, want %v", validation.Fields, test.want)
			}
			if len(repo.overlaps) != 0 {
				t.Errorf("repository was asked for %v", repo.overlaps)
			}
		})
	}
}

func TestListActiveEventsAcceptsAPeriodOfOneMinute(t *testing.T) {
	repo := newFakeEventRepo(christmasEvents(t)...)

	got := listActiveTitles(t, repo, christmasNoon(t), EventFilter{From: ptr("2026-12-24T18:00+01:00"), To: ptr("2026-12-24T18:00+01:00")})

	if want := []string{"Weihnachtsmarkt", "Christmette"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestListActiveEventsSortsByEffectiveStartThenID(t *testing.T) {
	laterB := storedEvent(t, "0192f0b1-0000-7000-8000-000000000312", "Konzert B", EventTimes{StartDate: christmasEve, StartTime: localTime(19, 0)})
	laterA := storedEvent(t, "0192f0b1-0000-7000-8000-000000000311", "Konzert A", EventTimes{StartDate: christmasEve, StartTime: localTime(19, 0)})
	earlier := storedEvent(t, "0192f0b1-0000-7000-8000-000000000313", "Andacht", EventTimes{StartDate: christmasEve, StartTime: localTime(16, 0)})
	repo := newFakeEventRepo(laterB, laterA, earlier)

	got := listActiveTitles(t, repo, christmasNoon(t), EventFilter{})

	if want := []string{"Andacht", "Konzert A", "Konzert B"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestListActiveEventsCompletesEachEvent(t *testing.T) {
	market := storedEvent(t, marketID, "Weihnachtsmarkt", EventTimes{StartDate: december(20), EndDate: christmasEve})
	market.Timetable = []TimetableEntry{
		{ID: "b", Description: "Posaunenchor", Date: christmasEve, StartTime: localTime(16, 0)},
		{ID: "a", Description: "Eröffnung", Date: december(20), StartTime: localTime(17, 0)},
	}
	concert := storedEvent(t, concertID, "Konzert", EventTimes{StartDate: christmasEve, StartTime: localTime(19, 0)})
	inThePark := storedEvent(t, newEventID, "Feuerschale", EventTimes{StartDate: christmasEve, AllDay: true})
	inThePark.LocationID = parkID
	repo := newFakeEventRepo(market, concert, inThePark)

	listed, err := newQueryService(repo, hall(), park()).ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{})
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}

	byTitle := map[string]ListedEvent{}
	for _, event := range listed {
		byTitle[event.Title] = event
		if event.Archived {
			t.Errorf("%s is archived", event.Title)
		}
		if zone := event.Period.Start.Location().String(); zone != berlinZoneName || event.Period.End.Location().String() != berlinZoneName {
			t.Errorf("%s period in zone %s, want %s", event.Title, zone, berlinZoneName)
		}
	}
	if byTitle["Weihnachtsmarkt"].Location != hall() || byTitle["Konzert"].Location != hall() || byTitle["Feuerschale"].Location != park() {
		t.Errorf("locations = %+v", byTitle)
	}
	if got := byTitle["Weihnachtsmarkt"].Timetable; len(got) != 2 || got[0].Description != "Eröffnung" {
		t.Errorf("timetable = %+v, want it sorted chronologically", got)
	}
	precisions := map[string][2]TimePrecision{
		"Weihnachtsmarkt": {TimePrecisionDateOnly, TimePrecisionDateOnly},
		"Konzert":         {TimePrecisionExact, ""},
		"Feuerschale":     {TimePrecisionAllDay, ""},
	}
	for title, want := range precisions {
		if got := [2]TimePrecision{byTitle[title].StartPrecision, byTitle[title].EndPrecision}; got != want {
			t.Errorf("%s precisions = %v, want %v", title, got, want)
		}
	}
}

func TestListActiveEventsShowsChangedCoordinatesForEveryEventAtTheLocation(t *testing.T) {
	repo := newFakeEventRepo(christmasEvents(t)...)
	locations := newFakeLocationRepo(hall())
	service := NewEventService(&fakeTx{}, repo, locations)
	moved := hall()
	moved.Latitude, moved.Longitude = 49.4431, 10.9541
	locations.locations[hallID] = moved

	listed, err := service.ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{})
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}
	if len(listed) < 2 {
		t.Fatalf("listed %d events, want at least two at the hall", len(listed))
	}
	for _, event := range listed {
		if event.Location != moved {
			t.Errorf("%s location = %+v, want %+v", event.Title, event.Location, moved)
		}
	}
}

func TestListActiveEventsPassesFailuresOn(t *testing.T) {
	t.Run("events", func(t *testing.T) {
		repo := newFakeEventRepo()
		repo.overlapErr = errDatabaseDown
		_, err := newQueryService(repo, hall()).ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{})
		if !errors.Is(err, errDatabaseDown) {
			t.Errorf("err = %v, want %v", err, errDatabaseDown)
		}
	})
	t.Run("locations", func(t *testing.T) {
		locations := newFakeLocationRepo()
		locations.listErr = errDatabaseDown
		_, err := NewEventService(&fakeTx{}, newFakeEventRepo(), locations).ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{})
		if !errors.Is(err, errDatabaseDown) {
			t.Errorf("err = %v, want %v", err, errDatabaseDown)
		}
	})
	t.Run("location of an event missing", func(t *testing.T) {
		repo := newFakeEventRepo(christmasEvents(t)...)
		_, err := newQueryService(repo).ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{})
		if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), hallID) {
			t.Errorf("err = %v, want ErrNotFound naming %s", err, hallID)
		}
	})
}

// assertOverlap checks the one overlap the repository was asked for, with
// a lower bound.
func assertOverlap(t *testing.T, repo *fakeEventRepo, lo time.Time, hi *time.Time) {
	t.Helper()
	assertOverlapBounds(t, repo, &lo, hi)
}

// assertOverlapBounds checks the one overlap the repository was asked for;
// a nil bound is open.
func assertOverlapBounds(t *testing.T, repo *fakeEventRepo, lo, hi *time.Time) {
	t.Helper()
	if len(repo.overlaps) != 1 {
		t.Fatalf("overlaps = %v, want exactly one", repo.overlaps)
	}
	got := repo.overlaps[0]
	if !sameBound(got.Lo, lo) {
		t.Errorf("lo = %v, want %v", got.Lo, lo)
	}
	if !sameBound(got.Hi, hi) {
		t.Errorf("hi = %v, want %v", got.Hi, hi)
	}
}

// sameBound reports whether two bounds are both open or the same instant.
func sameBound(got, want *time.Time) bool {
	return (got == nil) == (want == nil) && (want == nil || got.Equal(*want))
}

func ptr[T any](value T) *T { return &value }

func TestListActiveEventsListsThePeriodInEuropeBerlin(t *testing.T) {
	event := storedEvent(t, marketID, "Christmette", EventTimes{StartDate: christmasEve, StartTime: localTime(18, 0)})
	event.Period = Period{Start: event.Period.Start.UTC(), End: event.Period.End.UTC()}

	listed, err := newQueryService(newFakeEventRepo(event), hall()).ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{})
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("listed %d events, want 1", len(listed))
	}
	period := listed[0].Period
	if period.Start.Location().String() != berlinZoneName || period.End.Location().String() != berlinZoneName {
		t.Errorf("period zones = %s, %s, want %s", period.Start.Location(), period.End.Location(), berlinZoneName)
	}
	if !period.Start.Equal(event.Period.Start) || !period.End.Equal(event.Period.End) {
		t.Errorf("period = [%v, %v), want the same instants as [%v, %v)", period.Start, period.End, event.Period.Start, event.Period.End)
	}
}

func TestListActiveEventsTakesTodayFromEuropeBerlinForAClockInAnotherZone(t *testing.T) {
	// 2026-12-23T23:30Z is 2026-12-24 00:30 in Europe/Berlin.
	clock := fixedClock(time.Date(2026, time.December, 23, 23, 30, 0, 0, time.UTC))

	t.Run("default period", func(t *testing.T) {
		repo := newFakeEventRepo()
		if _, err := newQueryService(repo).ListActiveEvents(context.Background(), clock, EventFilter{}); err != nil {
			t.Fatalf("ListActiveEvents: %v", err)
		}
		assertOverlap(t, repo, clock.Now(), ptr(berlinInstant(t, december(25), 0, 0)))
	})
	t.Run("to today is valid", func(t *testing.T) {
		repo := newFakeEventRepo()
		if _, err := newQueryService(repo).ListActiveEvents(context.Background(), clock, EventFilter{To: ptr("2026-12-24")}); err != nil {
			t.Fatalf("ListActiveEvents: %v", err)
		}
		assertOverlap(t, repo, clock.Now(), ptr(berlinInstant(t, december(25), 0, 0)))
	})
	t.Run("to yesterday is before today", func(t *testing.T) {
		_, err := newQueryService(newFakeEventRepo()).ListActiveEvents(context.Background(), clock, EventFilter{To: ptr("2026-12-23")})
		var validation *ValidationError
		if !errors.As(err, &validation) || !slices.Equal(validation.Fields, []FieldError{{Field: FilterFieldTo, Problem: ProblemBeforeToday}}) {
			t.Errorf("err = %v, want to beforeToday", err)
		}
	})
}

func TestListActiveEventsCoversTheWholeDayWhenDaylightSavingTimeEnds(t *testing.T) {
	repo := newFakeEventRepo()
	clock := clockAt(t, LocalDate{2026, time.October, 20}, 12, 0)

	_, err := newQueryService(repo).ListActiveEvents(context.Background(), clock, EventFilter{From: ptr("2026-10-25"), To: ptr("2026-10-25")})
	if err != nil {
		t.Fatalf("ListActiveEvents: %v", err)
	}

	summer, winter := time.FixedZone("CEST", 2*60*60), time.FixedZone("CET", 60*60)
	lo := time.Date(2026, time.October, 25, 0, 0, 0, 0, summer)
	hi := time.Date(2026, time.October, 26, 0, 0, 0, 0, winter)
	assertOverlap(t, repo, lo, &hi)
	if hours := hi.Sub(lo).Hours(); hours != 25 {
		t.Errorf("period lasts %v hours, want 25", hours)
	}
}

// archiveEvents are christmasEvents plus events that ended before, at and
// after the clock of the examples.
func archiveEvents(t *testing.T) []Event {
	t.Helper()
	sommerfest := storedEvent(t, sommerfestID, "Sommerfest",
		EventTimes{StartDate: LocalDate{2026, time.June, 20}})
	sommerfest.Type = EventTypeFestival
	return append(christmasEvents(t),
		sommerfest,
		storedEvent(t, nikolausmarktID, "Nikolausmarkt",
			EventTimes{StartDate: december(5), EndDate: december(6)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000323", "Frühschoppen",
			EventTimes{StartDate: christmasEve, StartTime: localTime(10, 0), EndDate: christmasEve, EndTime: localTime(11, 59)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000324", "Mittagsläuten",
			EventTimes{StartDate: christmasEve, StartTime: localTime(11, 0), EndDate: christmasEve, EndTime: localTime(12, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000325", "Bescherung",
			EventTimes{StartDate: christmasEve, StartTime: localTime(11, 30), EndDate: christmasEve, EndTime: localTime(12, 1)}),
	)
}

// pastTitles are the titles of archiveEvents that are over at noon, by
// effective start descending.
var pastTitles = []string{"Mittagsläuten", "Frühschoppen", "Adventsbasar", "Nikolausmarkt", "Sommerfest"}

// listArchivedTitles runs ListArchivedEvents and returns the titles in
// order.
func listArchivedTitles(t *testing.T, repo *fakeEventRepo, clock Clock, filter EventFilter) []string {
	t.Helper()
	return listArchivedTitlesOf(t, newQueryService(repo, hall(), park()), clock, filter)
}

// listArchivedTitlesOf runs ListArchivedEvents of service and returns the
// titles in order.
func listArchivedTitlesOf(t *testing.T, service *EventService, clock Clock, filter EventFilter) []string {
	t.Helper()
	listed, err := service.ListArchivedEvents(context.Background(), clock, filter)
	if err != nil {
		t.Fatalf("ListArchivedEvents(%+v): %v", filter, err)
	}
	return titlesOf(listed)
}

func TestListArchivedEventsWithoutFilterListsEveryPastEventLatestFirst(t *testing.T) {
	repo := newFakeEventRepo(archiveEvents(t)...)

	got := listArchivedTitles(t, repo, christmasNoon(t), EventFilter{})

	if !slices.Equal(got, pastTitles) {
		t.Errorf("titles = %v, want %v", got, pastTitles)
	}
	assertOverlapBounds(t, repo, nil, ptr(berlinInstant(t, christmasEve, 12, 0)))
}

func TestListArchivedEventsContainsAnEventFromTheMinuteItEnds(t *testing.T) {
	got := listArchivedTitles(t, newFakeEventRepo(archiveEvents(t)...), christmasNoon(t), EventFilter{})

	for _, test := range []struct {
		title string
		want  bool
	}{
		{"Frühschoppen", true},  // ended 11:59
		{"Mittagsläuten", true}, // ends 12:00, now
		{"Bescherung", false},   // ends 12:01
	} {
		if slices.Contains(got, test.title) != test.want {
			t.Errorf("%s in archive = %v, want %v", test.title, !test.want, test.want)
		}
	}
}

func TestListArchivedEventsNormalizesThePeriodFilter(t *testing.T) {
	noon := berlinInstant(t, christmasEve, 12, 0)
	tests := []struct {
		name   string
		filter EventFilter
		lo     *time.Time
		hi     *time.Time
	}{
		{"only to is open at the start", EventFilter{To: ptr("2026-06-30")}, nil, ptr(berlinInstant(t, LocalDate{2026, time.July, 1}, 0, 0))},
		{"only from ends now", EventFilter{From: ptr("2026-12-01")}, ptr(berlinInstant(t, december(1), 0, 0)), &noon},
		{"instant in to counts its whole minute", EventFilter{To: ptr("2026-12-24T09:00+01:00")}, nil, ptr(berlinInstant(t, christmasEve, 9, 1))},
		{"from and to in the future", EventFilter{From: ptr("2027-01-01"), To: ptr("2027-01-31")},
			ptr(berlinInstant(t, LocalDate{2027, time.January, 1}, 0, 0)), ptr(berlinInstant(t, LocalDate{2027, time.February, 1}, 0, 0))},
		{"from just before now", EventFilter{From: ptr("2026-12-24T11:59+01:00")}, ptr(berlinInstant(t, christmasEve, 11, 59)), &noon},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeEventRepo()
			if _, err := newQueryService(repo).ListArchivedEvents(context.Background(), christmasNoon(t), test.filter); err != nil {
				t.Fatalf("ListArchivedEvents: %v", err)
			}
			assertOverlapBounds(t, repo, test.lo, test.hi)
		})
	}
}

func TestListArchivedEventsMatchesPastEventsByOverlap(t *testing.T) {
	tests := []struct {
		name   string
		filter EventFilter
		want   []string
	}{
		{"only to", EventFilter{To: ptr("2026-06-30")}, []string{"Sommerfest"}},
		{"only to on the day of an event", EventFilter{To: ptr("2026-06-20")}, []string{"Sommerfest"}},
		{"only to before every event", EventFilter{To: ptr("2026-06-19")}, nil},
		{"only from", EventFilter{From: ptr("2026-12-01")}, []string{"Mittagsläuten", "Frühschoppen", "Adventsbasar", "Nikolausmarkt"}},
		{"from on the last day of an event", EventFilter{From: ptr("2026-12-06")}, []string{"Mittagsläuten", "Frühschoppen", "Adventsbasar", "Nikolausmarkt"}},
		{"from and to", EventFilter{From: ptr("2026-12-06"), To: ptr("2026-12-23")}, []string{"Adventsbasar", "Nikolausmarkt"}},
		{"to in the future", EventFilter{To: ptr("2027-01-31")}, pastTitles},
		{"from and to in the future", EventFilter{From: ptr("2027-01-01"), To: ptr("2027-01-31")}, nil},
		{"running events overlapping the period are left out", EventFilter{From: ptr("2026-12-24T11:30+01:00"), To: ptr("2026-12-24")},
			[]string{"Mittagsläuten", "Frühschoppen"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := listArchivedTitles(t, newFakeEventRepo(archiveEvents(t)...), christmasNoon(t), test.filter)
			if !slices.Equal(got, test.want) {
				t.Errorf("titles = %v, want %v", got, test.want)
			}
		})
	}
}

func TestListArchivedEventsFiltersByAnyOfTheTypes(t *testing.T) {
	got := listArchivedTitles(t, newFakeEventRepo(archiveEvents(t)...), christmasNoon(t),
		EventFilter{Types: []string{"festival", "festival"}})

	if want := []string{"Sommerfest"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestListArchivedEventsRejectsInvalidFiltersWithoutAskingTheRepository(t *testing.T) {
	tests := []struct {
		name   string
		filter EventFilter
		want   []FieldError
	}{
		{"german date", EventFilter{From: ptr("24.12.2026")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"instant without offset", EventFilter{To: ptr("2026-12-24T18:00")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemInvalidFormat}}},
		{"from given empty", EventFilter{From: ptr("")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}}},
		{"unknown type", EventFilter{Types: []string{"foo"}}, []FieldError{{Field: FilterFieldType, Problem: ProblemUnknownCode}}},
		{"empty type", EventFilter{Types: []string{""}}, []FieldError{{Field: FilterFieldType, Problem: ProblemMissing}}},
		{"every parameter", EventFilter{From: ptr("gestern"), To: ptr(""), Types: []string{"foo"}}, []FieldError{
			{Field: FilterFieldFrom, Problem: ProblemInvalidFormat}, {Field: FilterFieldTo, Problem: ProblemInvalidFormat}, {Field: FilterFieldType, Problem: ProblemUnknownCode},
		}},
		{"empty period", EventFilter{From: ptr("2026-12-28"), To: ptr("2026-12-27")}, []FieldError{{Field: FilterFieldTo, Problem: ProblemEmptyPeriod}}},
		{"only from after now", EventFilter{From: ptr("2027-01-01")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemAfterNow}}},
		{"only from at now", EventFilter{From: ptr("2026-12-24T12:00+01:00")}, []FieldError{{Field: FilterFieldFrom, Problem: ProblemAfterNow}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeEventRepo(archiveEvents(t)...)

			_, err := newQueryService(repo, hall()).ListArchivedEvents(context.Background(), christmasNoon(t), test.filter)

			var validation *ValidationError
			if !errors.As(err, &validation) || !errors.Is(err, ErrValidation) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if !slices.Equal(validation.Fields, test.want) {
				t.Errorf("fields = %v, want %v", validation.Fields, test.want)
			}
			if len(repo.overlaps) != 0 {
				t.Errorf("repository was asked for %v", repo.overlaps)
			}
		})
	}
}

func TestListArchivedEventsSortsByEffectiveStartDescendingThenID(t *testing.T) {
	earlierB := storedEvent(t, "0192f0b1-0000-7000-8000-000000000332", "Probe B", EventTimes{StartDate: december(22), StartTime: localTime(19, 0)})
	earlierA := storedEvent(t, "0192f0b1-0000-7000-8000-000000000331", "Probe A", EventTimes{StartDate: december(22), StartTime: localTime(19, 0)})
	later := storedEvent(t, "0192f0b1-0000-7000-8000-000000000333", "Generalprobe", EventTimes{StartDate: december(23), StartTime: localTime(19, 0)})
	repo := newFakeEventRepo(earlierB, later, earlierA)

	got := listArchivedTitles(t, repo, christmasNoon(t), EventFilter{})

	if want := []string{"Generalprobe", "Probe A", "Probe B"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestListArchivedEventsCompletesEachEventAsArchived(t *testing.T) {
	market := storedEvent(t, marketID, "Nikolausmarkt", EventTimes{StartDate: december(5), EndDate: december(6)})
	market.Timetable = []TimetableEntry{
		{ID: "b", Description: "Nikolaus", Date: december(6), StartTime: localTime(16, 0)},
		{ID: "a", Description: "Eröffnung", Date: december(5), StartTime: localTime(17, 0)},
	}
	market.Period = Period{Start: market.Period.Start.UTC(), End: market.Period.End.UTC()}

	listed, err := newQueryService(newFakeEventRepo(market), hall()).ListArchivedEvents(context.Background(), christmasNoon(t), EventFilter{})
	if err != nil {
		t.Fatalf("ListArchivedEvents: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("listed %d events, want 1", len(listed))
	}
	got := listed[0]
	if !got.Archived || got.Location != hall() {
		t.Errorf("archived = %v, location = %+v, want archived at the hall", got.Archived, got.Location)
	}
	if got.Timetable[0].Description != "Eröffnung" {
		t.Errorf("timetable = %+v, want it sorted chronologically", got.Timetable)
	}
	if got.StartPrecision != TimePrecisionDateOnly || got.EndPrecision != TimePrecisionDateOnly {
		t.Errorf("precisions = %v, %v, want dateOnly", got.StartPrecision, got.EndPrecision)
	}
	if got.Period.Start.Location().String() != berlinZoneName {
		t.Errorf("period zone = %s, want %s", got.Period.Start.Location(), berlinZoneName)
	}
}

func TestEveryEventIsInExactlyOneOfActiveAndArchive(t *testing.T) {
	marked := map[string]bool{weihnachtskonzertID: true, sommerfestID: true}
	for _, clock := range []Clock{
		christmasNoon(t),
		clockAt(t, christmasEve, 11, 59),
		clockAt(t, christmasEve, 14, 0),
		clockAt(t, december(25), 0, 0),
	} {
		events := archiveEvents(t)
		// Both lists from one service, so both see the same review marks.
		service := newQueryService(newFakeEventRepo(events...), hall(), park())
		for id := range marked {
			service.markForReview(id)
		}
		active := listActiveTitlesOf(t, service, clock, EventFilter{From: ptr("1900-01-01")})
		archived := listArchivedTitlesOf(t, service, clock, EventFilter{})
		for _, event := range events {
			inActive, inArchive := slices.Contains(active, event.Title), slices.Contains(archived, event.Title)
			if marked[event.ID] {
				if inActive || inArchive {
					t.Errorf("at %v marked %s active = %v, archived = %v, want neither", clock.Now(), event.Title, inActive, inArchive)
				}
				continue
			}
			if inActive == inArchive {
				t.Errorf("at %v %s active = %v, archived = %v, want exactly one", clock.Now(), event.Title, inActive, inArchive)
			}
		}
	}
}

func TestListActiveEventsLeavesOutEventsMarkedForReview(t *testing.T) {
	service := newQueryService(newFakeEventRepo(archiveEvents(t)...), hall(), park())
	service.markForReview(weihnachtsmarktID)

	for _, filter := range []EventFilter{
		{},
		{From: ptr("2026-11-29"), To: ptr("2026-12-24")},
		{Types: []string{string(EventTypeMarket)}},
	} {
		got := listActiveTitlesOf(t, service, christmasNoon(t), filter)
		if slices.Contains(got, "Weihnachtsmarkt") {
			t.Errorf("filter %+v: titles = %v, want Weihnachtsmarkt left out", filter, got)
		}
		if !slices.Contains(got, "Krippenspiel") {
			t.Errorf("filter %+v: titles = %v, want the unmarked Krippenspiel", filter, got)
		}
	}
	assertNeedsReview(t, service, map[string]bool{weihnachtsmarktID: true})
}

func TestListArchivedEventsLeavesOutEventsMarkedForReview(t *testing.T) {
	service := newQueryService(newFakeEventRepo(archiveEvents(t)...), hall(), park())
	service.markForReview(nikolausmarktID)

	for _, filter := range []EventFilter{
		{},
		{From: ptr("2026-12-01"), To: ptr("2026-12-24")},
		{Types: []string{string(EventTypeMarket)}},
	} {
		got := listArchivedTitlesOf(t, service, christmasNoon(t), filter)
		if slices.Contains(got, "Nikolausmarkt") {
			t.Errorf("filter %+v: titles = %v, want Nikolausmarkt left out", filter, got)
		}
		if !slices.Contains(got, "Adventsbasar") {
			t.Errorf("filter %+v: titles = %v, want the unmarked Adventsbasar", filter, got)
		}
	}
	assertNeedsReview(t, service, map[string]bool{nikolausmarktID: true})
}

// publicListsHolding reports which public lists of service at clock hold
// an event titled title.
func publicListsHolding(t *testing.T, service *EventService, clock Clock, title string) (inActive, inArchive bool) {
	t.Helper()
	active := listActiveTitlesOf(t, service, clock, EventFilter{From: ptr("1900-01-01")})
	archived := listArchivedTitlesOf(t, service, clock, EventFilter{})
	return slices.Contains(active, title), slices.Contains(archived, title)
}

// assertInNoPublicList fails when an event titled title is in a public
// list of service at any of the clocks.
func assertInNoPublicList(t *testing.T, service *EventService, title string, clocks ...Clock) {
	t.Helper()
	for _, clock := range clocks {
		if inActive, inArchive := publicListsHolding(t, service, clock, title); inActive || inArchive {
			t.Errorf("at %v %s active = %v, archived = %v, want neither", clock.Now(), title, inActive, inArchive)
		}
	}
}

func TestAnEventFailingRecomputationIsPublicAgainOnlyAfterASuccessfulSave(t *testing.T) {
	repo := newFakeEventRepo(brokenEvent(t, marketID))
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	during, after := clockAt(t, kirchweihFriday, 12, 0), clockAt(t, kirchweihMonday, 12, 0)
	if _, err := service.RecomputeDerived(ctx); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}
	assertInNoPublicList(t, service, "Konzert", during, after)

	repo.writeErr = errDatabaseDown
	if _, err := service.SaveEvent(ctx, marketID, validEventInput(), RejectDuplicates); !errors.Is(err, errDatabaseDown) {
		t.Fatalf("failing SaveEvent: err = %v, want %v", err, errDatabaseDown)
	}
	repo.writeErr = nil
	assertInNoPublicList(t, service, "Konzert", during, after)

	if _, err := service.SaveEvent(ctx, marketID, validEventInput(), RejectDuplicates); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	for _, test := range []struct {
		clock                   Clock
		wantActive, wantArchive bool
	}{
		{during, true, false},
		{after, false, true},
	} {
		inActive, inArchive := publicListsHolding(t, service, test.clock, "Kirchweihmarkt")
		if inActive != test.wantActive || inArchive != test.wantArchive {
			t.Errorf("at %v saved event active = %v, archived = %v, want %v, %v",
				test.clock.Now(), inActive, inArchive, test.wantActive, test.wantArchive)
		}
	}
}

func TestADeletedEventMarkedForReviewIsInNoPublicList(t *testing.T) {
	repo := newFakeEventRepo(brokenEvent(t, marketID))
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	if _, err := service.RecomputeDerived(ctx); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}

	if err := service.DeleteEvent(ctx, marketID); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if service.needsReview(marketID) {
		t.Error("the deleted event is still marked for review")
	}
	assertInNoPublicList(t, service, "Konzert", clockAt(t, kirchweihFriday, 12, 0), clockAt(t, kirchweihMonday, 12, 0))
}

func TestPublicListsKeepAnEventHiddenWhoseMarkIsClearedWhileReading(t *testing.T) {
	repo := newFakeEventRepo(archiveEvents(t)...)
	service := newQueryService(repo, hall(), park())
	service.markForReview(weihnachtsmarktID)
	service.markForReview(nikolausmarktID)
	// A save committing while the repository reads clears the mark, but
	// the rows read still hold the values from before the save.
	repo.duringOverlap = func() {
		service.clearReview(weihnachtsmarktID)
		service.clearReview(nikolausmarktID)
	}
	if got := listActiveTitlesOf(t, service, christmasNoon(t), EventFilter{}); slices.Contains(got, "Weihnachtsmarkt") {
		t.Errorf("active titles = %v, want Weihnachtsmarkt left out", got)
	}

	service.markForReview(nikolausmarktID)
	if got := listArchivedTitlesOf(t, service, christmasNoon(t), EventFilter{}); slices.Contains(got, "Nikolausmarkt") {
		t.Errorf("archived titles = %v, want Nikolausmarkt left out", got)
	}
}

func TestPublicListsReportTheMissingLocationOfAMarkedEvent(t *testing.T) {
	var marked []Event
	for _, event := range archiveEvents(t) {
		if event.ID == weihnachtsmarktID || event.ID == nikolausmarktID {
			marked = append(marked, event)
		}
	}
	// No location is stored, and only marked events are.
	service := newQueryService(newFakeEventRepo(marked...))
	service.markForReview(weihnachtsmarktID)
	service.markForReview(nikolausmarktID)

	if _, err := service.ListActiveEvents(context.Background(), christmasNoon(t), EventFilter{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("ListActiveEvents: err = %v, want ErrNotFound", err)
	}
	if _, err := service.ListArchivedEvents(context.Background(), christmasNoon(t), EventFilter{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("ListArchivedEvents: err = %v, want ErrNotFound", err)
	}
}

func TestListArchivedEventsPassesFailuresOn(t *testing.T) {
	t.Run("events", func(t *testing.T) {
		repo := newFakeEventRepo()
		repo.overlapErr = errDatabaseDown
		_, err := newQueryService(repo, hall()).ListArchivedEvents(context.Background(), christmasNoon(t), EventFilter{})
		if !errors.Is(err, errDatabaseDown) {
			t.Errorf("err = %v, want %v", err, errDatabaseDown)
		}
	})
	t.Run("location of an event missing", func(t *testing.T) {
		_, err := newQueryService(newFakeEventRepo(archiveEvents(t)...)).ListArchivedEvents(context.Background(), christmasNoon(t), EventFilter{})
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}
