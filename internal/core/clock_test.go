package core

import (
	"testing"
	"time"
)

// fixedClock is a Clock that always shows the same instant.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

func clockAt(t *testing.T, date LocalDate, hour, minute int) Clock {
	t.Helper()
	return fixedClock(berlinInstant(t, date, hour, minute))
}

func TestPeriodIsOverWhenItsEndIsReached(t *testing.T) {
	today := LocalDate{2026, time.October, 16}
	kirchweihTuesday := kirchweihMonday.NextDay()
	endsTodayAt14 := EventTimes{StartDate: today, StartTime: localTime(10, 0), EndDate: today, EndTime: localTime(14, 0)}
	kirchweih := EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihMonday}
	exactUntilDate := EventTimes{StartDate: today, StartTime: localTime(19, 0), EndDate: today}
	singleDay := EventTimes{StartDate: today, EndDate: today}
	tests := []struct {
		name  string
		times EventTimes
		clock Clock
		want  bool
	}{
		{"end today 14:00 at 13:59", endsTodayAt14, clockAt(t, today, 13, 59), false},
		{"end today 14:00 at 14:00", endsTodayAt14, clockAt(t, today, 14, 0), true},
		{"kirchweih on Monday 00:00", kirchweih, clockAt(t, kirchweihMonday, 0, 0), false},
		{"kirchweih on Monday 23:59", kirchweih, clockAt(t, kirchweihMonday, 23, 59), false},
		{"kirchweih on Tuesday 00:00", kirchweih, clockAt(t, kirchweihTuesday, 0, 0), true},
		{"exact start with end date at 23:59 of the end day", exactUntilDate, clockAt(t, today, 23, 59), false},
		{"exact start with end date at the next midnight", exactUntilDate, clockAt(t, today.NextDay(), 0, 0), true},
		{"single day at 23:59", singleDay, clockAt(t, today, 23, 59), false},
		{"single day at the next midnight", singleDay, clockAt(t, today.NextDay(), 0, 0), true},
		{"before the start", singleDay, clockAt(t, LocalDate{2026, time.October, 1}, 12, 0), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			period, err := test.times.EffectivePeriod()
			if err != nil {
				t.Fatalf("EffectivePeriod: %v", err)
			}
			if got := period.IsOver(test.clock); got != test.want {
				t.Errorf("IsOver(%v) = %v, want %v", test.clock.Now(), got, test.want)
			}
		})
	}
}
