package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// ImportUseCases are the core use cases behind the import page. Checking a
// file writes nothing.
type ImportUseCases interface {
	PreviewImport(ctx context.Context, data []byte) (core.ImportPreview, error)
}

// Route and form of the import page.
const (
	importPath = adminPathPrefix + "/import"
	// importFileField is the multipart field that carries the file.
	importFileField = "file"
	// importMultipartOverhead is room for the multipart framing around the
	// file, so a file of exactly the maximum size still reaches the core.
	importMultipartOverhead = 64 << 10
	// maxImportRequestBytes bounds the upload. The whole request fits in
	// memory, so parsing it writes no temporary files.
	maxImportRequestBytes = core.MaxImportFileBytes + importMultipartOverhead
)

// German texts of the import page.
const (
	msgImportPersonalData = "Bitte vor dem Import prüfen, dass Titel, Notizen und Quellen keine Privatpersonen, Kontaktpersonen oder Telefonnummern enthalten."
	msgImportNothingSaved = "Die Datei wird nur geprüft; es wird nichts gespeichert."
	msgImportNoFile       = "Bitte eine JSON-Datei auswählen."
	msgImportTooLarge     = "Die Datei ist größer als 2 MiB."
	// msgImportUnreadable covers a rejection reason without a specific
	// message.
	msgImportUnreadable = "Die Datei kann nicht gelesen werden."
	// msgImportWrongType is the message for a value of the wrong JSON type.
	msgImportWrongType = "Dieser Wert hat den falschen Typ (zum Beispiel Zahl statt Text) oder ist kein Objekt."
	// msgImportDuplicateHint introduces the link to what an entry may
	// duplicate.
	msgImportDuplicateHint = "Mögliches Duplikat:"
	// msgImportEmptyValue shows an empty old or new value of a change.
	msgImportEmptyValue = "(leer)"
	msgImportYes        = "ja"
	msgImportNo         = "nein"
	// importCandidateFormat names a stored event by title and start date,
	// importEntryCandidateFormat an entry of the file by its position too.
	importCandidateFormat      = "%s (%s)"
	importEntryCandidateFormat = "Position %d: " + importCandidateFormat
	// importEntryAnchorFormat links the row of an entry on the page.
	importEntryAnchorFormat = "#import-entry-%d"
	// importPositionSeparator joins the positions of a new location.
	importPositionSeparator = ", "
	// importReasonFormat joins the German field name and the message.
	importReasonFormat = "%s: %s"
	// importEntryLabelFormat names a timetable entry by its position.
	importEntryLabelFormat      = "Programmpunkt %d"
	importEntryFieldLabelFormat = importEntryLabelFormat + ", %s"
)

// logMsgImportFailed is logged when checking a file fails for a reason the
// page cannot show.
const logMsgImportFailed = "admin import check failed"

// importFileMessages are the German messages for a file rejected as a
// whole.
var importFileMessages = map[core.ImportFileProblem]string{
	core.ImportProblemInvalidJSON:          "Die Datei ist kein gültiges JSON-Objekt.",
	core.ImportProblemMissingFormatVersion: "Die Datei nennt keine Formatversion (formatVersion).",
	core.ImportProblemUnknownFormatVersion: "Unbekannte Formatversion. Unterstützt wird nur formatVersion 1.",
	core.ImportProblemMissingEvents:        "Die Datei enthält keine Event-Liste (events).",
	core.ImportProblemNoEntries:            "Die Event-Liste der Datei ist leer.",
	core.ImportProblemTooLarge:             msgImportTooLarge,
}

// importLocationPrefix starts the import path of every field of the
// location an entry brings along.
const importLocationPrefix = core.EventFieldLocation + "."

