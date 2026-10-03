package core

import (
	"errors"
	"fmt"
	"time"
)

// berlinZoneName is the only time zone of the event store.
const berlinZoneName = "Europe/Berlin"

// berlin is loaded once at start; cmd/eventstore embeds time/tzdata so the
// zone exists even without system zone files.
var berlin = mustLoadZone(berlinZoneName)

// Year bounds of a LocalDate, both inclusive: dates are written as YYYY.
const (
	minYear = 1
	maxYear = 9999
)

// Upper bounds of a LocalTime, both exclusive.
const (
	hoursPerDay    = 24
	minutesPerHour = 60
)

// daysToNextDay is added to the day of month; time.Date carries it over
// into the next month or year.
const daysToNextDay = 1

// repeatedHourLength is how far the wall clock goes back when daylight
// saving time ends in Europe/Berlin.
const repeatedHourLength = time.Hour

// offsetProbeDistance is how far before and after a wall-clock value
// ToInstant looks up the zone offsets in force. Berlin changes its offset
// at most once around any wall-clock value, so the two offsets found there
// are the only candidates. This is a probe, not a day boundary.
const offsetProbeDistance = 24 * time.Hour

// Errors of ToInstant. Each matches ErrValidation with errors.Is.
var (
	// ErrInvalidLocalDate means the date does not exist in the calendar.
	ErrInvalidLocalDate = fmt.Errorf("%w: invalid local date", ErrValidation)
	// ErrInvalidLocalTime means the time is not between 00:00 and 23:59.
	ErrInvalidLocalTime = fmt.Errorf("%w: invalid local time", ErrValidation)
	// ErrNonexistentLocalTime means the local time falls into the gap when
	// daylight saving time starts in Europe/Berlin.
	ErrNonexistentLocalTime = fmt.Errorf("%w: local time does not exist because of the daylight saving time change", ErrValidation)
)

// LocalDate is a calendar day in Europe/Berlin. The zero value means the
// date is missing.
type LocalDate struct {
	Year  int
	Month time.Month
	Day   int
}

// IsZero reports whether the date is missing.
func (d LocalDate) IsZero() bool {
	return d == LocalDate{}
}

// IsValid reports whether the date exists in the calendar.
func (d LocalDate) IsValid() bool {
	if d.Year < minYear || d.Year > maxYear {
		return false
	}
	return dateOf(time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)) == d
}

// NextDay returns the calendar day after d, which must be valid.
func (d LocalDate) NextDay() LocalDate {
	return dateOf(time.Date(d.Year, d.Month, d.Day+daysToNextDay, 0, 0, 0, 0, time.UTC))
}

func dateOf(t time.Time) LocalDate {
	year, month, day := t.Date()
	return LocalDate{Year: year, Month: month, Day: day}
}

// LocalTime is a wall-clock time of day in Europe/Berlin, to the minute.
// A missing time is a nil *LocalTime and means "unknown", never 00:00.
type LocalTime struct {
	Hour   int
	Minute int
}

// IsValid reports whether the time lies between 00:00 and 23:59.
func (lt LocalTime) IsValid() bool {
	return lt.Hour >= 0 && lt.Hour < hoursPerDay && lt.Minute >= 0 && lt.Minute < minutesPerHour
}

// wallClock is a local date and time compared as one value.
type wallClock struct {
	date      LocalDate
	timeOfDay LocalTime
}

func wallClockOf(t time.Time) wallClock {
	t = t.In(berlin)
	return wallClock{date: dateOf(t), timeOfDay: LocalTime{Hour: t.Hour(), Minute: t.Minute()}}
}

// ToInstant is the only conversion from a local date and time to an
// instant. A nil time means 00:00, which is what day boundaries need. A
// time in the spring gap is rejected with ErrNonexistentLocalTime; a time
// in the repeated autumn hour gets the earlier offset (+02:00). The result
// is in the zone Europe/Berlin.
func ToInstant(date LocalDate, timeOfDay *LocalTime) (time.Time, error) {
	var wanted wallClock
	var errs []error
	if !date.IsValid() {
		errs = append(errs, ErrInvalidLocalDate)
	}
	if timeOfDay != nil {
		if !timeOfDay.IsValid() {
			errs = append(errs, ErrInvalidLocalTime)
		}
		wanted.timeOfDay = *timeOfDay
	}
	if errs != nil {
		return time.Time{}, errors.Join(errs...)
	}
	wanted.date = date
	return earliestInstantOf(wanted)
}

// earliestInstantOf returns the earliest instant whose Berlin wall clock is
// wanted. It decides the repeated hour itself instead of relying on
// time.Date, whose choice there is unspecified.
func earliestInstantOf(wanted wallClock) (time.Time, error) {
	naive := time.Date(wanted.date.Year, wanted.date.Month, wanted.date.Day,
		wanted.timeOfDay.Hour, wanted.timeOfDay.Minute, 0, 0, time.UTC)
	_, offsetBefore := naive.Add(-offsetProbeDistance).In(berlin).Zone()
	_, offsetAfter := naive.Add(offsetProbeDistance).In(berlin).Zone()
	// The larger offset gives the earlier instant, so it is tried first.
	for _, offset := range []int{max(offsetBefore, offsetAfter), min(offsetBefore, offsetAfter)} {
		candidate := naive.Add(-time.Duration(offset) * time.Second).In(berlin)
		if wallClockOf(candidate) == wanted {
			return candidate, nil
		}
	}
	return time.Time{}, ErrNonexistentLocalTime
}

// inRepeatedHour reports whether instant lies in the first pass of the hour
// that repeats when daylight saving time ends, which is the pass ToInstant
// picks.
func inRepeatedHour(instant time.Time) bool {
	return wallClockOf(instant.Add(repeatedHourLength)) == wallClockOf(instant)
}

// mustLoadZone loads a time zone and panics if it is missing, because the
// event store cannot interpret any time without it.
func mustLoadZone(name string) *time.Location {
	zone, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("load time zone %s: %v", name, err))
	}
	return zone
}
