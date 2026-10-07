package admin

import (
	"bytes"
	"context"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// failingImports fails every preview with err.
type failingImports struct{ err error }

func (f failingImports) PreviewImport(context.Context, []byte) (core.ImportPreview, error) {
	return core.ImportPreview{}, f.err
}

// validImportFile has an event at the stored hall and one that brings a new
// location along.
const validImportFile = `{
  "formatVersion": 1,
  "events": [
    {"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
     "startDate": "2026-10-16", "source": {"description": "Amtsblatt 41/2026"}},
    {"title": "Konzert an der Veste", "type": "culture", "startDate": "2026-10-17",
     "location": {"name": "Alte Veste", "address": {"street": "Burgweg 1", "postalCode": "90513", "city": "Zirndorf"},
                  "latitude": 49.4501, "longitude": 10.9376, "precision": "building"},
     "source": {"description": "Plakat"}}
  ]
}`

// importFileWithErrors has one valid entry and entries with every kind of
// problem the page names.
var importFileWithErrors = `{
  "formatVersion": 1,
  "events": [
    {"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
     "startDate": "2026-10-16", "source": {"description": "Amtsblatt 41/2026"}},
    {"title": "Flohmarkt", "type": "market", "startDate": "16.10.2026", "source": {"description": "Plakat"}},
    {"title": "` + strings.Repeat("T", core.MaxTitleLength+1) + `", "type": "market", "location": {"name": "Paul-Metz-Halle"},
     "startDate": "2026-10-16", "source": {"description": "Plakat"}},
    {"title": "Konzert", "type": "culture", "startDate": "2026-10-17",
     "location": {"name": "Alte Veste", "address": {"street": "Burgweg 1", "postalCode": "9051", "city": "Zirndorf"},
                  "latitude": 49.4501, "longitude": 10.9376, "precision": "building"},
     "source": {"description": "Plakat"},
     "timetable": [{"description": "Einlass", "date": "2026-10-17"}, "Vorband", {"description": "Abbau", "date": "2026-10-19", "startTime": "10:00"}]},
    "kein Objekt"
  ]
}`

func (ts *testServer) postImport(t *testing.T, field, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, "events.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, importPath, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return ts.do(withCookie(req, ts.validCookie()))
}

func TestImportPageOffersFileUploadWithHints(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(importPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`enctype="multipart/form-data"`, `action="`+importPath+`"`, `type="file"`, `name="`+importFileField+`"`,
		msgImportPersonalData, "Beim Prüfen wird nichts gespeichert.", `href="/v1/import-v1.schema.json"`, `href="`+homePath+`"`,
	)
	assertBodyLacks(t, rec, "<table")
}

func TestImportNamesEveryClassInGerman(t *testing.T) {
	for _, class := range core.ImportClasses() {
		if importClassLabels[class] == "" {
			t.Errorf("class %q has no German label", class)
		}
	}
}

func TestHomeLinksImport(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(homePath)

	assertBodyContains(t, rec, `href="`+importPath+`">Import</a>`)
}

func TestImportOfAValidFileShowsEveryEntryAsNewAndStoresNothing(t *testing.T) {
	ts := newTestServer(t)
	ts.seed(t, hallName)

	rec := ts.postImport(t, importFileField, validImportFile)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"<li>neu: 2</li>", "<li>Aktualisierung: 0</li>", "<li>unverändert: 0</li>", "<li>Duplikatverdacht: 0</li>", "<li>fehlerhaft: 0</li>",
		"Kirchweihmarkt", "Konzert an der Veste", msgImportPersonalData, msgImportNothingSaved,
		"<h2>Neue Orte</h2>", "<td>Alte Veste</td><td>2</td>",
	)
	if got := strings.Count(rec.Body.String(), "<td>"+importClassLabels[core.ImportClassNew]+"</td>"); got != 2 {
		t.Errorf("%d rows are new, want 2", got)
	}
	if len(ts.events.events) != 0 || len(ts.locations.locations) != 1 {
		t.Errorf("events = %d, locations = %d, want the store unchanged", len(ts.events.events), len(ts.locations.locations))
	}
}

