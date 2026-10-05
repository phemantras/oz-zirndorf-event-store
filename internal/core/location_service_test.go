package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	hallID  = "0192f0b1-0000-7000-8000-000000000001"
	parkID  = "0192f0b1-0000-7000-8000-000000000002"
	newID   = "0192f0b1-0000-7000-8000-000000000003"
	tentID  = "0192f0b1-0000-7000-8000-000000000004"
	otherID = "0192f0b1-0000-7000-8000-0000000000ff"
)

var errDatabaseDown = errors.New("database down")

// fakeLocationRepo keeps locations in memory and lets tests inject failures
// and a unique violation that the pre-check cannot see (a concurrent save).
type fakeLocationRepo struct {
	locations map[string]Location
	nextID    string

	listErr   error
	getErr    error
	findErr   error
	writeErr  error
	deleteErr error
	// nameKeyErr fails UpdateNameKey for the location with the key's ID.
	nameKeyErr map[string]error
	// raceWinner is inserted right before the write, simulating another
	// request that saved the same name_key after the pre-check.
	raceWinner *Location

	created []Location
	updated []Location
	deleted []string
	// nameKeysUpdated are the IDs whose name key UpdateNameKey stored.
	nameKeysUpdated []string
}

func newFakeLocationRepo(locations ...Location) *fakeLocationRepo {
	repo := &fakeLocationRepo{locations: map[string]Location{}, nextID: newID}
	for _, location := range locations {
		repo.locations[location.ID] = location
	}
	return repo
}

func (r *fakeLocationRepo) List(context.Context) ([]Location, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var all []Location
	for _, location := range r.locations {
		all = append(all, location)
	}
	// Deliberately reverse name order, so a missing core sort always fails.
	slices.SortFunc(all, func(a, b Location) int { return strings.Compare(b.NameKey, a.NameKey) })
	return all, nil
}

func (r *fakeLocationRepo) Get(_ context.Context, id string) (Location, error) {
	if r.getErr != nil {
		return Location{}, r.getErr
	}
	// Like PostgreSQL's uuid type, the lookup ignores the case of the id.
	location, ok := r.locations[strings.ToLower(id)]
	if !ok {
		return Location{}, ErrNotFound
	}
	return location, nil
}

func (r *fakeLocationRepo) FindByNameKey(_ context.Context, nameKey string) (Location, error) {
	if r.findErr != nil {
		return Location{}, r.findErr
	}
	for _, location := range r.locations {
		if location.NameKey == nameKey {
			return location, nil
		}
	}
	return Location{}, ErrNotFound
}

func (r *fakeLocationRepo) Create(ctx context.Context, location Location) (Location, error) {
	if err := r.beforeWrite(location); err != nil {
		return Location{}, err
	}
	location.ID = r.nextID
	r.locations[location.ID] = location
	r.created = append(r.created, location)
	return location, nil
}

func (r *fakeLocationRepo) Update(_ context.Context, location Location) (Location, error) {
	if err := r.beforeWrite(location); err != nil {
		return Location{}, err
	}
	if _, ok := r.locations[location.ID]; !ok {
		return Location{}, ErrNotFound
	}
	r.locations[location.ID] = location
	r.updated = append(r.updated, location)
	return location, nil
}

func (r *fakeLocationRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.locations[id]; !ok {
		return ErrNotFound
	}
	delete(r.locations, id)
	r.deleted = append(r.deleted, id)
	return nil
}

// UpdateNameKey enforces the unique name_key like the database does.
func (r *fakeLocationRepo) UpdateNameKey(_ context.Context, id, nameKey string) error {
	if err := r.nameKeyErr[id]; err != nil {
		return err
	}
	location, ok := r.locations[id]
	if !ok {
		return ErrNotFound
	}
	for _, existing := range r.locations {
		if existing.NameKey == nameKey && existing.ID != id {
			return ErrConflict
		}
	}
	location.NameKey = nameKey
	r.locations[id] = location
	r.nameKeysUpdated = append(r.nameKeysUpdated, id)
	return nil
}

