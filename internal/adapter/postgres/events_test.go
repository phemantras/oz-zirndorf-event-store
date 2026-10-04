package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// unknownEventID is a valid UUIDv7 that no test inserts.
const unknownEventID = "0192f0b1-0000-7000-8000-0000000001ff"

// restrictViolation is the SQLSTATE of a delete that ON DELETE RESTRICT
// prevents.
const restrictViolation = "23001"

// eventFixture is a migrated database with empty tables, one stored
// location and the event repository.
type eventFixture struct {
	repo      *postgres.EventRepo
	locations *postgres.LocationRepo
	pool      *pgxpool.Pool
	hall      core.Location
}

func newEventFixture(t *testing.T) eventFixture {
	t.Helper()
	locations, pool := migratedLocationRepo(t)
	return eventFixture{
		repo:      postgres.NewEventRepo(pool),
		locations: locations,
		pool:      pool,
		hall:      createLocation(t, locations, hallLocation()),
	}
}

func localTimeAt(hour, minute int) *core.LocalTime {
	return &core.LocalTime{Hour: hour, Minute: minute}
}

// eventWithTimes returns an event at locationID with the period the core
// derives from times.
func eventWithTimes(t *testing.T, locationID string, times core.EventTimes) core.Event {
	t.Helper()
	period, err := times.EffectivePeriod()
	if err != nil {
		t.Fatalf("EffectivePeriod: %v", err)
	}
	return core.Event{
		Title: "Kirchweihmarkt", TitleKey: "kirchweihmarkt", Type: core.EventTypeMarket, LocationID: locationID,
		Times:  times,
		Source: core.EventSource{Description: "Amtsblatt"},
		Period: period,
	}
}

// fullEvent uses every column, including times, link and note.
func fullEvent(t *testing.T, locationID string) core.Event {
	t.Helper()
	event := eventWithTimes(t, locationID, core.EventTimes{
		StartDate: core.LocalDate{Year: 2026, Month: time.October, Day: 16}, StartTime: localTimeAt(19, 30),
		EndDate: core.LocalDate{Year: 2026, Month: time.October, Day: 19}, EndTime: localTimeAt(23, 59),
	})
	event.Type = core.EventTypeFestival
	event.Source.URL = "https://www.zirndorf.de/amtsblatt"
	event.Note = "Mit Fahrgeschäften"
	return event
}

// minimalEvent has only a start date; the other columns are NULL.
func minimalEvent(t *testing.T, locationID string) core.Event {
	t.Helper()
	return eventWithTimes(t, locationID, core.EventTimes{StartDate: core.LocalDate{Year: 2026, Month: time.December, Day: 4}})
}

func createEvent(t *testing.T, repo *postgres.EventRepo, event core.Event) core.Event {
	t.Helper()
	created, err := repo.Create(context.Background(), event)
	if err != nil {
		t.Fatalf("Create %q: %v", event.Title, err)
	}
	return created
}

// assertSameEvent compares events by value: times behind pointers and
// instants with Equal, since PostgreSQL returns them in another zone.
func assertSameEvent(t *testing.T, got, want core.Event) {
	t.Helper()
	if !got.Period.Start.Equal(want.Period.Start) || !got.Period.End.Equal(want.Period.End) {
		t.Errorf("period = [%v, %v), want [%v, %v)", got.Period.Start, got.Period.End, want.Period.Start, want.Period.End)
	}
	if gotInput, wantInput := core.EventInputOf(got), core.EventInputOf(want); !reflect.DeepEqual(gotInput, wantInput) || got.ID != want.ID {
		t.Errorf("event = %s %+v, want %s %+v", got.ID, gotInput, want.ID, wantInput)
	}
	if got.TitleKey != want.TitleKey {
		t.Errorf("title key = %q, want %q", got.TitleKey, want.TitleKey)
	}
}

func TestEventRepoCreateGeneratesUUIDv7AndRoundTrips(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()

	for _, event := range []core.Event{fullEvent(t, fixture.hall.ID), minimalEvent(t, fixture.hall.ID)} {
		created := createEvent(t, fixture.repo, event)
		if len(created.ID) <= uuidVersionIndex || created.ID[uuidVersionIndex] != '7' {
			t.Errorf("id %q is not a UUIDv7", created.ID)
		}
		event.ID = created.ID
		assertSameEvent(t, created, event)
		got, err := fixture.repo.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		assertSameEvent(t, got, event)
	}
}

