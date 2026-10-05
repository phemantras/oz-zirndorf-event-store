package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	marketID   = "0192f0b1-0000-7000-8000-000000000101"
	concertID  = "0192f0b1-0000-7000-8000-000000000102"
	newEventID = "0192f0b1-0000-7000-8000-000000000103"
)

// fakeEventRepo keeps events in memory and lets tests inject failures.
type fakeEventRepo struct {
	events map[string]Event

	listErr          error
	getErr           error
	writeErr         error
	findErr          error
	deleteErr        error
	countErr         error
	updateDerivedErr map[string]error
	overlapErr       error
	archiveErr       error
	// duringOverlap, when set, runs inside ListOverlapping, as a concurrent
	// writer would.
	duringOverlap func()

	created        []Event
	updated        []Event
	deleted        []string
	derivedUpdated []string
	findCalls      int
	overlaps       []Overlap
	// archiveNows are the instants MarkArchived was called with; it
	// reports archivedCount marked events.
	archiveNows   []time.Time
	archivedCount int
}

func newFakeEventRepo(events ...Event) *fakeEventRepo {
	repo := &fakeEventRepo{events: map[string]Event{}}
	for _, event := range events {
		repo.events[event.ID] = event
	}
	return repo
}

func (r *fakeEventRepo) List(context.Context) ([]Event, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	all := make([]Event, 0, len(r.events))
	for _, event := range r.events {
		all = append(all, event)
	}
	// Deliberately reverse ID order, so a missing core sort always fails.
	slices.SortFunc(all, func(a, b Event) int { return strings.Compare(b.ID, a.ID) })
	return all, nil
}

// ListOverlapping applies the one filter predicate of AD-16 and records
// every overlap it was asked for.
func (r *fakeEventRepo) ListOverlapping(ctx context.Context, overlap Overlap) ([]Event, error) {
	r.overlaps = append(r.overlaps, overlap)
	if r.duringOverlap != nil {
		r.duringOverlap()
	}
	if r.overlapErr != nil {
		return nil, r.overlapErr
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var matches []Event
	for _, event := range all {
		if (overlap.Hi == nil || event.Period.Start.Before(*overlap.Hi)) && (overlap.Lo == nil || event.Period.End.After(*overlap.Lo)) {
			matches = append(matches, event)
		}
	}
	return matches, nil
}

func (r *fakeEventRepo) Get(_ context.Context, id string) (Event, error) {
	if r.getErr != nil {
		return Event{}, r.getErr
	}
	event, ok := r.events[strings.ToLower(id)]
	if !ok {
		return Event{}, ErrNotFound
	}
	return event, nil
}

func (r *fakeEventRepo) Create(_ context.Context, event Event) (Event, error) {
	if r.writeErr != nil {
		return Event{}, r.writeErr
	}
	event.ID = newEventID
	r.events[event.ID] = event
	r.created = append(r.created, event)
	return event, nil
}

func (r *fakeEventRepo) Update(_ context.Context, event Event) (Event, error) {
	if r.writeErr != nil {
		return Event{}, r.writeErr
	}
	r.events[event.ID] = event
	r.updated = append(r.updated, event)
	return event, nil
}

func (r *fakeEventRepo) FindByDuplicateKey(_ context.Context, key DuplicateKey) ([]Event, error) {
	r.findCalls++
	if r.findErr != nil {
		return nil, r.findErr
	}
	all, err := r.List(context.Background())
	if err != nil {
		return nil, err
	}
	var matches []Event
	for _, event := range all {
		if event.TitleKey == key.TitleKey && event.Times.StartDate == key.StartDate && event.LocationID == key.LocationID {
			matches = append(matches, event)
		}
	}
	return matches, nil
}

func (r *fakeEventRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.events[id]; !ok {
		return ErrNotFound
	}
	delete(r.events, id)
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *fakeEventRepo) CountByLocation(_ context.Context, locationID string) (int, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	count := 0
	for _, event := range r.events {
		if event.LocationID == locationID {
			count++
		}
	}
	return count, nil
}

func (r *fakeEventRepo) UpdateDerived(_ context.Context, id string, derived Derived) error {
	if err := r.updateDerivedErr[id]; err != nil {
		return err
	}
	event := r.events[id]
	event.Period, event.TitleKey = derived.Period, derived.TitleKey
	r.events[id] = event
	r.derivedUpdated = append(r.derivedUpdated, id)
	return nil
}

func (r *fakeEventRepo) MarkArchived(_ context.Context, now time.Time) (int, error) {
	r.archiveNows = append(r.archiveNows, now)
	if r.archiveErr != nil {
		return 0, r.archiveErr
	}
	return r.archivedCount, nil
}

// storedEvent returns a valid event as the repository would hold it.
func storedEvent(t *testing.T, id, title string, times EventTimes) Event {
	t.Helper()
	period, err := times.EffectivePeriod()
	if err != nil {
		t.Fatalf("EffectivePeriod: %v", err)
	}
	return Event{
		ID: id, Title: title, TitleKey: NormalizeKey(title), Type: EventTypeMarket, LocationID: hallID,
		Times: times, Source: EventSource{Description: "Amtsblatt"}, Period: period,
	}
}

// fakeTx runs fn directly on its repositories and passes fn's error on; it
// fails before fn when beginErr is set.
type fakeTx struct {
	repos    Repos
	beginErr error
	runs     int
}

func (tx *fakeTx) InTx(_ context.Context, fn func(Repos) error) error {
	if tx.beginErr != nil {
		return tx.beginErr
	}
	tx.runs++
	return fn(tx.repos)
}

// newEventServiceOn returns the use cases with a fake transaction on the
// same repositories.
func newEventServiceOn(events *fakeEventRepo, locations *fakeLocationRepo) *EventService {
	return NewEventService(&fakeTx{repos: Repos{Events: events, Locations: locations}}, events, locations)
}

func newTestEventService(events *fakeEventRepo, locations ...Location) *EventService {
	return newEventServiceOn(events, newFakeLocationRepo(locations...))
}

func TestSaveEventCreatesEventWithComputedPeriod(t *testing.T) {
	repo := newFakeEventRepo()

	saved, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", validEventInput(), RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if saved.ID != newEventID || len(repo.created) != 1 || len(repo.updated) != 0 {
		t.Errorf("saved = %+v, created %d, updated %d, want one create", saved, len(repo.created), len(repo.updated))
	}
	wantStart, wantEnd := berlinInstant(t, kirchweihFriday, 0, 0), berlinInstant(t, kirchweihFriday.NextDay(), 0, 0)
	if stored := repo.created[0].Period; !stored.Start.Equal(wantStart) || !stored.End.Equal(wantEnd) {
		t.Errorf("stored period = [%v, %v), want [%v, %v)", stored.Start, stored.End, wantStart, wantEnd)
	}
}

