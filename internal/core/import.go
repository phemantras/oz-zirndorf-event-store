package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The import file format v1 (api/v1/import-v1.schema.json): an object with
// formatVersion 1 and a list of at least one and at most MaxImportEntries
// events in the write form EventInput of the OpenAPI spec.
const (
	// MaxImportFileBytes is the largest import file accepted, 2 MiB.
	MaxImportFileBytes = 2 << 20
	// MaxImportEntries is the most events an import file may hold, so the
	// decision form of the admin stays within the parts a form may have.
	MaxImportEntries = 150
	// ImportFormatVersion is the only format version this core reads.
	ImportFormatVersion = 1
)

// Keys of the top level of an import file.
const (
	importKeyFormatVersion = "formatVersion"
	importKeyEvents        = "events"
)

// ImportFieldEntry is the field name of a whole import entry, used when
// the entry itself is not a JSON object.
const ImportFieldEntry = ""

// Field paths of an import entry join the names of nested objects with a
// dot, such as location.address.street.
const (
	fieldPathSeparator = "."
	// listIndexOpen and listIndexClose enclose the index of a list entry,
	// such as timetable[2].
	listIndexOpen  = "["
	listIndexClose = "]"
	// firstImportPosition is the position of the first entry; positions
	// count from 1 as a person reads the file.
	firstImportPosition = 1
)

// jsonNull is the JSON literal that the import treats like a missing
// field.
var jsonNull = []byte("null")

// utf8ByteOrderMark is the UTF-8 encoding of U+FEFF, which some editors
// put at the start of a file.
var utf8ByteOrderMark = []byte{0xEF, 0xBB, 0xBF}

// Coordinates arrive as JSON numbers and are handed to newLocation as
// text, in the shortest form that reads back exactly.
const (
	coordinateTextFormat    = 'f'
	coordinateTextPrecision = -1
)

// ImportFileProblem is the machine-readable reason a whole import file was
// rejected.
type ImportFileProblem string

// Reasons an import file is rejected as a whole, before any entry is read.
const (
	ImportProblemInvalidJSON          ImportFileProblem = "invalidJson"
	ImportProblemMissingFormatVersion ImportFileProblem = "missingFormatVersion"
	ImportProblemUnknownFormatVersion ImportFileProblem = "unknownFormatVersion"
	// ImportProblemMissingEvents means events is missing, null or not a
	// list.
	ImportProblemMissingEvents ImportFileProblem = "missingEvents"
	ImportProblemNoEntries     ImportFileProblem = "noEntries"
	ImportProblemTooLarge      ImportFileProblem = "tooLarge"
	// ImportProblemTooManyEntries means events holds more than
	// MaxImportEntries entries.
	ImportProblemTooManyEntries ImportFileProblem = "tooManyEntries"
	// ImportProblemInvalidEncoding means the file is not valid UTF-8, such
	// as a file saved in Windows-1252.
	ImportProblemInvalidEncoding ImportFileProblem = "invalidEncoding"
)

// ImportFileError reports that an import file was rejected as a whole. It
// matches ErrValidation with errors.Is.
type ImportFileError struct {
	Problem ImportFileProblem
}

func (e *ImportFileError) Error() string {
	return "import file rejected: " + string(e.Problem)
}

func (e *ImportFileError) Unwrap() error { return ErrValidation }

// ImportEntry is one event of an import file as read, checked and
// classified.
type ImportEntry struct {
	// Position counts the entries of the file from 1.
	Position int
	// Title is the normalized title, empty when it could not be read.
	Title string
	// Input is the event as read; Location is nil when the entry brings
	// none.
	Input EventInput
	// Problems lists every rejected field by its path in the import
	// schema, such as location.address.street or timetable[2].date; none
	// means the entry is valid.
	Problems []FieldError
	// Class is what importing the entry would do.
	Class ImportClass
	// TargetID is the stored event with the import key of the entry, set
	// for ImportClassUpdate and ImportClassUnchanged.
	TargetID string
	// TargetFingerprint is the EventFingerprint of the stored target, set
	// with TargetID.
	TargetFingerprint string
	// Changes lists the fields an update would change, in the order of
	// EventInput.
	Changes []ImportChange
	// Candidates are what an ImportClassDuplicateSuspect entry may
	// duplicate: stored events first, then other entries of the file.
	Candidates []ImportCandidate
	// Hints point out what does not change the class.
	Hints []ImportHint
	// NewLocation reports whether a valid entry brings a location that is
	// not stored yet.
	NewLocation bool
}

