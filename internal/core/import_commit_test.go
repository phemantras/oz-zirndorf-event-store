package core

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

// otherKey is an import key that no stored event has.
const otherKey = "anderer-schluessel"

// newCommitService returns the import use cases on events and locations,
// with a fake transaction on the same repositories.
func newCommitService(events *fakeEventRepo, locations *fakeLocationRepo) (*ImportService, *fakeTx) {
	tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
	return NewImportService(NewEventService(tx, events, locations)), tx
}

// decisionsOf returns what the form sends for every entry of preview, no
// duplicate decided.
func decisionsOf(preview ImportPreview) []ImportDecision {
	decisions := make([]ImportDecision, 0, len(preview.Entries))
	for _, entry := range preview.Entries {
		decisions = append(decisions, ImportDecision{
			Position: entry.Position, Class: entry.Class, TargetID: entry.TargetID, NewLocation: entry.NewLocation,
			CandidateIDs: entry.StoredCandidateIDs(), Fingerprints: entry.StoredFingerprints(),
		})
	}
	return decisions
}

// previewAndDecide previews data on service and returns the decisions of
// the form, no duplicate decided.
func previewAndDecide(t *testing.T, service *ImportService, data []byte) []ImportDecision {
	t.Helper()
	preview, err := service.PreviewImport(context.Background(), data)
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	return decisionsOf(preview)
}

func committedImport(t *testing.T, service *ImportService, data []byte, decisions []ImportDecision) ImportSummary {
	t.Helper()
	summary, err := service.CommitImport(context.Background(), data, decisions)
	if err != nil {
		t.Fatalf("CommitImport: %v", err)
	}
	return summary
}

func assertOutcomes(t *testing.T, summary ImportSummary, want ...ImportOutcome) {
	t.Helper()
	var got []ImportOutcome
	for _, result := range summary.Results {
		got = append(got, result.Outcome)
	}
	if !slices.Equal(got, want) {
		t.Errorf("outcomes = %v, want %v", got, want)
	}
}

func TestCommitImportCreatesNewEventsAndTheirNewLocationOnce(t *testing.T) {
	events, locations := newFakeEventRepo(), newFakeLocationRepo(hall())
	service, tx := newCommitService(events, locations)
	reading := concertEntry()
	reading["title"], reading["importKey"] = "Lesung an der Veste", nil
	location := concertEntry()["location"].(importObject)
	location["name"], location["note"] = "ALTE VESTE", "Eingang am Tor"
	reading["location"] = location
	data := marshalImport(t, importFile(marketEntry(), concertEntry(), reading))

	summary := committedImport(t, service, data, previewAndDecide(t, service, data))

	assertOutcomes(t, summary, ImportOutcomeCreated, ImportOutcomeCreated, ImportOutcomeCreated)
	if summary.CreatedLocations != 1 || len(locations.created) != 1 || locations.created[0].Note != "Zugang über den Wald" {
		t.Fatalf("created locations = %d, %+v, want the new location once with the details of its first entry", summary.CreatedLocations, locations.created)
	}
	wantLocations := []string{hallID, newID, newID}
	for index, created := range events.created {
		if created.LocationID != wantLocations[index] {
			t.Errorf("event %d at %q, want %q", index, created.LocationID, wantLocations[index])
		}
	}
	if len(events.created) != 3 || tx.runs != 1 {
		t.Errorf("created %d events in %d transactions, want 3 in one", len(events.created), tx.runs)
	}
	concert := events.events[events.created[1].ID]
	if concert.ImportKey != "konzert-2026" || len(concert.Timetable) != 1 {
		t.Errorf("concert = %+v, want import key konzert-2026 and its timetable", concert)
	}
	if summary.Results[1].Position != 2 || summary.Results[1].Title != "Konzert im Park" {
		t.Errorf("result 2 = %+v, want position 2 with its title", summary.Results[1])
	}
}

