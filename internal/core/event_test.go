package core

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

// validEventInput returns a complete input for the Kirchweih market at the
// hall; tests change single fields.
func validEventInput() EventInput {
	return EventInput{
		Title:      "Kirchweihmarkt",
		Type:       string(EventTypeMarket),
		LocationID: hallID,
		StartDate:  "2026-10-16",
		Source:     EventSource{Description: "Amtsblatt 41/2026", URL: "https://www.zirndorf.de/amtsblatt"},
		Note:       "Mit Fahrgeschäften",
	}
}

func TestEventTypesAreTheSevenCodesInOrder(t *testing.T) {
	want := []EventType{"festival", "market", "culture", "politics", "club", "sports", "other"}
	if got := EventTypes(); !slices.Equal(got, want) {
		t.Errorf("EventTypes() = %v, want %v", got, want)
	}
}

func TestListEventTypesLabelsEveryCodeInGermanInOrder(t *testing.T) {
	want := []EventTypeEntry{
		{Code: EventTypeFestival, Label: "Fest/Kirchweih"},
		{Code: EventTypeMarket, Label: "Markt"},
		{Code: EventTypeCulture, Label: "Kultur/Bühne"},
		{Code: EventTypePolitics, Label: "Politik/Sitzung"},
		{Code: EventTypeClub, Label: "Verein/Treff"},
		{Code: EventTypeSports, Label: "Sport"},
		{Code: EventTypeOther, Label: "Sonstiges"},
	}
	got := ListEventTypes()
	if !slices.Equal(got, want) {
		t.Errorf("ListEventTypes() = %v, want %v", got, want)
	}
	var codes []EventType
	for _, entry := range got {
		codes = append(codes, entry.Code)
	}
	if !slices.Equal(codes, EventTypes()) {
		t.Errorf("codes = %v, want the order of EventTypes() %v", codes, EventTypes())
	}
}

func TestEventFieldNamesBesideTheTimeModel(t *testing.T) {
	got := []string{EventFieldTitle, EventFieldType, EventFieldLocationID, EventFieldSourceDescription, EventFieldSourceURL, EventFieldNote}
	want := []string{"title", "type", "locationId", "source.description", "source.url", "note"}
	if !slices.Equal(got, want) {
		t.Errorf("event field names = %v, want %v", got, want)
	}
}

func TestCanonicalizeTrimsAndComposesEveryText(t *testing.T) {
	in := EventInput{
		Title:      "  " + decomposedOelmuehle + " ",
		Type:       " market ",
		LocationID: " " + hallID + " ",
		StartDate:  " 2026-10-16 ",
		StartTime:  " 19:00 ",
		EndDate:    "\t2026-10-19",
		EndTime:    "22:00 ",
		AllDay:     true,
		Source:     EventSource{Description: " Amtsblatt ", URL: " https://zirndorf.de "},
		Note:       "   ",
	}

	want := EventInput{
		Title:      composedOelmuehle,
		Type:       "market",
		LocationID: hallID,
		StartDate:  "2026-10-16",
		StartTime:  "19:00",
		EndDate:    "2026-10-19",
		EndTime:    "22:00",
		AllDay:     true,
		Source:     EventSource{Description: "Amtsblatt", URL: "https://zirndorf.de"},
		Note:       "",
	}
	if got := in.Canonicalize(); !reflect.DeepEqual(got, want) {
		t.Errorf("Canonicalize() = %+v, want %+v", got, want)
	}
}

func TestNewEventAcceptsValidInputAndComputesPeriod(t *testing.T) {
	in := validEventInput()
	in.Title = " Kirchweihmarkt "

	got, problems := newEvent(in)
	if problems != nil {
		t.Fatalf("newEvent problems = %v", problems)
	}
	want := Event{
		Title:      "Kirchweihmarkt",
		TitleKey:   "kirchweihmarkt",
		Type:       EventTypeMarket,
		LocationID: hallID,
		Times:      EventTimes{StartDate: kirchweihFriday},
		Source:     EventSource{Description: "Amtsblatt 41/2026", URL: "https://www.zirndorf.de/amtsblatt"},
		Note:       "Mit Fahrgeschäften",
	}
	wantStart, wantEnd := berlinInstant(t, kirchweihFriday, 0, 0), berlinInstant(t, kirchweihFriday.NextDay(), 0, 0)
	if !got.Period.Start.Equal(wantStart) || !got.Period.End.Equal(wantEnd) {
		t.Errorf("period = [%v, %v), want [%v, %v)", got.Period.Start, got.Period.End, wantStart, wantEnd)
	}
	got.Period = Period{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newEvent = %+v, want %+v", got, want)
	}
}

