package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// importObject is a JSON object of an import file under construction.
type importObject = map[string]any

// marketEntry returns a valid entry at the stored hall; tests change single
// fields.
func marketEntry() importObject {
	return importObject{
		"title":     "Kirchweihmarkt",
		"type":      "market",
		"location":  importObject{"name": "paul-metz-HALLE "},
		"startDate": "2026-10-16",
		"source":    importObject{"description": "Amtsblatt 41/2026", "url": "https://www.zirndorf.de/amtsblatt"},
	}
}

// concertEntry returns a valid entry that brings a complete new location.
func concertEntry() importObject {
	return importObject{
		"title":     "Konzert im Park",
		"type":      "culture",
		"startDate": "2026-10-17",
		"startTime": "19:00",
		"endDate":   "2026-10-17",
		"endTime":   "22:00",
		"location": importObject{
			"name":      "Alte Veste",
			"address":   importObject{"street": "Burgweg 1", "postalCode": "90513", "city": "Zirndorf"},
			"latitude":  49.4501,
			"longitude": 10.9376,
			"precision": "building",
			"note":      "Zugang über den Wald",
		},
		"source":    importObject{"description": "Plakat", "url": nil},
		"note":      nil,
		"timetable": []any{importObject{"description": "Vorband", "date": "2026-10-17", "startTime": "19:00", "endTime": nil}},
		"importKey": "konzert-2026",
	}
}

func importFile(entries ...any) importObject {
	return importObject{"formatVersion": ImportFormatVersion, "events": entries}
}

func marshalImport(t *testing.T, file any) []byte {
	t.Helper()
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("marshal import file: %v", err)
	}
	return data
}

func newTestImportService(locations *fakeLocationRepo) *ImportService {
	return NewImportService(newEventServiceOn(newFakeEventRepo(), locations))
}

func previewOf(t *testing.T, data []byte, locations ...Location) ImportPreview {
	t.Helper()
	return previewWithEvents(t, data, newFakeEventRepo(), locations...)
}

// previewWithEvents previews data against the stored events and locations.
func previewWithEvents(t *testing.T, data []byte, events *fakeEventRepo, locations ...Location) ImportPreview {
	t.Helper()
	preview, err := NewImportService(newEventServiceOn(events, newFakeLocationRepo(locations...))).PreviewImport(context.Background(), data)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	return preview
}

// validCount returns how many entries of preview have no problem.
func validCount(preview ImportPreview) int {
	return len(preview.Entries) - preview.CountOf(ImportClassError)
}

func TestPreviewImportAcceptsValidEntriesWithStoredAndNewLocations(t *testing.T) {
	locations := newFakeLocationRepo(hall())
	data := marshalImport(t, importFile(marketEntry(), concertEntry()))

	preview, err := newTestImportService(locations).PreviewImport(context.Background(), data)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}

	if validCount(preview) != 2 || preview.CountOf(ImportClassError) != 0 {
		t.Errorf("valid %d, errors %d, want 2 and 0; entries %+v", validCount(preview), preview.CountOf(ImportClassError), preview.Entries)
	}
	want := EventInput{
		Title:     "Konzert im Park",
		Type:      "culture",
		StartDate: "2026-10-17",
		StartTime: "19:00",
		EndDate:   "2026-10-17",
		EndTime:   "22:00",
		Location: &LocationInput{
			Name: "Alte Veste", Street: "Burgweg 1", PostalCode: "90513", City: "Zirndorf",
			Latitude: "49.4501", Longitude: "10.9376", Precision: "building", Note: "Zugang über den Wald",
		},
		Source:    EventSource{Description: "Plakat"},
		Timetable: []TimetableEntryInput{{Description: "Vorband", Date: "2026-10-17", StartTime: "19:00"}},
		ImportKey: "konzert-2026",
	}
	concert := preview.Entries[1]
	if concert.Position != 2 || concert.Title != "Konzert im Park" || !reflect.DeepEqual(concert.Input, want) {
		t.Errorf("entry 2 = %+v, want position 2 with input %+v", concert, want)
	}
	if market := preview.Entries[0]; market.Position != 1 || !market.IsValid() {
		t.Errorf("entry 1 = %+v, want valid at position 1", market)
	}
	if len(locations.created) != 0 || len(locations.updated) != 0 {
		t.Error("previewing an import wrote locations")
	}
}

