package core

import (
	"context"
	"errors"
	"fmt"
)

// LocationRepo is the storage port for locations. Only LocationService
// calls Create and Update; adapters never get the repository to write past
// the core (AD-6).
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
}

// LocationService holds the location use cases.
type LocationService struct {
	repo LocationRepo
}

// NewLocationService returns the location use cases backed by repo.
func NewLocationService(repo LocationRepo) *LocationService {
	return &LocationService{repo: repo}
}

// SaveLocation creates a location when id is empty and otherwise updates
// the location with id, keeping its ID. It returns ErrNotFound for an
// unknown id, *ValidationError for invalid input and *LocationConflictError
// when another location already has the same name key.
//
// Without a transaction the name check is not atomic; the unique name_key
// in the database closes that gap, and its ErrConflict is reported the same
// way.
func (s *LocationService) SaveLocation(ctx context.Context, id string, in LocationInput) (Location, error) {
	storedID, err := s.storedID(ctx, id)
	if err != nil {
		return Location{}, err
	}
	location, err := newLocation(in)
	if err != nil {
		return Location{}, err
	}
	if err := s.ensureNameIsFree(ctx, storedID, location.NameKey); err != nil {
		return Location{}, err
	}

	location.ID = storedID
	saved, err := s.write(ctx, location)
	if errors.Is(err, ErrConflict) {
		return Location{}, s.conflictWith(ctx, location.NameKey)
	}
	if err != nil {
		return Location{}, fmt.Errorf("save location: %w", err)
	}
	return saved, nil
}

// storedID returns the ID of the location to update as the repository
// spells it, so an id given in other letter case is not mistaken for another
// location. An empty id stays empty: a new location is created.
func (s *LocationService) storedID(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get location to update: %w", err)
	}
	return current.ID, nil
}

// ensureNameIsFree fails with *LocationConflictError when a location other
// than id already uses nameKey.
func (s *LocationService) ensureNameIsFree(ctx context.Context, id, nameKey string) error {
	existing, err := s.repo.FindByNameKey(ctx, nameKey)
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

// write creates location when it has no ID and updates it otherwise.
func (s *LocationService) write(ctx context.Context, location Location) (Location, error) {
	if location.ID == "" {
		return s.repo.Create(ctx, location)
	}
	return s.repo.Update(ctx, location)
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