func TestNewEventParsesTimesAndAllDay(t *testing.T) {
	tests := map[string]struct {
		change func(*EventInput)
		want   EventTimes
	}{
		"exact start and end": {
			func(in *EventInput) { in.StartTime, in.EndDate, in.EndTime = "19:00", "2026-10-16", "23:30" },
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0), EndDate: kirchweihFriday, EndTime: localTime(23, 30)},
		},
		"all day over several days": {
			func(in *EventInput) { in.EndDate, in.AllDay = "2026-10-19", true },
			EventTimes{StartDate: kirchweihFriday, EndDate: kirchweihMonday, AllDay: true},
		},
		"midnight is a known time": {
			func(in *EventInput) { in.StartTime = "00:00" },
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(0, 0)},
		},
		"last minute of the day": {
			func(in *EventInput) { in.StartTime = "23:59" },
			EventTimes{StartDate: kirchweihFriday, StartTime: localTime(23, 59)},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := validEventInput()
			tt.change(&in)

			got, problems := newEvent(in)
			if problems != nil {
				t.Fatalf("newEvent problems = %v", problems)
			}
			if !equalEventTimes(got.Times, tt.want) {
				t.Errorf("times = %s, want %s", formatTimes(got.Times), formatTimes(tt.want))
			}
		})
	}
}

func TestNewEventAcceptsEveryTypeAndHTTPLinks(t *testing.T) {
	for _, eventType := range EventTypes() {
		in := validEventInput()
		in.Type = string(eventType)
		if got, problems := newEvent(in); problems != nil || got.Type != eventType {
			t.Errorf("newEvent with %q = %+v, %v", eventType, got, problems)
		}
	}
	for _, link := range []string{"", "http://zirndorf.de", "HTTPS://zirndorf.de/amtsblatt?ausgabe=41#seite-3"} {
		in := validEventInput()
		in.Source.URL = link
		if got, problems := newEvent(in); problems != nil || got.Source.URL != link {
			t.Errorf("newEvent with link %q = %+v, %v", link, got.Source, problems)
		}
	}
}

func TestNewEventReportsEveryInvalidField(t *testing.T) {
	tests := map[string]struct {
		change func(*EventInput)
		want   []FieldError
	}{
		"all required fields missing or blank": {
			change: func(in *EventInput) {
				*in = EventInput{Title: "  ", Type: "", LocationID: " ", StartDate: "", Source: EventSource{Description: " "}}
			},
			want: []FieldError{
				{Field: EventFieldTitle, Problem: ProblemMissing},
				{Field: EventFieldType, Problem: ProblemMissing},
				{Field: EventFieldLocationID, Problem: ProblemMissing},
				{Field: EventFieldStartDate, Problem: ProblemMissing},
				{Field: EventFieldSourceDescription, Problem: ProblemMissing},
			},
		},
		"unknown type": {
			change: func(in *EventInput) { in.Type = "concert" },
			want:   []FieldError{{Field: EventFieldType, Problem: ProblemUnknownCode}},
		},
		"ftp link": {
			change: func(in *EventInput) { in.Source.URL = "ftp://x" },
			want:   []FieldError{{Field: EventFieldSourceURL, Problem: ProblemInvalidFormat}},
		},
		"link without scheme": {
			change: func(in *EventInput) { in.Source.URL = "amtsblatt.de" },
			want:   []FieldError{{Field: EventFieldSourceURL, Problem: ProblemInvalidFormat}},
		},
		"link without host": {
			change: func(in *EventInput) { in.Source.URL = "https://" },
			want:   []FieldError{{Field: EventFieldSourceURL, Problem: ProblemInvalidFormat}},
		},
		"unparsable link": {
			change: func(in *EventInput) { in.Source.URL = "http://zirndorf.de/%zz" },
			want:   []FieldError{{Field: EventFieldSourceURL, Problem: ProblemInvalidFormat}},
		},
		"German date and spoken time": {
			change: func(in *EventInput) { in.StartDate, in.StartTime = "16.10.2026", "7 Uhr" },
			want:   []FieldError{{Field: EventFieldStartDate, Problem: ProblemInvalidFormat}, {Field: EventFieldStartTime, Problem: ProblemInvalidFormat}},
		},
		"malformed end date and end time": {
			change: func(in *EventInput) { in.EndDate, in.EndTime = "2026-10-1", "7:00" },
			want:   []FieldError{{Field: EventFieldEndDate, Problem: ProblemInvalidFormat}, {Field: EventFieldEndTime, Problem: ProblemInvalidFormat}},
		},
		"date that does not exist": {
			change: func(in *EventInput) { in.StartDate = "2026-02-30" },
			want:   []FieldError{{Field: EventFieldStartDate, Problem: ProblemInvalidFormat}},
		},
		"year zero": {
			change: func(in *EventInput) { in.StartDate = "0000-01-01" },
			want:   []FieldError{{Field: EventFieldStartDate, Problem: ProblemInvalidFormat}},
		},
		"date with sign": {
			change: func(in *EventInput) { in.StartDate = "+026-10-16" },
			want:   []FieldError{{Field: EventFieldStartDate, Problem: ProblemInvalidFormat}},
		},
		"time out of range": {
			change: func(in *EventInput) { in.StartTime, in.EndDate, in.EndTime = "24:00", "2026-10-16", "12:60" },
			want:   []FieldError{{Field: EventFieldStartTime, Problem: ProblemInvalidFormat}, {Field: EventFieldEndTime, Problem: ProblemInvalidFormat}},
		},
		"time with seconds": {
			change: func(in *EventInput) { in.StartTime = "19:00:00" },
			want:   []FieldError{{Field: EventFieldStartTime, Problem: ProblemInvalidFormat}},
		},
		"end before start": {
			change: func(in *EventInput) { in.StartTime, in.EndDate, in.EndTime = "19:00", "2026-10-16", "18:00" },
			want:   []FieldError{{Field: EventFieldEndTime, Problem: ProblemNotAfterStart}},
		},
		"all day with time": {
			change: func(in *EventInput) { in.StartTime, in.AllDay = "19:00", true },
			want:   []FieldError{{Field: EventFieldStartTime, Problem: ProblemConflictsWithAllDay}},
		},
		"spring gap": {
			change: func(in *EventInput) { in.StartDate, in.StartTime = "2027-03-28", "02:30" },
			want:   []FieldError{{Field: EventFieldStartTime, Problem: ProblemNonexistentTime}},
		},
		"text problems with a time problem": {
			change: func(in *EventInput) { in.Title, in.EndTime = "", "22:00" },
			want:   []FieldError{{Field: EventFieldTitle, Problem: ProblemMissing}, {Field: EventFieldEndDate, Problem: ProblemMissing}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := validEventInput()
			tt.change(&in)

			_, problems := newEvent(in)

			if !slices.Equal(problems, tt.want) {
				t.Errorf("problems = %v, want %v", problems, tt.want)
			}
		})
	}
}

