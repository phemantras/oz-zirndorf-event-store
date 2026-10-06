package core

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

// marketKey is the import key of the stored market in the tests.
const marketKey = "kirchweihmarkt-2026"

// storedMarket returns the stored event that marketEntry describes, with
// importKey.
func storedMarket(t *testing.T, id, importKey string) Event {
	t.Helper()
	event := storedEvent(t, id, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	event.Source = EventSource{Description: "Amtsblatt 41/2026", URL: "https://www.zirndorf.de/amtsblatt"}
	event.ImportKey = importKey
	return event
}

// keyedMarketEntry is marketEntry with the import key of the stored market.
func keyedMarketEntry() importObject {
	entry := marketEntry()
	entry["importKey"] = marketKey
	return entry
}

func assertClasses(t *testing.T, preview ImportPreview, want ...ImportClass) {
	t.Helper()
	var got []ImportClass
	for _, entry := range preview.Entries {
		got = append(got, entry.Class)
	}
	if !slices.Equal(got, want) {
		t.Errorf("classes = %v, want %v", got, want)
	}
}

func TestPreviewImportClassifiesAnEntryWithoutMatchAsNew(t *testing.T) {
	events := newFakeEventRepo(storedEvent(t, concertID, "Konzert", EventTimes{StartDate: kirchweihFriday}))

	preview := previewWithEvents(t, marshalImport(t, importFile(marketEntry())), events, hall())

	assertClasses(t, preview, ImportClassNew)
	entry := preview.Entries[0]
	if entry.TargetID != "" || entry.Changes != nil || entry.Candidates != nil || entry.Hints != nil {
		t.Errorf("entry = %+v, want new without target, changes, candidates or hints", entry)
	}
}

func TestPreviewImportClassifiesAChangedEntryOfAStoredKeyAsUpdate(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, marketKey))
	entry := keyedMarketEntry()
	entry["title"] = "Kirchweihmarkt 2026"
	entry["importKey"] = " " + marketKey + " "

	preview := previewWithEvents(t, marshalImport(t, importFile(entry)), events, hall())

	assertClasses(t, preview, ImportClassUpdate)
	got := preview.Entries[0]
	want := []ImportChange{{Field: EventFieldTitle, Old: "Kirchweihmarkt", New: "Kirchweihmarkt 2026"}}
	if got.TargetID != marketID || !slices.Equal(got.Changes, want) {
		t.Errorf("target %q, changes %v, want %s with %v", got.TargetID, got.Changes, marketID, want)
	}
	if len(events.created) != 0 || len(events.updated) != 0 {
		t.Error("previewing an import wrote events")
	}
}

func TestPreviewImportClassifiesAnEqualEntryOfAnArchivedEventAsUnchanged(t *testing.T) {
	past := LocalDate{2025, time.October, 17}
	stored := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: past, EndDate: past.NextDay()})
	stored.Source, stored.ImportKey = EventSource{Description: "Amtsblatt"}, marketKey
	stored.Timetable = []TimetableEntry{
		{ID: "1", Description: "Musik", Date: past, StartTime: &LocalTime{Hour: 18}},
		{ID: "2", Description: "Abbau", Date: past.NextDay()},
	}
	entry := importObject{
		"title": " Kirchweihmarkt ", "type": "market", "location": importObject{"name": "Paul-Metz-Halle"},
		"startDate": "2025-10-17", "endDate": "2025-10-18 ", "source": importObject{"description": "Amtsblatt  "},
		"note": " ", "importKey": marketKey,
		"timetable": []any{
			importObject{"description": "Abbau ", "date": "2025-10-18"},
			importObject{"description": "Musik", "date": "2025-10-17", "startTime": "18:00"},
		},
	}

	preview := previewWithEvents(t, marshalImport(t, importFile(entry)), newFakeEventRepo(stored), hall())

	assertClasses(t, preview, ImportClassUnchanged)
	if got := preview.Entries[0]; got.TargetID != marketID || got.Changes != nil || got.Hints != nil {
		t.Errorf("entry = %+v, want unchanged target %s without changes or hints", got, marketID)
	}
}

