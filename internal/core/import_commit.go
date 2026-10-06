package core

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// ImportChoice is what a person chose for a duplicate suspect.
type ImportChoice string

// Choices for a duplicate suspect. Nothing is chosen by default.
const (
	// ImportChoiceNone leaves the suspect undecided; it is not imported.
	ImportChoiceNone ImportChoice = ""
	// ImportChoiceSkip does not import the entry.
	ImportChoiceSkip ImportChoice = "skip"
	// ImportChoiceCreate imports the entry as a new event.
	ImportChoiceCreate ImportChoice = "create"
	// ImportChoiceOverwrite replaces the stored event the decision names.
	ImportChoiceOverwrite ImportChoice = "overwrite"
)

// ImportDecision is what the import form sends back for one entry: what
// the preview showed and, for a duplicate suspect, what a person chose.
type ImportDecision struct {
	// Position is the entry of the file, counted from 1.
	Position int
	// Class is the class of the entry in the preview.
	Class ImportClass
	// TargetID is the target the preview showed for an update or unchanged
	// entry.
	TargetID string
	// NewLocation is whether the entry brought a new location in the
	// preview.
	NewLocation bool
	// Choice is the decision on a duplicate suspect.
	Choice ImportChoice
	// OverwriteID is the stored event that ImportChoiceOverwrite replaces.
	OverwriteID string
	// CandidateIDs are the stored events the preview showed as candidates
	// of a duplicate suspect.
	CandidateIDs []string
	// Fingerprints are the EventFingerprint values the preview showed for
	// the stored events the entry could write, by event ID: its target and
	// its stored candidates.
	Fingerprints map[string]string
}

// ImportOutcome is what committing an import did with an entry.
type ImportOutcome string

// Outcomes of an entry. An entry has exactly one.
const (
	ImportOutcomeCreated   ImportOutcome = "created"
	ImportOutcomeUpdated   ImportOutcome = "updated"
	ImportOutcomeUnchanged ImportOutcome = "unchanged"
	ImportOutcomeSkipped   ImportOutcome = "skipped"
	// ImportOutcomeUndecided is a duplicate suspect without decision.
	ImportOutcomeUndecided ImportOutcome = "undecided"
	// ImportOutcomeError is an entry with problems, one that shares its
	// target with another entry, or one that would overwrite an event of
	// another import key.
	ImportOutcomeError ImportOutcome = "error"
	// ImportOutcomeStale is an entry whose class, target or new location
	// differs from the preview, whose chosen event to overwrite is no
	// stored candidate any more, that has a stored candidate the preview
	// did not show, or whose event to write changed since the preview.
	ImportOutcomeStale ImportOutcome = "stale"
)

// ImportOutcomes returns every outcome in display order.
func ImportOutcomes() []ImportOutcome {
	return []ImportOutcome{
		ImportOutcomeCreated, ImportOutcomeUpdated, ImportOutcomeUnchanged, ImportOutcomeSkipped,
		ImportOutcomeUndecided, ImportOutcomeError, ImportOutcomeStale,
	}
}

// ImportResult is the outcome of one entry.
type ImportResult struct {
	Position int
	Title    string
	Outcome  ImportOutcome
}

// ImportSummary is what committing an import did: one result per entry in
// file order and how many locations it created.
type ImportSummary struct {
	Results          []ImportResult
	CreatedLocations int
}

// CountOf returns how many entries have outcome.
func (s ImportSummary) CountOf(outcome ImportOutcome) int {
	count := 0
	for _, result := range s.Results {
		if result.Outcome == outcome {
			count++
		}
	}
	return count
}

// CommitImport reads an import file again and, in one transaction, which
// waits for every other write, classifies it again, sorts out what the
// decisions do not allow and writes the rest: the new locations the
// written entries need, once per name, then the events, always allowing
// duplicates. An update or overwrite replaces the target with its
// timetable and clears its archive mark; a given import key is stored, a
// missing one keeps the target's. A file that cannot be read yields
// *ImportFileError; a failed write rolls everything back and yields its
// error. Afterwards the review marks of the written events are cleared.
func (s *ImportService) CommitImport(ctx context.Context, data []byte, decisions []ImportDecision) (ImportSummary, error) {
	rawEntries, err := readImportFile(data)
	if err != nil {
		return ImportSummary{}, err
	}
	var committed importCommit
	err = s.events.tx.InTx(ctx, func(repos Repos) error {
		var err error
		committed, err = commitImport(ctx, repos, rawEntries, decisionsByPosition(decisions))
		return err
	})
	if err != nil {
		return ImportSummary{}, err
	}
	for _, id := range committed.writtenIDs {
		s.events.clearReview(id)
	}
	return committed.summary, nil
}