func TestLocalDateAndTimeFormatAsTheyAreParsed(t *testing.T) {
	date := LocalDate{2026, time.March, 7}
	if got := date.String(); got != "2026-03-07" {
		t.Errorf("LocalDate.String() = %q, want 2026-03-07", got)
	}
	if got := (LocalTime{Hour: 7, Minute: 5}).String(); got != "07:05" {
		t.Errorf("LocalTime.String() = %q, want 07:05", got)
	}
	parsed, problem := parseLocalDate(date.String())
	if problem != "" || parsed != date {
		t.Errorf("parseLocalDate(%q) = %+v, %q", date.String(), parsed, problem)
	}
	parsedTime, problem := parseLocalTime("07:05")
	if problem != "" || parsedTime == nil || *parsedTime != (LocalTime{Hour: 7, Minute: 5}) {
		t.Errorf("parseLocalTime(07:05) = %v, %q", parsedTime, problem)
	}
}

func TestEventInputOfRoundTripsAStoredEvent(t *testing.T) {
	in := validEventInput()
	in.StartTime, in.EndDate, in.EndTime = "19:00", "2026-10-19", "22:00"
	event, problems := newEvent(in)
	if problems != nil {
		t.Fatalf("newEvent problems = %v", problems)
	}

	if got := EventInputOf(event); !reflect.DeepEqual(got, in) {
		t.Errorf("EventInputOf = %+v, want %+v", got, in)
	}
	allDay := EventInputOf(Event{Times: EventTimes{StartDate: kirchweihFriday, AllDay: true}})
	if allDay.StartDate != "2026-10-16" || allDay.StartTime != "" || allDay.EndDate != "" || allDay.EndTime != "" || !allDay.AllDay {
		t.Errorf("EventInputOf(all day) = %+v, want only start date and all day", allDay)
	}
}

func equalEventTimes(a, b EventTimes) bool {
	return formatTimes(a) == formatTimes(b)
}

// formatTimes spells out the times including the values behind the
// pointers, so comparisons and messages do not depend on addresses.
func formatTimes(times EventTimes) string {
	return EventInputOf(Event{Times: times}).timesText()
}

// timesText joins the time fields for test messages.
func (in EventInput) timesText() string {
	allDay := ""
	if in.AllDay {
		allDay = " allDay"
	}
	return in.StartDate + " " + in.StartTime + " - " + in.EndDate + " " + in.EndTime + allDay
}