func TestPreviewImportNamesEveryChangedField(t *testing.T) {
	stored := storedMarket(t, marketID, marketKey)
	stored.Timetable = []TimetableEntry{{ID: "1", Description: "Musik", Date: kirchweihFriday, StartTime: &LocalTime{Hour: 18}}}
	entry := keyedMarketEntry()
	entry["type"] = "festival"
	entry["startTime"] = "10:00"
	entry["endDate"] = "2026-10-18"
	entry["endTime"] = "18:00"
	entry["source"] = importObject{"description": "Plakat"}
	entry["note"] = "Mit Fahrgeschäften"
	entry["timetable"] = []any{
		importObject{"description": "Musik", "date": "2026-10-16", "startTime": "18:00", "endTime": "20:00"},
		importObject{"description": "Tanz", "date": "2026-10-17"},
	}

	preview := previewWithEvents(t, marshalImport(t, importFile(entry)), newFakeEventRepo(stored), hall())

	want := []ImportChange{
		{Field: EventFieldType, Old: "market", New: "festival"},
		{Field: EventFieldStartTime, Old: "", New: "10:00"},
		{Field: EventFieldEndDate, Old: "", New: "2026-10-18"},
		{Field: EventFieldEndTime, Old: "", New: "18:00"},
		{Field: EventFieldSourceDescription, Old: "Amtsblatt 41/2026", New: "Plakat"},
		{Field: EventFieldSourceURL, Old: "https://www.zirndorf.de/amtsblatt", New: ""},
		{Field: EventFieldNote, Old: "", New: "Mit Fahrgeschäften"},
		{Field: EventFieldTimetable, Old: "2026-10-16 18:00 Musik", New: "2026-10-16 18:00–20:00 Musik\n2026-10-17 Tanz"},
	}
	assertClasses(t, preview, ImportClassUpdate)
	if got := preview.Entries[0].Changes; !slices.Equal(got, want) {
		t.Errorf("changes = %v, want %v", got, want)
	}
}

func TestPreviewImportNamesChangedDateAndAllDay(t *testing.T) {
	entry := keyedMarketEntry()
	entry["startDate"] = "2026-10-17"
	entry["allDay"] = true

	preview := previewWithEvents(t, marshalImport(t, importFile(entry)), newFakeEventRepo(storedMarket(t, marketID, marketKey)), hall())

	want := []ImportChange{
		{Field: EventFieldStartDate, Old: "2026-10-16", New: "2026-10-17"},
		{Field: EventFieldAllDay, Old: "false", New: "true"},
	}
	if got := preview.Entries[0].Changes; !slices.Equal(got, want) {
		t.Errorf("changes = %v, want %v", got, want)
	}
}

func TestPreviewImportNamesAChangedLocationByName(t *testing.T) {
	tests := map[string]struct {
		location importObject
		want     string
	}{
		"other stored location": {location: importObject{"name": "BIBERTPARK"}, want: "Bibertpark"},
		"new location":          {location: concertEntry()["location"].(importObject), want: "Alte Veste"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			entry := keyedMarketEntry()
			entry["location"] = tt.location

			preview := previewWithEvents(t, marshalImport(t, importFile(entry)), newFakeEventRepo(storedMarket(t, marketID, marketKey)), hall(), park())

			assertClasses(t, preview, ImportClassUpdate)
			want := []ImportChange{{Field: EventFieldLocation, Old: "Paul-Metz-Halle", New: tt.want}}
			if got := preview.Entries[0].Changes; !slices.Equal(got, want) {
				t.Errorf("changes = %v, want %v", got, want)
			}
		})
	}
}

func TestPreviewImportOnlyHintsAtADuplicateOfAnEntryWithStoredKey(t *testing.T) {
	duplicate := storedMarket(t, concertID, "")
	entry := keyedMarketEntry()
	entry["note"] = "Mit Fahrgeschäften"

	preview := previewWithEvents(t, marshalImport(t, importFile(entry)), newFakeEventRepo(storedMarket(t, marketID, marketKey), duplicate), hall())

	assertClasses(t, preview, ImportClassUpdate)
	got := preview.Entries[0]
	want := []ImportHint{{Kind: ImportHintDuplicate, Candidate: ImportCandidate{EventID: concertID, Title: "Kirchweihmarkt", StartDate: kirchweihFriday}}}
	if got.TargetID != marketID || got.Candidates != nil || !slices.Equal(got.Hints, want) {
		t.Errorf("entry = %+v, want target %s with hints %v and no candidates", got, marketID, want)
	}
}

func TestPreviewImportSuspectsADuplicateOfAStoredEvent(t *testing.T) {
	stored := storedMarket(t, marketID, "")
	sameKey := marketEntry()
	sameKey["title"] = " KIRCHWEIHMARKT"
	sameKey["importKey"] = "unbekannt"
	otherDate := marketEntry()
	otherDate["startDate"] = "2026-10-23"

	preview := previewWithEvents(t, marshalImport(t, importFile(sameKey, otherDate)), newFakeEventRepo(stored), hall())

	assertClasses(t, preview, ImportClassDuplicateSuspect, ImportClassNew)
	want := []ImportCandidate{{EventID: marketID, Title: "Kirchweihmarkt", StartDate: kirchweihFriday}}
	if got := preview.Entries[0]; got.TargetID != "" || !slices.Equal(got.Candidates, want) {
		t.Errorf("entry = %+v, want candidates %v without target", got, want)
	}
}