// importFieldLabels are the German names of the fields of an import entry
// by their path in the import schema.
var importFieldLabels = map[string]string{
	core.ImportFieldEntry:                            "Eintrag",
	core.EventFieldTitle:                             "Titel",
	core.EventFieldType:                              "Event-Typ",
	core.EventFieldLocation:                          "Ort",
	core.EventFieldStartDate:                         "Beginn-Datum",
	core.EventFieldStartTime:                         "Beginn-Uhrzeit",
	core.EventFieldEndDate:                           "End-Datum",
	core.EventFieldEndTime:                           "End-Uhrzeit",
	core.EventFieldAllDay:                            "Ganztägig",
	core.EventFieldSource:                            "Quelle",
	core.EventFieldSourceDescription:                 "Quelle",
	core.EventFieldSourceURL:                         "Link zur Quelle",
	core.EventFieldNote:                              "Notiz",
	core.EventFieldTimetable:                         "Ablaufplan",
	core.EventFieldImportKey:                         "Import-Schlüssel",
	importLocationPrefix + core.LocationFieldName:    "Ortsname",
	importLocationPrefix + core.LocationFieldAddress: "Adresse",
	importLocationPrefix + core.LocationFieldAddress + "." + core.LocationFieldStreet:     "Straße und Hausnummer",
	importLocationPrefix + core.LocationFieldAddress + "." + core.LocationFieldPostalCode: "PLZ",
	importLocationPrefix + core.LocationFieldAddress + "." + core.LocationFieldCity:       "Stadt",
	importLocationPrefix + core.LocationFieldLatitude:                                     "Breite",
	importLocationPrefix + core.LocationFieldLongitude:                                    "Länge",
	importLocationPrefix + core.LocationFieldPrecision:                                    "Ortsgenauigkeit",
	importLocationPrefix + core.LocationFieldNote:                                         "Ortsnotiz",
}

// timetableFieldLabels are the German names of the fields of a timetable
// entry.
var timetableFieldLabels = map[string]string{
	core.TimetableFieldDescription: "Beschreibung",
	core.TimetableFieldDate:        "Datum",
	core.TimetableFieldStartTime:   "Beginn-Uhrzeit",
	core.TimetableFieldEndTime:     "End-Uhrzeit",
}

// importOnlyMessages are the German messages for problems only the import
// reports.
var importOnlyMessages = map[core.FieldError]string{
	{Field: core.EventFieldLocation, Problem: core.ProblemMissing}:          "Bitte zu jedem Event einen Ort mit Namen angeben.",
	{Field: core.EventFieldImportKey, Problem: core.ProblemDuplicateInFile}: "Diesen Import-Schlüssel tragen mehrere Einträge der Datei.",
}

// importClassLabels are the German names of the import classes.
var importClassLabels = map[core.ImportClass]string{
	core.ImportClassNew:              "neu",
	core.ImportClassUpdate:           "Aktualisierung",
	core.ImportClassUnchanged:        "unverändert",
	core.ImportClassDuplicateSuspect: "Duplikatverdacht",
	core.ImportClassError:            "fehlerhaft",
}

// importLocationHintFormats are the German texts of the location hints,
// which quote the German name of the differing field.
var importLocationHintFormats = map[core.ImportHintKind]string{
	core.ImportHintLocationDiffers:    "Angabe „%s“ weicht vom vorhandenen Ort ab; der Ort bleibt unverändert.",
	core.ImportHintNewLocationDiffers: "Angabe „%s“ weicht von der ersten Angabe dieses neuen Orts ab; es gilt die erste.",
}

// importPage is the data of the import page: the upload form, and after an
// upload either the messages of a rejected file or the result.
type importPage struct {
	Action       string
	FileField    string
	PrivacyHint  string
	NothingSaved string
	FileErrors   []string
	Result       *importResult
	BackURL      string
	SchemaURL    string
}

// importResult is the checked file: the count per class, one row per
// entry and the locations the import would create.
type importResult struct {
	Counts       []importCount
	Rows         []importRow
	NewLocations []importNewLocationRow
}

// importCount is the number of entries of one class.
type importCount struct {
	Label string
	Count int
}

type importRow struct {
	Position int
	Title    string
	// Status is the German name of the class.
	Status string
	// Reasons name each rejected field as "<German field name>: <message>".
	Reasons []string
	// TargetURL is the edit page of the event an update or unchanged entry
	// refers to.
	TargetURL  string
	Changes    []importChangeRow
	Candidates []importLink
	Hints      []importHintRow
}

// importLink is a link with its text.
type importLink struct {
	Text string
	URL  string
}

// importChangeRow is a changed field with its old and new value in German.
type importChangeRow struct {
	Label string
	Old   string
	New   string
}

// importHintRow is the German text of a hint and the link it names, if
// any.
type importHintRow struct {
	Text string
	Link *importLink
}

// importNewLocationRow is a location the import would create and the
// positions of the entries bringing it.
type importNewLocationRow struct {
	Name      string
	Positions string
}

// importSchemaURL is where the public API serves the import format.
const importSchemaURL = "/v1/import-v1.schema.json"

