package postgres_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// errAbortTransaction is what a test returns from InTx to roll back.
var errAbortTransaction = errors.New("abort transaction")

var (
	kirchweihFriday   = core.LocalDate{Year: 2026, Month: time.October, Day: 16}
	kirchweihSaturday = core.LocalDate{Year: 2026, Month: time.October, Day: 17}
)

// eventWithTimetable is the full event with three entries: one without
// times, one with a start and one past midnight.
func eventWithTimetable(t *testing.T, locationID string) core.Event {
	t.Helper()
	event := fullEvent(t, locationID)
	event.Timetable = []core.TimetableEntry{
		{Description: "Bieranstich", Date: kirchweihFriday, StartTime: localTimeAt(19, 30)},
		{Description: "Disco", Date: kirchweihFriday, StartTime: localTimeAt(22, 0), EndTime: localTimeAt(1, 0)},
		{Description: "Markttag", Date: kirchweihSaturday},
	}
	return event
}

// descriptionsOf returns the descriptions of entries in alphabetical
// order; the repository returns entries in no particular order.
func descriptionsOf(entries []core.TimetableEntry) []string {
	descriptions := make([]string, 0, len(entries))
	for _, entry := range entries {
		descriptions = append(descriptions, entry.Description)
	}
	slices.Sort(descriptions)
	return descriptions
}

func getEvent(t *testing.T, repo *postgres.EventRepo, id string) core.Event {
	t.Helper()
	event, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return event
}

func TestEventRepoStoresTimetableWithUUIDv7s(t *testing.T) {
	fixture := newEventFixture(t)

	created := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))

	want := []string{"Bieranstich", "Disco", "Markttag"}
	stored := getEvent(t, fixture.repo, created.ID)
	for _, event := range []core.Event{created, stored} {
		if got := descriptionsOf(event.Timetable); !slices.Equal(got, want) {
			t.Errorf("timetable = %v, want %v", got, want)
		}
		for _, entry := range event.Timetable {
			if len(entry.ID) <= uuidVersionIndex || entry.ID[uuidVersionIndex] != '7' {
				t.Errorf("entry id %q is not a UUIDv7", entry.ID)
			}
		}
	}
	for _, entry := range stored.Timetable {
		if entry.Description != "Disco" {
			continue
		}
		if entry.Date != kirchweihFriday || entry.StartTime == nil || *entry.StartTime != *localTimeAt(22, 0) ||
			entry.EndTime == nil || *entry.EndTime != *localTimeAt(1, 0) {
			t.Errorf("disco = %+v, want 16.10. 22:00-01:00 as entered", entry)
		}
	}
}

func TestEventRepoStoresUnknownEntryTimesAsNull(t *testing.T) {
	fixture := newEventFixture(t)
	event := minimalEvent(t, fixture.hall.ID)
	event.Timetable = []core.TimetableEntry{{Description: "Markttag", Date: event.Times.StartDate}}
	created := createEvent(t, fixture.repo, event)

	var allNull bool
	err := fixture.pool.QueryRow(context.Background(),
		`SELECT start_time IS NULL AND end_time IS NULL FROM timetable_entries WHERE event_id = $1`, created.ID).Scan(&allNull)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if !allNull {
		t.Error("unknown entry times were not stored as NULL")
	}
}

func TestEventRepoListReturnsEachEventWithItsOwnTimetable(t *testing.T) {
	fixture := newEventFixture(t)
	festival := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	plain := createEvent(t, fixture.repo, minimalEvent(t, fixture.hall.ID))

	events, err := fixture.repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byID := map[string]core.Event{}
	for _, event := range events {
		byID[event.ID] = event
	}
	if got := descriptionsOf(byID[festival.ID].Timetable); !slices.Equal(got, []string{"Bieranstich", "Disco", "Markttag"}) {
		t.Errorf("festival timetable = %v", got)
	}
	if got := byID[plain.ID].Timetable; got != nil {
		t.Errorf("plain event timetable = %+v, want none", got)
	}
}

func TestEventRepoUpdateReplacesTheWholeTimetable(t *testing.T) {
	fixture := newEventFixture(t)
	created := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	changed := created
	changed.Timetable = []core.TimetableEntry{{Description: "Kehraus", Date: kirchweihSaturday, StartTime: localTimeAt(20, 0)}}

	updated, err := fixture.repo.Update(context.Background(), changed)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	for _, stored := range []core.Event{updated, getEvent(t, fixture.repo, created.ID)} {
		if got := descriptionsOf(stored.Timetable); !slices.Equal(got, []string{"Kehraus"}) {
			t.Errorf("timetable = %v, want only Kehraus", got)
		}
	}
}