func TestEventRepoStoresUnknownTimesAndEmptyTextsAsNull(t *testing.T) {
	fixture := newEventFixture(t)
	created := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))

	var allNull bool
	err := fixture.pool.QueryRow(context.Background(),
		`SELECT start_time IS NULL AND end_date IS NULL AND end_time IS NULL AND source_url IS NULL AND note IS NULL
		 FROM events WHERE id = $1`, created.ID).Scan(&allNull)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if !allNull {
		t.Error("unknown times or empty texts were not stored as NULL")
	}
}

func TestEventRepoStoresMidnightAsKnownTime(t *testing.T) {
	fixture := newEventFixture(t)
	event := minimalEvent(t, fixture.hall.ID)
	event.Times.StartTime = localTimeAt(0, 0)

	created := createEvent(t, fixture.repo, event)

	if created.Times.StartTime == nil || *created.Times.StartTime != (core.LocalTime{}) {
		t.Errorf("start time = %v, want 00:00", created.Times.StartTime)
	}
}

func TestEventRepoListReturnsAllEvents(t *testing.T) {
	fixture := newEventFixture(t)
	full := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))
	minimal := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))

	got, err := fixture.repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d events, want 2", len(got))
	}
	byID := map[string]core.Event{got[0].ID: got[0], got[1].ID: got[1]}
	assertSameEvent(t, byID[full.ID], full)
	assertSameEvent(t, byID[minimal.ID], minimal)
}

// overlapIDs asks ListOverlapping and returns the IDs it found with the
// number of timetable entries of each.
func overlapIDs(t *testing.T, repo *postgres.EventRepo, overlap core.Overlap) map[string]int {
	t.Helper()
	events, err := repo.ListOverlapping(context.Background(), overlap)
	if err != nil {
		t.Fatalf("ListOverlapping: %v", err)
	}
	found := map[string]int{}
	for _, event := range events {
		found[event.ID] = len(event.Timetable)
	}
	return found
}

func TestEventRepoListOverlappingAppliesThePredicateAtBothBounds(t *testing.T) {
	fixture := newEventFixture(t)
	day := core.LocalDate{Year: 2026, Month: time.December, Day: 24}
	morning := eventWithTimes(t, fixture.hall.ID, core.EventTimes{
		StartDate: day, StartTime: localTimeAt(10, 0), EndDate: day, EndTime: localTimeAt(12, 0),
	})
	morning.Timetable = []core.TimetableEntry{
		{Description: "Begrüßung", Date: day, StartTime: localTimeAt(10, 0)},
		{Description: "Segen", Date: day, StartTime: localTimeAt(11, 30)},
	}
	stored := createEvent(t, fixture.repo, morning)
	later := eventWithTimes(t, fixture.hall.ID, core.EventTimes{StartDate: core.LocalDate{Year: 2026, Month: time.December, Day: 31}})
	later.Timetable = []core.TimetableEntry{{Description: "Countdown", Date: later.Times.StartDate}}
	other := createEvent(t, fixture.repo, later)
	start, end := stored.Period.Start, stored.Period.End

	tests := []struct {
		name    string
		overlap core.Overlap
		want    map[string]int
	}{
		{"effective start equal to hi is not included", core.Overlap{Lo: start.Add(-time.Hour), Hi: &start}, map[string]int{}},
		{"effective start just before hi is included", core.Overlap{Lo: start.Add(-time.Hour), Hi: ptrTo(start.Add(time.Minute))},
			map[string]int{stored.ID: 2}},
		{"effective end equal to lo is not included", core.Overlap{Lo: end, Hi: ptrTo(end.Add(time.Hour))}, map[string]int{}},
		{"effective end just after lo is included", core.Overlap{Lo: end.Add(-time.Minute), Hi: ptrTo(end.Add(time.Hour))},
			map[string]int{stored.ID: 2}},
		{"open hi includes every later event", core.Overlap{Lo: end.Add(-time.Minute)}, map[string]int{stored.ID: 2, other.ID: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := overlapIDs(t, fixture.repo, test.overlap); !reflect.DeepEqual(got, test.want) {
				t.Errorf("found = %v, want %v", got, test.want)
			}
		})
	}
}

func TestEventRepoListOverlappingReturnsCompleteEvents(t *testing.T) {
	fixture := newEventFixture(t)
	full := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))

	got, err := fixture.repo.ListOverlapping(context.Background(), core.Overlap{Lo: full.Period.Start})
	if err != nil {
		t.Fatalf("ListOverlapping: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListOverlapping returned %d events, want 1", len(got))
	}
	assertSameEvent(t, got[0], full)
}