func TestImportNamesEveryProblemByPositionTitleAndField(t *testing.T) {
	ts := newTestServer(t)
	ts.seed(t, hallName)

	rec := ts.postImport(t, importFileField, importFileWithErrors)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"<li>neu: 1</li>", "<li>fehlerhaft: 4</li>",
		"Ort: Bitte zu jedem Event einen Ort mit Namen angeben.",
		"Beginn-Datum: Das Beginn-Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
		"Titel: Höchstens 200 Zeichen.",
		"PLZ: Die PLZ muss aus genau fünf Ziffern bestehen.",
		"Programmpunkt 2: "+msgImportWrongType,
		"Programmpunkt 3, Beginn-Uhrzeit: "+msgOutsideEvent,
		"Eintrag: "+msgImportWrongType,
	)
	want := []string{"2", "Flohmarkt", importClassLabels[core.ImportClassError], "Beginn-Datum: Das Beginn-Datum ist kein gültiges Datum (Format JJJJ-MM-TT).", "Ort: Bitte zu jedem Event einen Ort mit Namen angeben."}
	if got := importRowTexts(rec, "2"); !slices.Equal(got, want) {
		t.Errorf("row 2 = %q, want %q", got, want)
	}
	if got := importRowTexts(rec, "1"); !slices.Equal(got, []string{"1", "Kirchweihmarkt", importClassLabels[core.ImportClassNew]}) {
		t.Errorf("row 1 = %q", got)
	}
	if len(ts.events.events) != 0 || len(ts.locations.locations) != 1 {
		t.Error("checking an import changed the store")
	}
}

func TestImportNamesAControlCharacterAndAnAmbiguousLocation(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	// A collision group RecomputeNameKeys left: the same NormalizeKey of
	// the name under another name key.
	twin := hall
	twin.ID, twin.Name, twin.NameKey = "0192f0b1-0000-7000-9000-999999999999", strings.ToUpper(hall.Name), hall.NameKey+"-2"
	ts.locations.locations[twin.ID] = twin
	file := `{"formatVersion": 1, "events": [
	  {"title": "Kirchweih\u0000markt", "type": "market", "startDate": "2026-10-16", "source": {"description": "Plakat"},
	   "location": {"name": "Alte Veste", "address": {"street": "Burgweg 1", "postalCode": "90513", "city": "Zirndorf"},
	                "latitude": 49.4501, "longitude": 10.9376, "precision": "building"}},
	  {"title": "Flohmarkt", "type": "market", "location": {"name": "paul-metz-halle"}, "startDate": "2026-10-16", "source": {"description": "Plakat"}}]}`

	rec := ts.postImport(t, importFileField, file)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<li>fehlerhaft: 2</li>")
	// A title with a control character is read as empty, so the row shows
	// none.
	wantRows := map[string][]string{
		"1": {"1", importClassLabels[core.ImportClassError], "Titel: " + msgImportControlCharacter},
		"2": {"2", "Flohmarkt", importClassLabels[core.ImportClassError], "Ortsname: " + msgImportAmbiguousLocation},
	}
	for position, want := range wantRows {
		if got := importRowTexts(rec, position); !slices.Equal(got, want) {
			t.Errorf("row %s = %q, want %q", position, got, want)
		}
	}
}

func TestImportLabelsEveryFieldOfTheFormatInGerman(t *testing.T) {
	fields := []string{
		core.EventFieldTitle, core.EventFieldType, core.EventFieldLocation, core.EventFieldStartDate,
		core.EventFieldStartTime, core.EventFieldEndDate, core.EventFieldEndTime, core.EventFieldAllDay,
		core.EventFieldSource, core.EventFieldSourceDescription, core.EventFieldSourceURL, core.EventFieldNote,
		core.EventFieldTimetable, core.EventFieldImportKey, core.ImportFieldEntry,
		"location.name", "location.address", "location.address.street", "location.address.postalCode",
		"location.address.city", "location.latitude", "location.longitude", "location.precision", "location.note",
	}
	for _, field := range fields {
		if importFieldLabels[field] == "" {
			t.Errorf("field %q has no German label", field)
		}
	}
	if got := importFieldLabel(core.TimetableField(0, core.TimetableFieldEndTime)); got != "Programmpunkt 1, End-Uhrzeit" {
		t.Errorf("label = %q", got)
	}
	if got := importFieldLabel(core.EventFieldTimetable + "[3]"); got != "Programmpunkt 4" {
		t.Errorf("label = %q", got)
	}
	for _, unknown := range []string{"unbekannt.feld", "timetable[3", "timetable[drei]"} {
		if got := importFieldLabel(unknown); got != unknown {
			t.Errorf("label of the unknown field %q = %q, want the field itself", unknown, got)
		}
	}
}