func TestPreviewImportIgnoresTheOtherFieldsOfAStoredLocation(t *testing.T) {
	entry := marketEntry()
	entry["location"] = importObject{
		"name":    "Paul-Metz-Halle",
		"address": importObject{"street": "", "postalCode": "9051", "city": nil},
		"note":    strings.Repeat("x", MaxLocationNoteLength+1),
	}

	preview := previewOf(t, marshalImport(t, importFile(entry)), hall())

	if !preview.Entries[0].IsValid() {
		t.Errorf("problems = %v, want none for a stored location", preview.Entries[0].Problems)
	}
}

func TestPreviewImportIgnoresUnknownFields(t *testing.T) {
	entry := marketEntry()
	entry["organizer"] = "Stadt Zirndorf"
	entry["location"].(importObject)["district"] = "Weiherhof"
	file := importFile(entry)
	file["generatedBy"] = "Recherche-Skript"

	preview := previewOf(t, marshalImport(t, file), hall())

	if !preview.Entries[0].IsValid() {
		t.Errorf("problems = %v, want none", preview.Entries[0].Problems)
	}
}

func TestPreviewImportRejectsTheWholeFile(t *testing.T) {
	tooLarge := append([]byte(`{"formatVersion":1,"events":[`), make([]byte, MaxImportFileBytes)...)
	tests := map[string]struct {
		data []byte
		want ImportFileProblem
	}{
		"no JSON":                     {data: []byte("title;type\nMarkt;market"), want: ImportProblemInvalidJSON},
		"JSON but no object":          {data: []byte(`[{"title":"Markt"}]`), want: ImportProblemInvalidJSON},
		"trailing data":               {data: []byte(`{"formatVersion":1,"events":[{}]} {}`), want: ImportProblemInvalidJSON},
		"without formatVersion":       {data: []byte(`{"events":[{}]}`), want: ImportProblemMissingFormatVersion},
		"formatVersion null":          {data: []byte(`{"formatVersion":null,"events":[{}]}`), want: ImportProblemMissingFormatVersion},
		"formatVersion 2":             {data: []byte(`{"formatVersion":2,"events":[{}]}`), want: ImportProblemUnknownFormatVersion},
		"formatVersion as text":       {data: []byte(`{"formatVersion":"1","events":[{}]}`), want: ImportProblemUnknownFormatVersion},
		"formatVersion not a integer": {data: []byte(`{"formatVersion":1.5,"events":[{}]}`), want: ImportProblemUnknownFormatVersion},
		"without events":              {data: []byte(`{"formatVersion":1}`), want: ImportProblemMissingEvents},
		"events null":                 {data: []byte(`{"formatVersion":1,"events":null}`), want: ImportProblemMissingEvents},
		"events not a list":           {data: []byte(`{"formatVersion":1,"events":{"title":"Markt"}}`), want: ImportProblemMissingEvents},
		"no entries":                  {data: []byte(`{"formatVersion":1,"events":[]}`), want: ImportProblemNoEntries},
		"more than 2 MiB":             {data: tooLarge, want: ImportProblemTooLarge},
		"more than 150 entries":       {data: importFileWithEntries(MaxImportEntries + 1), want: ImportProblemTooManyEntries},
		// 0xFC is ü in Windows-1252, as an editor saves a file in ANSI.
		"not UTF-8":                         {data: []byte(`{"formatVersion":1,"events":[{"title":"Gr` + "\xfc" + `n"}]}`), want: ImportProblemInvalidEncoding},
		"not UTF-8 after a byte order mark": {data: []byte("\xef\xbb\xbf" + `{"formatVersion":1,"events":[{"title":"Gr` + "\xfc" + `n"}]}`), want: ImportProblemInvalidEncoding},
		"neither UTF-8 nor JSON":            {data: []byte("Titel;Gr\xfcn"), want: ImportProblemInvalidEncoding},
		"more than 2 MiB and not UTF-8":     {data: slices.Repeat([]byte{0xFC}, MaxImportFileBytes+1), want: ImportProblemTooLarge},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			locations := newFakeLocationRepo()
			locations.listErr = errDatabaseDown

			preview, err := newTestImportService(locations).PreviewImport(context.Background(), tt.data)

			var fileErr *ImportFileError
			if !errors.As(err, &fileErr) || fileErr.Problem != tt.want {
				t.Fatalf("err = %v, want *ImportFileError %s", err, tt.want)
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("errors.Is(%v, ErrValidation) = false", err)
			}
			if !strings.Contains(err.Error(), string(tt.want)) {
				t.Errorf("message %q does not name %s", err.Error(), tt.want)
			}
			if preview.Entries != nil {
				t.Errorf("entries = %v, want none", preview.Entries)
			}
		})
	}
}

