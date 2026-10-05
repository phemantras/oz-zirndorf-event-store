package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres/db"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// SQLSTATEs that translateError reports as core.ErrConflict.
const (
	// uniqueViolation is a unique constraint violation.
	uniqueViolation = "23505"
	// foreignKeyViolation is a foreign key violation.
	foreignKeyViolation = "23503"
	// restrictViolation is what PostgreSQL reports when ON DELETE RESTRICT
	// refuses a delete, such as of a location that events refer to.
	restrictViolation = "23001"
)

// LocationRepo implements core.LocationRepo on the locations table.
type LocationRepo struct {
	queries *db.Queries
}

var _ core.LocationRepo = (*LocationRepo)(nil)

// NewLocationRepo returns a location repository on conn, usually the pool.
func NewLocationRepo(conn db.DBTX) *LocationRepo {
	return &LocationRepo{queries: db.New(conn)}
}

// List returns all locations in no particular order; the core sorts them.
func (r *LocationRepo) List(ctx context.Context) ([]core.Location, error) {
	rows, err := r.queries.ListLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	locations := make([]core.Location, 0, len(rows))
	for _, row := range rows {
		locations = append(locations, locationFromRow(locationRow(row)))
	}
	return locations, nil
}

// Get returns the location with id. An id that is no UUID cannot exist, so
// it yields core.ErrNotFound like a missing row.
func (r *LocationRepo) Get(ctx context.Context, id string) (core.Location, error) {
	uuid, err := parseID(locationKind, id)
	if err != nil {
		return core.Location{}, err
	}
	row, err := r.queries.GetLocation(ctx, uuid)
	if err != nil {
		return core.Location{}, translateError("get location", err)
	}
	return locationFromRow(locationRow(row)), nil
}

// FindByNameKey returns the location with nameKey or core.ErrNotFound.
func (r *LocationRepo) FindByNameKey(ctx context.Context, nameKey string) (core.Location, error) {
	row, err := r.queries.FindLocationByNameKey(ctx, nameKey)
	if err != nil {
		return core.Location{}, translateError("find location by name key", err)
	}
	return locationFromRow(locationRow(row)), nil
}

// Create inserts location; the database generates the UUIDv7 ID. A taken
// name key yields core.ErrConflict.
func (r *LocationRepo) Create(ctx context.Context, location core.Location) (core.Location, error) {
	row, err := r.queries.CreateLocation(ctx, db.CreateLocationParams{
		Name:       location.Name,
		NameKey:    location.NameKey,
		Street:     location.Street,
		PostalCode: location.PostalCode,
		City:       location.City,
		Latitude:   location.Latitude,
		Longitude:  location.Longitude,
		Precision:  string(location.Precision),
		Note:       optionalText(location.Note),
	})
	if err != nil {
		return core.Location{}, translateError("create location", err)
	}
	return locationFromRow(locationRow(row)), nil
}

// Update replaces the location with location.ID. A missing location yields
// core.ErrNotFound, a taken name key core.ErrConflict.
func (r *LocationRepo) Update(ctx context.Context, location core.Location) (core.Location, error) {
	uuid, err := parseID(locationKind, location.ID)
	if err != nil {
		return core.Location{}, err
	}
	row, err := r.queries.UpdateLocation(ctx, db.UpdateLocationParams{
		ID:         uuid,
		Name:       location.Name,
		NameKey:    location.NameKey,
		Street:     location.Street,
		PostalCode: location.PostalCode,
		City:       location.City,
		Latitude:   location.Latitude,
		Longitude:  location.Longitude,
		Precision:  string(location.Precision),
		Note:       optionalText(location.Note),
	})
	if err != nil {
		return core.Location{}, translateError("update location", err)
	}
	return locationFromRow(locationRow(row)), nil
}

// UpdateNameKey replaces only the name key of the location with id. A
// missing or unparsable id yields core.ErrNotFound, a taken name key
// core.ErrConflict.
func (r *LocationRepo) UpdateNameKey(ctx context.Context, id, nameKey string) error {
	uuid, err := parseID(locationKind, id)
	if err != nil {
		return err
	}
	affected, err := r.queries.UpdateLocationNameKey(ctx, db.UpdateLocationNameKeyParams{ID: uuid, NameKey: nameKey})
	if err != nil {
		return translateError("update location name key", err)
	}
	if affected == noRowsAffected {
		return fmt.Errorf("update name key of location %s: %w", id, core.ErrNotFound)
	}
	return nil
}

// Delete removes the location with id. A missing or unparsable id yields
// core.ErrNotFound; a location events still refer to is refused by the
// foreign key, which yields core.ErrConflict.
func (r *LocationRepo) Delete(ctx context.Context, id string) error {
	uuid, err := parseID(locationKind, id)
	if err != nil {
		return err
	}
	affected, err := r.queries.DeleteLocation(ctx, uuid)
	if err != nil {
		return translateError("delete location", err)
	}
	if affected == noRowsAffected {
		return fmt.Errorf("delete location %s: %w", id, core.ErrNotFound)
	}
	return nil
}

// Kinds of IDs, named in the error of parseID.
const (
	locationKind = "location"
	eventKind    = "event"
)

// parseID turns the id of an entity of kind into a UUID; an unparsable id
// is reported as core.ErrNotFound because no row can have it.
func parseID(kind, id string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		return pgtype.UUID{}, fmt.Errorf("%s id %q: %w", kind, id, core.ErrNotFound)
	}
	return uuid, nil
}

// translateError maps a missing row to core.ErrNotFound and a unique or
// foreign key violation to core.ErrConflict, keeping the cause for the log.
func translateError(action string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, core.ErrNotFound)
	}
	if isConflict(err) {
		return fmt.Errorf("%s: %w", action, errors.Join(core.ErrConflict, err))
	}
	return fmt.Errorf("%s: %w", action, err)
}

// isConflict reports whether err is a constraint violation that collides
// with stored rows: a taken unique key or a foreign key still in use.
func isConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case uniqueViolation, foreignKeyViolation, restrictViolation:
		return true
	default:
		return false
	}
}

// optionalText stores an empty note as NULL.
func optionalText(text string) pgtype.Text {
	return pgtype.Text{String: text, Valid: text != ""}
}

// locationRow is the row every location query returns. The queries list
// their columns to leave out the legacy address column (AD-17), so sqlc
// generates one row type per query; they all have the same fields and
// convert to this one.
type locationRow db.GetLocationRow

func locationFromRow(row locationRow) core.Location {
	return core.Location{
		ID:         row.ID.String(),
		Name:       row.Name,
		NameKey:    row.NameKey,
		Street:     row.Street,
		PostalCode: row.PostalCode,
		City:       row.City,
		Latitude:   row.Latitude,
		Longitude:  row.Longitude,
		Precision:  core.LocationPrecision(row.Precision),
		Note:       row.Note.String,
	}
}