func TestCommitImportUpdatesTheTargetOfAnImportKey(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, marketKey))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	entry := keyedMarketEntry()
	entry["title"] = "Kirchweihmarkt 2026"
	data := marshalImport(t, importFile(entry))

	summary := committedImport(t, service, data, previewAndDecide(t, service, data))

	assertOutcomes(t, summary, ImportOutcomeUpdated)
	if len(events.updated) != 1 || events.updated[0].ID != marketID || events.events[marketID].Title != "Kirchweihmarkt 2026" {
		t.Errorf("updated = %+v, want the market replaced", events.updated)
	}
	if got := events.events[marketID].ImportKey; got != marketKey {
		t.Errorf("import key = %q, want %q", got, marketKey)
	}
}

func TestCommitImportLeavesUnchangedAndErroneousEntriesAlone(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, marketKey))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	broken := concertEntry()
	broken["type"] = "concert"
	data := marshalImport(t, importFile(keyedMarketEntry(), broken))

	summary := committedImport(t, service, data, previewAndDecide(t, service, data))

	assertOutcomes(t, summary, ImportOutcomeUnchanged, ImportOutcomeError)
	if len(events.created)+len(events.updated) != 0 {
		t.Error("an unchanged or erroneous entry was written")
	}
}

// suspectFile has the market without import key, which may duplicate the
// stored market.
func suspectFile(t *testing.T, entries ...importObject) []byte {
	t.Helper()
	if entries == nil {
		entries = []importObject{marketEntry()}
	}
	all := make([]any, 0, len(entries))
	for _, entry := range entries {
		all = append(all, entry)
	}
	return marshalImport(t, importFile(all...))
}

func TestCommitImportFollowsTheDecisionOnADuplicateSuspect(t *testing.T) {
	tests := map[string]struct {
		decide func(*ImportDecision)
		want   ImportOutcome
	}{
		"no decision": {func(*ImportDecision) {}, ImportOutcomeUndecided},
		"skip":        {func(d *ImportDecision) { d.Choice = ImportChoiceSkip }, ImportOutcomeSkipped},
		"create":      {func(d *ImportDecision) { d.Choice = ImportChoiceCreate }, ImportOutcomeCreated},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo(storedMarket(t, marketID, ""))
			service, _ := newCommitService(events, newFakeLocationRepo(hall()))
			data := suspectFile(t)
			decisions := previewAndDecide(t, service, data)
			tt.decide(&decisions[0])

			summary := committedImport(t, service, data, decisions)

			assertOutcomes(t, summary, tt.want)
			wantCreated := 0
			if tt.want == ImportOutcomeCreated {
				wantCreated = 1
			}
			if len(events.created) != wantCreated || len(events.updated) != 0 {
				t.Errorf("created %d, updated %d, want %d created", len(events.created), len(events.updated), wantCreated)
			}
		})
	}
}

func TestCommitImportOverwritesTheChosenStoredEvent(t *testing.T) {
	tests := map[string]struct {
		storedKey, entryKey, wantKey string
	}{
		"key of the entry":  {"", otherKey, otherKey},
		"key of the target": {marketKey, "", marketKey},
		"same key":          {"", "", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			stored := storedMarket(t, marketID, tt.storedKey)
			stored.Note = "alt"
			events := newFakeEventRepo(stored)
			service, _ := newCommitService(events, newFakeLocationRepo(hall()))
			entry := marketEntry()
			entry["importKey"] = tt.entryKey
			entry["timetable"] = []any{importObject{"description": "Eröffnung", "date": "2026-10-16"}}
			data := suspectFile(t, entry)
			decisions := previewAndDecide(t, service, data)
			decisions[0].Choice, decisions[0].OverwriteID = ImportChoiceOverwrite, marketID

			summary := committedImport(t, service, data, decisions)

			assertOutcomes(t, summary, ImportOutcomeUpdated)
			got := events.events[marketID]
			if got.Note != "" || len(got.Timetable) != 1 || got.ImportKey != tt.wantKey || len(events.created) != 0 {
				t.Errorf("market = %+v, want it replaced with timetable and import key %q", got, tt.wantKey)
			}
		})
	}
}

func TestCommitImportRefusesToOverwriteAnEventWithAnotherImportKey(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, marketKey+"-alt"))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	entry := marketEntry()
	entry["importKey"] = otherKey
	data := suspectFile(t, entry)
	decisions := previewAndDecide(t, service, data)
	decisions[0].Choice, decisions[0].OverwriteID = ImportChoiceOverwrite, marketID

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeError)
	if len(events.updated) != 0 || len(events.importKeysSet) != 0 {
		t.Error("the event of another import key was overwritten")
	}
}