func TestSaveEventStoresTheLocationIDAsTheRepositorySpellsIt(t *testing.T) {
	repo := newFakeEventRepo()
	in := validEventInput()
	in.LocationID = strings.ToUpper(hallID)

	saved, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", in, RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if saved.LocationID != hallID {
		t.Errorf("location id = %q, want %q", saved.LocationID, hallID)
	}
}

func TestSaveEventUpdatesExistingEventKeepingItsID(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	repo := newFakeEventRepo(market)
	in := validEventInput()
	in.Title, in.LocationID = "Kirchweih", parkID

	saved, err := newTestEventService(repo, hall(), park()).SaveEvent(context.Background(), strings.ToUpper(marketID), in, RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if saved.ID != marketID || saved.Title != "Kirchweih" || saved.LocationID != parkID || len(repo.created) != 0 {
		t.Errorf("saved = %+v, created %d, want an update of %s", saved, len(repo.created), marketID)
	}
}

func TestSaveEventReportsUnknownLocationTogetherWithOtherProblems(t *testing.T) {
	repo := newFakeEventRepo()
	in := validEventInput()
	in.Title, in.LocationID = " ", otherID

	_, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", in, RejectDuplicates)

	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	want := []FieldError{{EventFieldTitle, ProblemMissing}, {EventFieldLocationID, ProblemNotFound}}
	if !slices.Equal(validation.Fields, want) {
		t.Errorf("problems = %v, want %v", validation.Fields, want)
	}
	if len(repo.created) != 0 {
		t.Error("an invalid event was written")
	}
}

func TestSaveEventDoesNotLookUpAMissingLocation(t *testing.T) {
	locations := newFakeLocationRepo()
	locations.getErr = errDatabaseDown
	in := validEventInput()
	in.LocationID = ""

	_, err := newEventServiceOn(newFakeEventRepo(), locations).SaveEvent(context.Background(), "", in, RejectDuplicates)

	var validation *ValidationError
	if !errors.As(err, &validation) || !slices.Equal(validation.Fields, []FieldError{{EventFieldLocationID, ProblemMissing}}) {
		t.Errorf("err = %v, want only locationId missing", err)
	}
}

func TestSaveEventReportsUnknownEventAsNotFoundBeforeValidation(t *testing.T) {
	in := validEventInput()
	in.Title = ""

	_, err := newTestEventService(newFakeEventRepo(), hall()).SaveEvent(context.Background(), otherID, in, RejectDuplicates)

	if !errors.Is(err, ErrNotFound) || errors.Is(err, ErrValidation) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSaveEventPassesRepositoryFailuresOn(t *testing.T) {
	tests := map[string]struct {
		id     string
		inject func(*fakeEventRepo, *fakeLocationRepo)
	}{
		"get event":    {marketID, func(e *fakeEventRepo, _ *fakeLocationRepo) { e.getErr = errDatabaseDown }},
		"get location": {"", func(_ *fakeEventRepo, l *fakeLocationRepo) { l.getErr = errDatabaseDown }},
		"create":       {"", func(e *fakeEventRepo, _ *fakeLocationRepo) { e.writeErr = errDatabaseDown }},
		"update":       {marketID, func(e *fakeEventRepo, _ *fakeLocationRepo) { e.writeErr = errDatabaseDown }},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))
			locations := newFakeLocationRepo(hall())
			tt.inject(events, locations)

			_, err := newEventServiceOn(events, locations).SaveEvent(context.Background(), tt.id, validEventInput(), AllowDuplicates)

			if !errors.Is(err, errDatabaseDown) {
				t.Errorf("err = %v, want %v", err, errDatabaseDown)
			}
		})
	}
}