func ptrTo(instant time.Time) *time.Time { return &instant }

func TestEventRepoUpdateKeepsIDAndReplacesEveryColumn(t *testing.T) {
	fixture := newEventFixture(t)
	park := createLocation(t, fixture.locations, parkLocation())
	created := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))
	ctx := context.Background()

	changed := minimalEvent(t, park.ID)
	changed.ID = created.ID
	changed.Title, changed.TitleKey = "Weihnachtsmarkt", "weihnachtsmarkt"
	updated, err := fixture.repo.Update(ctx, changed)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertSameEvent(t, updated, changed)
	got, err := fixture.repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertSameEvent(t, got, changed)
}

func TestEventRepoUpdateDerivedChangesOnlyPeriodAndTitleKey(t *testing.T) {
	fixture := newEventFixture(t)
	created := createEvent(t, fixture.repo, fullEvent(t, fixture.hall.ID))
	ctx := context.Background()

	derived := core.Derived{
		Period:   core.Period{Start: created.Period.Start.Add(-time.Hour), End: created.Period.End.Add(time.Hour)},
		TitleKey: "neuer schlüssel",
	}
	if err := fixture.repo.UpdateDerived(ctx, created.ID, derived); err != nil {
		t.Fatalf("UpdateDerived: %v", err)
	}
	got, err := fixture.repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	want := created
	want.Period, want.TitleKey = derived.Period, derived.TitleKey
	assertSameEvent(t, got, want)
}

func TestEventRepoFindByDuplicateKeyMatchesAllThreeKeys(t *testing.T) {
	fixture := newEventFixture(t)
	park := createLocation(t, fixture.locations, parkLocation())
	ctx := context.Background()
	// A past Friday: the match is archived, and the query still finds it.
	friday := core.LocalDate{Year: 2025, Month: time.October, Day: 17}
	morning := eventWithTimes(t, fixture.hall.ID, core.EventTimes{StartDate: friday, StartTime: localTimeAt(9, 0)})
	match := createEvent(t, fixture.repo, morning)
	createEvent(t, fixture.repo, eventWithTimes(t, fixture.hall.ID, core.EventTimes{StartDate: friday.NextDay()}))
	createEvent(t, fixture.repo, eventWithTimes(t, park.ID, core.EventTimes{StartDate: friday}))
	otherTitle := eventWithTimes(t, fixture.hall.ID, core.EventTimes{StartDate: friday})
	otherTitle.Title, otherTitle.TitleKey = "Flohmarkt", "flohmarkt"
	createEvent(t, fixture.repo, otherTitle)

	found, err := fixture.repo.FindByDuplicateKey(ctx, core.DuplicateKey{TitleKey: "kirchweihmarkt", StartDate: friday, LocationID: fixture.hall.ID})

	if err != nil || len(found) != 1 {
		t.Fatalf("FindByDuplicateKey = %+v, %v, want exactly the match", found, err)
	}
	assertSameEvent(t, found[0], match)
}

func TestEventRepoFindByDuplicateKeyRejectsMalformedLocationIDWithoutClaimingNotFound(t *testing.T) {
	fixture := newEventFixture(t)

	_, err := fixture.repo.FindByDuplicateKey(context.Background(), core.DuplicateKey{TitleKey: "kirchweihmarkt", LocationID: "kaputt"})

	if err == nil || errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want a non-NotFound error", err)
	}
}

func TestEventRepoReportsUnknownAndMalformedIDsAsNotFound(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()

	for _, id := range []string{unknownEventID, "kaputt", ""} {
		if _, err := fixture.repo.Get(ctx, id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get(%q) err = %v, want ErrNotFound", id, err)
		}
		event := minimalEvent(t, fixture.hall.ID)
		event.ID = id
		if _, err := fixture.repo.Update(ctx, event); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Update(%q) err = %v, want ErrNotFound", id, err)
		}
		if err := fixture.repo.UpdateDerived(ctx, id, core.Derived{Period: event.Period}); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("UpdateDerived(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}

func TestEventRepoRejectsMalformedLocationIDWithoutClaimingNotFound(t *testing.T) {
	fixture := newEventFixture(t)
	created := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))
	ctx := context.Background()

	broken := minimalEvent(t, "kaputt")
	if _, err := fixture.repo.Create(ctx, broken); err == nil || errors.Is(err, core.ErrNotFound) {
		t.Errorf("Create err = %v, want a non-NotFound error", err)
	}
	broken.ID = created.ID
	if _, err := fixture.repo.Update(ctx, broken); err == nil || errors.Is(err, core.ErrNotFound) {
		t.Errorf("Update err = %v, want a non-NotFound error", err)
	}
}