// importCommit is the result of the transaction-bound part of
// CommitImport.
type importCommit struct {
	summary    ImportSummary
	writtenIDs []string
}

// decisionsByPosition indexes decisions by the position of their entry.
func decisionsByPosition(decisions []ImportDecision) map[int]ImportDecision {
	byPosition := make(map[int]ImportDecision, len(decisions))
	for _, decision := range decisions {
		byPosition[decision.Position] = decision
	}
	return byPosition
}

// commitImport is the transaction-bound part of CommitImport.
func commitImport(ctx context.Context, repos Repos, rawEntries []json.RawMessage, decisions map[int]ImportDecision) (importCommit, error) {
	classified, err := classifyImport(ctx, repos, rawEntries)
	if err != nil {
		return importCommit{}, err
	}
	plans := planImport(classified, decisions)
	writer := newImportWriter(classified)
	var committed importCommit
	for index, entry := range classified.entries {
		plan := plans[index]
		committed.summary.Results = append(committed.summary.Results, ImportResult{Position: entry.Position, Title: entry.Title, Outcome: plan.outcome})
		if !plan.writes() {
			continue
		}
		id, err := writer.write(ctx, repos, index, plan.targetID)
		if err != nil {
			return importCommit{}, err
		}
		committed.writtenIDs = append(committed.writtenIDs, id)
	}
	committed.summary.CreatedLocations = len(writer.createdLocations)
	return committed, nil
}

// importPlan is what committing does with an entry.
type importPlan struct {
	outcome ImportOutcome
	// targetID is the stored event that an updated entry replaces.
	targetID string
}

// writes reports whether the entry is written.
func (p importPlan) writes() bool {
	return p.outcome == ImportOutcomeCreated || p.outcome == ImportOutcomeUpdated
}

// planImport decides the outcome of every entry, in this order: stale,
// error, shared target, then unchanged, skipped and undecided; the rest is
// created or updated.
func planImport(classified classifiedImport, decisions map[int]ImportDecision) []importPlan {
	plans := make([]importPlan, 0, len(classified.entries))
	targets := make(map[string]int)
	for _, entry := range classified.entries {
		decision, given := decisions[entry.Position]
		plan := planEntry(entry, decision, given, classified.storedEvents)
		if plan.targetID != "" {
			targets[plan.targetID]++
		}
		plans = append(plans, plan)
	}
	for index, plan := range plans {
		if targets[plan.targetID] > 1 {
			plans[index] = importPlan{outcome: ImportOutcomeError}
		}
	}
	return plans
}

// planEntry decides the outcome of one entry from its class now and the
// decision on it; given is false when the form sent none.
func planEntry(entry ImportEntry, decision ImportDecision, given bool, storedEvents map[string]Event) importPlan {
	switch {
	case !given || isStale(entry, decision):
		return importPlan{outcome: ImportOutcomeStale}
	case entry.Class == ImportClassError:
		return importPlan{outcome: ImportOutcomeError}
	case entry.Class == ImportClassUpdate:
		return importPlan{outcome: ImportOutcomeUpdated, targetID: entry.TargetID}
	case entry.Class == ImportClassUnchanged:
		return importPlan{outcome: ImportOutcomeUnchanged}
	case entry.Class == ImportClassNew:
		return importPlan{outcome: ImportOutcomeCreated}
	}
	// A duplicate suspect follows the decision.
	switch decision.Choice {
	case ImportChoiceSkip:
		return importPlan{outcome: ImportOutcomeSkipped}
	case ImportChoiceCreate:
		return importPlan{outcome: ImportOutcomeCreated}
	case ImportChoiceOverwrite:
		if hasOtherImportKey(storedEvents[decision.OverwriteID], entry) {
			return importPlan{outcome: ImportOutcomeError}
		}
		return importPlan{outcome: ImportOutcomeUpdated, targetID: decision.OverwriteID}
	default:
		return importPlan{outcome: ImportOutcomeUndecided}
	}
}

