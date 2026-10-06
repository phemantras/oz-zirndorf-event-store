package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// LocationRepo is the storage port for locations. Only LocationService and
// ImportService call Create, Update, UpdateNameKey and Delete; adapters
// never get the repository to write past the core (AD-6).
type LocationRepo interface {
	// List returns all locations in no particular order.
	List(ctx context.Context) ([]Location, error)
	// Get returns the location with id, or ErrNotFound, also for an id that
	// is not a valid UUID.
	Get(ctx context.Context, id string) (Location, error)
	// FindByNameKey returns the location with nameKey, or ErrNotFound.
	FindByNameKey(ctx context.Context, nameKey string) (Location, error)
	// Create stores a new location and returns it with its generated ID. A
	// taken name key yields ErrConflict.
	Create(ctx context.Context, location Location) (Location, error)
	// Update replaces the location with location.ID. A missing location
	// yields ErrNotFound, a taken name key ErrConflict.
	Update(ctx context.Context, location Location) (Location, error)
	// UpdateNameKey replaces only the stored name key of the location with
	// id. A missing location yields ErrNotFound, a taken name key
	// ErrConflict.
	UpdateNameKey(ctx context.Context, id, nameKey string) error
	// Delete removes the location with id. A missing location yields
	// ErrNotFound, a location events still refer to ErrConflict.
	Delete(ctx context.Context, id string) error
}

// LocationListEntry is one location of the admin list with what the list
// shows beside it.
type LocationListEntry struct {
	Location Location
	// NeedsReview marks a location whose name key could not be recomputed
	// at startup because it collides; it stays set until a location of its
	// collision group is saved or deleted.
	NeedsReview bool
}

// LocationRecomputeFailure names the locations whose name keys collide and
// why. They keep their stored name keys.
type LocationRecomputeFailure struct {
	// LocationIDs are the IDs of the collision group, sorted.
	LocationIDs []string
	// Err matches ErrConflict and names the name key.
	Err error
}

// LocationService holds the location use cases. It keeps the collision
// groups of locations that need review in memory only; they are recomputed
// at every start.
type LocationService struct {
	tx   TxRunner
	repo LocationRepo

	mu sync.Mutex
	// inReview maps the ID of every marked location to the sorted IDs of
	// its collision group.
	inReview map[string][]string
}

// NewLocationService returns the location use cases: saving, deleting and
// recomputing run in transactions of tx, reading uses repo directly.
func NewLocationService(tx TxRunner, repo LocationRepo) *LocationService {
	return &LocationService{tx: tx, repo: repo, inReview: map[string][]string{}}
}

// DeleteLocation removes the location with id in one transaction. It
// returns ErrNotFound for an unknown id, such as a location deleted before,
// and *LocationInUseError with their number when events, archived ones
// included, still refer to it. An event added concurrently after the count
// makes the database refuse the delete; that yields a plain ErrConflict.
// A successful delete clears the review mark of the collision group of the
// location.
func (s *LocationService) DeleteLocation(ctx context.Context, id string) error {
	var deletedID string
	err := s.tx.InTx(ctx, func(repos Repos) error {
		var err error
		deletedID, err = deleteLocation(ctx, repos, id)
		return err
	})
	if err != nil {
		return err
	}
	s.clearReview(deletedID)
	return nil
}

// deleteLocation is the transaction-bound part of DeleteLocation. It
// returns the ID of the deleted location as the repository spells it.
func deleteLocation(ctx context.Context, repos Repos, id string) (string, error) {
	current, err := repos.Locations.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get location to delete: %w", err)
	}
	eventCount, err := repos.Events.CountByLocation(ctx, current.ID)
	if err != nil {
		return "", fmt.Errorf("count events of location: %w", err)
	}
	if eventCount > 0 {
		return "", &LocationInUseError{EventCount: eventCount}
	}
	if err := repos.Locations.Delete(ctx, current.ID); err != nil {
		return "", fmt.Errorf("delete location: %w", err)
	}
	return current.ID, nil
}

// SaveLocation creates a location when id is empty and otherwise updates
// the location with id, keeping its ID, in one transaction. It returns
// ErrNotFound for an unknown id, *ValidationError for invalid input and
// *LocationConflictError when another location already has the same name
// key. A successful save clears the review mark of the collision group of
// the location.
func (s *LocationService) SaveLocation(ctx context.Context, id string, in LocationInput) (Location, error) {
	var saved Location
	err := s.tx.InTx(ctx, func(repos Repos) error {
		var err error
		saved, err = saveLocation(ctx, repos, id, in)
		return err
	})
	var taken *nameKeyTakenError
	if errors.As(err, &taken) {
		// The failed write ended the transaction, so the location that holds
		// the name key is loaded after it.
		return Location{}, s.conflictWith(ctx, taken.nameKey)
	}
	if err != nil {
		return Location{}, err
	}
	s.clearReview(saved.ID)
	return saved, nil
}