// IsValid reports whether the entry has no problem.
func (e ImportEntry) IsValid() bool {
	return len(e.Problems) == 0
}

// StoredCandidateIDs returns the IDs of the stored events among the
// candidates, in their order.
func (e ImportEntry) StoredCandidateIDs() []string {
	var ids []string
	for _, candidate := range e.Candidates {
		if candidate.EventID != "" {
			ids = append(ids, candidate.EventID)
		}
	}
	return ids
}

// StoredFingerprints returns the fingerprints of the stored events the
// entry could write, by event ID: its target and its stored candidates.
func (e ImportEntry) StoredFingerprints() map[string]string {
	fingerprints := make(map[string]string)
	if e.TargetID != "" {
		fingerprints[e.TargetID] = e.TargetFingerprint
	}
	for _, candidate := range e.Candidates {
		if candidate.EventID != "" {
			fingerprints[candidate.EventID] = candidate.Fingerprint
		}
	}
	return fingerprints
}

// ImportPreview is the result of checking an import file, one entry per
// event in file order, and the locations the import would create.
type ImportPreview struct {
	Entries []ImportEntry
	// NewLocations are the locations of valid entries that no stored
	// location has by name, once per NormalizeKey of the name, in the
	// order of their first entry.
	NewLocations []ImportNewLocation
}

// CountOf returns how many entries have class.
func (p ImportPreview) CountOf(class ImportClass) int {
	count := 0
	for _, entry := range p.Entries {
		if entry.Class == class {
			count++
		}
	}
	return count
}

// ImportService holds the import use cases.
type ImportService struct {
	events *EventService
}

// NewImportService returns the import use cases. They read through the
// repositories of events, write in its transactions and clear its review
// marks.
func NewImportService(events *EventService) *ImportService {
	return &ImportService{events: events}
}

// PreviewImport reads an import file, checks every entry with the rules
// of SaveEvent and SaveLocation and classifies it against all stored
// events, archived ones included. It writes nothing. A file that cannot be
// read as format v1 yields *ImportFileError; otherwise every entry is
// returned with its problems, which never affect the other entries. A
// location whose name matches exactly one stored one by NormalizeKey refers
// to it, and its other fields are only compared; a name matching several is
// ambiguous; a new location must be complete.
func (s *ImportService) PreviewImport(ctx context.Context, data []byte) (ImportPreview, error) {
	rawEntries, err := readImportFile(data)
	if err != nil {
		return ImportPreview{}, err
	}
	classified, err := classifyImport(ctx, Repos{Events: s.events.events, Locations: s.events.locations}, rawEntries)
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{Entries: classified.entries, NewLocations: classified.newLocations}, nil
}

// classifiedImport is an import file read and classified against the
// stored events and locations.
type classifiedImport struct {
	entries []ImportEntry
	// imported holds the parsed event of each entry at the same index.
	imported     []importedEvent
	newLocations []ImportNewLocation
	// storedEvents holds every stored event by ID.
	storedEvents map[string]Event
}

// classifyImport reads every entry of rawEntries and classifies it against
// what repos store.
func classifyImport(ctx context.Context, repos Repos, rawEntries []json.RawMessage) (classifiedImport, error) {
	classifier, err := newImportClassifier(ctx, repos)
	if err != nil {
		return classifiedImport{}, err
	}
	classified := classifiedImport{
		entries:      make([]ImportEntry, 0, len(rawEntries)),
		imported:     make([]importedEvent, 0, len(rawEntries)),
		storedEvents: classifier.eventsByID,
	}
	for index, raw := range rawEntries {
		entry, event := readImportEntry(raw, classifier.locationsByKey)
		entry.Position = index + firstImportPosition
		classified.entries = append(classified.entries, entry)
		classified.imported = append(classified.imported, event)
	}
	classified.newLocations, err = classifier.classify(ctx, classified.entries, classified.imported)
	if err != nil {
		return classifiedImport{}, err
	}
	return classified, nil
}

