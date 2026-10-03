package core

import (
	"errors"
	"fmt"
	"testing"
	"time"
	_ "time/tzdata"
)

func localTime(hour, minute int) *LocalTime {
	return &LocalTime{Hour: hour, Minute: minute}
}

func TestLocalDateIsZeroOnlyForTheZeroValue(t *testing.T) {
	if !(LocalDate{}).IsZero() {
		t.Error("LocalDate{}.IsZero() = false, want true")
	}
	if (LocalDate{Year: 2026, Month: time.October, Day: 16}).IsZero() {
		t.Error("2026-10-16 IsZero() = true, want false")
	}
}

func TestLocalDateNextDayFollowsTheCalendar(t *testing.T) {
	tests := []struct {
		date, want LocalDate
	}{
		{LocalDate{2026, time.October, 16}, LocalDate{2026, time.October, 17}},
		{LocalDate{2026, time.October, 31}, LocalDate{2026, time.November, 1}},
		{LocalDate{2026, time.December, 31}, LocalDate{2027, time.January, 1}},
		{LocalDate{2028, time.February, 28}, LocalDate{2028, time.February, 29}},
		{LocalDate{2027, time.February, 28}, LocalDate{2027, time.March, 1}},
	}
	for _, test := range tests {
		if got := test.date.NextDay(); got != test.want {
			t.Errorf("%+v.NextDay() = %+v, want %+v", test.date, got, test.want)
		}
	}
}

func TestLocalDateIsValidRejectsCalendarNonsense(t *testing.T) {
	tests := []struct {
		date LocalDate
		want bool
	}{
		{LocalDate{2026, time.October, 16}, true},
		{LocalDate{2028, time.February, 29}, true},
		{LocalDate{1, time.January, 1}, true},
		{LocalDate{9999, time.December, 31}, true},
		{LocalDate{}, false},
		{LocalDate{2026, time.February, 30}, false},
		{LocalDate{2027, time.February, 29}, false},
		{LocalDate{2026, 13, 1}, false},
		{LocalDate{2026, 0, 1}, false},
		{LocalDate{2026, time.October, 0}, false},
		{LocalDate{0, time.January, 1}, false},
		{LocalDate{10000, time.January, 1}, false},
	}
	for _, test := range tests {
		if got := test.date.IsValid(); got != test.want {
			t.Errorf("%+v.IsValid() = %v, want %v", test.date, got, test.want)
		}
	}
}

func TestLocalTimeIsValidRejectsClockNonsense(t *testing.T) {
	tests := []struct {
		timeOfDay LocalTime
		want      bool
	}{
		{LocalTime{0, 0}, true},
		{LocalTime{23, 59}, true},
		{LocalTime{24, 0}, false},
		{LocalTime{-1, 0}, false},
		{LocalTime{12, 60}, false},
		{LocalTime{12, -1}, false},
	}
	for _, test := range tests {
		if got := test.timeOfDay.IsValid(); got != test.want {
			t.Errorf("%+v.IsValid() = %v, want %v", test.timeOfDay, got, test.want)
		}
	}
}

func TestToInstantConvertsBerlinWallClock(t *testing.T) {
	tests := []struct {
		name      string
		date      LocalDate
		timeOfDay *LocalTime
		want      string
	}{
		{"summer time", LocalDate{2026, time.October, 16}, localTime(19, 0), "2026-10-16T17:00:00Z"},
		{"winter time", LocalDate{2026, time.December, 24}, localTime(16, 30), "2026-12-24T15:30:00Z"},
		{"no time means start of day", LocalDate{2026, time.October, 16}, nil, "2026-10-15T22:00:00Z"},
		{"repeated hour gets the earlier offset", LocalDate{2026, time.October, 25}, localTime(2, 30), "2026-10-25T00:30:00Z"},
		{"hour after the repeated one", LocalDate{2026, time.October, 25}, localTime(3, 0), "2026-10-25T02:00:00Z"},
		{"midnight before the spring gap", LocalDate{2027, time.March, 28}, nil, "2027-03-27T23:00:00Z"},
		{"hour after the spring gap", LocalDate{2027, time.March, 28}, localTime(3, 0), "2027-03-28T01:00:00Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ToInstant(test.date, test.timeOfDay)
			if err != nil {
				t.Fatalf("ToInstant: %v", err)
			}
			if want, _ := time.Parse(time.RFC3339, test.want); !got.Equal(want) {
				t.Errorf("ToInstant = %v, want %v", got.UTC(), want)
			}
			if got.Location().String() != berlinZoneName {
				t.Errorf("ToInstant zone = %v, want %v", got.Location(), berlinZoneName)
			}
		})
	}
}

func TestToInstantRejectsTheSpringGap(t *testing.T) {
	_, err := ToInstant(LocalDate{2027, time.March, 28}, localTime(2, 30))

	if !errors.Is(err, ErrNonexistentLocalTime) || !errors.Is(err, ErrValidation) {
		t.Errorf("ToInstant in spring gap = %v, want ErrNonexistentLocalTime and ErrValidation", err)
	}
}

func TestToInstantRejectsInvalidParts(t *testing.T) {
	tests := []struct {
		name      string
		date      LocalDate
		timeOfDay *LocalTime
		want      []error
		notWant   []error
	}{
		{"invalid date", LocalDate{2026, time.February, 30}, nil, []error{ErrInvalidLocalDate}, []error{ErrInvalidLocalTime}},
		{"invalid time", LocalDate{2026, time.October, 16}, localTime(24, 0), []error{ErrInvalidLocalTime}, []error{ErrInvalidLocalDate}},
		{"both invalid", LocalDate{}, localTime(12, 60), []error{ErrInvalidLocalDate, ErrInvalidLocalTime}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ToInstant(test.date, test.timeOfDay)
			if !errors.Is(err, ErrValidation) {
				t.Errorf("ToInstant = %v, want ErrValidation", err)
			}
			for _, want := range test.want {
				if !errors.Is(err, want) {
					t.Errorf("ToInstant = %v, want %v", err, want)
				}
			}
			for _, notWant := range test.notWant {
				if errors.Is(err, notWant) {
					t.Errorf("ToInstant = %v, must not match %v", err, notWant)
				}
			}
			if errors.Is(err, ErrNonexistentLocalTime) {
				t.Errorf("ToInstant = %v, must not report a gap for invalid parts", err)
			}
		})
	}
}

func TestMustLoadZonePanicsOnUnknownZone(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustLoadZone did not panic for an unknown zone")
		}
	}()
	mustLoadZone("No/Such_Zone")
}

func TestInRepeatedHourDetectsTheAutumnHour(t *testing.T) {
	tests := []struct {
		date      LocalDate
		timeOfDay *LocalTime
		want      bool
	}{
		{LocalDate{2026, time.October, 25}, localTime(2, 0), true},
		{LocalDate{2026, time.October, 25}, localTime(2, 59), true},
		{LocalDate{2026, time.October, 25}, localTime(1, 59), false},
		{LocalDate{2026, time.October, 25}, localTime(3, 0), false},
		{LocalDate{2026, time.October, 16}, localTime(2, 30), false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%+v %+v", test.date, test.timeOfDay), func(t *testing.T) {
			instant, err := ToInstant(test.date, test.timeOfDay)
			if err != nil {
				t.Fatalf("ToInstant: %v", err)
			}
			if got := inRepeatedHour(instant); got != test.want {
				t.Errorf("inRepeatedHour(%v) = %v, want %v", instant, got, test.want)
			}
		})
	}
}