func TestLocationWithEventCannotBeDeletedBySQL(t *testing.T) {
	fixture := newEventFixture(t)
	createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))

	_, err := fixture.pool.Exec(context.Background(), "DELETE FROM locations WHERE id = $1", fixture.hall.ID)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != restrictViolation {
		t.Errorf("delete err = %v, want a restrict violation of the foreign key", err)
	}
}

func TestEventRepoPassesDatabaseFailuresOnUntranslated(t *testing.T) {
	pool, err := postgres.Connect(context.Background(), unreachableDatabaseURL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()
	repo := postgres.NewEventRepo(pool)
	ctx := context.Background()
	event := minimalEvent(t, unknownLocationID)
	update := event
	update.ID = unknownEventID

	calls := map[string]func() error{
		"List": func() error { _, err := repo.List(ctx); return err },
		"ListOverlapping": func() error {
			_, err := repo.ListOverlapping(ctx, core.Overlap{Lo: event.Period.Start})
			return err
		},
		"Get":           func() error { _, err := repo.Get(ctx, unknownEventID); return err },
		"Create":        func() error { _, err := repo.Create(ctx, event); return err },
		"Update":        func() error { _, err := repo.Update(ctx, update); return err },
		"UpdateDerived": func() error { return repo.UpdateDerived(ctx, unknownEventID, core.Derived{Period: event.Period}) },
		"Delete":        func() error { return repo.Delete(ctx, unknownEventID) },
		"CountByLocation": func() error {
			_, err := repo.CountByLocation(ctx, unknownLocationID)
			return err
		},
		"FindByDuplicateKey": func() error {
			_, err := repo.FindByDuplicateKey(ctx, core.DuplicateKey{LocationID: unknownLocationID})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		if err == nil || errors.Is(err, core.ErrNotFound) {
			t.Errorf("%s err = %v, want an untranslated database error", name, err)
		}
	}
}

// TestEventServiceRunsAgainstDatabase saves, lists and recomputes through
// the core use cases on PostgreSQL.
func TestEventServiceRunsAgainstDatabase(t *testing.T) {
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
	stale := saved.Period.End.Add(time.Hour)
	if _, err := fixture.pool.Exec(ctx, "UPDATE events SET effective_end = $2, title_key = '' WHERE id = $1", saved.ID, stale); err != nil {
		t.Fatalf("make derived values stale: %v", err)
	}

	failures, err := service.RecomputeDerived(ctx)
	if err != nil || failures != nil {
		t.Fatalf("RecomputeDerived = %v, %v, want no failures", failures, err)
	}
	clock := fixedClock(saved.Period.End.Add(-time.Minute))
	entries, err := service.ListEvents(ctx, clock)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(entries) != 1 || entries[0].Archived || entries[0].Location != fixture.hall || !entries[0].Event.Period.End.Equal(saved.Period.End) {
		t.Errorf("entries = %+v, want the active market with its recomputed end", entries)
	}
	assertDuplicateCheckAgainstDatabase(t, service, in, saved.ID)
}

// assertDuplicateCheckAgainstDatabase saves in a second time: the
// recomputed title key makes the first event a candidate until allowed.
func assertDuplicateCheckAgainstDatabase(t *testing.T, service *core.EventService, in core.EventInput, firstID string) {
	t.Helper()
	ctx := context.Background()
	in.Title, in.StartTime = " KIRCHWEIHMARKT ", "20:00"
	_, err := service.SaveEvent(ctx, "", in, core.RejectDuplicates)
	var suspect *core.DuplicateSuspectError
	if !errors.As(err, &suspect) || len(suspect.Candidates) != 1 || suspect.Candidates[0].ID != firstID {
		t.Fatalf("SaveEvent err = %v, want a suspect naming %s", err, firstID)
	}
	if _, err := service.SaveEvent(ctx, "", in, core.AllowDuplicates); err != nil {
		t.Errorf("SaveEvent with AllowDuplicates: %v", err)
	}
}

// fixedClock is a core.Clock that always shows the same instant.
type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }
