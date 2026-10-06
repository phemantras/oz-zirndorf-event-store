package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// The import file format v1 (api/v1/import-v1.schema.json): an object with
// formatVersion 1 and a list of at least one event in the write form
// EventInput of the OpenAPI spec.
const (
	// MaxImportFileBytes is the largest import file accepted, 2 MiB.
	MaxImportFileBytes = 2 << 20
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
	// Changes lists the fields an update would change, in the order of
	// EventInput.
	Changes []ImportChange
	// Candidates are what an ImportClassDuplicateSuspect entry may
	// duplicate: stored events first, then other entries of the file.
	Candidates []ImportCandidate
	// Hints point out what does not change the class.
	Hints []ImportHint
}

// IsValid reports whether the entry has no problem.
func (e ImportEntry) IsValid() bool {
	return len(e.Problems) == 0
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
	locations LocationRepo
	events    EventRepo
}

// NewImportService returns the import use cases, which read the stored
// locations from locations and the stored events from events.
func NewImportService(locations LocationRepo, events EventRepo) *ImportService {
	return &ImportService{locations: locations, events: events}
}

// PreviewImport reads an import file, checks every entry with the rules
// of SaveEvent and SaveLocation and classifies it against all stored
// events, archived ones included. It writes nothing. A file that cannot be
// read as format v1 yields *ImportFileError; otherwise every entry is
// returned with its problems, which never affect the other entries. A
// location whose name matches a stored one by NormalizeKey refers to it,
// and its other fields are only compared; a new location must be complete.
func (s *ImportService) PreviewImport(ctx context.Context, data []byte) (ImportPreview, error) {
	rawEntries, err := readImportFile(data)
	if err != nil {
		return ImportPreview{}, err
	}
	classifier, err := s.newImportClassifier(ctx)
	if err != nil {
		return ImportPreview{}, err
	}
	entries := make([]ImportEntry, 0, len(rawEntries))
	imported := make([]importedEvent, 0, len(rawEntries))
	for index, raw := range rawEntries {
		entry, event := readImportEntry(raw, classifier.locationsByKey)
		entry.Position = index + firstImportPosition
		entries = append(entries, entry)
		imported = append(imported, event)
	}
	newLocations, err := classifier.classify(ctx, entries, imported)
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{Entries: entries, NewLocations: newLocations}, nil
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
// reported at their field as invalidFormat; the rules then report nothing
// more for that field or the fields within it.
func readImportEntry(raw json.RawMessage, storedLocations map[string]Location) (ImportEntry, importedEvent) {
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
		Title:     readJSON[string](reader, fields, EventFieldTitle, EventFieldTitle),
		Type:      readJSON[string](reader, fields, EventFieldType, EventFieldType),
		StartDate: readJSON[string](reader, fields, EventFieldStartDate, EventFieldStartDate),
		StartTime: readJSON[string](reader, fields, EventFieldStartTime, EventFieldStartTime),
		EndDate:   readJSON[string](reader, fields, EventFieldEndDate, EventFieldEndDate),
		EndTime:   readJSON[string](reader, fields, EventFieldEndTime, EventFieldEndTime),
		AllDay:    readJSON[bool](reader, fields, EventFieldAllDay, EventFieldAllDay),
		Note:      readJSON[string](reader, fields, EventFieldNote, EventFieldNote),
		ImportKey: readJSON[string](reader, fields, EventFieldImportKey, EventFieldImportKey),
	}
	if source, ok := reader.nestedObject(fields, EventFieldSource, EventFieldSource); ok {
		in.Source = EventSource{
			Description: readJSON[string](reader, source, sourceFieldDescription, EventFieldSourceDescription),
			URL:         readJSON[string](reader, source, sourceFieldURL, EventFieldSourceURL),
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
		Name:      readJSON[string](reader, fields, LocationFieldName, importLocationField(LocationFieldName)),
		Latitude:  readCoordinate(reader, fields, LocationFieldLatitude),
		Longitude: readCoordinate(reader, fields, LocationFieldLongitude),
		Precision: readJSON[string](reader, fields, LocationFieldPrecision, importLocationField(LocationFieldPrecision)),
		Note:      readJSON[string](reader, fields, LocationFieldNote, importLocationField(LocationFieldNote)),
	}
	addressPath := EventFieldLocation + fieldPathSeparator + LocationFieldAddress
	if address, ok := reader.nestedObject(fields, LocationFieldAddress, addressPath); ok {
		in.Street = readJSON[string](reader, address, LocationFieldStreet, importLocationField(LocationFieldStreet))
		in.PostalCode = readJSON[string](reader, address, LocationFieldPostalCode, importLocationField(LocationFieldPostalCode))
		in.City = readJSON[string](reader, address, LocationFieldCity, importLocationField(LocationFieldCity))
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
			Description: readJSON[string](reader, entryFields, TimetableFieldDescription, TimetableField(index, TimetableFieldDescription)),
			Date:        readJSON[string](reader, entryFields, TimetableFieldDate, TimetableField(index, TimetableFieldDate)),
			StartTime:   readJSON[string](reader, entryFields, TimetableFieldStartTime, TimetableField(index, TimetableFieldStartTime)),
			EndTime:     readJSON[string](reader, entryFields, TimetableFieldEndTime, TimetableField(index, TimetableFieldEndTime)),
		})
	}
	return timetable
}

// resolveImportLocation checks the location an entry brings along: it is
// required, its name too. A stored name needs nothing else and yields the
// stored location; a new location is checked like SaveLocation, its fields
// named by their import paths, and yielded without ID.
func resolveImportLocation(location *LocationInput, storedLocations map[string]Location) (Location, []FieldError) {
	if location == nil {
		return Location{}, []FieldError{{Field: EventFieldLocation, Problem: ProblemMissing}}
	}
	if normalizeText(location.Name) == "" {
		return Location{}, []FieldError{{Field: importLocationField(LocationFieldName), Problem: ProblemMissing}}
	}
	if stored, ok := storedLocations[NormalizeKey(location.Name)]; ok {
		return stored, nil
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

func (r *jsonReader) report(path string) {
	r.problems = append(r.problems, FieldError{Field: path, Problem: ProblemInvalidFormat})
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