// beforeWrite applies injected failures and the simulated race, and enforces
// the unique name_key like the database does.
func (r *fakeLocationRepo) beforeWrite(location Location) error {
	if r.writeErr != nil {
		return r.writeErr
	}
	if r.raceWinner != nil {
		r.locations[r.raceWinner.ID] = *r.raceWinner
		r.raceWinner = nil
	}
	for _, existing := range r.locations {
		if existing.NameKey == location.NameKey && existing.ID != location.ID {
			return ErrConflict
		}
	}
	return nil
}

// newLocationServiceOn returns the location use cases with a fake
// transaction on locations and events.
func newLocationServiceOn(locations *fakeLocationRepo, events *fakeEventRepo) *LocationService {
	return NewLocationService(&fakeTx{repos: Repos{Events: events, Locations: locations}}, locations)
}

func hall() Location {
	return Location{
		ID: hallID, Name: "Paul-Metz-Halle", NameKey: "paul-metz-halle",
		Street: "Volkhardtstraße 2", PostalCode: "90513", City: "Zirndorf",
		Latitude: 49.4424, Longitude: 10.9539,
		Precision: PrecisionBuilding,
	}
}

func park() Location {
	return Location{
		ID: parkID, Name: "Bibertpark", NameKey: "bibertpark",
		Street: "Bibertstraße", PostalCode: "90513", City: "Zirndorf",
		Latitude: 49.44, Longitude: 10.95,
		Precision: PrecisionArea,
	}
}

func assertConflictWith(t *testing.T, err error, want Location) {
	t.Helper()
	var conflict *LocationConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want *LocationConflictError", err)
	}
	if conflict.Existing != want {
		t.Errorf("conflicting location = %+v, want %+v", conflict.Existing, want)
	}
}

func TestSaveLocationCreatesNewLocationWhenIDIsEmpty(t *testing.T) {
	repo := newFakeLocationRepo()
	service := newLocationServiceOn(repo, newFakeEventRepo())

	saved, err := service.SaveLocation(context.Background(), "", validLocationInput())
	if err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}
	if saved.ID != newID || saved.NameKey != "paul-metz-halle" {
		t.Errorf("saved = %+v, want id %s and derived name key", saved, newID)
	}
	if len(repo.created) != 1 || len(repo.updated) != 0 {
		t.Errorf("created %d, updated %d, want 1 and 0", len(repo.created), len(repo.updated))
	}
}

func TestSaveLocationRejectsInvalidInputWithoutWriting(t *testing.T) {
	repo := newFakeLocationRepo()
	in := validLocationInput()
	in.Name = "   "

	_, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), "", in)

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
	if len(repo.created) != 0 {
		t.Error("an invalid location was written")
	}
}

func TestSaveLocationRejectsNameThatNormalizesToExistingKey(t *testing.T) {
	repo := newFakeLocationRepo(hall())
	in := validLocationInput()
	in.Name = " paul-metz-halle "

	_, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), "", in)

	assertConflictWith(t, err, hall())
	if len(repo.created) != 0 {
		t.Error("a conflicting location was written")
	}
}

func TestSaveLocationRejectsRenamingIntoAnotherLocationsName(t *testing.T) {
	repo := newFakeLocationRepo(hall(), park())
	in := validLocationInput()
	in.Name = "PAUL-METZ-HALLE"

	_, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), parkID, in)

	assertConflictWith(t, err, hall())
	if len(repo.updated) != 0 {
		t.Error("a conflicting rename was written")
	}
}

func TestSaveLocationKeepsOwnNameWithoutConflict(t *testing.T) {
	repo := newFakeLocationRepo(hall())
	in := validLocationInput()
	in.PostalCode, in.City = "90522", "Oberasbach"

	saved, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), hallID, in)
	if err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}
	if saved.ID != hallID || saved.PostalCode != in.PostalCode || saved.City != in.City {
		t.Errorf("saved = %+v, want same id with new postal code and city", saved)
	}
}