func TestCommitImportRejectsEveryEntryOfASharedTarget(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, marketKey))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	changed := keyedMarketEntry()
	changed["note"] = "geändert"
	twin := marketEntry()
	twin["startDate"] = "2026-10-23"
	overwriting := marketEntry()
	overwriting["note"] = "überschrieben"
	data := suspectFile(t, changed, twin, overwriting)
	decisions := previewAndDecide(t, service, data)
	decisions[2].Choice, decisions[2].OverwriteID = ImportChoiceOverwrite, marketID

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeError, ImportOutcomeCreated, ImportOutcomeError)
	if len(events.updated) != 0 {
		t.Error("a shared target was written")
	}
}

func TestCommitImportRejectsTwoOverwritesOfTheSameEvent(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, ""))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	first, second := marketEntry(), marketEntry()
	second["note"] = "zweiter Eintrag"
	data := suspectFile(t, first, second)
	decisions := previewAndDecide(t, service, data)
	for index := range decisions {
		decisions[index].Choice, decisions[index].OverwriteID = ImportChoiceOverwrite, marketID
	}

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeError, ImportOutcomeError)
	if len(events.updated) != 0 {
		t.Error("the shared target was written")
	}
}

func TestCommitImportReportsEntriesThatChangedSinceThePreviewAsStale(t *testing.T) {
	tests := map[string]struct {
		stored []Event
		decide func(*ImportDecision)
	}{
		"other class": {[]Event{storedMarket(t, marketID, marketKey)}, func(d *ImportDecision) {
			d.Class, d.TargetID = ImportClassNew, ""
		}},
		"other target":           {[]Event{storedMarket(t, marketID, marketKey)}, func(d *ImportDecision) { d.TargetID = concertID }},
		"location no longer new": {[]Event{storedMarket(t, marketID, marketKey)}, func(d *ImportDecision) { d.NewLocation = true }},
		"no decision sent":       {nil, func(d *ImportDecision) { d.Position = 0 }},
		"overwrite target no candidate": {[]Event{storedMarket(t, marketID, "")}, func(d *ImportDecision) {
			d.Class, d.TargetID = ImportClassDuplicateSuspect, ""
			d.Choice, d.OverwriteID = ImportChoiceOverwrite, concertID
		}},
		"new stored candidate": {[]Event{storedMarket(t, marketID, "")}, func(d *ImportDecision) {
			d.Choice, d.CandidateIDs = ImportChoiceSkip, nil
		}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo(tt.stored...)
			service, _ := newCommitService(events, newFakeLocationRepo(hall()))
			entry := keyedMarketEntry()
			entry["note"] = "geändert"
			if tt.stored != nil && tt.stored[0].ImportKey == "" {
				entry = marketEntry()
			}
			data := suspectFile(t, entry)
			decisions := previewAndDecide(t, service, data)
			tt.decide(&decisions[0])

			summary := committedImport(t, service, data, decisions)

			assertOutcomes(t, summary, ImportOutcomeStale)
			if len(events.created)+len(events.updated) != 0 {
				t.Error("a stale entry was written")
			}
		})
	}
}

func TestCommitImportDoesNotCreateALocationThatExistsMeanwhile(t *testing.T) {
	locations := newFakeLocationRepo(hall())
	events := newFakeEventRepo()
	service, _ := newCommitService(events, locations)
	data := marshalImport(t, importFile(concertEntry()))
	decisions := previewAndDecide(t, service, data)
	veste := park()
	veste.ID, veste.Name, veste.NameKey = otherID, "Alte Veste", NormalizeKey("Alte Veste")
	locations.locations[veste.ID] = veste

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeStale)
	if summary.CreatedLocations != 0 || len(locations.created) != 0 || len(events.created) != 0 {
		t.Errorf("created %d locations and %d events, want none", len(locations.created), len(events.created))
	}
}

