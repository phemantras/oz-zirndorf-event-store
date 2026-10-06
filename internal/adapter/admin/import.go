package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// ImportUseCases are the core use cases behind the import page. Checking a
// file writes nothing; committing it writes in one transaction.
type ImportUseCases interface {
	PreviewImport(ctx context.Context, data []byte) (core.ImportPreview, error)
	CommitImport(ctx context.Context, data []byte, decisions []core.ImportDecision) (core.ImportSummary, error)
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

// Route and fields of the commit form, which sends the file content back
// with what the preview showed and decided for each entry; the server
// keeps nothing in between.
const (
	importCommitPath = importPath + "/commit"
	// importContentField carries the content of the checked file.
	importContentField = "content"
	// importPositionField is sent once per entry with its position.
	importPositionField = "position"
	// The fields of an entry are named by a prefix and its position, such
	// as class-3.
	importDecisionFieldFormat    = "%s-%d"
	importClassFieldPrefix       = "class"
	importTargetFieldPrefix      = "target"
	importNewLocationFieldPrefix = "newLocation"
	importCandidatesFieldPrefix  = "candidates"
	importChoiceFieldPrefix      = "choice"
	// importCandidateSeparator separates the IDs of the stored candidates
	// of a duplicate suspect.
	importCandidateSeparator = " "
	// importNewLocationTrue marks an entry that brought a new location.
	importNewLocationTrue = "true"
	// importChoiceValueSeparator joins the overwrite choice and the ID of
	// the event to overwrite, such as overwrite:<id>.
	importChoiceValueSeparator = ":"
	// maxImportCommitBytes bounds the commit form: the file content and
	// as much again for the decisions on its entries.
	maxImportCommitBytes = 2*core.MaxImportFileBytes + importMultipartOverhead
)

// German texts of the import page.
const (
	msgImportPersonalData = "Bitte vor dem Import prüfen, dass Titel, Notizen und Quellen keine Privatpersonen, Kontaktpersonen oder Telefonnummern enthalten."
	msgImportNothingSaved = "Beim Prüfen wird nichts gespeichert."
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

// German texts of committing an import.
const (
	msgImportCommitHint   = "Gespeichert wird erst mit „Import übernehmen“. Ein Duplikatverdacht ohne Entscheidung wird nicht übernommen."
	msgImportCommitButton = "Import übernehmen"
	msgImportChoiceLegend = "Entscheidung"
	msgImportChoiceSkip   = "Überspringen"
	msgImportChoiceCreate = "Als neues Event anlegen"
	// msgImportChoiceOverwrite precedes the stored event to overwrite.
	msgImportChoiceOverwrite = "Überschreiben:"
	msgImportSummaryHeading  = "Ergebnis des Imports"
	// msgImportNotTakenHint explains the list of entries a commit did not
	// take.
	msgImportNotTakenHint     = "Diese Einträge wurden nicht übernommen. Die Datei erneut prüfen, um sie zu übernehmen."
	msgImportCommitFailed     = "Der Import ist fehlgeschlagen; es wurde nichts übernommen."
	msgImportCreatedLocations = "neu angelegte Orte"
	// importChoiceOverwriteFormat names the stored event to overwrite.
	importChoiceOverwriteFormat = msgImportChoiceOverwrite + " %s"
)

// logMsgImportFailed is logged when checking a file fails for a reason the
// page cannot show.
const logMsgImportFailed = "admin import check failed"

// logMsgImportCommitFailed is logged when committing a file fails; nothing
// was stored.
const logMsgImportCommitFailed = "admin import commit failed"

// importOutcomeLabels are the German names of the outcomes of a commit.
var importOutcomeLabels = map[core.ImportOutcome]string{
	core.ImportOutcomeCreated:   "neu angelegt",
	core.ImportOutcomeUpdated:   "aktualisiert",
	core.ImportOutcomeUnchanged: "unverändert",
	core.ImportOutcomeSkipped:   "übersprungen",
	core.ImportOutcomeUndecided: "ohne Entscheidung",
	core.ImportOutcomeError:     "fehlerhaft",
	core.ImportOutcomeStale:     "veraltet",
}

// importFileMessages are the German messages for a file rejected as a
// whole.
var importFileMessages = map[core.ImportFileProblem]string{
	core.ImportProblemInvalidJSON:          "Die Datei ist kein gültiges JSON-Objekt.",
	core.ImportProblemMissingFormatVersion: "Die Datei nennt keine Formatversion (formatVersion).",
	core.ImportProblemUnknownFormatVersion: "Unbekannte Formatversion. Unterstützt wird nur formatVersion 1.",
	core.ImportProblemMissingEvents:        "Die Datei enthält keine Event-Liste (events).",
	core.ImportProblemNoEntries:            "Die Event-Liste der Datei ist leer.",
	core.ImportProblemTooLarge:             msgImportTooLarge,
	core.ImportProblemTooManyEntries:       fmt.Sprintf(msgImportTooManyEntriesFormat, core.MaxImportEntries),
}

// msgImportTooManyEntriesFormat names the most events a file may hold.
const msgImportTooManyEntriesFormat = "Die Datei enthält mehr als %d Events."

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
// upload either the messages of a rejected file or the result with the
// commit form, after a commit its summary.
type importPage struct {
	Action       string
	FileField    string
	PrivacyHint  string
	NothingSaved string
	FileErrors   []string
	Result       *importResult
	Summary      *importSummaryView
	BackURL      string
	SchemaURL    string
	CommitAction string
	CommitHint   string
	CommitButton string
	ContentField string
	// Content is the checked file, sent back with the commit form.
	Content string
}

// importSummaryView is what a commit did: the count per outcome, how
// many locations it created and the entries it did not take for a reason
// a person should look at.
type importSummaryView struct {
	Heading          string
	Counts           []importCount
	LocationsLabel   string
	CreatedLocations int
	EventsURL        string
	// NotTakenHint explains Rows, the entries the commit did not take.
	NotTakenHint string
	Rows         []importOutcomeRow
}

// importOutcomeRow is an entry with the German name of its outcome.
type importOutcomeRow struct {
	Position int
	Title    string
	Outcome  string
}

// importOutcomesToList are the outcomes the summary lists entry by entry.
var importOutcomesToList = []core.ImportOutcome{core.ImportOutcomeStale, core.ImportOutcomeUndecided, core.ImportOutcomeError}

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
	// Hidden are the fields that send back what the preview showed.
	Hidden []importFormField
	// ChoiceLegend introduces Choices, the decisions a duplicate suspect
	// offers; none is preselected.
	ChoiceLegend string
	Choices      []importChoice
}

// importFormField is a field of the commit form with its value.
type importFormField struct {
	Name  string
	Value string
}

// importChoice is one decision on a duplicate suspect.
type importChoice struct {
	Name  string
	Value string
	Label string
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
	h.render(w, importTemplate, http.StatusOK, newImportCheckPage())
}

// checkImport reads the uploaded file, has the core check it and shows the
// result. A file rejected as a whole shows its message (422, 413 when far
// too large); nothing is ever stored.
func (h *handler) checkImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportRequestBytes)
	page := newImportCheckPage()
	data, rejection := readImportUpload(r)
	if rejection != nil {
		h.renderUploadRejection(w, page, rejection)
		return
	}
	preview, err := h.imports.PreviewImport(r.Context(), data)
	var fileErr *core.ImportFileError
	switch {
	case err == nil:
		page.Result = importResultOf(preview)
		page.Content = string(data)
		h.render(w, importTemplate, http.StatusOK, page)
	case errors.As(err, &fileErr):
		h.renderImportRejected(w, page, http.StatusUnprocessableEntity, importFileMessage(fileErr.Problem))
	default:
		h.logger.Error(logMsgImportFailed, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// commitImport has the core commit the file sent back with the decisions
// on its entries and shows what it did. A file rejected as a whole shows
// its message (422, 413 when far too large); a failed commit stored
// nothing and says so (500).
func (h *handler) commitImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportCommitBytes)
	page := newImportPage()
	if rejection := parseImportForm(r, maxImportCommitBytes); rejection != nil {
		h.renderUploadRejection(w, page, rejection)
		return
	}
	form := url.Values(r.MultipartForm.Value)
	summary, err := h.imports.CommitImport(r.Context(), []byte(form.Get(importContentField)), importDecisionsOf(form))
	var fileErr *core.ImportFileError
	switch {
	case err == nil:
		page.Summary = importSummaryViewOf(summary)
		h.render(w, importTemplate, http.StatusOK, page)
	case errors.As(err, &fileErr):
		h.renderImportRejected(w, page, http.StatusUnprocessableEntity, importFileMessage(fileErr.Problem))
	default:
		h.logger.Error(logMsgImportCommitFailed, "error", err)
		h.renderImportRejected(w, page, http.StatusInternalServerError, msgImportCommitFailed)
	}
}

