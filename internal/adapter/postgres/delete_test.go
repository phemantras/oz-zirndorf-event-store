package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// archivedEvent starts long before any test runs, so it is archived.
func archivedEvent(t *testing.T, locationID string) core.Event {
	t.Helper()
	return eventWithTimes(t, locationID, core.EventTimes{StartDate: core.LocalDate{Year: 2020, Month: time.May, Day: 1}})
}

func countRows(t *testing.T, fixture eventFixture, query string, args ...any) int {
	t.Helper()
	var count int
	if err := fixture.pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestEventRepoDeleteRemovesTheEventWithItsTimetable(t *testing.T) {
	fixture := newEventFixture(t)
	doomed := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	kept := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	ctx := context.Background()

	if err := fixture.repo.Delete(ctx, doomed.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := fixture.repo.Get(ctx, doomed.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Get after delete err = %v, want ErrNotFound", err)
	}
	if left := countRows(t, fixture, "SELECT count(*) FROM timetable_entries WHERE event_id = $1", doomed.ID); left != 0 {
		t.Errorf("%d timetable entries survived their event", left)
	}
	if got := getEvent(t, fixture.repo, kept.ID); len(got.Timetable) != len(kept.Timetable) {
		t.Errorf("other event has %d entries, want %d", len(got.Timetable), len(kept.Timetable))
	}
}

func TestEventRepoDeleteReportsUnknownAndMalformedIDsAsNotFound(t *testing.T) {
	fixture := newEventFixture(t)

	for _, id := range []string{unknownEventID, "kaputt", ""} {
		if err := fixture.repo.Delete(context.Background(), id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Delete(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}

func TestEventRepoCountByLocationCountsArchivedEventsToo(t *testing.T) {
	fixture := newEventFixture(t)
	park := createLocation(t, fixture.locations, parkLocation())
	createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))
	createEvent(t, fixture.repo, archivedEvent(t, fixture.hall.ID))
	createEvent(t, fixture.repo, archivedEvent(t, fixture.hall.ID))
	ctx := context.Background()

	if count, err := fixture.repo.CountByLocation(ctx, fixture.hall.ID); err != nil || count != 3 {
		t.Errorf("CountByLocation(hall) = %d, %v, want 3", count, err)
	}
	if count, err := fixture.repo.CountByLocation(ctx, park.ID); err != nil || count != 0 {
		t.Errorf("CountByLocation(park) = %d, %v, want 0", count, err)
	}
}

func TestEventRepoCountByLocationRejectsMalformedIDWithoutClaimingNotFound(t *testing.T) {
	fixture := newEventFixture(t)

	if _, err := fixture.repo.CountByLocation(context.Background(), "kaputt"); err == nil || errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want a non-NotFound error", err)
	}
}

func TestLocationRepoDeleteRemovesTheLocation(t *testing.T) {
	repo, _ := migratedLocationRepo(t)
	hall := createLocation(t, repo, hallLocation())
	park := createLocation(t, repo, parkLocation())
	ctx := context.Background()

	if err := repo.Delete(ctx, park.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, park.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Get after delete err = %v, want ErrNotFound", err)
	}
	if _, err := repo.Get(ctx, hall.ID); err != nil {
		t.Errorf("Get(other location) err = %v, want it kept", err)
	}
}

func TestLocationRepoDeleteReportsUnknownAndMalformedIDsAsNotFound(t *testing.T) {
	repo, _ := migratedLocationRepo(t)

	for _, id := range []string{unknownLocationID, "kaputt", ""} {
		if err := repo.Delete(context.Background(), id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Delete(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}

// TestLocationRepoDeleteReportsTheForeignKeyAsConflict covers the race the
// core cannot see: an event added after the count. The foreign key refuses
// the delete, and the repository reports it as core.ErrConflict.
func TestLocationRepoDeleteReportsTheForeignKeyAsConflict(t *testing.T) {
	fixture := newEventFixture(t)
	createEvent(t, fixture.repo, archivedEvent(t, fixture.hall.ID))

	err := fixture.locations.Delete(context.Background(), fixture.hall.ID)

	if !errors.Is(err, core.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
	if _, err := fixture.locations.Get(context.Background(), fixture.hall.ID); err != nil {
		t.Errorf("Get after refused delete err = %v, want the location kept", err)
	}
}

// TestDeleteUseCasesRunAgainstDatabase deletes through the core use cases
// on PostgreSQL: a location in use is refused with its event count until
// its events are gone, and a second delete finds nothing.
func TestDeleteUseCasesRunAgainstDatabase(t *testing.T) {
	fixture := newEventFixture(t)
	tx := postgres.NewTxRunner(fixture.pool)
	events := core.NewEventService(tx, fixture.repo, fixture.locations)
	locations := core.NewLocationService(tx, fixture.locations)
	active := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	archived := createEvent(t, fixture.repo, archivedEvent(t, fixture.hall.ID))
	ctx := context.Background()

	var inUse *core.LocationInUseError
	if err := locations.DeleteLocation(ctx, fixture.hall.ID); !errors.As(err, &inUse) || inUse.EventCount != 2 {
		t.Fatalf("DeleteLocation in use err = %v, want *LocationInUseError with 2 events", err)
	}
	for _, event := range []core.Event{active, archived} {
		if err := events.DeleteEvent(ctx, event.ID); err != nil {
			t.Fatalf("DeleteEvent: %v", err)
		}
		if err := events.DeleteEvent(ctx, event.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second DeleteEvent err = %v, want ErrNotFound", err)
		}
	}
	if err := locations.DeleteLocation(ctx, fixture.hall.ID); err != nil {
		t.Fatalf("DeleteLocation: %v", err)
	}
	if err := locations.DeleteLocation(ctx, fixture.hall.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("second DeleteLocation err = %v, want ErrNotFound", err)
	}
	if left := countRows(t, fixture, "SELECT count(*) FROM timetable_entries"); left != 0 {
		t.Errorf("%d timetable entries survived their events", left)
	}
}

// TestEventRepoReportsAMissingLocationAsConflict covers the reverse race:
// the location of an event is deleted between the core's check and the
// write, so the foreign key refuses the event with core.ErrConflict.
func TestEventRepoReportsAMissingLocationAsConflict(t *testing.T) {
	fixture := newEventFixture(t)

	_, err := fixture.repo.Create(context.Background(), minimalEvent(t, unknownLocationID))

	if !errors.Is(err, core.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

// TestEventRepoUpdateReportsAMissingLocationAsConflict covers the same race
// for an edit: the foreign key refuses the update with core.ErrConflict.
func TestEventRepoUpdateReportsAMissingLocationAsConflict(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	stored, err := fixture.repo.Create(ctx, minimalEvent(t, fixture.hall.ID))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	stored.LocationID = unknownLocationID

	_, err = fixture.repo.Update(ctx, stored)

	if !errors.Is(err, core.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

// locationForeignKey is the constraint that ties events to their location.
const locationForeignKey = "events_location_id_fkey"

// TestDirectSQLDeleteOfAReferencedLocationFailsAtRestrict shows that the
// foreign key itself, not only the core, keeps a location that an event
// refers to: ON DELETE RESTRICT refuses even a raw DELETE.
func TestDirectSQLDeleteOfAReferencedLocationFailsAtRestrict(t *testing.T) {
	fixture := newEventFixture(t)
	createEvent(t, fixture.repo, archivedEvent(t, fixture.hall.ID))

	_, err := fixture.pool.Exec(context.Background(), "DELETE FROM locations WHERE id = $1", fixture.hall.ID)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != restrictViolation || pgErr.ConstraintName != locationForeignKey {
		t.Fatalf("delete err = %v, want a restrict violation (%s) of %s", err, restrictViolation, locationForeignKey)
	}
	if left := countRows(t, fixture, "SELECT count(*) FROM locations WHERE id = $1", fixture.hall.ID); left != 1 {
		t.Errorf("%d locations with the id left, want the location kept", left)
	}
}
