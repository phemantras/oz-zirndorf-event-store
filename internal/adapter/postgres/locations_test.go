package postgres_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// unknownLocationID is a valid UUIDv7 that no test inserts.
const unknownLocationID = "0192f0b1-0000-7000-8000-0000000000ff"

// uuidVersionIndex is the position of the version digit in a UUID string.
const uuidVersionIndex = 14

// migratedLocationRepo returns a repository on a migrated test database with
// an empty locations table.
func migratedLocationRepo(t *testing.T) (*postgres.LocationRepo, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE locations"); err != nil {
		t.Fatalf("truncate locations: %v", err)
	}
	return postgres.NewLocationRepo(pool), pool
}

func hallLocation() core.Location {
	return core.Location{
		Name: "Paul-Metz-Halle", NameKey: "paul-metz-halle",
		Street: "Volkhardtstraße 2", PostalCode: "90513", City: "Zirndorf",
		Latitude: 49.4424, Longitude: 10.9539, Precision: core.PrecisionBuilding,
	}
}

func parkLocation() core.Location {
	return core.Location{
		Name: "Bibertpark", NameKey: "bibertpark",
		Street: "Bibertstraße", PostalCode: "90513", City: "Zirndorf",
		Latitude: 49.44, Longitude: 10.95,
		Precision: core.PrecisionArea, Note: "Zugang über die Brücke",
	}
}

func createLocation(t *testing.T, repo *postgres.LocationRepo, location core.Location) core.Location {
	t.Helper()
	created, err := repo.Create(context.Background(), location)
	if err != nil {
		t.Fatalf("Create %q: %v", location.Name, err)
	}
	return created
}

func TestLocationRepoCreateGeneratesUUIDv7AndRoundTrips(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	ctx := context.Background()

	for _, location := range []core.Location{hallLocation(), parkLocation()} {
		created := createLocation(t, repo, location)
		if len(created.ID) <= uuidVersionIndex || created.ID[uuidVersionIndex] != '7' {
			t.Errorf("id %q is not a UUIDv7", created.ID)
		}
		location.ID = created.ID
		if created != location {
			t.Errorf("Create = %+v, want %+v", created, location)
		}
		got, err := repo.Get(ctx, created.ID)
		if err != nil || got != location {
			t.Errorf("Get = %+v, %v, want %+v", got, err, location)
		}
	}
}

func TestLocationRepoStoresEmptyNoteAsNull(t *testing.T) {
	repo, pool := migratedLocationRepo(t)
	created := createLocation(t, repo, hallLocation())

	var noteIsNull bool
	err := pool.QueryRow(context.Background(), "SELECT note IS NULL FROM locations WHERE id = $1", created.ID).Scan(&noteIsNull)
	if err != nil {
		t.Fatalf("read note: %v", err)
	}
	if !noteIsNull {
		t.Error("empty note was not stored as NULL")
	}
}

