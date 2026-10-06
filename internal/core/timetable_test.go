package core

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

// festInput is the Kirchweih from Friday 16 October 2026, 18:00, to the
// end of Saturday 17 October: its effective period is
// [16.10. 18:00, 18.10. 00:00).
func festInput(entries ...TimetableEntryInput) EventInput {
	in := validEventInput()
	in.StartTime, in.EndDate = "18:00", "2026-10-17"
	in.Timetable = entries
	return in
}

func entryInput(description, date, startTime, endTime string) TimetableEntryInput {
	return TimetableEntryInput{Description: description, Date: date, StartTime: startTime, EndTime: endTime}
}

func TestTimetableFieldNames(t *testing.T) {
	got := []string{EventFieldTimetable, TimetableFieldDescription, TimetableFieldDate, TimetableFieldStartTime, TimetableFieldEndTime}
	want := []string{"timetable", "description", "date", "startTime", "endTime"}
	if !slices.Equal(got, want) {
		t.Errorf("timetable field names = %v, want %v", got, want)
	}
	if got := TimetableField(2, TimetableFieldDate); got != "timetable[2].date" {
		t.Errorf("TimetableField(2, date) = %q, want timetable[2].date", got)
	}
}

func TestSplitTimetableFieldInvertsTimetableField(t *testing.T) {
	index, field, ok := SplitTimetableField(TimetableField(12, TimetableFieldEndTime))
	if !ok || index != 12 || field != TimetableFieldEndTime {
		t.Errorf("SplitTimetableField = %d, %q, %v, want 12, endTime, true", index, field, ok)
	}
	for _, name := range []string{
		EventFieldTitle, "timetable", "timetable[2]", "timetable[2].", "timetable[x].date",
		"timetable[-1].date", "timetable[].date", "timetables[2].date",
	} {
		if index, field, ok := SplitTimetableField(name); ok {
			t.Errorf("SplitTimetableField(%q) = %d, %q, true, want false", name, index, field)
		}
	}
}

func TestProblemOutsideEventCode(t *testing.T) {
	if ProblemOutsideEvent != "outsideEvent" {
		t.Errorf("ProblemOutsideEvent = %q, want outsideEvent", ProblemOutsideEvent)
	}
}

func TestNewEventAcceptsTimetableAndSortsIt(t *testing.T) {
	saturday := LocalDate{2026, time.October, 17}
	in := festInput(
		entryInput("Disco", "2026-10-16", "22:00", "01:00"),
		entryInput(" Frühschoppen ", "2026-10-17", "10:00", ""),
		entryInput("Bieranstich", "2026-10-16", "18:00", ""),
		entryInput("Markttag", "2026-10-17", "", ""),
		entryInput("Ausklang", "2026-10-17", "", "23:59"),
	)

	event, problems := newEvent(in)
	if problems != nil {
		t.Fatalf("newEvent problems = %v", problems)
	}
	want := []TimetableEntry{
		{Description: "Bieranstich", Date: kirchweihFriday, StartTime: localTime(18, 0)},
		{Description: "Disco", Date: kirchweihFriday, StartTime: localTime(22, 0), EndTime: localTime(1, 0)},
		{Description: "Markttag", Date: saturday},
		{Description: "Ausklang", Date: saturday, EndTime: localTime(23, 59)},
		{Description: "Frühschoppen", Date: saturday, StartTime: localTime(10, 0)},
	}
	if !reflect.DeepEqual(event.Timetable, want) {
		t.Errorf("timetable = %s, want %s", formatTimetable(event.Timetable), formatTimetable(want))
	}
	wantPeriod := Period{Start: berlinInstant(t, kirchweihFriday, 18, 0), End: berlinInstant(t, saturday.NextDay(), 0, 0)}
	if !event.Period.Start.Equal(wantPeriod.Start) || !event.Period.End.Equal(wantPeriod.End) {
		t.Errorf("period = [%v, %v), want the event's own [%v, %v)", event.Period.Start, event.Period.End, wantPeriod.Start, wantPeriod.End)
	}
}