func TestPreviewImportAcceptsTheMostEntries(t *testing.T) {
	locations := newFakeLocationRepo()

	preview, err := newTestImportService(locations).PreviewImport(context.Background(), importFileWithEntries(MaxImportEntries))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if len(preview.Entries) != MaxImportEntries {
		t.Errorf("entries = %d, want %d", len(preview.Entries), MaxImportEntries)
	}
}

// importFileWithEntries returns an import file of count empty entries.
func importFileWithEntries(count int) []byte {
	entries := strings.TrimSuffix(strings.Repeat("{},", count), ",")
	return []byte(`{"formatVersion":1,"events":[` + entries + `]}`)
}

func TestPreviewImportAcceptsAFileWithByteOrderMark(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, marshalImport(t, importFile(marketEntry()))...)

	preview := previewOf(t, data, hall())

	if validCount(preview) != 1 {
		t.Errorf("entries = %+v, want one valid", preview.Entries)
	}
}

func TestPreviewImportAcceptsFormatVersionWrittenAsDecimal(t *testing.T) {
	data := []byte(strings.Replace(string(marshalImport(t, importFile(marketEntry()))), `"formatVersion":1`, `"formatVersion":1.0`, 1))

	preview := previewOf(t, data, hall())

	if validCount(preview) != 1 {
		t.Errorf("entries = %+v, want one valid", preview.Entries)
	}
}

func TestPreviewImportAcceptsAFileOfExactlyTheMaximumSize(t *testing.T) {
	data := marshalImport(t, importFile(marketEntry()))
	data = append(data, strings.Repeat(" ", MaxImportFileBytes-len(data))...)

	preview := previewOf(t, data, hall())

	if validCount(preview) != 1 {
		t.Errorf("entries = %+v, want one valid", preview.Entries)
	}
}

