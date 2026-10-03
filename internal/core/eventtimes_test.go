package core

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

var (
	kirchweihFriday = LocalDate{2026, time.October, 16}
	kirchweihMonday = LocalDate{2026, time.October, 19}
	autumnChangeDay = LocalDate{2026, time.October, 25}
	springChangeDay = LocalDate{2027, time.March, 28}
)

// berlinInstant returns the instant of a Berlin wall clock for expectations.
// It must not be used for values in the spring gap or the repeated hour,
// where time.Date does not specify the result.
func berlinInstant(t *testing.T, date LocalDate, hour, minute int) time.Time {
	t.Helper()
	return time.Date(date.Year, date.Month, date.Day, hour, minute, 0, 0, berlin)
}

func TestEventTimesDerivesPrecisions(t *testing.T) {
	tests := []struct {
		name      string
		times     EventTimes
		wantStart TimePrecision
		wantEnd   TimePrecision
	}{
		{"only start date", EventTimes{StartDate: kirchweihFriday}, TimePrecisionDateOnly, ""},
		{"exact start, no end", EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0)}, TimePrecisionExact, ""},
		{"exact until date", EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0), EndDate: kirchweihFriday}, TimePrecisionExact, TimePrecisionDateOnly},
		{"date until exact", EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihMonday, EndTime: localTime(22, 0)}, TimePrecisionDateOnly, TimePrecisionExact},
		{"all day", EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihMonday, AllDay: true}, TimePrecisionAllDay, TimePrecisionAllDay},
		{"all day without end", EventTimes{StartDate: kirchweihFriday, AllDay: true}, TimePrecisionAllDay, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.times.StartPrecision(); got != test.wantStart {
				t.Errorf("StartPrecision() = %q, want %q", got, test.wantStart)
			}
			if got := test.times.EndPrecision(); got != test.wantEnd {
				t.Errorf("EndPrecision() = %q, want %q", got, test.wantEnd)
			}
		})
	}
}

func TestTimePrecisionsAreTheThreeCodes(t *testing.T) {
	want := []TimePrecision{"exact", "dateOnly", "allDay"}
	if got := TimePrecisions(); !slices.Equal(got, want) {
		t.Errorf("TimePrecisions() = %v, want %v", got, want)
	}
}

func TestEventFieldNamesOfTheTimeModel(t *testing.T) {
	got := []string{EventFieldStartDate, EventFieldStartTime, EventFieldEndDate, EventFieldEndTime, EventFieldAllDay}
	want := []string{"startDate", "startTime", "endDate", "endTime", "allDay"}
	if !slices.Equal(got, want) {
		t.Errorf("event field names = %v, want %v", got, want)
	}
}