// TestSaveEventReportsALocationDeletedBeforeTheWriteAsNotFound covers the
// race in which the location disappears between the check and the write,
// so the repository refuses the event with ErrConflict.
func TestSaveEventReportsALocationDeletedBeforeTheWriteAsNotFound(t *testing.T) {
	for name, id := range map[string]string{"create": "", "update": marketID} {
		t.Run(name, func(t *testing.T) {
			events := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))
			events.writeErr = fmt.Errorf("%s event: %w", name, errors.Join(ErrConflict, errDatabaseDown))

			_, err := newTestEventService(events, hall()).SaveEvent(context.Background(), id, validEventInput(), AllowDuplicates)

			var validation *ValidationError
			if !errors.As(err, &validation) || !slices.Equal(validation.Fields, []FieldError{{EventFieldLocationID, ProblemNotFound}}) {
				t.Errorf("err = %v, want only locationId notFound", err)
			}
		})
	}
}

func TestGetEventReturnsStoredEventOrNotFound(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	service := newTestEventService(newFakeEventRepo(market), hall())

	got, err := service.GetEvent(context.Background(), marketID)
	if err != nil || got.ID != marketID || got.Title != market.Title {
		t.Errorf("GetEvent = %+v, %v, want the market", got, err)
	}
	if _, err := service.GetEvent(context.Background(), otherID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetEvent(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestListEventsShowsActiveChronologicallyThenArchivedNewestFirst(t *testing.T) {
	october := func(day int) LocalDate { return LocalDate{2026, time.October, day} }
	pastOld := storedEvent(t, "0192f0b1-0000-7000-8000-000000000201", "Altes Fest", EventTimes{StartDate: october(1)})
	pastRecent := storedEvent(t, "0192f0b1-0000-7000-8000-000000000202", "Flohmarkt", EventTimes{StartDate: october(10)})
	running := storedEvent(t, "0192f0b1-0000-7000-8000-000000000203", "Kirchweih", EventTimes{StartDate: october(14), EndDate: october(19)})
	laterB := storedEvent(t, "0192f0b1-0000-7000-8000-000000000205", "Konzert B", EventTimes{StartDate: october(20)})
	laterA := storedEvent(t, "0192f0b1-0000-7000-8000-000000000204", "Konzert A", EventTimes{StartDate: october(20)})
	pastTwinB := storedEvent(t, "0192f0b1-0000-7000-8000-000000000207", "Markt B", EventTimes{StartDate: october(5)})
	pastTwinA := storedEvent(t, "0192f0b1-0000-7000-8000-000000000206", "Markt A", EventTimes{StartDate: october(5)})
	service := newTestEventService(newFakeEventRepo(pastOld, pastRecent, running, laterB, laterA, pastTwinB, pastTwinA), hall())

	entries, err := service.ListEvents(context.Background(), clockAt(t, october(16), 12, 0))
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	var got []string
	for _, entry := range entries {
		got = append(got, entry.Event.Title)
		if entry.Location != hall() {
			t.Errorf("%s: location = %+v, want the hall", entry.Event.Title, entry.Location)
		}
		if wantArchived := entry.Event.Period.End.Before(berlinInstant(t, october(16), 12, 0)); entry.Archived != wantArchived {
			t.Errorf("%s: archived = %v, want %v", entry.Event.Title, entry.Archived, wantArchived)
		}
	}
	want := []string{"Kirchweih", "Konzert A", "Konzert B", "Flohmarkt", "Markt A", "Markt B", "Altes Fest"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestListEventsShowsCorrectedDateAsActive(t *testing.T) {
	past := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: LocalDate{2025, time.October, 17}})
	service := newTestEventService(newFakeEventRepo(past), hall())
	clock := clockAt(t, kirchweihFriday, 8, 0)
	ctx := context.Background()

	before, err := service.ListEvents(ctx, clock)
	if err != nil || len(before) != 1 || !before[0].Archived {
		t.Fatalf("before = %+v, %v, want one archived event", before, err)
	}
	if _, err := service.SaveEvent(ctx, marketID, validEventInput(), RejectDuplicates); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	after, err := service.ListEvents(ctx, clock)
	if err != nil || len(after) != 1 || after[0].Archived {
		t.Errorf("after = %+v, %v, want one active event", after, err)
	}
}

func TestListEventsPassesFailuresOn(t *testing.T) {
	t.Run("events", func(t *testing.T) {
		events := newFakeEventRepo()
		events.listErr = errDatabaseDown
		_, err := newTestEventService(events, hall()).ListEvents(context.Background(), clockAt(t, kirchweihFriday, 0, 0))
		if !errors.Is(err, errDatabaseDown) {
			t.Errorf("err = %v, want %v", err, errDatabaseDown)
		}
	})
	t.Run("locations", func(t *testing.T) {
		locations := newFakeLocationRepo()
		locations.listErr = errDatabaseDown
		_, err := newEventServiceOn(newFakeEventRepo(), locations).ListEvents(context.Background(), clockAt(t, kirchweihFriday, 0, 0))
		if !errors.Is(err, errDatabaseDown) {
			t.Errorf("err = %v, want %v", err, errDatabaseDown)
		}
	})
	t.Run("location of an event missing", func(t *testing.T) {
		market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
		_, err := newTestEventService(newFakeEventRepo(market)).ListEvents(context.Background(), clockAt(t, kirchweihFriday, 0, 0))
		if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), marketID) {
			t.Errorf("err = %v, want ErrNotFound naming %s", err, marketID)
		}
	})
}

// staleEvent is a stored event whose period no longer matches its times,
// as after a rule or tzdata change.
func staleEvent(t *testing.T, id string) Event {
	t.Helper()
	event := storedEvent(t, id, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	event.Period.End = event.Period.End.Add(time.Hour)
	return event
}

// brokenEvent is a stored event whose times the current rules reject.
func brokenEvent(t *testing.T, id string) Event {
	t.Helper()
	event := storedEvent(t, id, "Konzert", EventTimes{StartDate: kirchweihFriday})
	event.Times.StartTime = localTime(19, 0)
	event.Times.AllDay = true
	return event
}

func TestRecomputeDerivedWritesOnlyChangedPeriods(t *testing.T) {
	current := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	// The same instants in another zone are no change.
	current.Period = Period{Start: current.Period.Start.UTC(), End: current.Period.End.UTC()}
	stale := staleEvent(t, concertID)
	repo := newFakeEventRepo(current, stale)

	failures, err := newTestEventService(repo, hall()).RecomputeDerived(context.Background())
	if err != nil || failures != nil {
		t.Fatalf("RecomputeDerived = %v, %v, want no failures", failures, err)
	}
	if !slices.Equal(repo.derivedUpdated, []string{concertID}) {
		t.Errorf("derived values updated for %v, want only %s", repo.derivedUpdated, concertID)
	}
	want := storedEvent(t, concertID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}).Period
	if got := repo.events[concertID].Period; !got.End.Equal(want.End) {
		t.Errorf("recomputed end = %v, want %v", got.End, want.End)
	}
}

func TestRecomputeDerivedKeepsValuesOfFailingEventsAndMarksThemForReview(t *testing.T) {
	broken := brokenEvent(t, marketID)
	otherBroken := brokenEvent(t, concertID)
	fine := storedEvent(t, newEventID, "Flohmarkt", EventTimes{StartDate: kirchweihMonday})
	repo := newFakeEventRepo(broken, otherBroken, fine)
	service := newTestEventService(repo, hall())
	ctx := context.Background()

	failures, err := service.RecomputeDerived(ctx)
	if err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}

	var failedIDs []string
	for _, failure := range failures {
		failedIDs = append(failedIDs, failure.EventID)
		if failure.Err == nil {
			t.Errorf("failure for %s carries no error", failure.EventID)
		}
	}
	slices.Sort(failedIDs)
	if !slices.Equal(failedIDs, []string{marketID, concertID}) {
		t.Errorf("failed ids = %v, want %s and %s", failedIDs, marketID, concertID)
	}
	if got := repo.events[marketID].Period; got != broken.Period {
		t.Errorf("broken event period = %+v, want the stored %+v", got, broken.Period)
	}
	if len(repo.derivedUpdated) != 0 {
		t.Errorf("derived values updated for %v, want none: the title keys are current", repo.derivedUpdated)
	}
	assertNeedsReview(t, service, map[string]bool{marketID: true, concertID: true, newEventID: false})
}

