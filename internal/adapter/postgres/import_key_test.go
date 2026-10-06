package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// uniqueViolation is the SQLSTATE of a row that breaks a unique index.
const uniqueViolation = "23505"

// marketImportKey is the import key the tests give an event.
const marketImportKey = "kirchweihmarkt-2026"

// setImportKey stores key as the import key of the event with id, as the
// import commit of Story 3.3 will; nil stores NULL.
func setImportKey(ctx context.Context, pool *pgxpool.Pool, id string, key *string) error {
	_, err := pool.Exec(ctx, "UPDATE events SET import_key = $2 WHERE id = $1", id, key)
	return err
}

func importKeyOf(t *testing.T, pool *pgxpool.Pool, id string) *string {
	t.Helper()
	var key *string
	if err := pool.QueryRow(context.Background(), "SELECT import_key FROM events WHERE id = $1", id).Scan(&key); err != nil {
		t.Fatalf("read import key: %v", err)
	}
	return key
}

func TestMigrationAddsAnImportKeyThatIsUniqueWhenSet(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	first := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))
	second := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))
	third := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))
	if key := importKeyOf(t, fixture.pool, first.ID); key != nil {
		t.Errorf("new event has import key %q, want none", *key)
	}

	key := marketImportKey
	if err := setImportKey(ctx, fixture.pool, first.ID, &key); err != nil {
		t.Fatalf("set import key: %v", err)
	}
	err := setImportKey(ctx, fixture.pool, second.ID, &key)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolation {
		t.Errorf("second event with the same import key: err = %v, want unique violation", err)
	}
	for _, id := range []string{second.ID, third.ID} {
		if err := setImportKey(ctx, fixture.pool, id, nil); err != nil {
			t.Errorf("several events without import key: %v", err)
		}
	}
}

func TestEventRepoReadsTheImportKey(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	keyed := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))
	unkeyed := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))
	key := marketImportKey
	if err := setImportKey(ctx, fixture.pool, keyed.ID, &key); err != nil {
		t.Fatalf("set import key: %v", err)
	}
	want := map[string]string{keyed.ID: marketImportKey, unkeyed.ID: ""}

	got, err := fixture.repo.Get(ctx, keyed.ID)
	if err != nil || got.ImportKey != marketImportKey {
		t.Errorf("Get = %q, %v, want import key %q", got.ImportKey, err, marketImportKey)
	}
	listed, err := fixture.repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	overlapping, err := fixture.repo.ListOverlapping(ctx, core.Overlap{})
	if err != nil {
		t.Fatalf("ListOverlapping: %v", err)
	}
	matches, err := fixture.repo.FindByDuplicateKey(ctx, core.DuplicateKey{
		TitleKey: keyed.TitleKey, StartDate: keyed.Times.StartDate, LocationID: keyed.LocationID,
	})
	if err != nil {
		t.Fatalf("FindByDuplicateKey: %v", err)
	}
	for name, events := range map[string][]core.Event{"List": listed, "ListOverlapping": overlapping, "FindByDuplicateKey": matches} {
		for _, event := range events {
			if event.ImportKey != want[event.ID] {
				t.Errorf("%s: import key of %s = %q, want %q", name, event.ID, event.ImportKey, want[event.ID])
			}
		}
	}
}

// TestSavingAnEventInTheAdminKeepsItsImportKey saves a changed event
// through the core: the update writes every column but the import key.
func TestSavingAnEventInTheAdminKeepsItsImportKey(t *testing.T) {
	fixture := newEventFixture(t)
	service := core.NewEventService(postgres.NewTxRunner(fixture.pool), fixture.repo, fixture.locations)
	ctx := context.Background()
	in := core.EventInput{
		Title: "Kirchweihmarkt", Type: string(core.EventTypeMarket), LocationID: fixture.hall.ID,
		StartDate: "2026-10-16", Source: core.EventSource{Description: "Amtsblatt"},
	}
	saved, err := service.SaveEvent(ctx, "", in, core.RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	key := marketImportKey
	if err := setImportKey(ctx, fixture.pool, saved.ID, &key); err != nil {
		t.Fatalf("set import key: %v", err)
	}

	in.Title = "Kirchweih-Markt"
	updated, err := service.SaveEvent(ctx, saved.ID, in, core.AllowDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent update: %v", err)
	}
	if got := importKeyOf(t, fixture.pool, saved.ID); got == nil || *got != marketImportKey {
		t.Errorf("stored import key after admin save = %v, want %q", got, marketImportKey)
	}
	if updated.ImportKey != marketImportKey {
		t.Errorf("returned import key = %q, want %q", updated.ImportKey, marketImportKey)
	}
}

func TestEventRepoSetsTheImportKey(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	keyed := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))
	other := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))

	if err := fixture.repo.SetImportKey(ctx, keyed.ID, marketImportKey); err != nil {
		t.Fatalf("SetImportKey: %v", err)
	}

	if got := importKeyOf(t, fixture.pool, keyed.ID); got == nil || *got != marketImportKey {
		t.Errorf("stored import key = %v, want %q", got, marketImportKey)
	}
	if err := fixture.repo.SetImportKey(ctx, other.ID, marketImportKey); !errors.Is(err, core.ErrConflict) {
		t.Errorf("taken import key: err = %v, want ErrConflict", err)
	}
	for _, id := range []string{unknownEventID, "keine-uuid"} {
		if err := fixture.repo.SetImportKey(ctx, id, "frei"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("SetImportKey(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}