func TestEffectivePeriodOfValidTimes(t *testing.T) {
	monthEnd := LocalDate{2026, time.October, 31}
	tests := []struct {
		name      string
		times     EventTimes
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			"only start date lasts until the next day",
			EventTimes{StartDate: kirchweihFriday},
			berlinInstant(t, kirchweihFriday, 0, 0), berlinInstant(t, kirchweihFriday.NextDay(), 0, 0),
		},
		{
			"single day with end date equal to start date",
			EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihFriday},
			berlinInstant(t, kirchweihFriday, 0, 0), berlinInstant(t, kirchweihFriday.NextDay(), 0, 0),
		},
		{
			"exact start until end date",
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0), EndDate: kirchweihFriday},
			berlinInstant(t, kirchweihFriday, 19, 0), berlinInstant(t, kirchweihFriday.NextDay(), 0, 0),
		},
		{
			"exact start without end lasts until the next day",
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0)},
			berlinInstant(t, kirchweihFriday, 19, 0), berlinInstant(t, kirchweihFriday.NextDay(), 0, 0),
		},
		{
			"all day over several days",
			EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihMonday, AllDay: true},
			berlinInstant(t, kirchweihFriday, 0, 0), berlinInstant(t, LocalDate{2026, time.October, 20}, 0, 0),
		},
		{
			"exact start and end",
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0), EndDate: kirchweihFriday, EndTime: localTime(23, 30)},
			berlinInstant(t, kirchweihFriday, 19, 0), berlinInstant(t, kirchweihFriday, 23, 30),
		},
		{
			"end day across a month boundary",
			EventTimes{StartDate: monthEnd, EndDate: monthEnd},
			berlinInstant(t, monthEnd, 0, 0), berlinInstant(t, LocalDate{2026, time.November, 1}, 0, 0),
		},
		{
			// 01:30 is still summer time; 03:00 is already winter time.
			"across the repeated hour lasts 2h30m",
			EventTimes{StartDate: autumnChangeDay, StartTime: localTime(1, 30), EndDate: autumnChangeDay, EndTime: localTime(3, 0)},
			time.Date(2026, time.October, 24, 23, 30, 0, 0, time.UTC), time.Date(2026, time.October, 25, 2, 0, 0, 0, time.UTC),
		},
		{
			"within the repeated hour lasts 30m",
			EventTimes{StartDate: autumnChangeDay, StartTime: localTime(2, 15), EndDate: autumnChangeDay, EndTime: localTime(2, 45)},
			time.Date(2026, time.October, 25, 0, 15, 0, 0, time.UTC), time.Date(2026, time.October, 25, 0, 45, 0, 0, time.UTC),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.times.EffectivePeriod()
			if err != nil {
				t.Fatalf("EffectivePeriod: %v", err)
			}
			if !got.Start.Equal(test.wantStart) || !got.End.Equal(test.wantEnd) {
				t.Errorf("EffectivePeriod = [%v, %v), want [%v, %v)", got.Start, got.End, test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestEffectivePeriodOfDaylightSavingDaysFollowsTheCalendar(t *testing.T) {
	tests := []struct {
		date LocalDate
		want time.Duration
	}{
		{autumnChangeDay, 25 * time.Hour},
		{springChangeDay, 23 * time.Hour},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%+v", test.date), func(t *testing.T) {
			period, err := EventTimes{StartDate: test.date}.EffectivePeriod()
			if err != nil {
				t.Fatalf("EffectivePeriod: %v", err)
			}
			if got := period.End.Sub(period.Start); got != test.want {
				t.Errorf("%+v lasts %v, want %v", test.date, got, test.want)
			}
		})
	}
}

func TestEffectivePeriodGivesTheRepeatedHourTheEarlierOffset(t *testing.T) {
	const twoHoursInSeconds = 2 * 60 * 60
	got, err := EventTimes{StartDate: autumnChangeDay, StartTime: localTime(2, 30)}.EffectivePeriod()
	if err != nil {
		t.Fatalf("EffectivePeriod: %v", err)
	}
	want := time.Date(2026, time.October, 25, 0, 30, 0, 0, time.UTC)
	if !got.Start.Equal(want) {
		t.Errorf("start = %v, want %v", got.Start.UTC(), want)
	}
	if _, offset := got.Start.Zone(); offset != twoHoursInSeconds {
		t.Errorf("start offset = %ds, want +02:00", offset)
	}
}

func TestEffectivePeriodReportsEveryProblem(t *testing.T) {
	invalidDate := LocalDate{2026, time.February, 30}
	tests := []struct {
		name  string
		times EventTimes
		want  []FieldError
	}{
		{
			"no start date",
			EventTimes{},
			[]FieldError{{EventFieldStartDate, ProblemMissing}},
		},
		{
			"no start date with end",
			EventTimes{EndDate: kirchweihMonday},
			[]FieldError{{EventFieldStartDate, ProblemMissing}},
		},
		{
			"all day with start time",
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0), AllDay: true},
			[]FieldError{{EventFieldStartTime, ProblemConflictsWithAllDay}},
		},
		{
			"all day with end time",
			EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihMonday, EndTime: localTime(22, 0), AllDay: true},
			[]FieldError{{EventFieldEndTime, ProblemConflictsWithAllDay}},
		},
		{
			"end time without end date",
			EventTimes{StartDate: kirchweihFriday, EndTime: localTime(22, 0)},
			[]FieldError{{EventFieldEndDate, ProblemMissing}},
		},
		{
			"every structural problem at once",
			EventTimes{StartTime: localTime(19, 0), EndTime: localTime(22, 0), AllDay: true},
			[]FieldError{
				{EventFieldStartDate, ProblemMissing},
				{EventFieldStartTime, ProblemConflictsWithAllDay},
				{EventFieldEndTime, ProblemConflictsWithAllDay},
				{EventFieldEndDate, ProblemMissing},
			},
		},
		{
			"structural problems hide format problems",
			EventTimes{StartDate: invalidDate, StartTime: localTime(24, 0), AllDay: true},
			[]FieldError{{EventFieldStartTime, ProblemConflictsWithAllDay}},
		},
		{
			"same instant",
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(20, 0), EndDate: kirchweihFriday, EndTime: localTime(20, 0)},
			[]FieldError{{EventFieldEndTime, ProblemNotAfterStart}},
		},
		{
			"end date before start date",
			EventTimes{StartDate: kirchweihMonday, EndDate: kirchweihFriday},
			[]FieldError{{EventFieldEndDate, ProblemNotAfterStart}},
		},
		{
			"end date before exact start day",
			EventTimes{StartDate: kirchweihMonday, StartTime: localTime(19, 0), EndDate: kirchweihFriday},
			[]FieldError{{EventFieldEndDate, ProblemNotAfterStart}},
		},
		{
			"end in the repeated hour before start",
			EventTimes{StartDate: autumnChangeDay, StartTime: localTime(2, 30), EndDate: autumnChangeDay, EndTime: localTime(2, 15)},
			[]FieldError{{EventFieldEndTime, ProblemNotAfterStartRepeatedHour}},
		},
		{
			"same wall clock in the repeated hour",
			EventTimes{StartDate: autumnChangeDay, StartTime: localTime(2, 30), EndDate: autumnChangeDay, EndTime: localTime(2, 30)},
			[]FieldError{{EventFieldEndTime, ProblemNotAfterStartRepeatedHour}},
		},
		{
			"start in the repeated hour, end before it",
			EventTimes{StartDate: autumnChangeDay, StartTime: localTime(2, 30), EndDate: autumnChangeDay, EndTime: localTime(1, 0)},
			[]FieldError{{EventFieldEndTime, ProblemNotAfterStart}},
		},
		{
			"end in the repeated hour, start after it",
			EventTimes{StartDate: autumnChangeDay, StartTime: localTime(4, 0), EndDate: autumnChangeDay, EndTime: localTime(2, 30)},
			[]FieldError{{EventFieldEndTime, ProblemNotAfterStart}},
		},
		{
			"start in the spring gap",
			EventTimes{StartDate: springChangeDay, StartTime: localTime(2, 30)},
			[]FieldError{{EventFieldStartTime, ProblemNonexistentTime}},
		},
		{
			"end in the spring gap",
			EventTimes{StartDate: springChangeDay, EndDate: springChangeDay, EndTime: localTime(2, 0)},
			[]FieldError{{EventFieldEndTime, ProblemNonexistentTime}},
		},
		{
			"invalid start date",
			EventTimes{StartDate: invalidDate},
			[]FieldError{{EventFieldStartDate, ProblemInvalidFormat}},
		},
		{
			"invalid start date with end date",
			EventTimes{StartDate: invalidDate, EndDate: kirchweihMonday},
			[]FieldError{{EventFieldStartDate, ProblemInvalidFormat}},
		},
		{
			"invalid start time",
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(24, 0)},
			[]FieldError{{EventFieldStartTime, ProblemInvalidFormat}},
		},
		{
			"invalid end date without time",
			EventTimes{StartDate: kirchweihFriday, EndDate: LocalDate{2026, 13, 1}},
			[]FieldError{{EventFieldEndDate, ProblemInvalidFormat}},
		},
		{
			"invalid end date and time",
			EventTimes{StartDate: kirchweihFriday, EndDate: invalidDate, EndTime: localTime(24, 0)},
			[]FieldError{
				{EventFieldEndDate, ProblemInvalidFormat},
				{EventFieldEndTime, ProblemInvalidFormat},
			},
		},
		{
			"every format and gap problem at once",
			EventTimes{StartDate: invalidDate, StartTime: localTime(12, 60), EndDate: springChangeDay, EndTime: localTime(2, 30)},
			[]FieldError{
				{EventFieldStartDate, ProblemInvalidFormat},
				{EventFieldStartTime, ProblemInvalidFormat},
				{EventFieldEndTime, ProblemNonexistentTime},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.times.EffectivePeriod()
			if got != (Period{}) {
				t.Errorf("EffectivePeriod = %+v, want zero period on error", got)
			}
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("EffectivePeriod error = %v, want *ValidationError", err)
			}
			if !slices.Equal(validation.Fields, test.want) {
				t.Errorf("fields = %v, want %v", validation.Fields, test.want)
			}
		})
	}
}
