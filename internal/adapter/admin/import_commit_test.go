package admin

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

func (f failingImports) CommitImport(context.Context, []byte, []core.ImportDecision) (core.ImportSummary, error) {
	return core.ImportSummary{}, f.err
}

// formField is one field of a multipart form, in the order it is sent.
type formField struct{ name, value string }

// entryDecision is what the commit form sends for one entry.
type entryDecision struct {
	position    int
	class       core.ImportClass
	targetID    string
	newLocation bool
	choice      string
	// candidates are the IDs of the stored candidates the preview showed.
	candidates []string
	// fingerprints are the fingerprints of the stored events the preview
	// showed, by event ID.
	fingerprints map[string]string
}

// decisionFields returns the fields the commit form sends for decision.
func decisionFields(decision entryDecision) []formField {
	position := decision.position
	fields := []formField{
		{importPositionField, strconv.Itoa(position)},
		{importDecisionField(importClassFieldPrefix, position), string(decision.class)},
		{importDecisionField(importTargetFieldPrefix, position), decision.targetID},
	}
	if decision.newLocation {
		fields = append(fields, formField{importDecisionField(importNewLocationFieldPrefix, position), importNewLocationTrue})
	}
	if decision.candidates != nil {
		fields = append(fields, formField{importDecisionField(importCandidatesFieldPrefix, position), strings.Join(decision.candidates, importCandidateSeparator)})
	}
	if decision.fingerprints != nil {
		var pairs []string
		for id, fingerprint := range decision.fingerprints {
			pairs = append(pairs, id+importFingerprintPairSeparator+fingerprint)
		}
		slices.Sort(pairs)
		fields = append(fields, formField{importDecisionField(importFingerprintsFieldPrefix, position), strings.Join(pairs, importFingerprintSeparator)})
	}
	if decision.choice != "" {
		fields = append(fields, formField{importDecisionField(importChoiceFieldPrefix, position), decision.choice})
	}
	return fields
}

// fingerprintOf returns the fingerprints of event as the preview shows
// them.
func fingerprintOf(event core.Event) map[string]string {
	return map[string]string{event.ID: core.EventFingerprint(event)}
}