// importDecisionsOf reads the decision on every entry the form names by
// its position; a position that is no number is left out, so its entry
// counts as stale.
func importDecisionsOf(form url.Values) []core.ImportDecision {
	var decisions []core.ImportDecision
	for _, text := range form[importPositionField] {
		position, err := strconv.Atoi(text)
		if err != nil {
			continue
		}
		choice, overwriteID, _ := strings.Cut(form.Get(importDecisionField(importChoiceFieldPrefix, position)), importChoiceValueSeparator)
		decisions = append(decisions, core.ImportDecision{
			Position:    position,
			Class:       core.ImportClass(form.Get(importDecisionField(importClassFieldPrefix, position))),
			TargetID:    form.Get(importDecisionField(importTargetFieldPrefix, position)),
			NewLocation: form.Get(importDecisionField(importNewLocationFieldPrefix, position)) == importNewLocationTrue,
			Choice:      core.ImportChoice(choice),
			OverwriteID: overwriteID,
			// strings.Fields splits at the importCandidateSeparator.
			CandidateIDs: strings.Fields(form.Get(importDecisionField(importCandidatesFieldPrefix, position))),
		})
	}
	return decisions
}

// importDecisionField returns the name of the field prefix of the entry at
// position.
func importDecisionField(prefix string, position int) string {
	return fmt.Sprintf(importDecisionFieldFormat, prefix, position)
}

