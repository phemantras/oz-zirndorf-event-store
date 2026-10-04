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
	listed, err := newQueryService(repo, hall(), park()).ListActiveEvents(context.Background(), clock, filter)
	if err != nil {
		t.Fatalf("ListActiveEvents(%+v): %v", filter, err)
	}
	var titles []string
	for _, event := range listed {
		titles = append(titles, event.Title)
	}
	return titles
}

// christmasEvents are stored events around the clock of the examples.
func christmasEvents(t *testing.T) []Event {
	t.Helper()
	return []Event{
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000301", "Weihnachtsmarkt",
			EventTimes{StartDate: LocalDate{2026, time.November, 27}, EndDate: christmasEve}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000302", "Krippenspiel",
			EventTimes{StartDate: christmasEve, StartTime: localTime(10, 0), EndDate: christmasEve, EndTime: localTime(14, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000303", "Christmette",
			EventTimes{StartDate: christmasEve, StartTime: localTime(18, 0)}),
		storedEvent(t, "0192f0b1-0000-7000-8000-000000000304", "Weihnachtskonzert",
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
		{"german date", EventFilter{From: ptr("24.12.2026")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"nonexistent date", EventFilter{From: ptr("2026-02-30")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"instant without offset", EventFilter{To: ptr("2026-12-24T18:00")}, []FieldError{{FilterFieldTo, ProblemInvalidFormat}}},
		{"offset without colon", EventFilter{To: ptr("2026-12-24T18:00+0100")}, []FieldError{{FilterFieldTo, ProblemInvalidFormat}}},
		{"hour with one digit", EventFilter{From: ptr("2026-12-24T8:00+01:00")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"hour out of range", EventFilter{From: ptr("2026-12-24T24:00+01:00")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"nonexistent day in an instant", EventFilter{From: ptr("2026-02-30T10:00Z")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"lower-case separator", EventFilter{From: ptr("2026-12-24t18:00z")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"from given empty", EventFilter{From: ptr("")}, []FieldError{{FilterFieldFrom, ProblemInvalidFormat}}},
		{"to given empty", EventFilter{To: ptr("")}, []FieldError{{FilterFieldTo, ProblemInvalidFormat}}},
		{"from and to given empty", EventFilter{From: ptr(""), To: ptr("")},
			[]FieldError{{FilterFieldFrom, ProblemInvalidFormat}, {FilterFieldTo, ProblemInvalidFormat}}},
		{"unknown type", EventFilter{Types: []string{"market", "foo"}}, []FieldError{{FilterFieldType, ProblemUnknownCode}}},
		{"empty type", EventFilter{Types: []string{""}}, []FieldError{{FilterFieldType, ProblemMissing}}},
		{"every parameter", EventFilter{From: ptr("morgen"), To: ptr("übermorgen"), Types: []string{"foo"}}, []FieldError{
			{FilterFieldFrom, ProblemInvalidFormat}, {FilterFieldTo, ProblemInvalidFormat}, {FilterFieldType, ProblemUnknownCode},
		}},
		{"empty period", EventFilter{From: ptr("2026-12-28"), To: ptr("2026-12-27")}, []FieldError{{FilterFieldTo, ProblemEmptyPeriod}}},
		{"empty period of instants", EventFilter{From: ptr("2026-12-28T18:01+01:00"), To: ptr("2026-12-28T18:00+01:00")},
			[]FieldError{{FilterFieldTo, ProblemEmptyPeriod}}},
		{"only to before today", EventFilter{To: ptr("2026-12-23")}, []FieldError{{FilterFieldTo, ProblemBeforeToday}}},
		{"only to just before today", EventFilter{To: ptr("2026-12-23T23:59+01:00")}, []FieldError{{FilterFieldTo, ProblemBeforeToday}}},
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

// assertOverlap checks the one overlap the repository was asked for.
func assertOverlap(t *testing.T, repo *fakeEventRepo, lo time.Time, hi *time.Time) {
	t.Helper()
	if len(repo.overlaps) != 1 {
		t.Fatalf("overlaps = %v, want exactly one", repo.overlaps)
	}
	got := repo.overlaps[0]
	if !got.Lo.Equal(lo) {
		t.Errorf("lo = %v, want %v", got.Lo, lo)
	}
	if (got.Hi == nil) != (hi == nil) || (hi != nil && !got.Hi.Equal(*hi)) {
		t.Errorf("hi = %v, want %v", got.Hi, hi)
	}
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
		if !errors.As(err, &validation) || !slices.Equal(validation.Fields, []FieldError{{FilterFieldTo, ProblemBeforeToday}}) {
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