func (ts *testServer) postCommit(t *testing.T, content string, fields ...[]formField) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	all := append([]formField{{importContentField, content}}, joinFields(fields)...)
	for _, field := range all {
		if err := writer.WriteField(field.name, field.value); err != nil {
			t.Fatalf("write field %s: %v", field.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, importCommitPath, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return ts.do(withCookie(req, ts.validCookie()))
}

func joinFields(groups [][]formField) []formField {
	var joined []formField
	for _, group := range groups {
		joined = append(joined, group...)
	}
	return joined
}

// seedKeyedMarket stores the hall and the market that classifiedImportFile
// updates by its import key.
func (ts *testServer) seedKeyedMarket(t *testing.T) core.Event {
	t.Helper()
	hall := ts.seed(t, hallName)
	market := ts.seedEvent(t, marketForm(hall.ID))
	market.ImportKey = marketImportKey
	ts.events.events[market.ID] = market
	return market
}

func TestImportPreviewOffersToCommitTheFileWithEveryEntry(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.postImport(t, importFileField, classifiedImportFile)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`action="`+importCommitPath+`"`, `enctype="multipart/form-data"`,
		`name="`+importContentField+`" value="`+classifiedImportFile+`"`,
		`name="position" value="1"`, `name="class-1" value="update"`, `name="target-1" value="`+market.ID+`"`,
		`name="class-2" value="duplicateSuspect"`, `name="target-2" value=""`,
		`name="class-3" value="new"`, `name="newLocation-3" value="true"`, `name="newLocation-4" value="true"`,
		`name="choice-2" value="skip"`, `name="choice-2" value="create"`, `name="choice-2" value="overwrite:`+market.ID+`"`,
		`name="candidates-2" value="`+market.ID+`"`,
		`name="fingerprints-1" value="`+market.ID+`:`+core.EventFingerprint(market)+`"`,
		`name="fingerprints-2" value="`+market.ID+`:`+core.EventFingerprint(market)+`"`, "<fieldset><legend>"+msgImportChoiceLegend+"</legend>",
		msgImportChoiceSkip, msgImportChoiceCreate, msgImportChoiceOverwrite+" Kirchweihmarkt (16.10.2026)",
		msgImportCommitHint, ">"+msgImportCommitButton+"</button>",
	)
	assertBodyLacks(t, rec, "checked", `name="newLocation-1"`, `name="choice-1"`, `name="choice-3"`, `name="candidates-1"`, `name="candidates-3"`, `name="fingerprints-3"`)
}

func TestImportOffersNoOverwriteForADuplicateOnlyOfTheFile(t *testing.T) {
	ts := newTestServer(t)
	ts.seed(t, hallName)
	entry := `{"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
	           "startDate": "2026-10-16", "source": {"description": "Plakat"}}`

	rec := ts.postImport(t, importFileField, `{"formatVersion": 1, "events": [`+entry+`, `+entry+`]}`)

	assertBodyContains(t, rec, `name="choice-1" value="skip"`, `name="choice-2" value="create"`, `name="candidates-1" value=""`)
	assertBodyLacks(t, rec, `value="overwrite:`, msgImportChoiceOverwrite)
}

func TestImportCommitStoresTheDecidedEntriesAndShowsTheSummary(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.postCommit(t, classifiedImportFile,
		decisionFields(entryDecision{position: 1, class: core.ImportClassUpdate, targetID: market.ID, fingerprints: fingerprintOf(market)}),
		decisionFields(entryDecision{position: 2, class: core.ImportClassDuplicateSuspect, choice: string(core.ImportChoiceCreate), candidates: []string{market.ID}}),
		decisionFields(entryDecision{position: 3, class: core.ImportClassNew, newLocation: true}),
		decisionFields(entryDecision{position: 4, class: core.ImportClassNew, newLocation: true}),
	)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"<h2>"+msgImportSummaryHeading+"</h2>",
		"<li>neu angelegt: 3</li>", "<li>aktualisiert: 1</li>", "<li>unverändert: 0</li>", "<li>übersprungen: 0</li>",
		"<li>ohne Entscheidung: 0</li>", "<li>fehlerhaft: 0</li>", "<li>veraltet: 0</li>", "<li>neu angelegte Orte: 1</li>",
		`href="`+eventsPath+`"`,
	)
	assertBodyLacks(t, rec, msgImportNotTakenHint)
	if len(ts.events.events) != 4 || len(ts.locations.locations) != 2 || ts.events.events[market.ID].Type != core.EventTypeFestival {
		t.Errorf("events = %d, locations = %d, want 4 and 2 with the market updated", len(ts.events.events), len(ts.locations.locations))
	}
}

func TestImportCommitOverwritesTheChosenEvent(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)
	file := `{"formatVersion": 1, "events": [{"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
	          "startDate": "2026-10-16", "source": {"description": "Plakat"}}]}`

	rec := ts.postCommit(t, file,
		decisionFields(entryDecision{
			position: 1, class: core.ImportClassDuplicateSuspect, candidates: []string{market.ID},
			choice:       string(core.ImportChoiceOverwrite) + importChoiceValueSeparator + market.ID,
			fingerprints: fingerprintOf(market),
		}))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<li>aktualisiert: 1</li>")
	if got := ts.events.events[market.ID]; got.Source.Description != "Plakat" || got.ImportKey != marketImportKey {
		t.Errorf("market = %+v, want it overwritten with its import key kept", got)
	}
}