func TestLocationRepoListReturnsAllLocations(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	hall := createLocation(t, repo, hallLocation())
	park := createLocation(t, repo, parkLocation())

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	core.SortLocations(got)
	if want := []core.Location{park, hall}; !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestLocationRepoFindByNameKey(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	hall := createLocation(t, repo, hallLocation())
	ctx := context.Background()

	got, err := repo.FindByNameKey(ctx, hall.NameKey)
	if err != nil || got != hall {
		t.Errorf("FindByNameKey = %+v, %v, want %+v", got, err, hall)
	}
	if _, err := repo.FindByNameKey(ctx, "unbekannt"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("FindByNameKey(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestLocationRepoReportsUnknownAndMalformedIDsAsNotFound(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	ctx := context.Background()

	for _, id := range []string{unknownLocationID, "kaputt", ""} {
		if _, err := repo.Get(ctx, id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get(%q) err = %v, want ErrNotFound", id, err)
		}
		location := hallLocation()
		location.ID = id
		if _, err := repo.Update(ctx, location); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Update(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}

func TestLocationRepoReportsDuplicateNameKeyAsConflict(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	createLocation(t, repo, hallLocation())
	park := createLocation(t, repo, parkLocation())
	ctx := context.Background()

	duplicate := parkLocation()
	duplicate.Name, duplicate.NameKey = " paul-metz-halle ", hallLocation().NameKey
	if _, err := repo.Create(ctx, duplicate); !errors.Is(err, core.ErrConflict) {
		t.Errorf("Create duplicate err = %v, want ErrConflict", err)
	}
	duplicate.ID = park.ID
	if _, err := repo.Update(ctx, duplicate); !errors.Is(err, core.ErrConflict) {
		t.Errorf("Update into duplicate err = %v, want ErrConflict", err)
	}
}

func TestLocationRepoUpdateKeepsID(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	hall := createLocation(t, repo, hallLocation())
	ctx := context.Background()

	renamed := hall
	renamed.Name, renamed.NameKey = "Paul-Metz-Halle Zirndorf", "paul-metz-halle zirndorf"
	renamed.Street, renamed.PostalCode, renamed.City = "Volkhardtstraße 2a", "90522", "Oberasbach"
	renamed.Note = "Neu"
	updated, err := repo.Update(ctx, renamed)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated != renamed {
		t.Errorf("Update = %+v, want %+v", updated, renamed)
	}
	got, err := repo.Get(ctx, hall.ID)
	if err != nil || got != renamed {
		t.Errorf("Get after update = %+v, %v, want %+v", got, err, renamed)
	}
}

// insertLegacyLocation inserts a location the way the code before Story 1.12
// wrote it: with the free-text address and without address parts.
func insertLegacyLocation(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO locations (name, name_key, address, latitude, longitude, precision)
		 VALUES ('Alte Feuerwache', 'alte feuerwache', 'Fürther Straße 10, 90513 Zirndorf', 49.44, 10.95, 'building')
		 RETURNING id::text`).Scan(&id)
	if err != nil {
		t.Fatalf("insert legacy location: %v", err)
	}
	return id
}

func TestLocationRepoReadsLegacyLocationWithEmptyAddressParts(t *testing.T) {
	repo, pool := migratedLocationRepo(t)
	id := insertLegacyLocation(t, pool)
	ctx := context.Background()

	want := core.Location{
		ID: id, Name: "Alte Feuerwache", NameKey: "alte feuerwache",
		Latitude: 49.44, Longitude: 10.95, Precision: core.PrecisionBuilding,
	}
	got, err := repo.Get(ctx, id)
	if err != nil || got != want {
		t.Errorf("Get = %+v, %v, want %+v", got, err, want)
	}
	list, err := repo.List(ctx)
	if err != nil || !slices.Equal(list, []core.Location{want}) {
		t.Errorf("List = %v, %v, want %v", list, err, want)
	}
}

func TestLocationRepoUpdateCompletesLegacyLocation(t *testing.T) {
	repo, pool := migratedLocationRepo(t)
	id := insertLegacyLocation(t, pool)
	ctx := context.Background()

	completed, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	completed.Street, completed.PostalCode, completed.City = "Fürther Straße 10", "90513", "Zirndorf"
	updated, err := repo.Update(ctx, completed)
	if err != nil || updated != completed {
		t.Errorf("Update = %+v, %v, want %+v", updated, err, completed)
	}

	var address string
	if err := pool.QueryRow(ctx, "SELECT address FROM locations WHERE id = $1", id).Scan(&address); err != nil {
		t.Fatalf("read address: %v", err)
	}
	if address != "Fürther Straße 10, 90513 Zirndorf" {
		t.Errorf("address = %q, want the legacy address untouched", address)
	}
}

// TestLocationRepoStoresEmptyNonNullAddressForNewLocations guards the
// rollback: the code before Story 1.12 scans address into a string and fails
// on NULL.
func TestLocationRepoStoresEmptyNonNullAddressForNewLocations(t *testing.T) {
	repo, pool := migratedLocationRepo(t)
	created := createLocation(t, repo, hallLocation())

	var address *string
	err := pool.QueryRow(context.Background(), "SELECT address FROM locations WHERE id = $1", created.ID).Scan(&address)
	if err != nil {
		t.Fatalf("read address: %v", err)
	}
	if address == nil || *address != "" {
		t.Errorf("address = %v, want an empty, non-NULL text", address)
	}
}

// TestLocationServiceReportsConflictAgainstDatabase runs the use case against
// PostgreSQL: a name that only collides after NormalizeKey is rejected with
// the existing location.
func TestLocationServiceReportsConflictAgainstDatabase(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	service := core.NewLocationService(repo)
	ctx := context.Background()
	in := core.LocationInput{
		Name: "Paul-Metz-Halle", Street: "Volkhardtstraße 2", PostalCode: "90513", City: "Zirndorf",
		Latitude: "49.4424", Longitude: "10.9539", Precision: string(core.PrecisionBuilding),
	}
	hall, err := service.SaveLocation(ctx, "", in)
	if err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}

	in.Name = " paul-metz-halle "
	_, err = service.SaveLocation(ctx, "", in)

	var conflict *core.LocationConflictError
	if !errors.As(err, &conflict) || conflict.Existing != hall {
		t.Errorf("err = %v, want conflict with %+v", err, hall)
	}
}

func TestLocationRepoPassesDatabaseFailuresOnUntranslated(t *testing.T) {
	pool, err := postgres.Connect(context.Background(), unreachableDatabaseURL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()
	repo := postgres.NewLocationRepo(pool)
	ctx := context.Background()
	update := hallLocation()
	update.ID = unknownLocationID

	calls := map[string]func() error{
		"List":          func() error { _, err := repo.List(ctx); return err },
		"Get":           func() error { _, err := repo.Get(ctx, unknownLocationID); return err },
		"FindByNameKey": func() error { _, err := repo.FindByNameKey(ctx, "x"); return err },
		"Create":        func() error { _, err := repo.Create(ctx, hallLocation()); return err },
		"Update":        func() error { _, err := repo.Update(ctx, update); return err },
	}
	for name, call := range calls {
		err := call()
		if err == nil || errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrConflict) {
			t.Errorf("%s err = %v, want an untranslated database error", name, err)
		}
	}
}