func TestRecomputeDerivedStoresTheTitleKeyOfAnEventWhosePeriodFails(t *testing.T) {
	broken := brokenEvent(t, marketID)
	broken.TitleKey = ""
	repo := newFakeEventRepo(broken)
	service := newTestEventService(repo, hall())

	failures, err := service.RecomputeDerived(context.Background())
	if err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}
	if len(failures) != 1 || failures[0].EventID != marketID {
		t.Errorf("failures = %v, want one for %s", failures, marketID)
	}
	stored := repo.events[marketID]
	if stored.TitleKey != NormalizeKey(broken.Title) {
		t.Errorf("title key = %q, want %q", stored.TitleKey, NormalizeKey(broken.Title))
	}
	if stored.Period != broken.Period {
		t.Errorf("period = %+v, want the stored %+v", stored.Period, broken.Period)
	}
	assertNeedsReview(t, service, map[string]bool{marketID: true})
}

// eventWithTimetable is a stored event on kirchweihFriday whose stored
// period still runs two days longer, as before a rule change shortened it,
// with entries as its timetable and a stale title key.
func eventWithTimetable(t *testing.T, entries ...TimetableEntry) Event {
	t.Helper()
	event := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	event.Period.End = event.Period.End.AddDate(0, 0, 2)
	event.TitleKey = ""
	event.Timetable = entries
	return event
}