// nameKeyTakenError reports that the database refused a location because
// another one holds its name key, although the check before found none.
// It matches ErrConflict with errors.Is.
type nameKeyTakenError struct {
	nameKey string
	err     error
}

func (e *nameKeyTakenError) Error() string {
	return fmt.Sprintf("name key %q taken on write: %v", e.nameKey, e.err)
}

func (e *nameKeyTakenError) Unwrap() error { return ErrConflict }

// saveLocation is the transaction-bound part of SaveLocation. A name key
// that the database reports as taken yields *nameKeyTakenError.
func saveLocation(ctx context.Context, repos Repos, id string, in LocationInput) (Location, error) {
	storedID, err := storedLocationIDToUpdate(ctx, repos.Locations, id)
	if err != nil {
		return Location{}, err
	}
	location, err := newLocation(in)
	if err != nil {
		return Location{}, err
	}
	if err := ensureNameIsFree(ctx, repos.Locations, storedID, location.NameKey); err != nil {
		return Location{}, err
	}

	location.ID = storedID
	saved, err := writeLocation(ctx, repos.Locations, location)
	if errors.Is(err, ErrConflict) {
		return Location{}, &nameKeyTakenError{nameKey: location.NameKey, err: err}
	}
	if err != nil {
		return Location{}, fmt.Errorf("save location: %w", err)
	}
	return saved, nil
}

// storedLocationIDToUpdate returns the ID of the location to update as the
// repository spells it, so an id given in other letter case is not mistaken
// for another location. An empty id stays empty: a new location is created.
func storedLocationIDToUpdate(ctx context.Context, locations LocationRepo, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	current, err := locations.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get location to update: %w", err)
	}
	return current.ID, nil
}

// ensureNameIsFree fails with *LocationConflictError when a location other
// than id already uses nameKey.
func ensureNameIsFree(ctx context.Context, locations LocationRepo, id, nameKey string) error {
	existing, err := locations.FindByNameKey(ctx, nameKey)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check location name: %w", err)
	}
	if existing.ID != id {
		return &LocationConflictError{Existing: existing}
	}
	return nil
}

// writeLocation creates location when it has no ID and updates it
// otherwise.
func writeLocation(ctx context.Context, locations LocationRepo, location Location) (Location, error) {
	if location.ID == "" {
		return locations.Create(ctx, location)
	}
	return locations.Update(ctx, location)
}

// conflictWith loads the location that won a concurrent save of nameKey,
// so the conflict names it.
func (s *LocationService) conflictWith(ctx context.Context, nameKey string) error {
	existing, err := s.repo.FindByNameKey(ctx, nameKey)
	if err != nil {
		return fmt.Errorf("load conflicting location: %w", err)
	}
	return &LocationConflictError{Existing: existing}
}

// GetLocation returns the location with id, or ErrNotFound.
func (s *LocationService) GetLocation(ctx context.Context, id string) (Location, error) {
	location, err := s.repo.Get(ctx, id)
	if err != nil {
		return Location{}, fmt.Errorf("get location: %w", err)
	}
	return location, nil
}

// ListLocations returns all locations sorted by SortLocations.
func (s *LocationService) ListLocations(ctx context.Context) ([]Location, error) {
	locations, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	SortLocations(locations)
	return locations, nil
}

// ListLocationEntries returns all locations sorted by SortLocations, each
// with its review mark.
func (s *LocationService) ListLocationEntries(ctx context.Context) ([]LocationListEntry, error) {
	locations, err := s.ListLocations(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]LocationListEntry, 0, len(locations))
	for _, location := range locations {
		entries = append(entries, LocationListEntry{Location: location, NeedsReview: s.needsReview(location.ID)})
	}
	return entries, nil
}

// RecomputeNameKeys recomputes the name key of every location from its name
// and stores it where it changed (AD-16, ENT-17); names stay as they are.
// It reads and writes in one transaction, so it sees what the transactions
// before it committed. Locations whose new keys collide keep their stored
// keys, are marked for review as one group and are returned as one failure
// per group. That holds for collisions found among all locations before
// writing and for a key the database reports as taken, whose holder joins
// the group. A location deleted meanwhile is skipped. Failing to list, look
// up or store, and an ended ctx, are errors: they mark nothing.
func (s *LocationService) RecomputeNameKeys(ctx context.Context) ([]LocationRecomputeFailure, error) {
	var failures []LocationRecomputeFailure
	err := s.tx.InTx(ctx, func(repos Repos) error {
		var err error
		failures, err = recomputeNameKeys(ctx, repos.Locations)
		return err
	})
	if err != nil {
		return nil, err
	}
	for _, failure := range failures {
		s.markForReview(failure.LocationIDs)
	}
	return failures, nil
}

