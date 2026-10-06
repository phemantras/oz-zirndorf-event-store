package core

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ImportClass says what importing an entry would do.
type ImportClass string

// Classes of an import entry. An entry has exactly one; error goes before
// update and unchanged, these before duplicateSuspect, and that before new.
const (
	// ImportClassNew creates a new event.
	ImportClassNew ImportClass = "new"
	// ImportClassUpdate changes the stored event with the import key of
	// the entry.
	ImportClassUpdate ImportClass = "update"
	// ImportClassUnchanged matches the stored event with the import key of
	// the entry in every field.
	ImportClassUnchanged ImportClass = "unchanged"
	// ImportClassDuplicateSuspect may duplicate a stored event or another
	// entry of the file; a person decides.
	ImportClassDuplicateSuspect ImportClass = "duplicateSuspect"
	// ImportClassError has problems and is never imported.
	ImportClassError ImportClass = "error"
)

// ImportClasses returns every import class in display order.
func ImportClasses() []ImportClass {
	return []ImportClass{
		ImportClassNew, ImportClassUpdate, ImportClassUnchanged, ImportClassDuplicateSuspect, ImportClassError,
	}
}

// ImportChange is one field an update would change, both values as text:
// the location by its name, the timetable as one field with one line per
// entry.
type ImportChange struct {
	Field string
	Old   string
	New   string
}

// ImportCandidate is what an entry may duplicate: a stored event or
// another entry of the file.
type ImportCandidate struct {
	// EventID is the stored event, empty for an entry of the file.
	EventID string
	// Position is the entry of the file, 0 for a stored event.
	Position  int
	Title     string
	StartDate LocalDate
	// Fingerprint is the EventFingerprint of the stored event, empty for an
	// entry of the file.
	Fingerprint string
}

// ImportHintKind says what an import hint points out.
type ImportHintKind string

// Kinds of import hints.
const (
	// ImportHintDuplicate names a stored event or entry the entry may
	// duplicate, where its import key already decides its target.
	ImportHintDuplicate ImportHintKind = "duplicate"
	// ImportHintLocationDiffers names a given field of a stored location
	// that differs from the stored value; the stored location stays as it
	// is.
	ImportHintLocationDiffers ImportHintKind = "locationDiffers"
	// ImportHintNewLocationDiffers names a field of a new location that
	// differs from the first entry bringing it, whose details count.
	ImportHintNewLocationDiffers ImportHintKind = "newLocationDiffers"
)

// ImportHint points out something about an entry that does not change its
// class.
type ImportHint struct {
	Kind ImportHintKind
	// Field is the import path of the location field of a location hint.
	Field string
	// Candidate is what a duplicate hint names.
	Candidate ImportCandidate
}

// ImportNewLocation is a location the import would create, with the
// positions of the entries that bring it along.
type ImportNewLocation struct {
	Name      string
	Positions []int
}

// The text form of a timetable in an ImportChange: one line per entry,
// date, times and description separated by a space, the end time after a
// dash.
const (
	timetableLineSeparator  = "\n"
	timetablePartSeparator  = " "
	timetableRangeSeparator = "–"
)

// importClassifier classifies the entries of an import file against the
// stored events and locations, read once before.
type importClassifier struct {
	events EventRepo
	// locationsByKey holds the stored locations by NormalizeKey of their
	// name.
	locationsByKey map[string]Location
	// locationNames holds the names of the stored locations by ID.
	locationNames map[string]string
	// eventsByImportKey holds the stored events that have an import key.
	eventsByImportKey map[string]Event
	// eventsByID holds every stored event by ID.
	eventsByID map[string]Event
}

// newImportClassifier reads all locations and events that repos store,
// archived ones included.
func newImportClassifier(ctx context.Context, repos Repos) (*importClassifier, error) {
	locations, err := repos.Locations.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations for import: %w", err)
	}
	events, err := repos.Events.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events for import: %w", err)
	}
	classifier := &importClassifier{
		events:            repos.Events,
		locationsByKey:    make(map[string]Location, len(locations)),
		locationNames:     make(map[string]string, len(locations)),
		eventsByImportKey: make(map[string]Event),
		eventsByID:        make(map[string]Event, len(events)),
	}
	for _, location := range locations {
		classifier.locationsByKey[NormalizeKey(location.Name)] = location
		classifier.locationNames[location.ID] = location.Name
	}
	for _, event := range events {
		classifier.eventsByID[event.ID] = event
		if event.ImportKey != "" {
			classifier.eventsByImportKey[event.ImportKey] = event
		}
	}
	return classifier, nil
}

// classify sets the class of every entry read: import keys given twice,
// then the stored events, then the other entries of the file. It returns
// the new locations of the valid entries. imported holds the parsed event
// of each entry at the same index.
func (c *importClassifier) classify(ctx context.Context, entries []ImportEntry, imported []importedEvent) ([]ImportNewLocation, error) {
	markImportKeysGivenTwice(entries)
	for index := range entries {
		if entries[index].Class == ImportClassError {
			continue
		}
		if err := c.classifyAgainstStore(ctx, &entries[index], imported[index]); err != nil {
			return nil, err
		}
	}
	markDuplicatesInFile(entries, imported)
	return collectNewLocations(entries, imported), nil
}