func TestRecomputeDerivedKeepsThePeriodWhenTheTimetableNoLongerFits(t *testing.T) {
	saturday := kirchweihFriday.NextDay()
	allDayFriday := TimetableEntry{ID: "entry-fits", Description: "Markttag", Date: kirchweihFriday}
	tests := map[string]struct {
		entries []TimetableEntry
		entryID string
		field   string
	}{
		"day entry": {
			entries: []TimetableEntry{{ID: "entry-day", Description: "Markttag", Date: saturday}, allDayFriday},
			entryID: "entry-day",
			field:   TimetableField(1, TimetableFieldDate),
		},
		"timed entry": {
			entries: []TimetableEntry{
				{ID: "entry-timed", Description: "Disco", Date: kirchweihFriday, StartTime: localTime(22, 0), EndTime: localTime(1, 0)},
				allDayFriday,
			},
			entryID: "entry-timed",
			field:   TimetableField(1, TimetableFieldEndTime),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			event := eventWithTimetable(t, tt.entries...)
			repo := newFakeEventRepo(event)
			service := newTestEventService(repo, hall())

			failures, err := service.RecomputeDerived(context.Background())
			if err != nil {
				t.Fatalf("RecomputeDerived: %v", err)
			}
			if len(failures) != 1 || failures[0].EventID != marketID {
				t.Fatalf("failures = %v, want one for %s", failures, marketID)
			}
			message := failures[0].Err.Error()
			for _, want := range []string{marketID, tt.entryID, tt.field, string(ProblemOutsideEvent)} {
				if !strings.Contains(message, want) {
					t.Errorf("err = %q, want it to name %q", message, want)
				}
			}
			stored := repo.events[marketID]
			if stored.Period != event.Period {
				t.Errorf("period = %+v, want the stored %+v", stored.Period, event.Period)
			}
			if stored.TitleKey != NormalizeKey(event.Title) {
				t.Errorf("title key = %q, want %q", stored.TitleKey, NormalizeKey(event.Title))
			}
			assertNeedsReview(t, service, map[string]bool{marketID: true})
		})
	}
}

func TestRecomputeDerivedStoresThePeriodWhenTheTimetableFits(t *testing.T) {
	event := eventWithTimetable(t,
		TimetableEntry{ID: "entry-timed", Description: "Disco", Date: kirchweihFriday, StartTime: localTime(22, 0), EndTime: localTime(23, 30)},
		TimetableEntry{ID: "entry-day", Description: "Markttag", Date: kirchweihFriday},
	)
	repo := newFakeEventRepo(event)
	service := newTestEventService(repo, hall())

	failures, err := service.RecomputeDerived(context.Background())
	if err != nil || failures != nil {
		t.Fatalf("RecomputeDerived = %v, %v, want no failures", failures, err)
	}
	want := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}).Period
	if got := repo.events[marketID].Period; !got.End.Equal(want.End) {
		t.Errorf("recomputed end = %v, want %v", got.End, want.End)
	}
	assertNeedsReview(t, service, map[string]bool{marketID: false})
}

func TestRecomputeDerivedStopsAtStorageFailuresWithoutMarking(t *testing.T) {
	keyless := brokenEvent(t, concertID)
	keyless.TitleKey = ""
	tests := map[string]Event{
		"recomputed period": staleEvent(t, concertID),
		"title key only":    keyless,
	}
	for name, event := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newFakeEventRepo(event)
			repo.updateDerivedErr = map[string]error{concertID: errDatabaseDown}
			service := newTestEventService(repo, hall())

			failures, err := service.RecomputeDerived(context.Background())

			if !errors.Is(err, errDatabaseDown) || failures != nil {
				t.Errorf("RecomputeDerived = %v, %v, want only %v", failures, err, errDatabaseDown)
			}
			assertNeedsReview(t, service, map[string]bool{concertID: false})
		})
	}
}