func TestCommitImportCreatesANewLocationOnlyForEntriesItWrites(t *testing.T) {
	locations := newFakeLocationRepo(hall())
	service, _ := newCommitService(newFakeEventRepo(), locations)
	data := marshalImport(t, importFile(concertEntry()))
	decisions := previewAndDecide(t, service, data)
	decisions[0].Class = ImportClassUpdate

	summary := committedImport(t, service, data, decisions)

	if summary.CreatedLocations != 0 || len(locations.created) != 0 {
		t.Errorf("created %d locations for a stale entry, want none", len(locations.created))
	}
}

func TestCommitImportTwiceCreatesNoDuplicate(t *testing.T) {
	events := newFakeEventRepo()
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	data := marshalImport(t, importFile(marketEntry(), concertEntry()))
	decisions := previewAndDecide(t, service, data)
	committedImport(t, service, data, decisions)

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeStale, ImportOutcomeStale)
	if len(events.created) != 2 || summary.CreatedLocations != 0 {
		t.Errorf("created %d events, %d locations in the second commit, want only the two of the first", len(events.created), summary.CreatedLocations)
	}
}

func TestCommitImportTwiceWithChoiceCreateCreatesNoDuplicate(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, ""))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	data := suspectFile(t)
	decisions := previewAndDecide(t, service, data)
	decisions[0].Choice = ImportChoiceCreate
	committedImport(t, service, data, decisions)

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeStale)
	if len(events.created) != 1 {
		t.Errorf("created %d events over both commits, want 1", len(events.created))
	}
}

func TestCommitImportKeepsAnEntryWhoseStoredCandidateIsGone(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, ""))
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	twin := marketEntry()
	twin["note"] = "zweimal"
	data := suspectFile(t, marketEntry(), twin)
	decisions := previewAndDecide(t, service, data)
	decisions[0].Choice, decisions[1].Choice = ImportChoiceSkip, ImportChoiceCreate
	delete(events.events, marketID)

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeSkipped, ImportOutcomeCreated)
}

func TestCommitImportCountsEveryEntryOnce(t *testing.T) {
	events := newFakeEventRepo(storedMarket(t, marketID, marketKey), storedMarket(t, concertID, ""))
	events.events[concertID] = func() Event {
		concert := events.events[concertID]
		concert.Times.StartDate = LocalDate{2026, 10, 23}
		concert.Period.Start = concert.Period.Start.AddDate(0, 0, 7)
		concert.Period.End = concert.Period.End.AddDate(0, 0, 7)
		return concert
	}()
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	suspect := marketEntry()
	suspect["startDate"] = "2026-10-23"
	skipped := marketEntry()
	skipped["startDate"] = "2026-10-23"
	skipped["note"] = "zweimal"
	broken := marketEntry()
	broken["type"] = "concert"
	data := suspectFile(t, keyedMarketEntry(), concertEntry(), suspect, skipped, broken)
	decisions := previewAndDecide(t, service, data)
	decisions[3].Choice = ImportChoiceSkip
	decisions[1].Class = ImportClassUpdate

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeUnchanged, ImportOutcomeStale, ImportOutcomeUndecided, ImportOutcomeSkipped, ImportOutcomeError)
	total := 0
	for _, outcome := range ImportOutcomes() {
		total += summary.CountOf(outcome)
	}
	if total != len(decisions) || len(ImportOutcomes()) != 7 {
		t.Errorf("counts add up to %d over %v, want %d", total, ImportOutcomes(), len(decisions))
	}
}

func TestCommitImportClearsTheReviewMarkOfWrittenEvents(t *testing.T) {
	events, locations := newFakeEventRepo(storedMarket(t, marketID, marketKey), storedMarket(t, concertID, "")), newFakeLocationRepo(hall())
	tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
	eventService := NewEventService(tx, events, locations)
	eventService.markForReview(marketID)
	eventService.markForReview(concertID)
	service := NewImportService(eventService)
	entry := keyedMarketEntry()
	entry["note"] = "geändert"
	data := marshalImport(t, importFile(entry))

	committedImport(t, service, data, previewAndDecide(t, service, data))

	if eventService.needsReview(marketID) || !eventService.needsReview(concertID) {
		t.Errorf("review marks: market %v, concert %v, want only the written market cleared", eventService.needsReview(marketID), eventService.needsReview(concertID))
	}
}