// markImportKeysGivenTwice makes every entry an error whose normalized
// import key another entry of the file gives too.
func markImportKeysGivenTwice(entries []ImportEntry) {
	counts := make(map[string]int)
	for _, entry := range entries {
		if key := normalizeText(entry.Input.ImportKey); key != "" {
			counts[key]++
		}
	}
	for index := range entries {
		entry := &entries[index]
		if counts[normalizeText(entry.Input.ImportKey)] > 1 {
			entry.Problems = append(entry.Problems, FieldError{Field: EventFieldImportKey, Problem: ProblemDuplicateInFile})
			entry.Class = ImportClassError
		}
	}
}

// classifyAgainstStore compares a valid entry with the stored event of its
// import key, if there is one, and looks for stored duplicates. A new
// location has no stored events, so it has no stored duplicates either.
func (c *importClassifier) classifyAgainstStore(ctx context.Context, entry *ImportEntry, imported importedEvent) error {
	imported.event.LocationID = imported.location.ID
	if !imported.isNewLocation() {
		entry.Hints = append(entry.Hints, storedLocationHints(*entry.Input.Location, imported.location)...)
	}
	if target, found := c.eventsByImportKey[normalizeText(entry.Input.ImportKey)]; found {
		imported.event.ID = target.ID
		entry.TargetID = target.ID
		entry.TargetFingerprint = EventFingerprint(target)
		entry.Changes = changesOf(target, c.locationNames[target.LocationID], imported)
		entry.Class = ImportClassUnchanged
		if entry.Changes != nil {
			entry.Class = ImportClassUpdate
		}
	}
	if imported.isNewLocation() {
		return nil
	}
	candidates, err := FindDuplicateCandidates(ctx, c.events, imported.event)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		// The candidates come without timetable; the stored event read
		// before has it.
		addDuplicate(entry, ImportCandidate{
			EventID: candidate.ID, Title: candidate.Title, StartDate: candidate.Times.StartDate,
			Fingerprint: EventFingerprint(c.eventsByID[candidate.ID]),
		})
	}
	return nil
}

// addDuplicate records that entry may duplicate candidate: as hint when
// its import key already gives it a target, otherwise as candidate of a
// duplicate suspect.
func addDuplicate(entry *ImportEntry, candidate ImportCandidate) {
	if entry.TargetID != "" {
		entry.Hints = append(entry.Hints, ImportHint{Kind: ImportHintDuplicate, Candidate: candidate})
		return
	}
	entry.Candidates = append(entry.Candidates, candidate)
	entry.Class = ImportClassDuplicateSuspect
}

// changesOf compares the canonical forms of the stored event and the
// imported one, which refers to its location by ID, empty for a new one.
// It returns every changed field in the order of EventInput, none when
// they are equal.
func changesOf(stored Event, storedLocationName string, imported importedEvent) []ImportChange {
	before := EventInputOf(stored).Canonicalize()
	after := EventInputOf(imported.event).Canonicalize()
	var changes []ImportChange
	add := func(field, old, updated string) {
		if old != updated {
			changes = append(changes, ImportChange{Field: field, Old: old, New: updated})
		}
	}
	add(EventFieldTitle, before.Title, after.Title)
	add(EventFieldType, before.Type, after.Type)
	if before.LocationID != after.LocationID {
		changes = append(changes, ImportChange{Field: EventFieldLocation, Old: storedLocationName, New: imported.location.Name})
	}
	add(EventFieldStartDate, before.StartDate, after.StartDate)
	add(EventFieldStartTime, before.StartTime, after.StartTime)
	add(EventFieldEndDate, before.EndDate, after.EndDate)
	add(EventFieldEndTime, before.EndTime, after.EndTime)
	add(EventFieldAllDay, strconv.FormatBool(before.AllDay), strconv.FormatBool(after.AllDay))
	add(EventFieldSourceDescription, before.Source.Description, after.Source.Description)
	add(EventFieldSourceURL, before.Source.URL, after.Source.URL)
	add(EventFieldNote, before.Note, after.Note)
	if !slices.Equal(before.Timetable, after.Timetable) {
		changes = append(changes, ImportChange{Field: EventFieldTimetable, Old: timetableText(before.Timetable), New: timetableText(after.Timetable)})
	}
	return changes
}

// timetableText returns the text form of a timetable, one line per entry
// such as "2026-10-17 19:00–22:00 Vorband".
func timetableText(entries []TimetableEntryInput) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		times := entry.StartTime
		if entry.EndTime != "" {
			times += timetableRangeSeparator + entry.EndTime
		}
		parts := slices.DeleteFunc([]string{entry.Date, times, entry.Description}, func(part string) bool { return part == "" })
		lines = append(lines, strings.Join(parts, timetablePartSeparator))
	}
	return strings.Join(lines, timetableLineSeparator)
}

