package postgres_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
)

// testDatabaseURLVariable names the environment variable that points the
// integration tests at a real PostgreSQL 18 database.
const testDatabaseURLVariable = "EVENTSTORE_TEST_DATABASE_URL"

// unreachableDatabaseURL points at a port where no server listens, so every
// connection attempt fails fast.
const unreachableDatabaseURL = "postgres://eventstore:eventstore@127.0.0.1:1/eventstore?connect_timeout=1"

const baselineVersion = 1

// ciVariable is set by CI runners; there a missing test database is an error,
// not a reason to skip.
const ciVariable = "CI"

// unparsableDatabaseURLWithPassword carries the password "secret" and fails
// in pgxpool.ParseConfig because of the invalid sslmode.
const unparsableDatabaseURLWithPassword = "postgres://eventstore:secret@localhost:5432/eventstore?sslmode=bogus"

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv(testDatabaseURLVariable)
	if url == "" {
		if os.Getenv(ciVariable) != "" {
			t.Fatalf("%s must be set in CI", testDatabaseURLVariable)
		}
		t.Skipf("%s not set, skipping PostgreSQL integration test", testDatabaseURLVariable)
	}
	return url
}

func TestConnectRejectsMalformedURL(t *testing.T) {
	_, err := postgres.Connect(context.Background(), "postgres://%zz")
	if err == nil {
		t.Fatal("Connect returned no error for a malformed URL")
	}
}

func TestConnectErrorDoesNotLeakPassword(t *testing.T) {
	_, err := postgres.Connect(context.Background(), unparsableDatabaseURLWithPassword)
	if err == nil {
		t.Fatal("Connect returned no error for an invalid sslmode")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q leaks the database password", err)
	}
}

func TestMigrateFailsWhenDatabaseIsUnreachable(t *testing.T) {
	pool, err := postgres.Connect(context.Background(), unreachableDatabaseURL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if _, err := postgres.Migrate(context.Background(), pool); err == nil {
		t.Fatal("Migrate returned no error for an unreachable database")
	}
}

func TestMigrateAppliesBaselineAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if applied != 0 {
		t.Errorf("second Migrate applied %d migrations, want 0", applied)
	}

	var version int64
	err = pool.QueryRow(ctx,
		"SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1",
	).Scan(&version)
	if err != nil {
		t.Fatalf("read goose version: %v", err)
	}
	if version < baselineVersion {
		t.Errorf("goose version = %d, want at least %d", version, baselineVersion)
	}
}

func TestConnectedPoolAnswersPing(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestDatabaseIsPostgres18(t *testing.T) {
	const wantMajorVersion = 18
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	var versionNum int
	if err := pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&versionNum); err != nil {
		t.Fatalf("read server version: %v", err)
	}
	const versionNumDivisor = 10000
	if got := versionNum / versionNumDivisor; got != wantMajorVersion {
		t.Errorf("PostgreSQL major version = %d, want %d", got, wantMajorVersion)
	}
}

// addressPartsVersion is the migration that splits the address (Story 1.12).
const addressPartsVersion = 3

// legacySchema isolates the migration test from the shared test schema, so
// it can start from an empty database.
const legacySchema = "migration_address_parts"

// poolInLegacySchema returns a pool on a freshly created schema that is
// dropped after the test.
func poolInLegacySchema(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	admin, err := postgres.Connect(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(admin.Close)
	dropSchema := func() error {
		_, err := admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+legacySchema+" CASCADE")
		return err
	}
	if err := dropSchema(); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+legacySchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if err := dropSchema(); err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})

	databaseURL, err := url.Parse(testDatabaseURL(t))
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := databaseURL.Query()
	query.Set("search_path", legacySchema)
	databaseURL.RawQuery = query.Encode()
	pool, err := postgres.Connect(ctx, databaseURL.String())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestMigrateKeepsLegacyLocationsWhenSplittingAddress applies the migrations
// up to the free-text address, stores a location as the code before Story
// 1.12 did, and then migrates the rest: the row survives with its address
// and empty address parts.
func TestMigrateKeepsLegacyLocationsWhenSplittingAddress(t *testing.T) {
	ctx := context.Background()
	pool := poolInLegacySchema(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"))
	if err != nil {
		t.Fatalf("create migration provider: %v", err)
	}
	if _, err := provider.UpTo(ctx, addressPartsVersion-1); err != nil {
		t.Fatalf("migrate to version %d: %v", addressPartsVersion-1, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO locations (name, name_key, address, latitude, longitude, precision)
		VALUES ('Alte Feuerwache', 'alte feuerwache', 'Fürther Straße 10, 90513 Zirndorf', 49.44, 10.95, 'building')`)
	if err != nil {
		t.Fatalf("insert legacy location: %v", err)
	}

	if _, err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var address, street, postalCode, city string
	err = pool.QueryRow(ctx, "SELECT address, street, postal_code, city FROM locations WHERE name_key = 'alte feuerwache'").
		Scan(&address, &street, &postalCode, &city)
	if err != nil {
		t.Fatalf("read legacy location: %v", err)
	}
	if address != "Fürther Straße 10, 90513 Zirndorf" || street != "" || postalCode != "" || city != "" {
		t.Errorf("legacy location = %q, %q, %q, %q, want address kept and empty parts", address, street, postalCode, city)
	}
}
