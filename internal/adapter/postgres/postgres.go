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
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
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

// cancelSocketDelay is how long a connection whose context ended waits for
// the database server to cancel the query before it closes the socket.
// The cancel request takes milliseconds; the delay stays far below the
// gap between the request deadline and the write timeout.
const cancelSocketDelay = 2 * time.Second

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Connect creates a connection pool for databaseURL with at most maxConns
// connections. The pool connects lazily, so an unreachable database
// surfaces on first use. A query whose context ends is cancelled on the
// server at once, so it does not hold a backend beyond the deadline.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	config.MaxConns = maxConns
	config.ConnConfig.BuildContextWatcherHandler = cancelQueryOnServer
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	return pool, nil
}

// cancelQueryOnServer has the database server cancel the query of conn as
// soon as its context ends. pgx by default only closes the socket, and the
// server may go on running the query until it next writes to the client.
func cancelQueryOnServer(conn *pgconn.PgConn) ctxwatch.Handler {
	return &pgconn.CancelRequestContextWatcherHandler{Conn: conn, DeadlineDelay: cancelSocketDelay}
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
