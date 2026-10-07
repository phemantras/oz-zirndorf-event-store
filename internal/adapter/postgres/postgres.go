// Package postgres is the PostgreSQL adapter: connection pool, embedded goose
// migrations and the repository implementations of the core ports, built on
// the sqlc-generated queries in package db.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

const migrationsDir = "migrations"

// maxConns caps the pool regardless of the CPU count and of pool_max_conns
// in the database URL, so a slow database or a flood of requests lets
// requests wait for a connection instead of piling up connections (AD-18).
const maxConns = 10

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Connect creates a connection pool for databaseURL with at most maxConns
// connections. The pool connects lazily, so an unreachable database
// surfaces on first use.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	config.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending embedded migrations and returns how many it
// applied. It holds a PostgreSQL advisory lock while migrating, so a second
// instance starting at the same time waits instead of migrating alongside;
// goose gives up after five minutes, or earlier when ctx ends.
func Migrate(ctx context.Context, pool *pgxpool.Pool) (applied int, err error) {
	migrations, err := fs.Sub(embeddedMigrations, migrationsDir)
	if err != nil {
		return 0, fmt.Errorf("open embedded migrations: %w", err)
	}
	// Closing db releases its connections back to the pool but leaves the
	// pool itself open.
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close migration connection: %w", closeErr))
		}
	}()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return 0, fmt.Errorf("create migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return 0, fmt.Errorf("create migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return len(results), fmt.Errorf("apply migrations: %w", err)
	}
	return len(results), nil
}