// recomputeNameKeys is the transaction-bound part of RecomputeNameKeys.
func recomputeNameKeys(ctx context.Context, repo LocationRepo) ([]LocationRecomputeFailure, error) {
	locations, err := repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations to recompute: %w", err)
	}
	// Name order makes the writes, and so the outcome of key chains,
	// repeatable.
	SortLocations(locations)
	groups := groupByNewNameKey(locations)

	var failures []LocationRecomputeFailure
	for _, location := range locations {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("recompute name keys: %w", err)
		}
		nameKey := NormalizeKey(location.Name)
		group := groups[nameKey]
		if len(group) > 1 {
			// One failure per group, reported at its first member.
			if group[0].ID == location.ID {
				failures = append(failures, collisionFailure(nameKey, locationIDs(group)...))
			}
			continue
		}
		failure, err := recomputeNameKey(ctx, repo, location, nameKey)
		if err != nil {
			return nil, err
		}
		if failure != nil {
			failures = append(failures, *failure)
		}
	}
	return failures, nil
}

// groupByNewNameKey groups locations by the name key NormalizeKey derives
// from their names, keeping the order of locations within each group.
func groupByNewNameKey(locations []Location) map[string][]Location {
	groups := make(map[string][]Location, len(locations))
	for _, location := range locations {
		nameKey := NormalizeKey(location.Name)
		groups[nameKey] = append(groups[nameKey], location)
	}
	return groups
}

// recomputeNameKey stores nameKey for location if it differs from the
// stored one. It returns a failure when the database reports the key as
// taken, and an error when storing or looking up the holder fails.
func recomputeNameKey(ctx context.Context, repo LocationRepo, location Location, nameKey string) (*LocationRecomputeFailure, error) {
	if location.NameKey == nameKey {
		return nil, nil
	}
	err := repo.UpdateNameKey(ctx, location.ID, nameKey)
	switch {
	case err == nil, errors.Is(err, ErrNotFound):
		return nil, nil
	case errors.Is(err, ErrConflict):
		return storedCollision(ctx, repo, location.ID, nameKey)
	default:
		return nil, fmt.Errorf("store name key of location %s: %w", location.ID, err)
	}
}

// storedCollision returns the failure of the location with id whose new
// nameKey another location still holds, that holder included. A holder gone
// meanwhile leaves the location alone in its group.
func storedCollision(ctx context.Context, repo LocationRepo, id, nameKey string) (*LocationRecomputeFailure, error) {
	holder, err := repo.FindByNameKey(ctx, nameKey)
	if errors.Is(err, ErrNotFound) {
		failure := collisionFailure(nameKey, id)
		return &failure, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find holder of name key %q: %w", nameKey, err)
	}
	failure := collisionFailure(nameKey, id, holder.ID)
	return &failure, nil
}

// collisionFailure returns the failure of the locations with ids that
// collide on nameKey, their IDs sorted.
func collisionFailure(nameKey string, ids ...string) LocationRecomputeFailure {
	sorted := slices.Sorted(slices.Values(ids))
	return LocationRecomputeFailure{
		LocationIDs: sorted,
		Err:         fmt.Errorf("locations %v collide on name key %q: %w", sorted, nameKey, ErrConflict),
	}
}

func locationIDs(locations []Location) []string {
	ids := make([]string, 0, len(locations))
	for _, location := range locations {
		ids = append(ids, location.ID)
	}
	return ids
}

// markForReview marks the locations with ids as one collision group. A
// group that shares a location with an already marked one merges with it,
// so clearing any member clears all.
func (s *LocationService) markForReview(ids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	merged := slices.Clone(ids)
	for _, id := range ids {
		merged = append(merged, s.inReview[id]...)
	}
	slices.Sort(merged)
	merged = slices.Compact(merged)
	for _, id := range merged {
		s.inReview[id] = merged
	}
}

// clearReview removes the review mark of the location with id and of every
// other location in its collision group.
func (s *LocationService) clearReview(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, member := range s.inReview[id] {
		delete(s.inReview, member)
	}
}

func (s *LocationService) needsReview(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.inReview[id]
	return ok
}