func TestCommitImportWritesNothingWhenTheDatabaseFails(t *testing.T) {
	tests := map[string]func(*fakeTx, *fakeEventRepo, *fakeLocationRepo){
		"begin":           func(tx *fakeTx, _ *fakeEventRepo, _ *fakeLocationRepo) { tx.beginErr = errDatabaseDown },
		"list events":     func(_ *fakeTx, e *fakeEventRepo, _ *fakeLocationRepo) { e.listErr = errDatabaseDown },
		"create location": func(_ *fakeTx, _ *fakeEventRepo, l *fakeLocationRepo) { l.writeErr = errDatabaseDown },
		"write event":     func(_ *fakeTx, e *fakeEventRepo, _ *fakeLocationRepo) { e.writeErr = errDatabaseDown },
		"set import key":  func(_ *fakeTx, e *fakeEventRepo, _ *fakeLocationRepo) { e.importKeyErr = errDatabaseDown },
		"find duplicates": func(_ *fakeTx, e *fakeEventRepo, _ *fakeLocationRepo) { e.findErr = errDatabaseDown },
		"name taken on write": func(_ *fakeTx, _ *fakeEventRepo, l *fakeLocationRepo) {
			l.raceWinner = &Location{ID: otherID, NameKey: NormalizeKey("Alte Veste")}
		},
	}
	for name, fail := range tests {
		t.Run(name, func(t *testing.T) {
			events, locations := newFakeEventRepo(storedMarket(t, marketID, marketKey)), newFakeLocationRepo(hall())
			service, tx := newCommitService(events, locations)
			service.events.markForReview(marketID)
			entry := keyedMarketEntry()
			entry["note"] = "geändert"
			data := marshalImport(t, importFile(entry, concertEntry()))
			decisions := previewAndDecide(t, service, data)
			fail(tx, events, locations)

			summary, err := service.CommitImport(context.Background(), data, decisions)

			if err == nil || summary.Results != nil {
				t.Errorf("CommitImport = %+v, %v, want an error", summary, err)
			}
			if !service.events.needsReview(marketID) {
				t.Error("a failed commit cleared a review mark")
			}
		})
	}
}

func TestCommitImportRejectsAnUnreadableFile(t *testing.T) {
	service, tx := newCommitService(newFakeEventRepo(), newFakeLocationRepo(hall()))

	_, err := service.CommitImport(context.Background(), []byte("kein JSON"), nil)

	var fileErr *ImportFileError
	if !errors.As(err, &fileErr) || fileErr.Problem != ImportProblemInvalidJSON || tx.runs != 0 {
		t.Errorf("err = %v, runs = %d, want an invalid file before any transaction", err, tx.runs)
	}
}

func TestCommitImportRejectsAFileWithTooManyEntries(t *testing.T) {
	service, tx := newCommitService(newFakeEventRepo(), newFakeLocationRepo(hall()))

	_, err := service.CommitImport(context.Background(), importFileWithEntries(MaxImportEntries+1), nil)

	var fileErr *ImportFileError
	if !errors.As(err, &fileErr) || fileErr.Problem != ImportProblemTooManyEntries || tx.runs != 0 {
		t.Errorf("err = %v, runs = %d, want too many entries before any transaction", err, tx.runs)
	}
}

func TestCommitImportReportsANewLocationNameTakenOnWriteAsConflict(t *testing.T) {
	events, locations := newFakeEventRepo(), newFakeLocationRepo(hall())
	service, _ := newCommitService(events, locations)
	data := marshalImport(t, importFile(concertEntry()))
	decisions := previewAndDecide(t, service, data)
	locations.raceWinner = &Location{ID: otherID, NameKey: NormalizeKey("Alte Veste")}

	_, err := service.CommitImport(context.Background(), data, decisions)

	if !errors.Is(err, ErrConflict) || len(events.created) != 0 {
		t.Errorf("err = %v, created %d events, want ErrConflict and nothing written", err, len(events.created))
	}
}