func TestPreviewImportReportsProblemsPerEntry(t *testing.T) {
	tests := map[string]struct {
		change func(importObject)
		want   []FieldError
	}{
		"invalid date": {
			change: func(e importObject) { e["startDate"] = "16.10.2026" },
			want:   []FieldError{{Field: EventFieldStartDate, Problem: ProblemInvalidFormat}},
		},
		"unknown type": {
			change: func(e importObject) { e["type"] = "concert" },
			want:   []FieldError{{Field: EventFieldType, Problem: ProblemUnknownCode}},
		},
		"without location": {
			change: func(e importObject) { delete(e, "location") },
			want:   []FieldError{{Field: EventFieldLocation, Problem: ProblemMissing}},
		},
		"location null": {
			change: func(e importObject) { e["location"] = nil },
			want:   []FieldError{{Field: EventFieldLocation, Problem: ProblemMissing}},
		},
		"without location name": {
			change: func(e importObject) { e["location"] = importObject{"name": "  ", "precision": "building"} },
			want:   []FieldError{{Field: "location.name", Problem: ProblemMissing}},
		},
		"new location without address, coordinates and precision": {
			change: func(e importObject) { e["location"] = importObject{"name": "Alte Veste"} },
			want: []FieldError{
				{Field: "location.address.street", Problem: ProblemMissing},
				{Field: "location.address.postalCode", Problem: ProblemMissing},
				{Field: "location.address.city", Problem: ProblemMissing},
				{Field: "location.latitude", Problem: ProblemMissing},
				{Field: "location.longitude", Problem: ProblemMissing},
				{Field: "location.precision", Problem: ProblemMissing},
			},
		},
		"new location with four-digit postal code": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["address"].(importObject)["postalCode"] = "9051"
				e["location"] = location
			},
			want: []FieldError{{Field: "location.address.postalCode", Problem: ProblemInvalidFormat}},
		},
		"new location with too long a name": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["name"] = overLimit(MaxLocationNameLength)
				e["location"] = location
			},
			want: []FieldError{{Field: "location.name", Problem: ProblemTooLong, Limit: MaxLocationNameLength}},
		},
		"title too long": {
			change: func(e importObject) { e["title"] = overLimit(MaxTitleLength) },
			want:   []FieldError{{Field: EventFieldTitle, Problem: ProblemTooLong, Limit: MaxTitleLength}},
		},
		"too many timetable entries": {
			change: func(e importObject) {
				entry := importObject{"description": "Musik", "date": "2026-10-16"}
				e["timetable"] = slices.Repeat([]any{entry}, MaxTimetableEntries+1)
			},
			want: []FieldError{{Field: EventFieldTimetable, Problem: ProblemTooMany, Limit: MaxTimetableEntries}},
		},
		"timetable entry outside the event": {
			change: func(e importObject) {
				e["timetable"] = []any{
					importObject{"description": "Musik", "date": "2026-10-16"},
					importObject{"description": "Abbau", "date": "2026-10-18", "startTime": "10:00"},
				}
			},
			want: []FieldError{{Field: "timetable[1].startTime", Problem: ProblemOutsideEvent}},
		},
		"import key too long": {
			change: func(e importObject) { e["importKey"] = overLimit(MaxImportKeyLength) },
			want:   []FieldError{{Field: EventFieldImportKey, Problem: ProblemTooLong, Limit: MaxImportKeyLength}},
		},
		"without source": {
			change: func(e importObject) { delete(e, "source") },
			want:   []FieldError{{Field: EventFieldSourceDescription, Problem: ProblemMissing}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			broken := marketEntry()
			tt.change(broken)

			preview := previewOf(t, marshalImport(t, importFile(marketEntry(), broken, concertEntry())), hall())

			if validCount(preview) != 2 || preview.CountOf(ImportClassError) != 1 {
				t.Errorf("valid %d, errors %d, want 2 and 1", validCount(preview), preview.CountOf(ImportClassError))
			}
			got := preview.Entries[1]
			if got.Position != 2 || got.Title != broken["title"] || got.IsValid() {
				t.Errorf("entry = %+v, want invalid at position 2 with its title", got)
			}
			if !slices.Equal(got.Problems, tt.want) {
				t.Errorf("problems = %v, want %v", got.Problems, tt.want)
			}
		})
	}
}