func TestTimetableEntryEndingBeforeItsStartEndsTheNextDay(t *testing.T) {
	disco := TimetableEntry{Date: kirchweihFriday, StartTime: localTime(22, 0), EndTime: localTime(1, 0)}
	if got := disco.endDate(); got != kirchweihFriday.NextDay() {
		t.Errorf("end date of 22:00-01:00 = %v, want the next day", got)
	}
	evening := TimetableEntry{Date: kirchweihFriday, StartTime: localTime(20, 0), EndTime: localTime(22, 0)}
	if got := evening.endDate(); got != kirchweihFriday {
		t.Errorf("end date of 20:00-22:00 = %v, want the same day", got)
	}

	// Ending at 01:00 on Sunday lies outside an event that ends at 00:00.
	in := festInput(entryInput("Disco", "2026-10-17", "22:00", "01:00"))
	_, problems := newEvent(in)
	want := []FieldError{{Field: TimetableField(0, TimetableFieldEndTime), Problem: ProblemOutsideEvent}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestNewEventAcceptsEntriesAtTheEdgesOfTheEvent(t *testing.T) {
	tests := map[string]EventInput{
		"starts with the event": festInput(entryInput("Bieranstich", "2026-10-16", "18:00", "")),
		"ends with the event": func() EventInput {
			in := validEventInput()
			in.StartTime, in.EndDate, in.EndTime = "19:00", "2026-10-16", "23:00"
			in.Timetable = []TimetableEntryInput{entryInput("Feuerwerk", "2026-10-16", "22:00", "23:00")}
			return in
		}(),
		"ends at midnight of the end":   festInput(entryInput("Disco", "2026-10-17", "22:00", "00:00")),
		"only end just after the start": festInput(entryInput("Aufbau", "2026-10-16", "", "18:01")),
		"day of the start":              festInput(entryInput("Rummel", "2026-10-16", "", "")),
		"last minute":                   festInput(entryInput("Zapfenstreich", "2026-10-17", "23:59", "")),
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			if _, problems := newEvent(in); problems != nil {
				t.Errorf("problems = %v, want none", problems)
			}
		})
	}
}

func TestNewEventRejectsEntriesOutsideTheEvent(t *testing.T) {
	tests := map[string]struct {
		in   EventInput
		want FieldError
	}{
		"day after the event": {
			festInput(entryInput("Kehraus", "2026-10-18", "", "")),
			FieldError{Field: TimetableField(0, TimetableFieldDate), Problem: ProblemOutsideEvent},
		},
		"day before the event": {
			festInput(entryInput("Aufbau", "2026-10-15", "", "")),
			FieldError{Field: TimetableField(0, TimetableFieldDate), Problem: ProblemOutsideEvent},
		},
		"starts before the event": {
			festInput(entryInput("Aufbau", "2026-10-16", "17:00", "19:00")),
			FieldError{Field: TimetableField(0, TimetableFieldStartTime), Problem: ProblemOutsideEvent},
		},
		"starts when the event ends": {
			festInput(entryInput("Kehraus", "2026-10-18", "00:00", "")),
			FieldError{Field: TimetableField(0, TimetableFieldStartTime), Problem: ProblemOutsideEvent},
		},
		"only end at the start of the event": {
			festInput(entryInput("Aufbau", "2026-10-16", "", "18:00")),
			FieldError{Field: TimetableField(0, TimetableFieldEndTime), Problem: ProblemOutsideEvent},
		},
		"ends after a shortened event": {
			func() EventInput {
				in := festInput(entryInput("Feuerwerk", "2026-10-16", "22:00", "23:30"))
				in.EndDate, in.EndTime = "2026-10-16", "23:00"
				return in
			}(),
			FieldError{Field: TimetableField(0, TimetableFieldEndTime), Problem: ProblemOutsideEvent},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, problems := newEvent(tt.in)
			if !slices.Equal(problems, []FieldError{tt.want}) {
				t.Errorf("problems = %v, want %v", problems, []FieldError{tt.want})
			}
		})
	}
}