func TestRecomputeDerivedSkipsEventsDeletedMeanwhile(t *testing.T) {
	repo := newFakeEventRepo(staleEvent(t, concertID))
	repo.updateDerivedErr = map[string]error{concertID: fmt.Errorf("update derived: %w", ErrNotFound)}
	service := newTestEventService(repo, hall())

	failures, err := service.RecomputeDerived(context.Background())

	if err != nil || failures != nil {
		t.Errorf("RecomputeDerived = %v, %v, want no failures", failures, err)
	}
	assertNeedsReview(t, service, map[string]bool{concertID: false})
}

func TestRecomputeDerivedStopsWhenTheContextEnds(t *testing.T) {
	repo := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	failures, err := newTestEventService(repo, hall()).RecomputeDerived(ctx)

	if !errors.Is(err, context.Canceled) || failures != nil {
		t.Errorf("RecomputeDerived = %v, %v, want %v", failures, err, context.Canceled)
	}
}

func TestSuccessfulSaveClearsTheReviewMark(t *testing.T) {
	repo := newFakeEventRepo(brokenEvent(t, marketID), brokenEvent(t, concertID))
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	if _, err := service.RecomputeDerived(ctx); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}

	in := validEventInput()
	in.Title = ""
	if _, err := service.SaveEvent(ctx, marketID, in, RejectDuplicates); err == nil {
		t.Fatal("SaveEvent accepted an invalid event")
	}
	assertNeedsReview(t, service, map[string]bool{marketID: true, concertID: true})

	if _, err := service.SaveEvent(ctx, strings.ToUpper(marketID), validEventInput(), RejectDuplicates); err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	assertNeedsReview(t, service, map[string]bool{marketID: false, concertID: true})
}

func TestRecomputeDerivedFailsWhenEventsCannotBeListed(t *testing.T) {
	repo := newFakeEventRepo()
	repo.listErr = errDatabaseDown

	if _, err := newTestEventService(repo).RecomputeDerived(context.Background()); !errors.Is(err, errDatabaseDown) {
		t.Errorf("err = %v, want %v", err, errDatabaseDown)
	}
}

func TestReviewMarksAreSafeForConcurrentUse(t *testing.T) {
	repo := newFakeEventRepo()
	service := newTestEventService(repo, hall())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			service.markForReview(marketID)
			service.clearReview(marketID)
			_ = service.needsReview(marketID)
		})
	}
	wg.Wait()
}

func assertNeedsReview(t *testing.T, service *EventService, want map[string]bool) {
	t.Helper()
	entries, err := service.ListEvents(context.Background(), clockAt(t, kirchweihFriday, 0, 0))
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	for _, entry := range entries {
		if wanted, ok := want[entry.Event.ID]; ok && entry.NeedsReview != wanted {
			t.Errorf("%s: needs review = %v, want %v", entry.Event.ID, entry.NeedsReview, wanted)
		}
	}
}

// festTimetable is entered out of order: Disco runs past midnight.
func festTimetable() []TimetableEntryInput {
	return []TimetableEntryInput{
		entryInput("Disco", "2026-10-16", "22:00", "01:00"),
		entryInput("Bieranstich", "2026-10-16", "18:00", ""),
		entryInput("Markttag", "2026-10-17", "", ""),
	}
}

func timetableDescriptions(entries []TimetableEntry) []string {
	var descriptions []string
	for _, entry := range entries {
		descriptions = append(descriptions, entry.Description)
	}
	return descriptions
}