func TestPreviewImportReportsJSONTypeErrorsOnlyAtTheirField(t *testing.T) {
	tests := map[string]struct {
		change    func(importObject)
		want      []FieldError
		wantTitle string
	}{
		"title as number": {
			change:    func(e importObject) { e["title"] = 42 },
			want:      []FieldError{{Field: EventFieldTitle, Problem: ProblemInvalidFormat}},
			wantTitle: "",
		},
		"allDay as text": {
			change: func(e importObject) { e["allDay"] = "ja" },
			want:   []FieldError{{Field: EventFieldAllDay, Problem: ProblemInvalidFormat}},
		},
		"source as text": {
			change: func(e importObject) { e["source"] = "Amtsblatt" },
			want:   []FieldError{{Field: "source", Problem: ProblemInvalidFormat}},
		},
		"source url as number": {
			change: func(e importObject) { e["source"] = importObject{"description": "Amtsblatt", "url": 7} },
			want:   []FieldError{{Field: EventFieldSourceURL, Problem: ProblemInvalidFormat}},
		},
		"location as text": {
			change: func(e importObject) { e["location"] = "Paul-Metz-Halle" },
			want:   []FieldError{{Field: EventFieldLocation, Problem: ProblemInvalidFormat}},
		},
		"location name as number": {
			change: func(e importObject) { e["location"] = importObject{"name": 1} },
			want:   []FieldError{{Field: "location.name", Problem: ProblemInvalidFormat}},
		},
		"address as list": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["address"] = []any{"Burgweg 1"}
				e["location"] = location
			},
			want: []FieldError{{Field: "location.address", Problem: ProblemInvalidFormat}},
		},
		"postal code as number": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["address"].(importObject)["postalCode"] = 90513
				e["location"] = location
			},
			want: []FieldError{{Field: "location.address.postalCode", Problem: ProblemInvalidFormat}},
		},
		"latitude as text": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["latitude"] = "49,45"
				e["location"] = location
			},
			want: []FieldError{{Field: "location.latitude", Problem: ProblemInvalidFormat}},
		},
		"latitude as text at a stored location": {
			change: func(e importObject) { e["location"] = importObject{"name": "Paul-Metz-Halle", "latitude": "49,45"} },
			want:   []FieldError{{Field: "location.latitude", Problem: ProblemInvalidFormat}},
		},
		"timetable as text": {
			change: func(e importObject) { e["timetable"] = "ab 18 Uhr Musik" },
			want:   []FieldError{{Field: EventFieldTimetable, Problem: ProblemInvalidFormat}},
		},
		"timetable entry as text": {
			change: func(e importObject) {
				e["timetable"] = []any{importObject{"description": "Musik", "date": "2026-10-16"}, "Abbau"}
			},
			want: []FieldError{{Field: "timetable[1]", Problem: ProblemInvalidFormat}},
		},
		"timetable date as number": {
			change: func(e importObject) {
				e["timetable"] = []any{importObject{"description": "Musik", "date": 20261016}}
			},
			want: []FieldError{{Field: "timetable[0].date", Problem: ProblemInvalidFormat}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			broken := marketEntry()
			tt.change(broken)
			wantTitle := "Kirchweihmarkt"
			if _, isText := broken["title"].(string); !isText {
				wantTitle = tt.wantTitle
			}

			preview := previewOf(t, marshalImport(t, importFile(broken, marketEntry())), hall())

			got := preview.Entries[0]
			if !slices.Equal(got.Problems, tt.want) {
				t.Errorf("problems = %v, want %v", got.Problems, tt.want)
			}
			if got.Title != wantTitle {
				t.Errorf("title = %q, want %q", got.Title, wantTitle)
			}
			if !preview.Entries[1].IsValid() {
				t.Errorf("the next entry has problems %v", preview.Entries[1].Problems)
			}
		})
	}
}