func TestPreviewImportRejectsAnImportKeyGivenTwiceInTheFile(t *testing.T) {
	first := marketEntry()
	first["importKey"] = "a"
	other := concertEntry()
	third := marketEntry()
	third["importKey"] = " a"
	third["startDate"] = "2026-10-23"
	broken := marketEntry()
	broken["importKey"] = "a"
	broken["type"] = "concert"

	preview := previewOf(t, marshalImport(t, importFile(first, other, third, broken)), hall())

	assertClasses(t, preview, ImportClassError, ImportClassNew, ImportClassError, ImportClassError)
	duplicateKey := FieldError{Field: EventFieldImportKey, Problem: ProblemDuplicateInFile}
	for _, index := range []int{0, 2} {
		if got := preview.Entries[index].Problems; !slices.Equal(got, []FieldError{duplicateKey}) {
			t.Errorf("problems of entry %d = %v, want %v", index+1, got, duplicateKey)
		}
	}
	if got := preview.Entries[3].Problems; !slices.Equal(got, []FieldError{{Field: EventFieldType, Problem: ProblemUnknownCode}, duplicateKey}) {
		t.Errorf("problems of entry 4 = %v", got)
	}
}

func TestPreviewImportGivesAnErrorPrecedenceOverAStoredKey(t *testing.T) {
	entry := keyedMarketEntry()
	entry["startDate"] = "16.10.2026"

	preview := previewWithEvents(t, marshalImport(t, importFile(entry)), newFakeEventRepo(storedMarket(t, marketID, marketKey)), hall())

	assertClasses(t, preview, ImportClassError)
	if got := preview.Entries[0]; got.TargetID != "" || got.Changes != nil {
		t.Errorf("entry = %+v, want an error without target", got)
	}
}

func TestPreviewImportSuspectsEqualEntriesOfTheFile(t *testing.T) {
	second := marketEntry()
	fourth := marketEntry()
	fourth["title"] = "kirchweihmarkt"
	fourth["location"] = importObject{"name": "PAUL-METZ-HALLE"}
	broken := marketEntry()
	broken["type"] = "concert"

	preview := previewOf(t, marshalImport(t, importFile(concertEntry(), second, broken, fourth)), hall())

	assertClasses(t, preview, ImportClassNew, ImportClassDuplicateSuspect, ImportClassError, ImportClassDuplicateSuspect)
	wantOf := map[int][]ImportCandidate{
		1: {{Position: 4, Title: "kirchweihmarkt", StartDate: kirchweihFriday}},
		3: {{Position: 2, Title: "Kirchweihmarkt", StartDate: kirchweihFriday}},
	}
	for index, want := range wantOf {
		if got := preview.Entries[index].Candidates; !slices.Equal(got, want) {
			t.Errorf("candidates of entry %d = %v, want %v", index+1, got, want)
		}
	}
}

func TestPreviewImportKeepsAStoredKeyBeforeAnEqualEntryOfTheFile(t *testing.T) {
	stored := storedMarket(t, marketID, marketKey)

	preview := previewWithEvents(t, marshalImport(t, importFile(keyedMarketEntry(), marketEntry())), newFakeEventRepo(stored), hall())

	assertClasses(t, preview, ImportClassUnchanged, ImportClassDuplicateSuspect)
	wantHints := []ImportHint{{Kind: ImportHintDuplicate, Candidate: ImportCandidate{Position: 2, Title: "Kirchweihmarkt", StartDate: kirchweihFriday}}}
	if got := preview.Entries[0].Hints; !slices.Equal(got, wantHints) {
		t.Errorf("hints of entry 1 = %v, want %v", got, wantHints)
	}
	wantCandidates := []ImportCandidate{
		{EventID: marketID, Title: "Kirchweihmarkt", StartDate: kirchweihFriday},
		{Position: 1, Title: "Kirchweihmarkt", StartDate: kirchweihFriday},
	}
	if got := preview.Entries[1].Candidates; !slices.Equal(got, wantCandidates) {
		t.Errorf("candidates of entry 2 = %v, want %v", got, wantCandidates)
	}
}