func TestImportMessagesFollowTheFormsOfEventAndLocation(t *testing.T) {
	tests := map[core.FieldError]string{
		{Field: core.EventFieldType, Problem: core.ProblemUnknownCode}:                                  "Bitte einen der angebotenen Event-Typen auswählen.",
		{Field: "location.latitude", Problem: core.ProblemOutOfRange}:                                   "Die Breite muss zwischen −90 und 90 liegen.",
		{Field: "location.address.city", Problem: core.ProblemTooLong, Limit: 100}:                      "Höchstens 100 Zeichen.",
		{Field: core.TimetableField(4, core.TimetableFieldDate), Problem: core.ProblemMissing}:          "Bitte ein Datum angeben.",
		{Field: core.EventFieldTimetable, Problem: core.ProblemTooMany, Limit: 100}:                     "Höchstens 100 Programmpunkte.",
		{Field: core.EventFieldAllDay, Problem: core.ProblemInvalidFormat}:                              msgImportWrongType,
		{Field: "location.address", Problem: core.ProblemInvalidFormat}:                                 msgImportWrongType,
		{Field: core.EventFieldNote, Problem: core.ProblemOutOfRange}:                                   msgFieldInvalid,
		{Field: core.EventFieldImportKey, Problem: core.ProblemDuplicateInFile}:                         "Diesen Import-Schlüssel tragen mehrere Einträge der Datei.",
		{Field: "location.name", Problem: core.ProblemAmbiguous}:                                        msgImportAmbiguousLocation,
		{Field: core.EventFieldTitle, Problem: core.ProblemControlCharacter}:                            msgImportControlCharacter,
		{Field: "location.address.postalCode", Problem: core.ProblemControlCharacter}:                   msgImportControlCharacter,
		{Field: core.TimetableField(0, core.TimetableFieldDate), Problem: core.ProblemControlCharacter}: msgImportControlCharacter,
	}
	for field, want := range tests {
		if got := importProblemMessage(field); got != want {
			t.Errorf("message of %v = %q, want %q", field, got, want)
		}
	}
}

func TestImportRejectsTheWholeFileWithAGermanMessage(t *testing.T) {
	tooLarge := `{"formatVersion":1,"events":[` + strings.Repeat(" ", core.MaxImportFileBytes) + `]}`
	tests := map[string]struct {
		content string
		want    string
	}{
		"no JSON":           {content: "Titel;Typ", want: "Die Datei ist kein gültiges JSON-Objekt."},
		"no format version": {content: `{"events":[{}]}`, want: "Die Datei nennt keine Formatversion (formatVersion)."},
		"unknown version":   {content: `{"formatVersion":2,"events":[{}]}`, want: "Unbekannte Formatversion. Unterstützt wird nur formatVersion 1."},
		"no events":         {content: `{"formatVersion":1}`, want: "Die Datei enthält keine Event-Liste (events)."},
		"no entries":        {content: `{"formatVersion":1,"events":[]}`, want: "Die Event-Liste der Datei ist leer."},
		"larger than 2 MiB": {content: tooLarge, want: msgImportTooLarge},
		// 0xFC is ü in Windows-1252, as an editor saves a file in ANSI.
		"not UTF-8":     {content: `{"formatVersion":1,"events":[{"title":"Gr` + "\xfc" + `n"}]}`, want: "Die Datei ist nicht in UTF-8 gespeichert."},
		"more than 150": {content: `{"formatVersion":1,"events":[` + strings.Repeat("{},", core.MaxImportEntries) + `{}]}`, want: "Die Datei enthält mehr als 150 Events."},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)

			rec := ts.postImport(t, importFileField, tt.content)

			assertStatusCode(t, rec, http.StatusUnprocessableEntity)
			assertBodyContains(t, rec, tt.want, `type="file"`)
			assertBodyLacks(t, rec, "<table")
		})
	}
}