func TestImportCommitOverwritesTheChosenOfTwoStoredCandidates(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)
	twin := market
	// The ID sorts after the market's, so it is the second pair of the
	// fingerprints field.
	twin.ID, twin.ImportKey, twin.Note = "0192f0b1-0000-7000-9000-999999999999", "", "Zwilling"
	ts.events.events[twin.ID] = twin
	file := `{"formatVersion": 1, "events": [{"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
	          "startDate": "2026-10-16", "source": {"description": "Plakat"}}]}`

	rec := ts.postCommit(t, file,
		decisionFields(entryDecision{
			position: 1, class: core.ImportClassDuplicateSuspect, candidates: []string{market.ID, twin.ID},
			choice:       string(core.ImportChoiceOverwrite) + importChoiceValueSeparator + twin.ID,
			fingerprints: map[string]string{market.ID: core.EventFingerprint(market), twin.ID: core.EventFingerprint(twin)},
		}))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<li>aktualisiert: 1</li>")
	if got := ts.events.events[twin.ID]; got.Source.Description != "Plakat" || got.Note != "" {
		t.Errorf("twin = %+v, want it overwritten", got)
	}
	if got := ts.events.events[market.ID]; got.Source.Description == "Plakat" {
		t.Errorf("market = %+v, want it unchanged", got)
	}
}

func TestImportCommitLeavesUndecidedAndChangedEntriesAlone(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.postCommit(t, classifiedImportFile,
		decisionFields(entryDecision{position: 1, class: core.ImportClassNew}),
		decisionFields(entryDecision{position: 2, class: core.ImportClassDuplicateSuspect, choice: "vielleicht", candidates: []string{market.ID}}),
		[]formField{{importPositionField, "drei"}},
		decisionFields(entryDecision{position: 4, class: core.ImportClassNew, newLocation: true}),
	)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<li>neu angelegt: 1</li>", "<li>ohne Entscheidung: 1</li>", "<li>veraltet: 2</li>", "<li>neu angelegte Orte: 1</li>")
	if len(ts.events.events) != 2 || ts.events.events[market.ID].Type != core.EventTypeMarket {
		t.Errorf("events = %d, want the market unchanged and one new event", len(ts.events.events))
	}
}

func TestImportCommitListsTheEntriesItDidNotTake(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.postCommit(t, classifiedImportFile,
		decisionFields(entryDecision{position: 1, class: core.ImportClassNew}),
		decisionFields(entryDecision{position: 2, class: core.ImportClassDuplicateSuspect, candidates: []string{market.ID}}),
		decisionFields(entryDecision{position: 3, class: core.ImportClassNew, newLocation: true}),
		decisionFields(entryDecision{position: 4, class: core.ImportClassNew, newLocation: true}),
	)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"<h2>Ergebnis des Imports</h2>",
		"<tr><td>1</td><td>Kirchweihmarkt</td><td>veraltet</td></tr>",
		"<tr><td>2</td><td>Kirchweihmarkt</td><td>ohne Entscheidung</td></tr>",
		`<p class="hint">`+msgImportNotTakenHint+"</p>",
	)
	assertBodyLacks(t, rec, msgImportNothingSaved, "<td>Konzert an der Veste</td>", "<td>neu angelegt</td>")
}

// The cap on entries keeps the commit form within the parts a multipart
// form may have; a file of the most entries, each sending the most fields
// an entry can have, is still read. An entry that brings a new location
// never has stored candidates, so at most a duplicate suspect sends 6
// fields and an update with a new location 5.
func TestImportCommitReadsTheFormOfTheMostEntries(t *testing.T) {
	ts := newTestServer(t)
	content := `{"formatVersion":1,"events":[` + strings.TrimSuffix(strings.Repeat("{},", core.MaxImportEntries), ",") + `]}`
	var decisions [][]formField
	for position := 1; position <= core.MaxImportEntries; position++ {
		decisions = append(decisions, decisionFields(entryDecision{
			position: position, class: core.ImportClassDuplicateSuspect, targetID: "ziel",
			candidates: []string{"erster", "zweiter"}, choice: string(core.ImportChoiceSkip),
			fingerprints: map[string]string{"erster": strings.Repeat("a", 64), "zweiter": strings.Repeat("b", 64)},
		}))
	}

	rec := ts.postCommit(t, content, decisions...)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<h2>"+msgImportSummaryHeading+"</h2>")
}