func TestPreviewImportReportsControlCharactersOnlyAtTheirField(t *testing.T) {
	tests := map[string]struct {
		change func(importObject)
		want   []FieldError
	}{
		"NUL in the title": {
			change: func(e importObject) { e["title"] = "A\u0000B" },
			want:   []FieldError{{Field: EventFieldTitle, Problem: ProblemControlCharacter}},
		},
		"only NUL as title": {
			change: func(e importObject) { e["title"] = "\u0000" },
			want:   []FieldError{{Field: EventFieldTitle, Problem: ProblemControlCharacter}},
		},
		"escape in the type": {
			change: func(e importObject) { e["type"] = "market\u001b" },
			want:   []FieldError{{Field: EventFieldType, Problem: ProblemControlCharacter}},
		},
		"DEL in the start date": {
			change: func(e importObject) { e["startDate"] = "2026-10-16\u007f" },
			want:   []FieldError{{Field: EventFieldStartDate, Problem: ProblemControlCharacter}},
		},
		"C1 control in the note": {
			change: func(e importObject) { e["note"] = "a\u0085b" },
			want:   []FieldError{{Field: EventFieldNote, Problem: ProblemControlCharacter}},
		},
		"NUL in the import key": {
			change: func(e importObject) { e["importKey"] = "markt\u00002026" },
			want:   []FieldError{{Field: EventFieldImportKey, Problem: ProblemControlCharacter}},
		},
		"NUL in the source url": {
			change: func(e importObject) {
				e["source"] = importObject{"description": "Amtsblatt", "url": "https://www.zirndorf.de/\u0000"}
			},
			want: []FieldError{{Field: EventFieldSourceURL, Problem: ProblemControlCharacter}},
		},
		"NUL in the source description": {
			change: func(e importObject) {
				e["source"] = importObject{"description": "Amtsblatt\u0000", "url": "https://www.zirndorf.de/amtsblatt"}
			},
			want: []FieldError{{Field: EventFieldSourceDescription, Problem: ProblemControlCharacter}},
		},
		"NUL in the note of a new location": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["note"] = "Zugang\u0000"
				e["location"] = location
			},
			want: []FieldError{{Field: "location.note", Problem: ProblemControlCharacter}},
		},
		"NUL in the city of a new location": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["address"].(importObject)["city"] = "Zirndorf\u0000"
				e["location"] = location
			},
			want: []FieldError{{Field: "location.address.city", Problem: ProblemControlCharacter}},
		},
		"NUL in the location name": {
			change: func(e importObject) { e["location"] = importObject{"name": "Paul-Metz-Halle\u0000"} },
			want:   []FieldError{{Field: "location.name", Problem: ProblemControlCharacter}},
		},
		"NUL in the street of a new location": {
			change: func(e importObject) {
				location := concertEntry()["location"].(importObject)
				location["address"].(importObject)["street"] = "Burgweg\u00001"
				e["location"] = location
			},
			want: []FieldError{{Field: "location.address.street", Problem: ProblemControlCharacter}},
		},
		"NUL in a timetable description": {
			change: func(e importObject) {
				e["timetable"] = []any{importObject{"description": "Musik\u0000", "date": "2026-10-16"}}
			},
			want: []FieldError{{Field: "timetable[0].description", Problem: ProblemControlCharacter}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			broken := marketEntry()
			tt.change(broken)

			preview := previewOf(t, marshalImport(t, importFile(broken, marketEntry())), hall())

			got := preview.Entries[0]
			if got.Class != ImportClassError || !slices.Equal(got.Problems, tt.want) {
				t.Errorf("entry = %s with problems %v, want error with %v", got.Class, got.Problems, tt.want)
			}
			if !preview.Entries[1].IsValid() {
				t.Errorf("the next entry has problems %v", preview.Entries[1].Problems)
			}
		})
	}
}

func TestPreviewImportAcceptsTabsAndLineBreaksInText(t *testing.T) {
	entry := marketEntry()
	entry["note"] = "Erste Zeile\nzweite Zeile\r\nmit\tTabulator"
	entry["title"] = "Kirchweih\tmarkt"

	preview := previewOf(t, marshalImport(t, importFile(entry)), hall())

	if !preview.Entries[0].IsValid() {
		t.Errorf("problems = %v, want none", preview.Entries[0].Problems)
	}
}

