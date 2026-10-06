package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestLimitsAreThoseOfENT24(t *testing.T) {
	got := map[string]int{
		"title":                 MaxTitleLength,
		"location.name":         MaxLocationNameLength,
		"street":                MaxStreetLength,
		"importKey":             MaxImportKeyLength,
		"city":                  MaxCityLength,
		"source.description":    MaxSourceDescriptionLength,
		"timetable description": MaxTimetableDescriptionLength,
		"note":                  MaxNoteLength,
		"location note":         MaxLocationNoteLength,
		"source.url":            MaxSourceURLLength,
		"timetable":             MaxTimetableEntries,
	}
	want := map[string]int{
		"title":                 200,
		"location.name":         200,
		"street":                200,
		"importKey":             200,
		"city":                  100,
		"source.description":    500,
		"timetable description": 500,
		"note":                  2000,
		"location note":         2000,
		"source.url":            2000,
		"timetable":             100,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("limits = %v, want %v", got, want)
	}
}

func TestCheckLengthCountsCodePoints(t *testing.T) {
	const limit = 3
	tests := []struct {
		name string
		text string
		want []FieldError
	}{
		{name: "empty", text: "", want: nil},
		{name: "at the limit in multi-byte letters", text: "äöü", want: nil},
		{name: "one over the limit", text: "äöüß", want: []FieldError{{Field: EventFieldTitle, Problem: ProblemTooLong, Limit: limit}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkLength(EventFieldTitle, tt.text, limit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("checkLength(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// overLimit returns a text of limit+1 letters, one too many.
func overLimit(limit int) string {
	return strings.Repeat("x", limit+1)
}