// readImportFile checks the top level of an import file and returns its
// entries unread.
func readImportFile(data []byte) ([]json.RawMessage, error) {
	if len(data) > MaxImportFileBytes {
		return nil, &ImportFileError{Problem: ImportProblemTooLarge}
	}
	// Windows tools often write a UTF-8 byte order mark, which JSON does
	// not allow.
	data = bytes.TrimPrefix(data, utf8ByteOrderMark)
	// encoding/json would silently replace invalid bytes with U+FFFD.
	if !utf8.Valid(data) {
		return nil, &ImportFileError{Problem: ImportProblemInvalidEncoding}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil || top == nil {
		return nil, &ImportFileError{Problem: ImportProblemInvalidJSON}
	}
	version, given := top[importKeyFormatVersion]
	if !given || bytes.Equal(version, jsonNull) {
		return nil, &ImportFileError{Problem: ImportProblemMissingFormatVersion}
	}
	// JSON Schema compares numbers by value, so 1.0 is the integer 1.
	var number float64
	if err := json.Unmarshal(version, &number); err != nil || number != ImportFormatVersion {
		return nil, &ImportFileError{Problem: ImportProblemUnknownFormatVersion}
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(top[importKeyEvents], &entries); err != nil || entries == nil {
		return nil, &ImportFileError{Problem: ImportProblemMissingEvents}
	}
	if len(entries) == 0 {
		return nil, &ImportFileError{Problem: ImportProblemNoEntries}
	}
	if len(entries) > MaxImportEntries {
		return nil, &ImportFileError{Problem: ImportProblemTooManyEntries}
	}
	return entries, nil
}

// importedEvent is an entry as the core would store it: the parsed event
// and the location it refers to. Only valid entries use it.
type importedEvent struct {
	// event has no location ID; location says where it takes place.
	event Event
	// location is the stored location the entry names, or the new one it
	// brings along, without ID.
	location Location
}

// isNewLocation reports whether the entry brings a location that is not
// stored yet.
func (i importedEvent) isNewLocation() bool {
	return i.location.ID == ""
}

// readImportEntry reads one entry and checks it, its class ImportClassError
// when it has problems and ImportClassNew otherwise. JSON type errors are
// reported at their field as invalidFormat, texts with a control character
// as controlCharacter; the rules then report nothing more for that field or
// the fields within it. storedLocations holds the stored locations by
// NormalizeKey of their name.
func readImportEntry(raw json.RawMessage, storedLocations map[string][]Location) (ImportEntry, importedEvent) {
	reader := &jsonReader{}
	fields, isObject := reader.object(raw, ImportFieldEntry)
	if !isObject {
		return ImportEntry{Problems: reader.problems, Class: ImportClassError}, importedEvent{}
	}
	in := readEventInput(reader, fields)
	// The import names its location only by the object location, so the
	// admin's location ID must not be asked for.
	checked := in
	if checked.Location == nil {
		checked.Location = &LocationInput{}
	}
	event, eventProblems := newEvent(checked)
	location, locationProblems := resolveImportLocation(in.Location, storedLocations)
	problems := slices.Concat(eventProblems, locationProblems)
	entry := ImportEntry{
		Title:    normalizeText(in.Title),
		Input:    in,
		Problems: slices.Concat(reader.problems, withoutFieldsWithin(problems, reader.problems)),
		Class:    ImportClassNew,
	}
	if !entry.IsValid() {
		entry.Class = ImportClassError
	}
	return entry, importedEvent{event: event, location: location}
}

// readEventInput reads the fields of an entry by the names of EventInput
// in the OpenAPI spec. Unknown fields are ignored.
func readEventInput(reader *jsonReader, fields map[string]json.RawMessage) EventInput {
	in := EventInput{
		Title:     readText(reader, fields, EventFieldTitle, EventFieldTitle),
		Type:      readText(reader, fields, EventFieldType, EventFieldType),
		StartDate: readText(reader, fields, EventFieldStartDate, EventFieldStartDate),
		StartTime: readText(reader, fields, EventFieldStartTime, EventFieldStartTime),
		EndDate:   readText(reader, fields, EventFieldEndDate, EventFieldEndDate),
		EndTime:   readText(reader, fields, EventFieldEndTime, EventFieldEndTime),
		AllDay:    readJSON[bool](reader, fields, EventFieldAllDay, EventFieldAllDay),
		Note:      readText(reader, fields, EventFieldNote, EventFieldNote),
		ImportKey: readText(reader, fields, EventFieldImportKey, EventFieldImportKey),
	}
	if source, ok := reader.nestedObject(fields, EventFieldSource, EventFieldSource); ok {
		in.Source = EventSource{
			Description: readText(reader, source, sourceFieldDescription, EventFieldSourceDescription),
			URL:         readText(reader, source, sourceFieldURL, EventFieldSourceURL),
		}
	}
	if location, ok := reader.nestedObject(fields, EventFieldLocation, EventFieldLocation); ok {
		in.Location = readLocationInput(reader, location)
	}
	in.Timetable = readTimetable(reader, fields)
	return in
}

// readLocationInput reads the location an entry brings along.
func readLocationInput(reader *jsonReader, fields map[string]json.RawMessage) *LocationInput {
	in := &LocationInput{
		Name:      readText(reader, fields, LocationFieldName, importLocationField(LocationFieldName)),
		Latitude:  readCoordinate(reader, fields, LocationFieldLatitude),
		Longitude: readCoordinate(reader, fields, LocationFieldLongitude),
		Precision: readText(reader, fields, LocationFieldPrecision, importLocationField(LocationFieldPrecision)),
		Note:      readText(reader, fields, LocationFieldNote, importLocationField(LocationFieldNote)),
	}
	addressPath := EventFieldLocation + fieldPathSeparator + LocationFieldAddress
	if address, ok := reader.nestedObject(fields, LocationFieldAddress, addressPath); ok {
		in.Street = readText(reader, address, LocationFieldStreet, importLocationField(LocationFieldStreet))
		in.PostalCode = readText(reader, address, LocationFieldPostalCode, importLocationField(LocationFieldPostalCode))
		in.City = readText(reader, address, LocationFieldCity, importLocationField(LocationFieldCity))
	}
	return in
}

// readCoordinate reads a JSON number as text for newLocation; a missing
// number is empty text.
func readCoordinate(reader *jsonReader, fields map[string]json.RawMessage, name string) string {
	value, ok := readOptionalJSON[float64](reader, fields, name, importLocationField(name))
	if !ok {
		return ""
	}
	return strconv.FormatFloat(value, coordinateTextFormat, coordinateTextPrecision, coordinateBitSize)
}

// readTimetable reads the timetable entries in file order. An entry that
// is not an object stays in the list as an empty one, so the indices of
// the problems match the file.
func readTimetable(reader *jsonReader, fields map[string]json.RawMessage) []TimetableEntryInput {
	rawEntries := readJSON[[]json.RawMessage](reader, fields, EventFieldTimetable, EventFieldTimetable)
	var timetable []TimetableEntryInput
	for index, raw := range rawEntries {
		entryPath := EventFieldTimetable + listIndexOpen + strconv.Itoa(index) + listIndexClose
		entryFields, _ := reader.object(raw, entryPath)
		timetable = append(timetable, TimetableEntryInput{
			Description: readText(reader, entryFields, TimetableFieldDescription, TimetableField(index, TimetableFieldDescription)),
			Date:        readText(reader, entryFields, TimetableFieldDate, TimetableField(index, TimetableFieldDate)),
			StartTime:   readText(reader, entryFields, TimetableFieldStartTime, TimetableField(index, TimetableFieldStartTime)),
			EndTime:     readText(reader, entryFields, TimetableFieldEndTime, TimetableField(index, TimetableFieldEndTime)),
		})
	}
	return timetable
}

// resolveImportLocation checks the location an entry brings along: it is
// required, its name too. A name of exactly one stored location needs
// nothing else and yields it; a name of several is ambiguous, as no stored
// location is preferred. A new location is checked like SaveLocation, its
// fields named by their import paths, and yielded without ID.
func resolveImportLocation(location *LocationInput, storedLocations map[string][]Location) (Location, []FieldError) {
	if location == nil {
		return Location{}, []FieldError{{Field: EventFieldLocation, Problem: ProblemMissing}}
	}
	if normalizeText(location.Name) == "" {
		return Location{}, []FieldError{{Field: importLocationField(LocationFieldName), Problem: ProblemMissing}}
	}
	stored := storedLocations[NormalizeKey(location.Name)]
	if len(stored) == 1 {
		return stored[0], nil
	}
	if len(stored) > 1 {
		return Location{}, []FieldError{{Field: importLocationField(LocationFieldName), Problem: ProblemAmbiguous}}
	}
	created, err := newLocation(*location)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		return created, nil
	}
	problems := make([]FieldError, 0, len(validation.Fields))
	for _, problem := range validation.Fields {
		problem.Field = importLocationField(problem.Field)
		problems = append(problems, problem)
	}
	return Location{}, problems
}

// importLocationField returns the import path of a location field:
// address parts within location.address, the others within location.
func importLocationField(field string) string {
	switch field {
	case LocationFieldStreet, LocationFieldPostalCode, LocationFieldCity:
		return EventFieldLocation + fieldPathSeparator + LocationFieldAddress + fieldPathSeparator + field
	default:
		return EventFieldLocation + fieldPathSeparator + field
	}
}

// withoutFieldsWithin drops the problems of every field that is, or lies
// within, a field of reported, so a field with a JSON type error is not
// also reported as missing.
func withoutFieldsWithin(problems, reported []FieldError) []FieldError {
	var kept []FieldError
	for _, problem := range problems {
		if !slices.ContainsFunc(reported, func(r FieldError) bool { return isFieldWithin(problem.Field, r.Field) }) {
			kept = append(kept, problem)
		}
	}
	return kept
}

// isFieldWithin reports whether field is outer or a field nested in it,
// such as location.name within location or timetable[1].date within
// timetable[1].
func isFieldWithin(field, outer string) bool {
	return field == outer ||
		strings.HasPrefix(field, outer+fieldPathSeparator) ||
		strings.HasPrefix(field, outer+listIndexOpen)
}

// jsonReader decodes the fields of an import entry one by one and collects
// a problem for every field whose JSON type does not fit.
type jsonReader struct {
	problems []FieldError
}

// object decodes raw as a JSON object. Anything else, null included, is
// reported at path; the returned fields are then empty.
func (r *jsonReader) object(raw json.RawMessage, path string) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		r.report(path)
		return nil, false
	}
	return fields, true
}