func TestDeletingAnEventDeletesItsTimetable(t *testing.T) {
	fixture := newEventFixture(t)
	created := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	ctx := context.Background()

	if _, err := fixture.pool.Exec(ctx, "DELETE FROM events WHERE id = $1", created.ID); err != nil {
		t.Fatalf("delete event: %v", err)
	}

	var remaining int
	if err := fixture.pool.QueryRow(ctx, "SELECT count(*) FROM timetable_entries").Scan(&remaining); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d timetable entries survived their event", remaining)
	}
}

func TestTxRunnerCommitsWhenFnSucceeds(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()
	var created core.Event

	err := postgres.NewTxRunner(fixture.pool).InTx(ctx, func(repos core.Repos) error {
		var err error
		created, err = repos.Events.Create(ctx, eventWithTimetable(t, fixture.hall.ID))
		return err
	})

	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if got := getEvent(t, fixture.repo, created.ID).Timetable; len(got) != 3 {
		t.Errorf("committed timetable = %+v, want three entries", got)
	}
}

func TestTxRunnerRollsBackEventAndTimetableWhenFnFails(t *testing.T) {
	fixture := newEventFixture(t)
	created := createEvent(t, fixture.repo, eventWithTimetable(t, fixture.hall.ID))
	ctx := context.Background()

	err := postgres.NewTxRunner(fixture.pool).InTx(ctx, func(repos core.Repos) error {
		changed := created
		changed.Title = "Weihnachtsmarkt"
		changed.Timetable = changed.Timetable[:1]
		if _, err := repos.Events.Update(ctx, changed); err != nil {
			return err
		}
		if _, err := repos.Events.Create(ctx, minimalEvent(t, fixture.hall.ID)); err != nil {
			return err
		}
		return errAbortTransaction
	})

	if !errors.Is(err, errAbortTransaction) {
		t.Fatalf("InTx err = %v, want %v", err, errAbortTransaction)
	}
	stored := getEvent(t, fixture.repo, created.ID)
	if stored.Title != created.Title || len(stored.Timetable) != 3 {
		t.Errorf("stored = %q with %d entries, want the original with three", stored.Title, len(stored.Timetable))
	}
	events, err := fixture.repo.List(ctx)
	if err != nil || len(events) != 1 {
		t.Errorf("List = %d events, %v, want only the original", len(events), err)
	}
}

func TestTxRunnerKeepsTypedErrorsOfTheCore(t *testing.T) {
	fixture := newEventFixture(t)
	ctx := context.Background()

	err := postgres.NewTxRunner(fixture.pool).InTx(ctx, func(repos core.Repos) error {
		_, err := repos.Locations.Get(ctx, unknownLocationID)
		return err
	})

	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestTxRunnerFailsBeforeFnWhenDatabaseIsUnreachable(t *testing.T) {
	pool, err := postgres.Connect(context.Background(), unreachableDatabaseURL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()
	called := false

	err = postgres.NewTxRunner(pool).InTx(context.Background(), func(core.Repos) error {
		called = true
		return nil
	})

	if err == nil || called {
		t.Errorf("InTx err = %v, fn called = %v, want an error before fn", err, called)
	}
}

// TestEventServiceSavesTimetableAgainstDatabase saves an event with an
// unsorted timetable through the core, reads it sorted, saves it again
// unchanged and rejects shortening the event below its entries.
func TestEventServiceSavesTimetableAgainstDatabase(t *testing.T) {
	fixture := newEventFixture(t)
	service := core.NewEventService(postgres.NewTxRunner(fixture.pool), fixture.repo, fixture.locations)
	ctx := context.Background()
	in := core.EventInput{
		Title: "Kirchweih", Type: string(core.EventTypeFestival), LocationID: fixture.hall.ID,
		StartDate: "2026-10-16", StartTime: "18:00", EndDate: "2026-10-17",
		Source: core.EventSource{Description: "Amtsblatt"},
		Timetable: []core.TimetableEntryInput{
			{Description: "Disco", Date: "2026-10-16", StartTime: "22:00", EndTime: "01:00"},
			{Description: "Bieranstich", Date: "2026-10-16", StartTime: "18:00"},
		},
	}
	saved, err := service.SaveEvent(ctx, "", in, core.RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}

	got, err := service.GetEvent(ctx, saved.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if len(got.Timetable) != 2 || got.Timetable[0].Description != "Bieranstich" || got.Timetable[1].Description != "Disco" {
		t.Fatalf("timetable = %+v, want Bieranstich before Disco", got.Timetable)
	}
	resaved, err := service.SaveEvent(ctx, saved.ID, core.EventInputOf(got), core.RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent again: %v", err)
	}
	assertSameEvent(t, resaved, got)

	in.EndDate, in.EndTime = "2026-10-16", "23:00"
	_, err = service.SaveEvent(ctx, saved.ID, in, core.RejectDuplicates)
	var validation *core.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("SaveEvent with shortened event err = %v, want a validation error", err)
	}
	unchanged, err := service.GetEvent(ctx, saved.ID)
	if err != nil {
		t.Fatalf("GetEvent after rejection: %v", err)
	}
	assertSameEvent(t, unchanged, resaved)
}