func TestPreviewImportListsANewLocationOnceWithEveryPositionBringingIt(t *testing.T) {
	second := concertEntry()
	second["title"] = "Lesung an der Veste"
	second["importKey"] = "lesung-2026"
	location := concertEntry()["location"].(importObject)
	location["name"] = "alte  veste"
	location["address"] = importObject{"street": "Burgweg 3", "postalCode": "90513", "city": "Zirndorf"}
	location["note"] = nil
	second["location"] = location
	brokenWithNewPlace := concertEntry()
	brokenWithNewPlace["type"] = "concert"
	brokenWithNewPlace["location"] = importObject{"name": "Neuer Platz"}
	brokenWithNewPlace["importKey"] = nil

	preview := previewOf(t, marshalImport(t, importFile(concertEntry(), second, marketEntry(), brokenWithNewPlace)), hall())

	assertClasses(t, preview, ImportClassNew, ImportClassNew, ImportClassNew, ImportClassError)
	wantLocations := []ImportNewLocation{{Name: "Alte Veste", Positions: []int{1, 2}}}
	if !reflect.DeepEqual(preview.NewLocations, wantLocations) {
		t.Errorf("new locations = %+v, want %+v", preview.NewLocations, wantLocations)
	}
	wantHints := []ImportHint{
		{Kind: ImportHintNewLocationDiffers, Field: "location.name"},
		{Kind: ImportHintNewLocationDiffers, Field: "location.address.street"},
		{Kind: ImportHintNewLocationDiffers, Field: "location.note"},
	}
	if got := preview.Entries[1].Hints; !slices.Equal(got, wantHints) {
		t.Errorf("hints of entry 2 = %v, want %v", got, wantHints)
	}
	if preview.Entries[0].Hints != nil {
		t.Errorf("hints of entry 1 = %v, want none", preview.Entries[0].Hints)
	}
}

func TestPreviewImportHintsAtGivenLocationDetailsThatDifferFromTheStoredOnes(t *testing.T) {
	entry := marketEntry()
	entry["location"] = importObject{
		"name":      "Paul-Metz-Halle",
		"address":   importObject{"street": "Volkhardtstraße 2", "postalCode": "90522", "city": " Zirndorf"},
		"latitude":  49.4424,
		"longitude": 10.95,
		"precision": "area",
		"note":      "anders",
	}
	outOfRange := marketEntry()
	outOfRange["location"] = importObject{"name": "Paul-Metz-Halle", "address": importObject{"street": "Andere Straße"}, "latitude": 91}
	outOfRange["startDate"] = "2026-10-23"

	preview := previewOf(t, marshalImport(t, importFile(entry, outOfRange)), hall())

	assertClasses(t, preview, ImportClassNew, ImportClassNew)
	wantOf := [][]ImportHint{
		{
			{Kind: ImportHintLocationDiffers, Field: "location.address.postalCode"},
			{Kind: ImportHintLocationDiffers, Field: "location.longitude"},
		},
		{
			{Kind: ImportHintLocationDiffers, Field: "location.address.street"},
			{Kind: ImportHintLocationDiffers, Field: "location.latitude"},
		},
	}
	for index, want := range wantOf {
		if hints := preview.Entries[index].Hints; !slices.Equal(hints, want) {
			t.Errorf("hints of entry %d = %v, want %v", index+1, hints, want)
		}
	}
}

func TestPreviewImportCountsEntriesPerClass(t *testing.T) {
	stored := storedMarket(t, marketID, marketKey)
	changed := keyedMarketEntry()
	changed["note"] = "neu"
	suspect := marketEntry()
	broken := marketEntry()
	broken["type"] = "concert"

	preview := previewWithEvents(t, marshalImport(t, importFile(changed, suspect, concertEntry(), broken)), newFakeEventRepo(stored), hall())

	want := map[ImportClass]int{ImportClassNew: 1, ImportClassUpdate: 1, ImportClassUnchanged: 0, ImportClassDuplicateSuspect: 1, ImportClassError: 1}
	for _, class := range ImportClasses() {
		if got := preview.CountOf(class); got != want[class] {
			t.Errorf("CountOf(%s) = %d, want %d", class, got, want[class])
		}
	}
	if len(ImportClasses()) != len(want) {
		t.Errorf("ImportClasses() = %v", ImportClasses())
	}
}

func TestPreviewImportPassesEventRepositoryFailuresOn(t *testing.T) {
	tests := map[string]func(*fakeEventRepo){
		"list": func(r *fakeEventRepo) { r.listErr = errDatabaseDown },
		"find": func(r *fakeEventRepo) { r.findErr = errDatabaseDown },
	}
	for name, fail := range tests {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo()
			fail(events)

			_, err := NewImportService(newFakeLocationRepo(hall()), events).PreviewImport(context.Background(), marshalImport(t, importFile(marketEntry())))

			if !errors.Is(err, errDatabaseDown) {
				t.Errorf("err = %v, want %v", err, errDatabaseDown)
			}
		})
	}
}