func TestImportCommitStaleWhenTheFormShowedNoStoredCandidate(t *testing.T) {
	ts := newTestServer(t)
	ts.seedKeyedMarket(t)
	file := `{"formatVersion": 1, "events": [{"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
	          "startDate": "2026-10-16", "source": {"description": "Plakat"}}]}`

	rec := ts.postCommit(t, file,
		decisionFields(entryDecision{position: 1, class: core.ImportClassDuplicateSuspect, choice: string(core.ImportChoiceCreate)}))

	assertBodyContains(t, rec, "<li>veraltet: 1</li>", "<li>neu angelegt: 0</li>")
}

func TestImportCommitFailureSavesNothingAndSaysSo(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.imports = failingImports{err: errStorageDown}

	rec := ts.postCommit(t, validImportFile, decisionFields(entryDecision{position: 1, class: core.ImportClassNew}))

	assertStatusCode(t, rec, http.StatusInternalServerError)
	assertBodyContains(t, rec, msgImportCommitFailed, `type="file"`)
	assertBodyLacks(t, rec, msgImportNothingSaved)
	if !strings.Contains(ts.logs.String(), logMsgImportCommitFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
		t.Errorf("log %q does not record the failure", ts.logs.String())
	}
}

func TestImportCommitOfAnUnreadableFileShowsItsMessage(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.postCommit(t, "Titel;Typ")

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, "Die Datei ist kein gültiges JSON-Objekt.")
}

func TestImportCommitRejectsRequestsThatAreNoCommitForm(t *testing.T) {
	ts := newTestServer(t)

	assertStatusCode(t, ts.post(importCommitPath, nil), http.StatusBadRequest)
	rec := ts.postCommit(t, strings.Repeat("x", maxImportCommitBytes))
	assertStatusCode(t, rec, http.StatusRequestEntityTooLarge)
	assertBodyContains(t, rec, msgImportTooLarge)
}

func TestImportCommitRequiresSessionAndSameOrigin(t *testing.T) {
	ts := newTestServer(t)

	assertRedirect(t, ts.do(httptest.NewRequest(http.MethodPost, importCommitPath, strings.NewReader(validImportFile))), loginPath)

	crossSite := httptest.NewRequest(http.MethodPost, importCommitPath, strings.NewReader(validImportFile))
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	assertStatusCode(t, ts.do(withCookie(crossSite, ts.validCookie())), http.StatusForbidden)
}

func TestImportNamesEveryOutcomeInGerman(t *testing.T) {
	for _, outcome := range core.ImportOutcomes() {
		if importOutcomeLabels[outcome] == "" {
			t.Errorf("outcome %q has no German label", outcome)
		}
	}
}

func TestImportCommitStaleWhenTheTargetChangedOrTheFormSentNoFingerprint(t *testing.T) {
	const changedNote = "vor dem Import geändert"
	// Each case returns the fingerprints the form sends and may change the
	// stored market after the preview.
	tests := map[string]func(*testServer, core.Event) map[string]string{
		"target changed": func(ts *testServer, market core.Event) map[string]string {
			previewed := fingerprintOf(market)
			market.Note = changedNote
			ts.events.events[market.ID] = market
			return previewed
		},
		"no fingerprint":           func(*testServer, core.Event) map[string]string { return nil },
		"pair without fingerprint": func(_ *testServer, market core.Event) map[string]string { return map[string]string{market.ID: ""} },
	}
	for name, fingerprints := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			market := ts.seedKeyedMarket(t)
			sent := fingerprints(ts, market)
			stored := ts.events.events[market.ID]

			rec := ts.postCommit(t, classifiedImportFile,
				decisionFields(entryDecision{position: 1, class: core.ImportClassUpdate, targetID: market.ID, fingerprints: sent}))

			assertBodyContains(t, rec, "<tr><td>1</td><td>Kirchweihmarkt</td><td>veraltet</td></tr>", "<li>aktualisiert: 0</li>")
			if got := ts.events.events[market.ID]; !reflect.DeepEqual(got, stored) {
				t.Errorf("market = %+v, want it kept as %+v", got, stored)
			}
		})
	}
}