// nestedObject decodes the field name as a JSON object. A missing or null
// field is not reported; any other value that is no object is reported at
// path.
func (r *jsonReader) nestedObject(fields map[string]json.RawMessage, name, path string) (map[string]json.RawMessage, bool) {
	raw, given := fields[name]
	if !given || bytes.Equal(raw, jsonNull) {
		return nil, false
	}
	return r.object(raw, path)
}

// report records a value of the wrong JSON type at path.
func (r *jsonReader) report(path string) {
	r.reportProblem(path, ProblemInvalidFormat)
}

// reportProblem records problem at path.
func (r *jsonReader) reportProblem(path string, problem FieldProblem) {
	r.problems = append(r.problems, FieldError{Field: path, Problem: problem})
}

// allowedControlCharacters are the control characters a text may hold:
// tab, line feed and carriage return.
const allowedControlCharacters = "\t\n\r"

// readText is readJSON for a text. A text with any other control character
// is reported at path as ProblemControlCharacter and yields empty text, as
// PostgreSQL rejects NUL and none of them belongs in an event.
func readText(reader *jsonReader, fields map[string]json.RawMessage, name, path string) string {
	text := readJSON[string](reader, fields, name, path)
	if strings.ContainsFunc(text, isForbiddenControlCharacter) {
		reader.reportProblem(path, ProblemControlCharacter)
		return ""
	}
	return text
}

// isForbiddenControlCharacter reports whether r is a control character
// other than the allowedControlCharacters.
func isForbiddenControlCharacter(r rune) bool {
	return unicode.IsControl(r) && !strings.ContainsRune(allowedControlCharacters, r)
}

// readJSON decodes the field name into T. Missing and null yield the zero
// value; a value of another JSON type is reported at path.
func readJSON[T any](reader *jsonReader, fields map[string]json.RawMessage, name, path string) T {
	value, _ := readOptionalJSON[T](reader, fields, name, path)
	return value
}

// readOptionalJSON is readJSON that also says whether a value was read.
func readOptionalJSON[T any](reader *jsonReader, fields map[string]json.RawMessage, name, path string) (T, bool) {
	var value T
	raw, given := fields[name]
	if !given || bytes.Equal(raw, jsonNull) {
		return value, false
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		reader.report(path)
		return value, false
	}
	return value, true
}