// storedLocationHints names every given street, postal code, city or
// coordinate that differs from the stored location.
func storedLocationHints(given LocationInput, stored Location) []ImportHint {
	var hints []ImportHint
	hints = appendLocationHint(hints, ImportHintLocationDiffers, LocationFieldStreet, givenTextDiffers(given.Street, stored.Street))
	hints = appendLocationHint(hints, ImportHintLocationDiffers, LocationFieldPostalCode, givenTextDiffers(given.PostalCode, stored.PostalCode))
	hints = appendLocationHint(hints, ImportHintLocationDiffers, LocationFieldCity, givenTextDiffers(given.City, stored.City))
	hints = appendLocationHint(hints, ImportHintLocationDiffers, LocationFieldLatitude, givenCoordinateDiffers(given.Latitude, stored.Latitude, maxLatitude))
	hints = appendLocationHint(hints, ImportHintLocationDiffers, LocationFieldLongitude, givenCoordinateDiffers(given.Longitude, stored.Longitude, maxLongitude))
	return hints
}

// givenTextDiffers reports whether text is given and differs from stored.
func givenTextDiffers(text, stored string) bool {
	text = normalizeText(text)
	return text != "" && text != stored
}

// givenCoordinateDiffers reports whether text is given and is no valid
// coordinate within limit or another one than stored.
func givenCoordinateDiffers(text string, stored, limit float64) bool {
	if normalizeText(text) == "" {
		return false
	}
	value, problem := parseCoordinate(text, limit)
	return problem != "" || value != stored
}

// appendLocationHint appends a hint of kind at the location field when
// differs.
func appendLocationHint(hints []ImportHint, kind ImportHintKind, field string, differs bool) []ImportHint {
	if !differs {
		return hints
	}
	return append(hints, ImportHint{Kind: kind, Field: importLocationField(field)})
}

// fileDuplicateKey is what two entries of a file share when one may
// duplicate the other: the title key, the start date and the location by
// NormalizeKey of its name.
type fileDuplicateKey struct {
	titleKey    string
	startDate   LocalDate
	locationKey string
}

func fileDuplicateKeyOf(imported importedEvent) fileDuplicateKey {
	return fileDuplicateKey{
		titleKey:    imported.event.TitleKey,
		startDate:   imported.event.Times.StartDate,
		locationKey: NormalizeKey(imported.location.Name),
	}
}

// markDuplicatesInFile records for every valid entry the other valid
// entries of the file with the same duplicate key, in file order.
func markDuplicatesInFile(entries []ImportEntry, imported []importedEvent) {
	positions := make(map[fileDuplicateKey][]int)
	for index, entry := range entries {
		if entry.Class != ImportClassError {
			key := fileDuplicateKeyOf(imported[index])
			positions[key] = append(positions[key], index)
		}
	}
	for index := range entries {
		if entries[index].Class == ImportClassError {
			continue
		}
		for _, other := range positions[fileDuplicateKeyOf(imported[index])] {
			if other != index {
				addDuplicate(&entries[index], ImportCandidate{
					Position: entries[other].Position, Title: entries[other].Title, StartDate: imported[other].event.Times.StartDate,
				})
			}
		}
	}
}

// collectNewLocations returns the new locations of the valid entries, once
// per NormalizeKey of the name with the positions bringing it. The first
// entry's details count; a later entry whose details differ gets a hint
// for each differing field.
func collectNewLocations(entries []ImportEntry, imported []importedEvent) []ImportNewLocation {
	var newLocations []ImportNewLocation
	var firsts []Location
	indexOf := make(map[string]int)
	for index := range entries {
		entry := &entries[index]
		if entry.Class == ImportClassError || !imported[index].isNewLocation() {
			continue
		}
		entry.NewLocation = true
		location := imported[index].location
		first, seen := indexOf[location.NameKey]
		if !seen {
			indexOf[location.NameKey] = len(newLocations)
			newLocations = append(newLocations, ImportNewLocation{Name: location.Name, Positions: []int{entry.Position}})
			firsts = append(firsts, location)
			continue
		}
		newLocations[first].Positions = append(newLocations[first].Positions, entry.Position)
		entry.Hints = append(entry.Hints, newLocationHints(firsts[first], location)...)
	}
	return newLocations
}

// newLocationHints names every field in which a later entry's new location
// differs from the first one.
func newLocationHints(first, later Location) []ImportHint {
	differences := []struct {
		field   string
		differs bool
	}{
		{LocationFieldName, first.Name != later.Name},
		{LocationFieldStreet, first.Street != later.Street},
		{LocationFieldPostalCode, first.PostalCode != later.PostalCode},
		{LocationFieldCity, first.City != later.City},
		{LocationFieldLatitude, first.Latitude != later.Latitude},
		{LocationFieldLongitude, first.Longitude != later.Longitude},
		{LocationFieldPrecision, first.Precision != later.Precision},
		{LocationFieldNote, first.Note != later.Note},
	}
	var hints []ImportHint
	for _, difference := range differences {
		hints = appendLocationHint(hints, ImportHintNewLocationDiffers, difference.field, difference.differs)
	}
	return hints
}