func TestNewEventReportsTimetableProblemsByEnteredIndex(t *testing.T) {
	in := festInput(
		entryInput("Kehraus", "2026-10-17", "20:00", ""),
		entryInput(" ", "", "", ""),
		entryInput("Bieranstich", "16.10.2026", "6 Uhr", "spät"),
		entryInput("Gleich", "2026-10-16", "20:00", "20:00"),
	)
	_, problems := newEvent(in)
	want := []FieldError{
		{Field: TimetableField(1, TimetableFieldDescription), Problem: ProblemMissing},
		{Field: TimetableField(1, TimetableFieldDate), Problem: ProblemMissing},
		{Field: TimetableField(2, TimetableFieldDate), Problem: ProblemInvalidFormat},
		{Field: TimetableField(2, TimetableFieldStartTime), Problem: ProblemInvalidFormat},
		{Field: TimetableField(2, TimetableFieldEndTime), Problem: ProblemInvalidFormat},
		{Field: TimetableField(3, TimetableFieldEndTime), Problem: ProblemNotAfterStart},
	}
	if !slices.Equal(problems, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestNewEventRejectsEntryTimesInTheSpringGap(t *testing.T) {
	spring := func(entry TimetableEntryInput) EventInput {
		in := validEventInput()
		in.StartDate, in.EndDate = "2026-03-28", "2026-03-29"
		in.Timetable = []TimetableEntryInput{entry}
		return in
	}
	tests := map[string]struct {
		in   EventInput
		want FieldError
	}{
		"start": {
			spring(entryInput("Nachtwanderung", "2026-03-29", "02:30", "")),
			FieldError{Field: TimetableField(0, TimetableFieldStartTime), Problem: ProblemNonexistentTime},
		},
		"end on the next day": {
			spring(entryInput("Disco", "2026-03-28", "23:00", "02:30")),
			FieldError{Field: TimetableField(0, TimetableFieldEndTime), Problem: ProblemNonexistentTime},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, problems := newEvent(tt.in)
			if !slices.Equal(problems, []FieldError{tt.want}) {
				t.Errorf("problems = %v, want %v", problems, []FieldError{tt.want})
			}
		})
	}
}

func TestNewEventChecksTimetableBoundsOnlyForAValidPeriod(t *testing.T) {
	in := festInput(
		entryInput("Kehraus", "2026-10-20", "", ""),
		entryInput("", "2026-10-16", "", ""),
		entryInput("Nachfeier", "2026-10-20", "20:00", "22:00"),
	)
	in.EndDate = "2026-10-15"

	_, problems := newEvent(in)

	want := []FieldError{
		{Field: EventFieldEndDate, Problem: ProblemNotAfterStart},
		{Field: TimetableField(1, TimetableFieldDescription), Problem: ProblemMissing},
	}
	if !slices.Equal(problems, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestTimetableInstantReportsInvalidValues(t *testing.T) {
	if _, problem := timetableInstant(LocalDate{2026, time.February, 30}, nil); problem != ProblemInvalidFormat {
		t.Errorf("problem of 30 February = %q, want invalidFormat", problem)
	}
	if _, problem := timetableInstant(LocalDate{2026, time.March, 29}, localTime(2, 30)); problem != ProblemNonexistentTime {
		t.Errorf("problem of 02:30 on 29 March 2026 = %q, want nonexistentTime", problem)
	}
}

func TestDayEntryOnAnInvalidDateIsReported(t *testing.T) {
	fields := timetableEntryFieldsAt(0)
	problems := dayEntryProblems(TimetableEntry{Date: LocalDate{2026, time.February, 30}}, fields, nil)
	if !slices.Equal(problems, []FieldError{{Field: fields.date, Problem: ProblemInvalidFormat}}) {
		t.Errorf("problems = %v, want date invalidFormat", problems)
	}
}

func TestSortedTimetableOrdersChronologicallyThenByTextAndID(t *testing.T) {
	entries := []TimetableEntry{
		{ID: "b", Description: "Musik", Date: kirchweihFriday, StartTime: localTime(20, 0)},
		{ID: "a", Description: "Musik", Date: kirchweihFriday, StartTime: localTime(20, 0)},
		{ID: "c", Description: "Bühne", Date: kirchweihFriday, StartTime: localTime(20, 0)},
		{ID: "d", Description: "Lang", Date: kirchweihFriday, StartTime: localTime(20, 0), EndTime: localTime(23, 0)},
		{ID: "e", Description: "Kurz", Date: kirchweihFriday, StartTime: localTime(20, 0), EndTime: localTime(21, 0)},
		{ID: "f", Description: "Früh", Date: kirchweihFriday, StartTime: localTime(9, 30)},
		{ID: "g", Description: "Ganzer Tag", Date: kirchweihFriday},
		{ID: "h", Description: "Vorabend", Date: LocalDate{2026, time.October, 15}, StartTime: localTime(23, 0)},
		{ID: "i", Description: "Später Monat", Date: LocalDate{2026, time.November, 1}},
		{ID: "j", Description: "Später Monat", Date: LocalDate{2027, time.January, 1}},
		{ID: "k", Description: "Früher", Date: kirchweihFriday, StartTime: localTime(9, 5)},
	}
	original := slices.Clone(entries)

	got := sortedTimetable(entries)

	var ids []string
	for _, entry := range got {
		ids = append(ids, entry.ID)
	}
	if want := []string{"h", "g", "k", "f", "c", "a", "b", "e", "d", "i", "j"}; !slices.Equal(ids, want) {
		t.Errorf("order = %v, want %v", ids, want)
	}
	if !reflect.DeepEqual(entries, original) {
		t.Error("sortedTimetable changed its argument")
	}
}

func TestCanonicalizeNormalizesAndSortsTheTimetable(t *testing.T) {
	in := EventInput{Timetable: []TimetableEntryInput{
		entryInput(" Disco ", " 2026-10-16 ", "22:00 ", " 01:00"),
		entryInput("Kehraus", "2026-10-17", "", ""),
		entryInput("Bieranstich", "2026-10-16", "18:00", ""),
		entryInput(decomposedOelmuehle, "2026-10-16", "", ""),
		entryInput("Bühne", "2026-10-16", "18:00", ""),
	}}

	got := in.Canonicalize().Timetable

	want := []TimetableEntryInput{
		entryInput(composedOelmuehle, "2026-10-16", "", ""),
		entryInput("Bieranstich", "2026-10-16", "18:00", ""),
		entryInput("Bühne", "2026-10-16", "18:00", ""),
		entryInput("Disco", "2026-10-16", "22:00", "01:00"),
		entryInput("Kehraus", "2026-10-17", "", ""),
	}
	if !slices.Equal(got, want) {
		t.Errorf("Canonicalize().Timetable = %v, want %v", got, want)
	}
	if in.Timetable[0].Description != " Disco " {
		t.Error("Canonicalize changed its receiver")
	}
	if got := (EventInput{}).Canonicalize().Timetable; got != nil {
		t.Errorf("timetable of an input without entries = %v, want nil", got)
	}
}

func TestEventInputOfRoundTripsTheTimetable(t *testing.T) {
	in := festInput(
		entryInput("Bieranstich", "2026-10-16", "18:00", ""),
		entryInput("Disco", "2026-10-16", "22:00", "01:00"),
		entryInput("Markttag", "2026-10-17", "", ""),
	)
	event, problems := newEvent(in)
	if problems != nil {
		t.Fatalf("newEvent problems = %v", problems)
	}

	if got := EventInputOf(event).Timetable; !slices.Equal(got, in.Timetable) {
		t.Errorf("EventInputOf().Timetable = %v, want %v", got, in.Timetable)
	}
}

// formatTimetable spells out entries including the times behind pointers.
func formatTimetable(entries []TimetableEntry) string {
	return EventInputOf(Event{Timetable: entries}).timetableText()
}

func (in EventInput) timetableText() string {
	text := ""
	for _, entry := range in.Timetable {
		text += "[" + entry.Description + " " + entry.Date + " " + entry.StartTime + "-" + entry.EndTime + "]"
	}
	return text
}

func TestNewEventLimitsTheDescriptionOfAnEntry(t *testing.T) {
	in := festInput(
		entryInput(overLimit(MaxTimetableDescriptionLength-1), "2026-10-17", "", ""),
		entryInput(overLimit(MaxTimetableDescriptionLength), "2026-10-17", "", ""),
	)

	_, problems := newEvent(in)

	want := []FieldError{{Field: TimetableField(1, TimetableFieldDescription), Problem: ProblemTooLong, Limit: MaxTimetableDescriptionLength}}
	if !slices.Equal(problems, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}

func TestNewEventReportsTooManyEntriesBesideTheirOwnProblems(t *testing.T) {
	entries := slices.Repeat([]TimetableEntryInput{entryInput("Musik", "2026-10-17", "", "")}, MaxTimetableEntries+1)
	entries[MaxTimetableEntries] = entryInput("", "2026-10-17", "", "")

	_, problems := newEvent(festInput(entries...))

	want := []FieldError{
		{Field: EventFieldTimetable, Problem: ProblemTooMany, Limit: MaxTimetableEntries},
		{Field: TimetableField(MaxTimetableEntries, TimetableFieldDescription), Problem: ProblemMissing},
	}
	if !slices.Equal(problems, want) {
		t.Errorf("problems = %v, want %v", problems, want)
	}
}
