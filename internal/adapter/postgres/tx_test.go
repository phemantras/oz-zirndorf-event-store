package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// waitingForAdvisoryLock counts the sessions that wait for an advisory
// lock, as a transaction behind the write lock does.
const waitingForAdvisoryLock = "SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted"

// writeLockHolder is a transaction of TxRunner that holds the write lock
// until release is called.
type writeLockHolder struct {
	release func()
	done    chan error
}

// holdWriteLock starts a transaction of TxRunner on pool that runs fn and
// then holds the write lock until release; it returns once fn has run.
func holdWriteLock(t *testing.T, pool *pgxpool.Pool, fn func(core.Repos) error) writeLockHolder {
	t.Helper()
	inside := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	holder := writeLockHolder{release: func() { once.Do(func() { close(release) }) }, done: make(chan error, 1)}
	// A failing test must not leave the transaction open, or closing the
	// pool would wait for it forever.
	t.Cleanup(holder.release)
	go func() {
		holder.done <- postgres.NewTxRunner(pool).InTx(context.Background(), func(repos core.Repos) error {
			err := fn(repos)
			close(inside)
			<-release
			return err
		})
	}()
	select {
	case <-inside:
	case err := <-holder.done:
		t.Fatalf("holding transaction ended early: %v", err)
	}
	return holder
}

// finish releases the lock and fails the test when the transaction failed.
func (h writeLockHolder) finish(t *testing.T) {
	t.Helper()
	h.release()
	if err := <-h.done; err != nil {
		t.Fatalf("holding transaction: %v", err)
	}
}

// awaitLockWaiter returns once a session waits for an advisory lock and
// fails the test when none does in time.
func awaitLockWaiter(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(lockWaitTimeout)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(), waitingForAdvisoryLock).Scan(&waiting); err != nil {
			t.Fatalf("count lock waiters: %v", err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(lockWaitPollInterval)
	}
	t.Fatal("no transaction waits for the write lock")
}

// runInBackground runs fn in a goroutine and returns its error channel.
func runInBackground(fn func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	return done
}

func TestTxRunnerLetsASecondTransactionWaitForTheFirst(t *testing.T) {
	repo, pool := migratedLocationRepo(t)
	ctx := context.Background()
	holder := holdWriteLock(t, pool, func(repos core.Repos) error {
		_, err := repos.Locations.Create(ctx, hallLocation())
		return err
	})
	var seen []core.Location

	second := runInBackground(func() error {
		return postgres.NewTxRunner(pool).InTx(ctx, func(repos core.Repos) error {
			var err error
			seen, err = repos.Locations.List(ctx)
			return err
		})
	})
	awaitLockWaiter(t, pool)
	select {
	case err := <-second:
		t.Fatalf("second transaction ended while the first held the lock: %v", err)
	default:
	}
	holder.finish(t)

	if err := <-second; err != nil {
		t.Fatalf("second InTx: %v", err)
	}
	if len(seen) != 1 || seen[0].Name != hallLocation().Name {
		t.Errorf("second transaction saw %+v, want the location of the first", seen)
	}
	if stored, err := repo.List(ctx); err != nil || len(stored) != 1 {
		t.Errorf("stored = %+v, %v, want one location", stored, err)
	}
}

// assertStillWaiting fails the test when done already has a result.
func assertStillWaiting(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("finished while another transaction held the write lock: %v", err)
	default:
	}
}

// TestSaveEventWaitsForAnImportAndSeesItsEvent saves in the event form
// while another transaction, as an import does, holds the write lock and
// has written the same event: the save waits and then finds the duplicate.
func TestSaveEventWaitsForAnImportAndSeesItsEvent(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	holder := holdWriteLock(t, fixture.pool, func(repos core.Repos) error {
		_, err := repos.Events.Create(ctx, minimalEvent(t, fixture.hall.ID))
		return err
	})
	service := core.NewEventService(postgres.NewTxRunner(fixture.pool), fixture.repo, fixture.locations)
	in := core.EventInput{
		Title: "Kirchweihmarkt", Type: string(core.EventTypeMarket), LocationID: fixture.hall.ID,
		StartDate: "2026-12-04", Source: core.EventSource{Description: "Amtsblatt"},
	}

	saved := runInBackground(func() error {
		_, err := service.SaveEvent(ctx, "", in, core.RejectDuplicates)
		return err
	})
	awaitLockWaiter(t, fixture.pool)
	assertStillWaiting(t, saved)
	holder.finish(t)

	if err := <-saved; !errors.Is(err, core.ErrDuplicateSuspect) {
		t.Errorf("SaveEvent err = %v, want a duplicate suspect of the imported event", err)
	}
}

// TestRecomputeDerivedWaitsForTheWriteLockAndReadsAfterIt recomputes while
// another transaction holds the write lock and has written an event with a
// stale title key: the recomputation reads that event and fixes its key.
func TestRecomputeDerivedWaitsForTheWriteLockAndReadsAfterIt(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	var created core.Event
	holder := holdWriteLock(t, fixture.pool, func(repos core.Repos) error {
		stale := minimalEvent(t, fixture.hall.ID)
		stale.TitleKey = "veraltet"
		var err error
		created, err = repos.Events.Create(ctx, stale)
		return err
	})
	service := core.NewEventService(postgres.NewTxRunner(fixture.pool), fixture.repo, fixture.locations)

	recomputed := runInBackground(func() error {
		_, err := service.RecomputeDerived(ctx)
		return err
	})
	awaitLockWaiter(t, fixture.pool)
	assertStillWaiting(t, recomputed)
	holder.finish(t)

	if err := <-recomputed; err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}
	if got := getEvent(t, fixture.repo, created.ID).TitleKey; got != "kirchweihmarkt" {
		t.Errorf("title key = %q, want kirchweihmarkt", got)
	}
}

// TestRecomputeNameKeysWaitsForTheWriteLockAndReadsAfterIt recomputes
// while another transaction holds the write lock and has written a location
// with a stale name key: the recomputation reads it and fixes the key.
func TestRecomputeNameKeysWaitsForTheWriteLockAndReadsAfterIt(t *testing.T) {
	repo, pool := migratedLocationRepo(t)
	ctx := context.Background()
	var created core.Location
	holder := holdWriteLock(t, pool, func(repos core.Repos) error {
		stale := hallLocation()
		stale.NameKey = "veraltet"
		var err error
		created, err = repos.Locations.Create(ctx, stale)
		return err
	})
	service := core.NewLocationService(postgres.NewTxRunner(pool), repo)

	recomputed := runInBackground(func() error {
		_, err := service.RecomputeNameKeys(ctx)
		return err
	})
	awaitLockWaiter(t, pool)
	assertStillWaiting(t, recomputed)
	holder.finish(t)

	if err := <-recomputed; err != nil {
		t.Fatalf("RecomputeNameKeys: %v", err)
	}
	if stored, err := repo.Get(ctx, created.ID); err != nil || stored.NameKey != hallLocation().NameKey {
		t.Errorf("location = %+v, %v, want name key %s", stored, err, hallLocation().NameKey)
	}
}