func TestImportOfAFileFarOverTheLimitIsRejectedBeforeReading(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.postImport(t, importFileField, strings.Repeat("x", maxImportRequestBytes))

	assertStatusCode(t, rec, http.StatusRequestEntityTooLarge)
	assertBodyContains(t, rec, msgImportTooLarge)
}

func TestImportWithoutFileAsksForOne(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.postImport(t, "other", validImportFile)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, msgImportNoFile)
}

func TestImportThatIsNoMultipartFormIsABadRequest(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.post(importPath, nil)

	assertStatusCode(t, rec, http.StatusBadRequest)
}

func TestImportShowsAGenericMessageForAnUnknownFileProblem(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.imports = failingImports{err: &core.ImportFileError{Problem: "brandNew"}}

	rec := ts.postImport(t, importFileField, validImportFile)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, msgImportUnreadable)
}

func TestImportFailureIsAServerErrorAndLogged(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.imports = failingImports{err: errStorageDown}

	rec := ts.postImport(t, importFileField, validImportFile)

	assertStatusCode(t, rec, http.StatusInternalServerError)
	if !strings.Contains(ts.logs.String(), logMsgImportFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
		t.Errorf("log %q does not record the failure", ts.logs.String())
	}
	assertLoggedAt(t, ts, slog.LevelError, logMsgImportFailed)
}

func TestImportPastTheDeadlineIsAServerErrorAndLoggedAsWarning(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.imports = failingImports{err: errRequestTimedOut}

	rec := ts.postImport(t, importFileField, validImportFile)

	assertStatusCode(t, rec, http.StatusInternalServerError)
	assertLoggedAsWarningOnly(t, ts, logMsgImportFailed)
}

func TestImportRequiresSessionAndSameOrigin(t *testing.T) {
	ts := newTestServer(t)

	assertRedirect(t, ts.do(httptest.NewRequest(http.MethodGet, importPath, nil)), loginPath)
	assertRedirect(t, ts.do(httptest.NewRequest(http.MethodPost, importPath, strings.NewReader(validImportFile))), loginPath)

	crossSite := httptest.NewRequest(http.MethodPost, importPath, strings.NewReader(validImportFile))
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	assertStatusCode(t, ts.do(withCookie(crossSite, ts.validCookie())), http.StatusForbidden)
}

// importRowTexts returns the texts of the result row at position: its
// cells, the details of the last one as separate texts.
func importRowTexts(rec *httptest.ResponseRecorder, position string) []string {
	for _, row := range strings.Split(html.UnescapeString(rec.Body.String()), "<tr")[1:] {
		_, row, _ = strings.Cut(row, ">")
		row, _, found := strings.Cut(row, "</tr>")
		if !found || !strings.HasPrefix(strings.TrimSpace(row), "<td>"+position+"</td>") {
			continue
		}
		var texts []string
		for _, part := range strings.FieldsFunc(row, func(r rune) bool { return r == '<' || r == '>' }) {
			if text := strings.TrimSpace(part); text != "" && !isMarkup(text) {
				texts = append(texts, text)
			}
		}
		return texts
	}
	return nil
}

// isMarkup reports whether a part between angle brackets is a tag name.
func isMarkup(text string) bool {
	return slices.Contains([]string{"td", "/td", "ul", "/ul", "li", "/li", "p", "/p", "a", "/a", "span", "/span", "label", "/label", "fieldset", "/fieldset", "legend", "/legend"}, text) ||
		strings.HasPrefix(text, "ul ") || strings.HasPrefix(text, "a ") || strings.HasPrefix(text, "span ") || strings.HasPrefix(text, "input ")
}

// failingFile fails to read or to close.
type failingFile struct {
	io.Reader
	closeErr error
}

func (f failingFile) Close() error { return f.closeErr }

func TestUploadedFileThatCannotBeReadOrClosedIsABadRequest(t *testing.T) {
	for name, file := range map[string]failingFile{
		"read":  {Reader: iotest.ErrReader(errStorageDown)},
		"close": {Reader: strings.NewReader(validImportFile), closeErr: errStorageDown},
	} {
		t.Run(name, func(t *testing.T) {
			data, rejection := readUploadedFile(file)

			if data != nil || rejection != rejectNoForm {
				t.Errorf("readUploadedFile = %q, %+v, want the bad request", data, rejection)
			}
		})
	}
}