func TestSaveLocationUsesStoredIDWhenIDIsSpelledDifferently(t *testing.T) {
	repo := newFakeLocationRepo(hall())

	saved, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), strings.ToUpper(hallID), validLocationInput())
	if err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}
	if saved.ID != hallID || len(repo.created) != 0 {
		t.Errorf("saved = %+v, created %d, want an update of %s", saved, len(repo.created), hallID)
	}
}

func TestSaveLocationRenameKeepsID(t *testing.T) {
	repo := newFakeLocationRepo(hall())
	in := validLocationInput()
	in.Name = "Paul-Metz-Halle Zirndorf"

	saved, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), hallID, in)
	if err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}
	if saved.ID != hallID || repo.locations[hallID].Name != in.Name {
		t.Errorf("saved = %+v, want id %s with new name", saved, hallID)
	}
}

func TestSaveLocationReportsUnknownIDAsNotFound(t *testing.T) {
	repo := newFakeLocationRepo(hall())
	in := validLocationInput()
	in.Name = "   "

	_, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), otherID, in)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound before validation", err)
	}
}

func TestSaveLocationTurnsDatabaseUniqueViolationIntoConflict(t *testing.T) {
	winner := hall()
	tests := map[string]string{"create": "", "update": parkID}
	for name, id := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newFakeLocationRepo(park())
			repo.raceWinner = &winner

			_, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), id, validLocationInput())

			assertConflictWith(t, err, winner)
		})
	}
}

func TestSaveLocationPassesRepositoryFailuresOn(t *testing.T) {
	tests := map[string]struct {
		id     string
		inject func(*fakeLocationRepo)
	}{
		"get":                   {hallID, func(r *fakeLocationRepo) { r.getErr = errDatabaseDown }},
		"find by name key":      {"", func(r *fakeLocationRepo) { r.findErr = errDatabaseDown }},
		"create":                {"", func(r *fakeLocationRepo) { r.writeErr = errDatabaseDown }},
		"update":                {hallID, func(r *fakeLocationRepo) { r.writeErr = errDatabaseDown }},
		"reload after conflict": {"", func(r *fakeLocationRepo) { r.writeErr = ErrConflict }},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newFakeLocationRepo(hall())
			tt.inject(repo)
			in := validLocationInput()
			in.Name = "Neuer Ort"

			_, err := newLocationServiceOn(repo, newFakeEventRepo()).SaveLocation(context.Background(), tt.id, in)

			if err == nil {
				t.Fatal("SaveLocation returned no error")
			}
			var conflict *LocationConflictError
			if errors.As(err, &conflict) {
				t.Errorf("err = %v, want no conflict with an empty location", err)
			}
		})
	}
}

func TestGetLocationReturnsStoredLocationOrNotFound(t *testing.T) {
	service := newLocationServiceOn(newFakeLocationRepo(hall()), newFakeEventRepo())

	got, err := service.GetLocation(context.Background(), hallID)
	if err != nil || got != hall() {
		t.Errorf("GetLocation = %+v, %v, want the hall", got, err)
	}
	if _, err := service.GetLocation(context.Background(), otherID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetLocation(unknown) err = %v, want ErrNotFound", err)
	}
}

func TestListLocationsReturnsSortedLocations(t *testing.T) {
	service := newLocationServiceOn(newFakeLocationRepo(hall(), park()), newFakeEventRepo())

	got, err := service.ListLocations(context.Background())
	if err != nil {
		t.Fatalf("ListLocations: %v", err)
	}
	want := []Location{park(), hall()}
	if !slices.Equal(got, want) {
		t.Errorf("ListLocations = %v, want %v", got, want)
	}
}