// importSummaryViewOf shows the count of every outcome in display order
// and lists the entries that are stale, undecided or erroneous.
func importSummaryViewOf(summary core.ImportSummary) *importSummaryView {
	view := &importSummaryView{
		Heading:          msgImportSummaryHeading,
		NotTakenHint:     msgImportNotTakenHint,
		LocationsLabel:   msgImportCreatedLocations,
		CreatedLocations: summary.CreatedLocations,
		EventsURL:        eventsPath,
	}
	for _, outcome := range core.ImportOutcomes() {
		view.Counts = append(view.Counts, importCount{Label: importOutcomeLabels[outcome], Count: summary.CountOf(outcome)})
	}
	for _, result := range summary.Results {
		if slices.Contains(importOutcomesToList, result.Outcome) {
			view.Rows = append(view.Rows, importOutcomeRow{Position: result.Position, Title: result.Title, Outcome: importOutcomeLabels[result.Outcome]})
		}
	}
	return view
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
	if rejection := parseImportForm(r, maxImportRequestBytes); rejection != nil {
		return nil, rejection
	}
	file, _, err := r.FormFile(importFileField)
	if err != nil {
		return nil, rejectNoFile
	}
	return readUploadedFile(file)
}

// parseImportForm reads a multipart form of at most maxBytes into memory.
func parseImportForm(r *http.Request, maxBytes int64) *uploadRejection {
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return rejectTooLarge
		}
		return rejectNoForm
	}
	return nil
}