func TestSaveEventStoresTimetableSortedInOneTransaction(t *testing.T) {
	events, locations := newFakeEventRepo(), newFakeLocationRepo(hall())
	tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
	service := NewEventService(tx, events, locations)

	saved, err := service.SaveEvent(context.Background(), "", festInput(festTimetable()...), RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	if tx.runs != 1 || len(events.created) != 1 {
		t.Fatalf("transactions = %d, created = %d, want one create in one transaction", tx.runs, len(events.created))
	}
	want := []string{"Bieranstich", "Disco", "Markttag"}
	if got := timetableDescriptions(events.created[0].Timetable); !slices.Equal(got, want) {
		t.Errorf("stored timetable = %v, want %v", got, want)
	}
	if got := timetableDescriptions(saved.Timetable); !slices.Equal(got, want) {
		t.Errorf("returned timetable = %v, want %v", got, want)
	}
	withoutTimetable := festInput()
	plain, err := service.SaveEvent(context.Background(), "", withoutTimetable, AllowDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent without timetable: %v", err)
	}
	if !plain.Period.Start.Equal(saved.Period.Start) || !plain.Period.End.Equal(saved.Period.End) {
		t.Errorf("period with timetable = %+v, without = %+v, want equal", saved.Period, plain.Period)
	}
}

func TestSaveEventReplacesTheWholeTimetable(t *testing.T) {
	repo := newFakeEventRepo()
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	created, err := service.SaveEvent(ctx, "", festInput(festTimetable()...), RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}

	in := festInput(entryInput("Kehraus", "2026-10-17", "20:00", ""))
	if _, err := service.SaveEvent(ctx, created.ID, in, RejectDuplicates); err != nil {
		t.Fatalf("SaveEvent update: %v", err)
	}
	if got := timetableDescriptions(repo.events[created.ID].Timetable); !slices.Equal(got, []string{"Kehraus"}) {
		t.Errorf("stored timetable = %v, want only Kehraus", got)
	}
}

func TestResavingAnUnchangedEventKeepsPeriodAndTimetable(t *testing.T) {
	repo := newFakeEventRepo()
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	created, err := service.SaveEvent(ctx, "", festInput(festTimetable()...), RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	stored, err := service.GetEvent(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	resaved, err := service.SaveEvent(ctx, created.ID, EventInputOf(stored), RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent again: %v", err)
	}
	if !resaved.Period.Start.Equal(stored.Period.Start) || !resaved.Period.End.Equal(stored.Period.End) {
		t.Errorf("period = %+v, want unchanged %+v", resaved.Period, stored.Period)
	}
	if got, want := EventInputOf(resaved).Timetable, EventInputOf(stored).Timetable; !slices.Equal(got, want) {
		t.Errorf("timetable = %v, want unchanged %v", got, want)
	}
}

func TestSaveEventRejectsEntryOutsideAShortenedEventWithoutWriting(t *testing.T) {
	repo := newFakeEventRepo()
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	created, err := service.SaveEvent(ctx, "", festInput(festTimetable()...), RejectDuplicates)
	if err != nil {
		t.Fatalf("SaveEvent: %v", err)
	}
	in := festInput(festTimetable()...)
	in.EndDate = "2026-10-16"

	_, err = service.SaveEvent(ctx, created.ID, in, RejectDuplicates)

	var validation *ValidationError
	want := []FieldError{{TimetableField(0, TimetableFieldEndTime), ProblemOutsideEvent}, {TimetableField(2, TimetableFieldDate), ProblemOutsideEvent}}
	if !errors.As(err, &validation) || !slices.Equal(validation.Fields, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
	if len(repo.updated) != 0 {
		t.Error("an invalid timetable was written")
	}
}

func TestSaveEventPassesTransactionFailuresOnAndKeepsTheReviewMark(t *testing.T) {
	events, locations := newFakeEventRepo(brokenEvent(t, marketID)), newFakeLocationRepo(hall())
	service := NewEventService(&fakeTx{beginErr: errDatabaseDown}, events, locations)
	ctx := context.Background()
	if _, err := service.RecomputeDerived(ctx); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}

	_, err := service.SaveEvent(ctx, marketID, validEventInput(), RejectDuplicates)

	if !errors.Is(err, errDatabaseDown) {
		t.Errorf("err = %v, want %v", err, errDatabaseDown)
	}
	if len(events.updated) != 0 {
		t.Error("an event was written without transaction")
	}
	assertNeedsReview(t, service, map[string]bool{marketID: true})
}

func TestReadingEventsSortsTheirTimetable(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	market.Timetable = []TimetableEntry{
		{ID: "2", Description: "Abend", Date: kirchweihFriday, StartTime: localTime(19, 0)},
		{ID: "1", Description: "Ganzer Tag", Date: kirchweihFriday},
		{ID: "3", Description: "Abend", Date: kirchweihFriday, StartTime: localTime(19, 0)},
	}
	service := newTestEventService(newFakeEventRepo(market), hall())
	ctx := context.Background()
	want := []string{"Ganzer Tag", "Abend", "Abend"}

	got, err := service.GetEvent(ctx, marketID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if descriptions := timetableDescriptions(got.Timetable); !slices.Equal(descriptions, want) || got.Timetable[1].ID != "2" {
		t.Errorf("GetEvent timetable = %+v, want %v with ties by ID", got.Timetable, want)
	}
	entries, err := service.ListEvents(ctx, clockAt(t, kirchweihFriday, 0, 0))
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if descriptions := timetableDescriptions(entries[0].Event.Timetable); !slices.Equal(descriptions, want) {
		t.Errorf("ListEvents timetable = %v, want %v", descriptions, want)
	}
}

func TestDeleteEventRemovesTheStoredEventInOneTransaction(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	market.Timetable = []TimetableEntry{{ID: "1", Description: "Bieranstich", Date: kirchweihFriday}}
	concert := storedEvent(t, concertID, "Konzert", EventTimes{StartDate: kirchweihFriday})
	events, locations := newFakeEventRepo(market, concert), newFakeLocationRepo(hall())
	tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}

	err := NewEventService(tx, events, locations).DeleteEvent(context.Background(), strings.ToUpper(marketID))
	if err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if tx.runs != 1 || !slices.Equal(events.deleted, []string{marketID}) {
		t.Errorf("transactions = %d, deleted = %v, want %s deleted in one transaction", tx.runs, events.deleted, marketID)
	}
	if _, ok := events.events[concertID]; !ok {
		t.Error("another event was deleted")
	}
}

func TestDeleteEventReportsAnUnknownEventAsNotFound(t *testing.T) {
	events := newFakeEventRepo()

	err := newTestEventService(events, hall()).DeleteEvent(context.Background(), marketID)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if len(events.deleted) != 0 {
		t.Errorf("deleted = %v, want nothing", events.deleted)
	}
}

func TestDeleteEventClearsTheReviewMark(t *testing.T) {
	repo := newFakeEventRepo(brokenEvent(t, marketID), brokenEvent(t, concertID))
	service := newTestEventService(repo, hall())
	ctx := context.Background()
	if _, err := service.RecomputeDerived(ctx); err != nil {
		t.Fatalf("RecomputeDerived: %v", err)
	}

	if err := service.DeleteEvent(ctx, strings.ToUpper(marketID)); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if service.needsReview(marketID) || !service.needsReview(concertID) {
		t.Errorf("review marks: %s = %v, %s = %v, want only %s marked",
			marketID, service.needsReview(marketID), concertID, service.needsReview(concertID), concertID)
	}
}

func TestDeleteEventPassesFailuresOnAndKeepsTheReviewMark(t *testing.T) {
	tests := map[string]func(*fakeTx, *fakeEventRepo){
		"begin":  func(tx *fakeTx, _ *fakeEventRepo) { tx.beginErr = errDatabaseDown },
		"get":    func(_ *fakeTx, e *fakeEventRepo) { e.getErr = errDatabaseDown },
		"delete": func(_ *fakeTx, e *fakeEventRepo) { e.deleteErr = errDatabaseDown },
	}
	for name, inject := range tests {
		t.Run(name, func(t *testing.T) {
			events, locations := newFakeEventRepo(brokenEvent(t, marketID)), newFakeLocationRepo(hall())
			tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
			service := NewEventService(tx, events, locations)
			ctx := context.Background()
			if _, err := service.RecomputeDerived(ctx); err != nil {
				t.Fatalf("RecomputeDerived: %v", err)
			}
			inject(tx, events)

			err := service.DeleteEvent(ctx, marketID)

			if !errors.Is(err, errDatabaseDown) {
				t.Errorf("err = %v, want %v", err, errDatabaseDown)
			}
			if !service.needsReview(marketID) {
				t.Error("a failed delete cleared the review mark")
			}
		})
	}
}

// countingClock shows a fixed instant and counts how often it was read.
type countingClock struct {
	now   time.Time
	reads int
}

func (c *countingClock) Now() time.Time {
	c.reads++
	return c.now
}

func TestMarkArchivedMarksAtTheClockInstantInOneTransaction(t *testing.T) {
	events, locations := newFakeEventRepo(), newFakeLocationRepo()
	events.archivedCount = 2
	tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
	clock := &countingClock{now: christmasNoon(t).Now()}

	marked, err := NewEventService(tx, events, locations).MarkArchived(context.Background(), clock)
	if err != nil {
		t.Fatalf("MarkArchived: %v", err)
	}
	if marked != 2 {
		t.Errorf("marked = %d, want 2", marked)
	}
	if tx.runs != 1 || clock.reads != 1 || len(events.archiveNows) != 1 || !events.archiveNows[0].Equal(clock.now) {
		t.Errorf("transactions = %d, clock reads = %d, repository instants = %v, want one of each at %v",
			tx.runs, clock.reads, events.archiveNows, clock.now)
	}
}

func TestMarkArchivedPassesFailuresOn(t *testing.T) {
	tests := map[string]func(*fakeEventRepo, *fakeTx){
		"repository":  func(e *fakeEventRepo, _ *fakeTx) { e.archiveErr = errDatabaseDown },
		"transaction": func(_ *fakeEventRepo, tx *fakeTx) { tx.beginErr = errDatabaseDown },
	}
	for name, inject := range tests {
		t.Run(name, func(t *testing.T) {
			events, locations := newFakeEventRepo(), newFakeLocationRepo()
			events.archivedCount = 1
			tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
			inject(events, tx)

			marked, err := NewEventService(tx, events, locations).MarkArchived(context.Background(), christmasNoon(t))

			if !errors.Is(err, errDatabaseDown) || marked != 0 {
				t.Errorf("MarkArchived = %d, %v, want 0, %v", marked, err, errDatabaseDown)
			}
		})
	}
}