func TestListLocationsPassesRepositoryFailureOn(t *testing.T) {
	repo := newFakeLocationRepo()
	repo.listErr = errDatabaseDown

	if _, err := newLocationServiceOn(repo, newFakeEventRepo()).ListLocations(context.Background()); !errors.Is(err, errDatabaseDown) {
		t.Errorf("err = %v, want %v", err, errDatabaseDown)
	}
}

func TestDeleteLocationRemovesAnUnusedLocationInOneTransaction(t *testing.T) {
	locations := newFakeLocationRepo(hall(), park())
	events := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))
	tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}

	err := NewLocationService(tx, locations).DeleteLocation(context.Background(), strings.ToUpper(parkID))
	if err != nil {
		t.Fatalf("DeleteLocation: %v", err)
	}
	if tx.runs != 1 || !slices.Equal(locations.deleted, []string{parkID}) {
		t.Errorf("transactions = %d, deleted = %v, want %s deleted in one transaction", tx.runs, locations.deleted, parkID)
	}
}

func TestDeleteLocationRefusesALocationEventsReferToWithTheirCount(t *testing.T) {
	locations := newFakeLocationRepo(hall(), park())
	events := newFakeEventRepo(
		storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}),
		storedEvent(t, concertID, "Konzert", EventTimes{StartDate: kirchweihMonday}),
		storedEvent(t, newEventID, "Flohmarkt", EventTimes{StartDate: kirchweihFriday}),
	)

	err := newLocationServiceOn(locations, events).DeleteLocation(context.Background(), hallID)

	var inUse *LocationInUseError
	if !errors.As(err, &inUse) || inUse.EventCount != 3 || !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want *LocationInUseError with 3 events", err)
	}
	if len(locations.deleted) != 0 {
		t.Errorf("deleted = %v, want nothing", locations.deleted)
	}
}

func TestDeleteLocationReportsAnUnknownLocationAsNotFound(t *testing.T) {
	locations := newFakeLocationRepo(hall())

	err := newLocationServiceOn(locations, newFakeEventRepo()).DeleteLocation(context.Background(), otherID)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if len(locations.deleted) != 0 {
		t.Errorf("deleted = %v, want nothing", locations.deleted)
	}
}

func TestDeleteLocationReportsAnEventAddedConcurrentlyAsPlainConflict(t *testing.T) {
	locations := newFakeLocationRepo(park())
	locations.deleteErr = ErrConflict

	err := newLocationServiceOn(locations, newFakeEventRepo()).DeleteLocation(context.Background(), parkID)

	var inUse *LocationInUseError
	if !errors.Is(err, ErrConflict) || errors.As(err, &inUse) {
		t.Errorf("err = %v, want ErrConflict without event count", err)
	}
}

func TestDeleteLocationPassesFailuresOn(t *testing.T) {
	tests := map[string]func(*fakeTx, *fakeLocationRepo, *fakeEventRepo){
		"begin":  func(tx *fakeTx, _ *fakeLocationRepo, _ *fakeEventRepo) { tx.beginErr = errDatabaseDown },
		"get":    func(_ *fakeTx, l *fakeLocationRepo, _ *fakeEventRepo) { l.getErr = errDatabaseDown },
		"count":  func(_ *fakeTx, _ *fakeLocationRepo, e *fakeEventRepo) { e.countErr = errDatabaseDown },
		"delete": func(_ *fakeTx, l *fakeLocationRepo, _ *fakeEventRepo) { l.deleteErr = errDatabaseDown },
	}
	for name, inject := range tests {
		t.Run(name, func(t *testing.T) {
			locations, events := newFakeLocationRepo(park()), newFakeEventRepo()
			tx := &fakeTx{repos: Repos{Events: events, Locations: locations}}
			inject(tx, locations, events)

			err := NewLocationService(tx, locations).DeleteLocation(context.Background(), parkID)

			if !errors.Is(err, errDatabaseDown) {
				t.Errorf("err = %v, want %v", err, errDatabaseDown)
			}
		})
	}
}
