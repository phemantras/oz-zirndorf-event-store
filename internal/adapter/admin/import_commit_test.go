package admin

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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

// decisionFields returns the fields the commit form sends for the entry at
// position.
func decisionFields(position int, class core.ImportClass, targetID string, newLocation bool, choice string) []formField {
	fields := []formField{
		{importPositionField, strconv.Itoa(position)},
		{importDecisionField(importClassFieldPrefix, position), string(class)},
		{importDecisionField(importTargetFieldPrefix, position), targetID},
	}
	if newLocation {
		fields = append(fields, formField{importDecisionField(importNewLocationFieldPrefix, position), importNewLocationTrue})
	}
	if choice != "" {
		fields = append(fields, formField{importDecisionField(importChoiceFieldPrefix, position), choice})
	}
	return fields
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
		msgImportChoiceSkip, msgImportChoiceCreate, msgImportChoiceOverwrite+" Kirchweihmarkt (16.10.2026)",
		msgImportCommitHint, ">"+msgImportCommitButton+"</button>",
	)
	assertBodyLacks(t, rec, "checked", `name="newLocation-1"`, `name="choice-1"`, `name="choice-3"`)
}

func TestImportOffersNoOverwriteForADuplicateOnlyOfTheFile(t *testing.T) {
	ts := newTestServer(t)
	ts.seed(t, hallName)
	entry := `{"title": "Kirchweihmarkt", "type": "market", "location": {"name": "Paul-Metz-Halle"},
	           "startDate": "2026-10-16", "source": {"description": "Plakat"}}`

	rec := ts.postImport(t, importFileField, `{"formatVersion": 1, "events": [`+entry+`, `+entry+`]}`)

	assertBodyContains(t, rec, `name="choice-1" value="skip"`, `name="choice-2" value="create"`)
	assertBodyLacks(t, rec, `value="overwrite:`, msgImportChoiceOverwrite)
}

func TestImportCommitStoresTheDecidedEntriesAndShowsTheSummary(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.postCommit(t, classifiedImportFile,
		decisionFields(1, core.ImportClassUpdate, market.ID, false, ""),
		decisionFields(2, core.ImportClassDuplicateSuspect, "", false, string(core.ImportChoiceCreate)),
		decisionFields(3, core.ImportClassNew, "", true, ""),
		decisionFields(4, core.ImportClassNew, "", true, ""),
	)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		"<h2>"+msgImportCommitted+"</h2>",
		"<li>neu angelegt: 3</li>", "<li>aktualisiert: 1</li>", "<li>unverändert: 0</li>", "<li>übersprungen: 0</li>",
		"<li>ohne Entscheidung: 0</li>", "<li>fehlerhaft: 0</li>", "<li>veraltet: 0</li>", "<li>neu angelegte Orte: 1</li>",
		`href="`+eventsPath+`"`,
	)
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
		decisionFields(1, core.ImportClassDuplicateSuspect, "", false, string(core.ImportChoiceOverwrite)+importChoiceValueSeparator+market.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<li>aktualisiert: 1</li>")
	if got := ts.events.events[market.ID]; got.Source.Description != "Plakat" || got.ImportKey != marketImportKey {
		t.Errorf("market = %+v, want it overwritten with its import key kept", got)
	}
}

func TestImportCommitLeavesUndecidedAndChangedEntriesAlone(t *testing.T) {
	ts := newTestServer(t)
	market := ts.seedKeyedMarket(t)

	rec := ts.postCommit(t, classifiedImportFile,
		decisionFields(1, core.ImportClassNew, "", false, ""),
		decisionFields(2, core.ImportClassDuplicateSuspect, "", false, "vielleicht"),
		[]formField{{importPositionField, "drei"}},
		decisionFields(4, core.ImportClassNew, "", true, ""),
	)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "<li>neu angelegt: 1</li>", "<li>ohne Entscheidung: 1</li>", "<li>veraltet: 2</li>", "<li>neu angelegte Orte: 1</li>")
	if len(ts.events.events) != 2 || ts.events.events[market.ID].Type != core.EventTypeMarket {
		t.Errorf("events = %d, want the market unchanged and one new event", len(ts.events.events))
	}
}

func TestImportCommitFailureSavesNothingAndSaysSo(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.imports = failingImports{err: errStorageDown}

	rec := ts.postCommit(t, validImportFile, decisionFields(1, core.ImportClassNew, "", false, ""))

	assertStatusCode(t, rec, http.StatusInternalServerError)
	assertBodyContains(t, rec, msgImportCommitFailed, `type="file"`)
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