func TestPreviewImportRejectsALocationNameOfSeveralStoredLocations(t *testing.T) {
	// RecomputeNameKeys leaves such a collision group as it is: the names
	// differ, their NormalizeKey does not.
	twin := Location{ID: parkID, Name: "Paul-Metz-Halle ", NameKey: "paul-metz-halle-2", Precision: PrecisionBuilding}

	preview := previewOf(t, marshalImport(t, importFile(marketEntry(), concertEntry())), hall(), twin)

	want := []FieldError{{Field: "location.name", Problem: ProblemAmbiguous}}
	if got := preview.Entries[0]; got.Class != ImportClassError || !slices.Equal(got.Problems, want) {
		t.Errorf("entry = %s with problems %v, want error with %v", got.Class, got.Problems, want)
	}
	if !preview.Entries[1].IsValid() {
		t.Errorf("the next entry has problems %v", preview.Entries[1].Problems)
	}
}

func TestPreviewImportReportsAnEntryThatIsNoObject(t *testing.T) {
	preview := previewOf(t, []byte(`{"formatVersion":1,"events":["Kirchweihmarkt",null]}`))

	want := []FieldError{{Field: ImportFieldEntry, Problem: ProblemInvalidFormat}}
	for _, entry := range preview.Entries {
		if !slices.Equal(entry.Problems, want) || entry.Title != "" {
			t.Errorf("entry %d = %+v, want only %v", entry.Position, entry, want)
		}
	}
	if preview.CountOf(ImportClassError) != 2 {
		t.Errorf("errors = %d, want 2", preview.CountOf(ImportClassError))
	}
}

func TestPreviewImportPassesALocationListFailureOn(t *testing.T) {
	locations := newFakeLocationRepo()
	locations.listErr = errDatabaseDown

	_, err := newTestImportService(locations).PreviewImport(context.Background(), marshalImport(t, importFile(marketEntry())))

	if !errors.Is(err, errDatabaseDown) {
		t.Errorf("err = %v, want %v", err, errDatabaseDown)
	}
}

func TestImportFileFormatConstants(t *testing.T) {
	if MaxImportFileBytes != 2*1024*1024 || ImportFormatVersion != 1 {
		t.Errorf("MaxImportFileBytes = %d, ImportFormatVersion = %d", MaxImportFileBytes, ImportFormatVersion)
	}
	if ImportProblemInvalidEncoding != "invalidEncoding" {
		t.Errorf("ImportProblemInvalidEncoding = %q", ImportProblemInvalidEncoding)
	}
	if LocationFieldAddress != "address" {
		t.Errorf("LocationFieldAddress = %q", LocationFieldAddress)
	}
}

// The test collection in format v1 (Story 3.4). It holds 39 events, not the
// 41 of the source file: Andreas merged three autumn market entries into one
// event with a timetable. The Weihnachtsmarkt is entered in the admin.
const (
	testCollectionPath      = "../../testdata/zirndorf_events.v1.json"
	testCollectionEvents    = 39
	testCollectionLocations = 19
)

func TestPreviewImportAcceptsTheTestCollection(t *testing.T) {
	data, err := os.ReadFile(testCollectionPath)
	if err != nil {
		t.Fatalf("read test collection: %v", err)
	}

	preview := previewOf(t, data)

	if len(preview.Entries) != testCollectionEvents {
		t.Errorf("entries = %d, want %d", len(preview.Entries), testCollectionEvents)
	}
	for _, entry := range preview.Entries {
		if entry.Class != ImportClassNew {
			t.Errorf("entry %d %q is %s with problems %+v, want new", entry.Position, entry.Title, entry.Class, entry.Problems)
		}
	}
	if len(preview.NewLocations) != testCollectionLocations {
		t.Errorf("new locations = %d, want %d", len(preview.NewLocations), testCollectionLocations)
	}
	importKeys := map[string]bool{}
	for _, entry := range preview.Entries {
		key := entry.Input.ImportKey
		if key == "" || importKeys[key] {
			t.Errorf("entry %d %q has import key %q, want a unique one", entry.Position, entry.Title, key)
		}
		importKeys[key] = true
	}
}