func TestEventFieldNamesOfTheImport(t *testing.T) {
	got := []string{EventFieldLocation, EventFieldImportKey}
	want := []string{"location", "importKey"}
	if !slices.Equal(got, want) {
		t.Errorf("import field names = %v, want %v", got, want)
	}
}

func TestCanonicalizeNormalizesImportKeyAndBroughtLocation(t *testing.T) {
	in := validEventInput()
	in.LocationID = ""
	in.ImportKey = " kirchweih-2026 "
	in.Location = &LocationInput{
		Name: " " + decomposedOelmuehle + " ", Street: " Am Bach 1 ", PostalCode: " 90513 ", City: " Zirndorf ",
		Latitude: " 49.4 ", Longitude: " 10.9 ", Precision: " building ", Note: "  ",
	}

	got := in.Canonicalize()

	if got.ImportKey != "kirchweih-2026" {
		t.Errorf("ImportKey = %q, want it trimmed", got.ImportKey)
	}
	want := LocationInput{
		Name: composedOelmuehle, Street: "Am Bach 1", PostalCode: "90513", City: "Zirndorf",
		Latitude: "49.4", Longitude: "10.9", Precision: "building", Note: "",
	}
	if got.Location == nil || *got.Location != want {
		t.Errorf("Location = %+v, want %+v", got.Location, want)
	}
	if in.Location.Name == composedOelmuehle {
		t.Error("Canonicalize changed the location of its receiver")
	}
}

func TestNewEventAsksForNoLocationIDWhenTheLocationIsBroughtAlong(t *testing.T) {
	in := validEventInput()
	in.LocationID = ""
	in.Location = &LocationInput{Name: "Paul-Metz-Halle"}

	if _, problems := newEvent(in); problems != nil {
		t.Errorf("problems = %v, want none", problems)
	}
}

func TestNewEventChecksTheLimitsOfENT24(t *testing.T) {
	tests := map[string]struct {
		change func(*EventInput)
		want   []FieldError
	}{
		"title": {
			change: func(in *EventInput) { in.Title = overLimit(MaxTitleLength) },
			want:   []FieldError{{Field: EventFieldTitle, Problem: ProblemTooLong, Limit: MaxTitleLength}},
		},
		"source description": {
			change: func(in *EventInput) { in.Source.Description = overLimit(MaxSourceDescriptionLength) },
			want:   []FieldError{{Field: EventFieldSourceDescription, Problem: ProblemTooLong, Limit: MaxSourceDescriptionLength}},
		},
		"source url": {
			change: func(in *EventInput) { in.Source.URL = "https://zirndorf.de/" + overLimit(MaxSourceURLLength) },
			want:   []FieldError{{Field: EventFieldSourceURL, Problem: ProblemTooLong, Limit: MaxSourceURLLength}},
		},
		"note": {
			change: func(in *EventInput) { in.Note = overLimit(MaxNoteLength) },
			want:   []FieldError{{Field: EventFieldNote, Problem: ProblemTooLong, Limit: MaxNoteLength}},
		},
		"import key": {
			change: func(in *EventInput) { in.ImportKey = overLimit(MaxImportKeyLength) },
			want:   []FieldError{{Field: EventFieldImportKey, Problem: ProblemTooLong, Limit: MaxImportKeyLength}},
		},
		"timetable": {
			change: func(in *EventInput) {
				in.Timetable = slices.Repeat([]TimetableEntryInput{{Description: "Musik", Date: "2026-10-16"}}, MaxTimetableEntries+1)
			},
			want: []FieldError{{Field: EventFieldTimetable, Problem: ProblemTooMany, Limit: MaxTimetableEntries}},
		},
		"surrounding whitespace does not count": {
			change: func(in *EventInput) { in.Title = "  " + overLimit(MaxTitleLength-1) + "  " },
			want:   nil,
		},
		"every limit at once is exactly allowed": {
			change: func(in *EventInput) {
				in.Title = overLimit(MaxTitleLength - 1)
				in.Source.Description = overLimit(MaxSourceDescriptionLength - 1)
				in.Note = overLimit(MaxNoteLength - 1)
				in.ImportKey = overLimit(MaxImportKeyLength - 1)
				in.Timetable = slices.Repeat([]TimetableEntryInput{{Description: "Musik", Date: "2026-10-16"}}, MaxTimetableEntries)
			},
			want: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := validEventInput()
			tt.change(&in)

			_, problems := newEvent(in)

			if !slices.Equal(problems, tt.want) {
				t.Errorf("problems = %v, want %v", problems, tt.want)
			}
		})
	}
}
