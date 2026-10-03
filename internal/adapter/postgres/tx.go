package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// TxRunner implements core.TxRunner with transactions on the pool.
type TxRunner struct {
	pool *pgxpool.Pool
}

var _ core.TxRunner = (*TxRunner)(nil)

// NewTxRunner returns a transaction runner on pool.
func NewTxRunner(pool *pgxpool.Pool) *TxRunner {
	return &TxRunner{pool: pool}
}

// InTx runs fn with repositories bound to one transaction. It commits when
// fn returns nil and rolls back otherwise, returning fn's error unchanged
// so the core's typed errors survive.
func (r *TxRunner) InTx(ctx context.Context, fn func(core.Repos) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(core.Repos{Events: NewEventRepo(tx), Locations: NewLocationRepo(tx)})
	})
}