func TestCommitImportReportsAnEntryWhoseTargetChangedSinceThePreviewAsStale(t *testing.T) {
	tests := map[string]func(*Event){
		"start time": func(e *Event) { e.Times.StartTime = &LocalTime{Hour: 10} },
		"note":       func(e *Event) { e.Note = "im Admin geändert" },
		"timetable": func(e *Event) {
			e.Timetable = []TimetableEntry{{ID: "1", Description: "Bieranstich", Date: kirchweihFriday}}
		},
		"location": func(e *Event) { e.LocationID = parkID },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo(storedMarket(t, marketID, marketKey))
			service, _ := newCommitService(events, newFakeLocationRepo(hall(), park()))
			entry := keyedMarketEntry()
			entry["title"] = "Kirchweihmarkt 2026"
			data := marshalImport(t, importFile(entry))
			decisions := previewAndDecide(t, service, data)
			edited := events.events[marketID]
			change(&edited)
			events.events[marketID] = edited

			summary := committedImport(t, service, data, decisions)

			assertOutcomes(t, summary, ImportOutcomeStale)
			if len(events.updated) != 0 || !reflect.DeepEqual(events.events[marketID], edited) {
				t.Errorf("market = %+v, want the edit kept", events.events[marketID])
			}
		})
	}
}

func TestCommitImportReportsAnEntryWithoutTheFingerprintOfItsTargetAsStale(t *testing.T) {
	tests := map[string]map[string]string{
		"missing":   nil,
		"falsified": {marketID: EventFingerprint(Event{})},
	}
	for name, fingerprints := range tests {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo(storedMarket(t, marketID, marketKey))
			service, _ := newCommitService(events, newFakeLocationRepo(hall()))
			entry := keyedMarketEntry()
			entry["note"] = "geändert"
			data := marshalImport(t, importFile(entry))
			decisions := previewAndDecide(t, service, data)
			decisions[0].Fingerprints = fingerprints

			summary := committedImport(t, service, data, decisions)

			assertOutcomes(t, summary, ImportOutcomeStale)
			if len(events.updated) != 0 {
				t.Error("an entry without the fingerprint of its target was written")
			}
		})
	}
}

// storedTwinMarkets returns the stored market and a twin at marketID and
// concertID, both candidates of a market entry without import key.
func storedTwinMarkets(t *testing.T) *fakeEventRepo {
	t.Helper()
	twin := storedMarket(t, concertID, "")
	twin.Note = "Zwilling"
	return newFakeEventRepo(storedMarket(t, marketID, ""), twin)
}

func TestCommitImportReportsAnOverwriteOfAnEventChangedSinceThePreviewAsStale(t *testing.T) {
	events := storedTwinMarkets(t)
	service, _ := newCommitService(events, newFakeLocationRepo(hall()))
	data := suspectFile(t)
	decisions := previewAndDecide(t, service, data)
	decisions[0].Choice, decisions[0].OverwriteID = ImportChoiceOverwrite, marketID
	edited := events.events[marketID]
	edited.Note = "im Admin geändert"
	events.events[marketID] = edited

	summary := committedImport(t, service, data, decisions)

	assertOutcomes(t, summary, ImportOutcomeStale)
	if len(events.updated) != 0 || events.events[marketID].Note != "im Admin geändert" {
		t.Errorf("market = %+v, want the edit kept", events.events[marketID])
	}
}

func TestCommitImportKeepsADecisionWhenACandidateItDoesNotWriteChanged(t *testing.T) {
	tests := map[ImportChoice]ImportOutcome{
		ImportChoiceOverwrite: ImportOutcomeUpdated,
		ImportChoiceCreate:    ImportOutcomeCreated,
		ImportChoiceSkip:      ImportOutcomeSkipped,
	}
	for choice, want := range tests {
		t.Run(string(choice), func(t *testing.T) {
			events := storedTwinMarkets(t)
			service, _ := newCommitService(events, newFakeLocationRepo(hall()))
			data := suspectFile(t)
			decisions := previewAndDecide(t, service, data)
			decisions[0].Choice = choice
			if choice == ImportChoiceOverwrite {
				decisions[0].OverwriteID = marketID
			}
			twin := events.events[concertID]
			twin.Note = "im Admin geändert"
			events.events[concertID] = twin

			summary := committedImport(t, service, data, decisions)

			assertOutcomes(t, summary, want)
			if events.events[concertID].Note != "im Admin geändert" {
				t.Error("the changed candidate was written")
			}
		})
	}
}
