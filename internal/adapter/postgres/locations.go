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

// uniqueViolation is the SQLSTATE of a unique constraint violation.
const uniqueViolation = "23505"

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
		locations = append(locations, locationFromRow(row))
	}
	return locations, nil
}

// Get returns the location with id. An id that is no UUID cannot exist, so
// it yields core.ErrNotFound like a missing row.
func (r *LocationRepo) Get(ctx context.Context, id string) (core.Location, error) {
	uuid, err := parseID(id)
	if err != nil {
		return core.Location{}, err
	}
	row, err := r.queries.GetLocation(ctx, uuid)
	if err != nil {
		return core.Location{}, translateError("get location", err)
	}
	return locationFromRow(row), nil
}

// FindByNameKey returns the location with nameKey or core.ErrNotFound.
func (r *LocationRepo) FindByNameKey(ctx context.Context, nameKey string) (core.Location, error) {
	row, err := r.queries.FindLocationByNameKey(ctx, nameKey)
	if err != nil {
		return core.Location{}, translateError("find location by name key", err)
	}
	return locationFromRow(row), nil
}

// Create inserts location; the database generates the UUIDv7 ID. A taken
// name key yields core.ErrConflict.
func (r *LocationRepo) Create(ctx context.Context, location core.Location) (core.Location, error) {
	row, err := r.queries.CreateLocation(ctx, db.CreateLocationParams{
		Name:      location.Name,
		NameKey:   location.NameKey,
		Address:   location.Address,
		Latitude:  location.Latitude,
		Longitude: location.Longitude,
		Precision: string(location.Precision),
		Note:      optionalText(location.Note),
	})
	if err != nil {
		return core.Location{}, translateError("create location", err)
	}
	return locationFromRow(row), nil
}

// Update replaces the location with location.ID. A missing location yields
// core.ErrNotFound, a taken name key core.ErrConflict.
func (r *LocationRepo) Update(ctx context.Context, location core.Location) (core.Location, error) {
	uuid, err := parseID(location.ID)
	if err != nil {
		return core.Location{}, err
	}
	row, err := r.queries.UpdateLocation(ctx, db.UpdateLocationParams{
		ID:        uuid,
		Name:      location.Name,
		NameKey:   location.NameKey,
		Address:   location.Address,
		Latitude:  location.Latitude,
		Longitude: location.Longitude,
		Precision: string(location.Precision),
		Note:      optionalText(location.Note),
	})
	if err != nil {
		return core.Location{}, translateError("update location", err)
	}
	return locationFromRow(row), nil
}

// parseID turns id into a UUID; an unparsable id is reported as
// core.ErrNotFound because no row can have it.
func parseID(id string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(id); err != nil {
		return pgtype.UUID{}, fmt.Errorf("location id %q: %w", id, core.ErrNotFound)
	}
	return uuid, nil
}

// translateError maps a missing row to core.ErrNotFound and a unique
// violation to core.ErrConflict, keeping the cause for the log.
func translateError(action string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", action, core.ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return fmt.Errorf("%s: %w", action, errors.Join(core.ErrConflict, err))
	}
	return fmt.Errorf("%s: %w", action, err)
}

// optionalText stores an empty note as NULL.
func optionalText(text string) pgtype.Text {
	return pgtype.Text{String: text, Valid: text != ""}
}

func locationFromRow(row db.Location) core.Location {
	return core.Location{
		ID:        row.ID.String(),
		Name:      row.Name,
		NameKey:   row.NameKey,
		Address:   row.Address,
		Latitude:  row.Latitude,
		Longitude: row.Longitude,
		Precision: core.LocationPrecision(row.Precision),
		Note:      row.Note.String,
	}
}