func (h *handler) showImport(w http.ResponseWriter, _ *http.Request) {
	h.render(w, importTemplate, http.StatusOK, newImportPage())
}

// checkImport reads the uploaded file, has the core check it and shows the
// result. A file rejected as a whole shows its message (422, 413 when far
// too large); nothing is ever stored.
func (h *handler) checkImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportRequestBytes)
	data, rejection := readImportUpload(r)
	if rejection != nil {
		h.renderUploadRejection(w, rejection)
		return
	}
	preview, err := h.imports.PreviewImport(r.Context(), data)
	var fileErr *core.ImportFileError
	switch {
	case err == nil:
		page := newImportPage()
		page.Result = importResultOf(preview)
		h.render(w, importTemplate, http.StatusOK, page)
	case errors.As(err, &fileErr):
		h.renderImportRejected(w, http.StatusUnprocessableEntity, importFileMessage(fileErr.Problem))
	default:
		h.logger.Error(logMsgImportFailed, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// uploadRejection says why an upload did not reach the core: the status
// and the German message for the page, or no message when the request was
// no upload form at all.
type uploadRejection struct {
	status  int
	message string
}

var (
	rejectNoForm   = &uploadRejection{status: http.StatusBadRequest}
	rejectTooLarge = &uploadRejection{status: http.StatusRequestEntityTooLarge, message: msgImportTooLarge}
	rejectNoFile   = &uploadRejection{status: http.StatusUnprocessableEntity, message: msgImportNoFile}
)

// readImportUpload returns the content of the uploaded file.
func readImportUpload(r *http.Request) ([]byte, *uploadRejection) {
	if err := r.ParseMultipartForm(maxImportRequestBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, rejectTooLarge
		}
		return nil, rejectNoForm
	}
	file, _, err := r.FormFile(importFileField)
	if err != nil {
		return nil, rejectNoFile
	}
	return readUploadedFile(file)
}

// readUploadedFile reads and closes file.
func readUploadedFile(file io.ReadCloser) ([]byte, *uploadRejection) {
	data, err := io.ReadAll(file)
	if err = errors.Join(err, file.Close()); err != nil {
		return nil, rejectNoForm
	}
	return data, nil
}

// renderUploadRejection shows the message of rejection on the page, or
// only its status when it has none.
func (h *handler) renderUploadRejection(w http.ResponseWriter, rejection *uploadRejection) {
	if rejection.message == "" {
		http.Error(w, http.StatusText(rejection.status), rejection.status)
		return
	}
	h.renderImportRejected(w, rejection.status, rejection.message)
}

func (h *handler) renderImportRejected(w http.ResponseWriter, status int, message string) {
	page := newImportPage()
	page.FileErrors = []string{message}
	h.render(w, importTemplate, status, page)
}

func newImportPage() importPage {
	return importPage{
		Action:       importPath,
		FileField:    importFileField,
		PrivacyHint:  msgImportPersonalData,
		NothingSaved: msgImportNothingSaved,
		BackURL:      homePath,
		SchemaURL:    importSchemaURL,
	}
}

func importFileMessage(problem core.ImportFileProblem) string {
	if message, ok := importFileMessages[problem]; ok {
		return message
	}
	return msgImportUnreadable
}

func importResultOf(preview core.ImportPreview) *importResult {
	result := &importResult{}
	for _, class := range core.ImportClasses() {
		result.Counts = append(result.Counts, importCount{Label: importClassLabels[class], Count: preview.CountOf(class)})
	}
	for _, entry := range preview.Entries {
		result.Rows = append(result.Rows, importRowOf(entry))
	}
	for _, location := range preview.NewLocations {
		positions := make([]string, 0, len(location.Positions))
		for _, position := range location.Positions {
			positions = append(positions, strconv.Itoa(position))
		}
		result.NewLocations = append(result.NewLocations, importNewLocationRow{
			Name: location.Name, Positions: strings.Join(positions, importPositionSeparator),
		})
	}
	return result
}

// importRowOf shows an entry with its class and what explains it.
func importRowOf(entry core.ImportEntry) importRow {
	row := importRow{Position: entry.Position, Title: entry.Title, Status: importClassLabels[entry.Class]}
	for _, problem := range entry.Problems {
		row.Reasons = append(row.Reasons, fmt.Sprintf(importReasonFormat, importFieldLabel(problem.Field), importProblemMessage(problem)))
	}
	if entry.TargetID != "" {
		row.TargetURL = eventURL(entry.TargetID)
	}
	for _, change := range entry.Changes {
		row.Changes = append(row.Changes, importChangeRow{
			Label: importFieldLabel(change.Field),
			Old:   importChangeValue(change.Field, change.Old),
			New:   importChangeValue(change.Field, change.New),
		})
	}
	for _, candidate := range entry.Candidates {
		row.Candidates = append(row.Candidates, importCandidateLink(candidate))
	}
	for _, hint := range entry.Hints {
		row.Hints = append(row.Hints, importHintRowOf(hint))
	}
	return row
}

// importChangeValue shows a value of a changed field in German: an event
// type by its label, all-day as ja or nein, an empty value as (leer).
func importChangeValue(field, value string) string {
	switch {
	case value == "":
		return msgImportEmptyValue
	case field == core.EventFieldAllDay && value == strconv.FormatBool(true):
		return msgImportYes
	case field == core.EventFieldAllDay:
		return msgImportNo
	}
	if label, ok := eventTypeLabels[core.EventType(value)]; ok && field == core.EventFieldType {
		return label
	}
	return value
}

// importCandidateLink links a stored event to its edit page and an entry
// of the file to its row.
func importCandidateLink(candidate core.ImportCandidate) importLink {
	date := formatMoment(candidate.StartDate, nil, false)
	if candidate.EventID != "" {
		return importLink{Text: fmt.Sprintf(importCandidateFormat, candidate.Title, date), URL: eventURL(candidate.EventID)}
	}
	return importLink{
		Text: fmt.Sprintf(importEntryCandidateFormat, candidate.Position, candidate.Title, date),
		URL:  fmt.Sprintf(importEntryAnchorFormat, candidate.Position),
	}
}

// importHintRowOf shows a hint in German: a duplicate with its link, a
// location hint with the German name of its field.
func importHintRowOf(hint core.ImportHint) importHintRow {
	if hint.Kind == core.ImportHintDuplicate {
		link := importCandidateLink(hint.Candidate)
		return importHintRow{Text: msgImportDuplicateHint, Link: &link}
	}
	return importHintRow{Text: fmt.Sprintf(importLocationHintFormats[hint.Kind], importFieldLabel(hint.Field))}
}

// importFieldLabel returns the German name of a field path; a timetable
// field names its entry from 1. An unknown path is shown as it is.
func importFieldLabel(field string) string {
	if label, ok := importFieldLabels[field]; ok {
		return label
	}
	if index, name, ok := core.SplitTimetableField(field); ok {
		return fmt.Sprintf(importEntryFieldLabelFormat, index+1, timetableFieldLabels[name])
	}
	if index, ok := timetableEntryIndex(field); ok {
		return fmt.Sprintf(importEntryLabelFormat, index+1)
	}
	return field
}

// timetableEntryIndex returns the index of a whole timetable entry path
// such as timetable[2].
func timetableEntryIndex(field string) (int, bool) {
	rest, found := strings.CutPrefix(field, core.EventFieldTimetable+"[")
	if !found {
		return 0, false
	}
	indexText, found := strings.CutSuffix(rest, "]")
	if !found {
		return 0, false
	}
	index, err := strconv.Atoi(indexText)
	return index, err == nil
}

// importProblemMessage returns the German message for a problem of an
// import entry, the same as the admin forms show where they have one. A
// value of the wrong JSON type has its own message.
func importProblemMessage(problem core.FieldError) string {
	if message, ok := importOnlyMessages[core.FieldError{Field: problem.Field, Problem: problem.Problem}]; ok {
		return message
	}
	if rest, found := strings.CutPrefix(problem.Field, importLocationPrefix); found {
		problem.Field = rest[strings.LastIndex(rest, ".")+1:]
		return importFieldMessage(problem, locationFieldMessages)
	}
	if _, name, ok := core.SplitTimetableField(problem.Field); ok {
		problem.Field = name
		return importFieldMessage(problem, timetableFieldMessages)
	}
	return importFieldMessage(problem, eventFieldMessages)
}

// importFieldMessage is fieldErrorMessage with the wrong-type message for
// an invalid format the form has no message for.
func importFieldMessage(problem core.FieldError, messages map[core.FieldError]string) string {
	message := fieldErrorMessage(problem, messages)
	if message == msgFieldInvalid && problem.Problem == core.ProblemInvalidFormat {
		return msgImportWrongType
	}
	return message
}