// readUploadedFile reads and closes file.
func readUploadedFile(file io.ReadCloser) ([]byte, *uploadRejection) {
	data, err := io.ReadAll(file)
	if err = errors.Join(err, file.Close()); err != nil {
		return nil, rejectNoForm
	}
	return data, nil
}

// renderUploadRejection shows the message of rejection on page, or only
// its status when it has none.
func (h *handler) renderUploadRejection(w http.ResponseWriter, page importPage, rejection *uploadRejection) {
	if rejection.message == "" {
		http.Error(w, http.StatusText(rejection.status), rejection.status)
		return
	}
	h.renderImportRejected(w, page, rejection.status, rejection.message)
}

func (h *handler) renderImportRejected(w http.ResponseWriter, page importPage, status int, message string) {
	page.FileErrors = []string{message}
	h.render(w, importTemplate, status, page)
}

// newImportCheckPage returns the import page for checking a file, which
// says that checking stores nothing.
func newImportCheckPage() importPage {
	page := newImportPage()
	page.NothingSaved = msgImportNothingSaved
	return page
}

// newImportPage returns the import page without the hint that nothing is
// stored, as a commit shows it.
func newImportPage() importPage {
	return importPage{
		Action:       importPath,
		FileField:    importFileField,
		PrivacyHint:  msgImportPersonalData,
		BackURL:      homePath,
		SchemaURL:    importSchemaURL,
		CommitAction: importCommitPath,
		CommitHint:   msgImportCommitHint,
		CommitButton: msgImportCommitButton,
		ContentField: importContentField,
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
	row.Hidden = importHiddenFieldsOf(entry)
	if entry.Class == core.ImportClassDuplicateSuspect {
		row.ChoiceLegend, row.Choices = msgImportChoiceLegend, importChoicesOf(entry)
	}
	return row
}

// importHiddenFieldsOf returns the fields that send back what the preview
// showed of an entry: its position, class, target, if so that it brings a
// new location, and for a duplicate suspect its stored candidates.
func importHiddenFieldsOf(entry core.ImportEntry) []importFormField {
	fields := []importFormField{
		{Name: importPositionField, Value: strconv.Itoa(entry.Position)},
		{Name: importDecisionField(importClassFieldPrefix, entry.Position), Value: string(entry.Class)},
		{Name: importDecisionField(importTargetFieldPrefix, entry.Position), Value: entry.TargetID},
	}
	if entry.NewLocation {
		fields = append(fields, importFormField{Name: importDecisionField(importNewLocationFieldPrefix, entry.Position), Value: importNewLocationTrue})
	}
	if entry.Class == core.ImportClassDuplicateSuspect {
		fields = append(fields, importFormField{
			Name:  importDecisionField(importCandidatesFieldPrefix, entry.Position),
			Value: strings.Join(entry.StoredCandidateIDs(), importCandidateSeparator),
		})
	}
	return fields
}

// importChoicesOf returns the decisions on a duplicate suspect: skip,
// create, and overwrite for every stored event it may duplicate.
func importChoicesOf(entry core.ImportEntry) []importChoice {
	name := importDecisionField(importChoiceFieldPrefix, entry.Position)
	choices := []importChoice{
		{Name: name, Value: string(core.ImportChoiceSkip), Label: msgImportChoiceSkip},
		{Name: name, Value: string(core.ImportChoiceCreate), Label: msgImportChoiceCreate},
	}
	for _, candidate := range entry.Candidates {
		if candidate.EventID == "" {
			continue
		}
		choices = append(choices, importChoice{
			Name:  name,
			Value: string(core.ImportChoiceOverwrite) + importChoiceValueSeparator + candidate.EventID,
			Label: fmt.Sprintf(importChoiceOverwriteFormat, importCandidateLink(candidate).Text),
		})
	}
	return choices
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