// isStale reports whether the entry differs from what the preview showed:
// another class, target or new location, stale candidates of a duplicate
// suspect, or an event to write whose fingerprint the decision does not
// carry, because it changed since the preview or the form sent none.
func isStale(entry ImportEntry, decision ImportDecision) bool {
	if decision.Class != entry.Class || decision.TargetID != entry.TargetID || decision.NewLocation != entry.NewLocation {
		return true
	}
	if entry.Class == ImportClassDuplicateSuspect && hasStaleCandidates(entry, decision) {
		return true
	}
	written := storedEventToWrite(entry, decision)
	return written != "" && decision.Fingerprints[written] != entry.StoredFingerprints()[written]
}

// hasStaleCandidates reports whether a duplicate suspect has a chosen event
// to overwrite that is no stored candidate any more, or a stored candidate
// the preview did not show, such as the event a first commit of the same
// form created.
func hasStaleCandidates(entry ImportEntry, decision ImportDecision) bool {
	storedCandidates := entry.StoredCandidateIDs()
	if decision.Choice == ImportChoiceOverwrite && !slices.Contains(storedCandidates, decision.OverwriteID) {
		return true
	}
	return slices.ContainsFunc(storedCandidates, func(id string) bool {
		return !slices.Contains(decision.CandidateIDs, id)
	})
}

// storedEventToWrite returns the ID of the stored event the entry would
// replace: the target of an update or the event a duplicate suspect
// overwrites; empty when it replaces none.
func storedEventToWrite(entry ImportEntry, decision ImportDecision) string {
	switch {
	case entry.Class == ImportClassUpdate:
		return entry.TargetID
	case entry.Class == ImportClassDuplicateSuspect && decision.Choice == ImportChoiceOverwrite:
		return decision.OverwriteID
	default:
		return ""
	}
}

// hasOtherImportKey reports whether target has an import key and the entry
// gives another one.
func hasOtherImportKey(target Event, entry ImportEntry) bool {
	key := normalizeText(entry.Input.ImportKey)
	return key != "" && target.ImportKey != "" && target.ImportKey != key
}

// importWriter writes the entries of a classified import, creating each
// new location the first time a written entry needs it.
type importWriter struct {
	classified classifiedImport
	// firstLocations holds, by name key, the location of the first valid
	// entry that brings it as new; its details count.
	firstLocations map[string]*LocationInput
	// createdLocations holds the IDs of the created locations by name key.
	createdLocations map[string]string
}

func newImportWriter(classified classifiedImport) *importWriter {
	writer := &importWriter{
		classified:       classified,
		firstLocations:   make(map[string]*LocationInput),
		createdLocations: make(map[string]string),
	}
	for index, entry := range classified.entries {
		nameKey := classified.imported[index].location.NameKey
		if _, seen := writer.firstLocations[nameKey]; entry.NewLocation && !seen {
			writer.firstLocations[nameKey] = entry.Input.Location
		}
	}
	return writer
}

// write stores the entry at index as a new event when targetID is empty
// and otherwise replaces the event with targetID, and returns the ID of
// the written event. A given import key is stored with it.
func (w *importWriter) write(ctx context.Context, repos Repos, index int, targetID string) (string, error) {
	entry := w.classified.entries[index]
	locationID, err := w.locationID(ctx, repos, w.classified.imported[index])
	if err != nil {
		return "", err
	}
	in := entry.Input
	in.Location, in.LocationID = nil, locationID
	saved, err := saveEvent(ctx, repos, targetID, in, AllowDuplicates)
	if err != nil {
		return "", fmt.Errorf("write import entry %d: %w", entry.Position, err)
	}
	if key := normalizeText(entry.Input.ImportKey); key != "" && key != saved.ImportKey {
		if err := repos.Events.SetImportKey(ctx, saved.ID, key); err != nil {
			return "", fmt.Errorf("set import key of entry %d: %w", entry.Position, err)
		}
	}
	return saved.ID, nil
}

// locationID returns the ID of the location of imported, creating a new
// location the first time an entry needs it.
func (w *importWriter) locationID(ctx context.Context, repos Repos, imported importedEvent) (string, error) {
	if !imported.isNewLocation() {
		return imported.location.ID, nil
	}
	nameKey := imported.location.NameKey
	if id, created := w.createdLocations[nameKey]; created {
		return id, nil
	}
	location, err := saveLocation(ctx, repos, "", *w.firstLocations[nameKey])
	if err != nil {
		return "", fmt.Errorf("create import location: %w", err)
	}
	w.createdLocations[nameKey] = location.ID
	return location.ID, nil
}
