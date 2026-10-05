package core

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
)

// DuplicatePolicy tells SaveEvent what to do with a suspected duplicate
// (AD-11).
type DuplicatePolicy string

// Duplicate policies of SaveEvent. Any value other than AllowDuplicates
// rejects, so a forgotten policy never writes a silent duplicate.
const (
	// RejectDuplicates does not save a suspected duplicate and reports
	// *DuplicateSuspectError with the candidates.
	RejectDuplicates DuplicatePolicy = "rejectDuplicates"
	// AllowDuplicates saves without looking for duplicates, after a person
	// confirmed the warning or an import decided it.
	AllowDuplicates DuplicatePolicy = "allowDuplicates"
)

// DuplicateKey is what two events share when one is suspected to duplicate
// the other: the same title key, start date (without time) and location.
type DuplicateKey struct {
	TitleKey   string
	StartDate  LocalDate
	LocationID string
}

// duplicateKeyOf returns the duplicate key of event.
func duplicateKeyOf(event Event) DuplicateKey {
	return DuplicateKey{TitleKey: event.TitleKey, StartDate: event.Times.StartDate, LocationID: event.LocationID}
}

// DuplicateConfirmationOf returns a fingerprint of the input that forms the
// duplicate key: title key, start date and location ID as entered. A form
// that confirms a duplicate warning sends the fingerprint of the input it
// was shown for, so the confirmation lapses once that input changes (AD-11).
func DuplicateConfirmationOf(in EventInput) string {
	// Quoting separates the parts unambiguously.
	parts := strconv.Quote(NormalizeKey(in.Title)) + strconv.Quote(in.StartDate) + strconv.Quote(in.LocationID)
	sum := sha256.Sum256([]byte(parts))
	return hex.EncodeToString(sum[:])
}

// FindDuplicateCandidates returns the stored events, archived ones
// included, that share the duplicate key of event, earliest start first and
// ties by ID. The event itself, identified by its stored ID, is no
// candidate.
func FindDuplicateCandidates(ctx context.Context, events EventRepo, event Event) ([]Event, error) {
	matches, err := events.FindByDuplicateKey(ctx, duplicateKeyOf(event))
	if err != nil {
		return nil, fmt.Errorf("find duplicate candidates: %w", err)
	}
	candidates := slices.DeleteFunc(matches, func(match Event) bool { return match.ID == event.ID })
	slices.SortFunc(candidates, func(a, b Event) int {
		return cmp.Or(a.Period.Start.Compare(b.Period.Start), cmp.Compare(a.ID, b.ID))
	})
	return candidates, nil
}
