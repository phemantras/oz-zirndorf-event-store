package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"

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
