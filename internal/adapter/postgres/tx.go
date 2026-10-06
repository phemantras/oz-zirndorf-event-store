package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres/db"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// writeLockID is the key of the advisory lock every transaction of
// TxRunner takes first, "oz-event" in ASCII. It differs from the lock goose
// holds while migrating.
const writeLockID int64 = 0x6f7a2d6576656e74

// TxRunner implements core.TxRunner with transactions on the pool.
type TxRunner struct {
	pool *pgxpool.Pool
}

var _ core.TxRunner = (*TxRunner)(nil)

// NewTxRunner returns a transaction runner on pool.
func NewTxRunner(pool *pgxpool.Pool) *TxRunner {
	return &TxRunner{pool: pool}
}

// InTx runs fn with repositories bound to one transaction. The transaction
// first waits for the write lock, so transactions of TxRunner, in this and
// in any other instance, run one after the other and each sees what the
// one before committed. It commits when fn returns nil and rolls back
// otherwise, returning fn's error unchanged so the core's typed errors
// survive.
func (r *TxRunner) InTx(ctx context.Context, fn func(core.Repos) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := db.New(tx).TakeWriteLock(ctx, writeLockID); err != nil {
			return fmt.Errorf("take write lock: %w", err)
		}
		return fn(core.Repos{Events: NewEventRepo(tx), Locations: NewLocationRepo(tx)})
	})
}
