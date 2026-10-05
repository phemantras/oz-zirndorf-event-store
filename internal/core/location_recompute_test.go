package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
)

// tent is a location whose stored name key is stale, as after a change of
// NormalizeKey.
func tent() Location {
	return Location{
		ID: tentID, Name: "Festzelt", NameKey: "alt",
		Street: "Marktplatz", PostalCode: "90513", City: "Zirndorf",
		Latitude: 49.443, Longitude: 10.954,
		Precision: PrecisionArea,
	}
}

// renamed returns location with name and the stored name key nameKey.
func renamed(location Location, name, nameKey string) Location {
	location.Name, location.NameKey = name, nameKey
	return location
}

// failureIDs returns the location IDs of every failure, one group per entry.
func failureIDs(failures []LocationRecomputeFailure) [][]string {
	var groups [][]string
	for _, failure := range failures {
		groups = append(groups, failure.LocationIDs)
	}
	return groups
}

func assertLocationsNeedReview(t *testing.T, service *LocationService, want map[string]bool) {
	t.Helper()
	entries, err := service.ListLocationEntries(context.Background())
	if err != nil {
		t.Fatalf("ListLocationEntries: %v", err)
	}
	for _, entry := range entries {
		if wanted, ok := want[entry.Location.ID]; ok && entry.NeedsReview != wanted {
			t.Errorf("%s: needs review = %v, want %v", entry.Location.ID, entry.NeedsReview, wanted)
		}
	}
}

func TestRecomputeNameKeysStoresOnlyStaleKeys(t *testing.T) {
	repo := newFakeLocationRepo(hall(), park(), tent())
	service := newLocationServiceOn(repo, newFakeEventRepo())

	failures, err := service.RecomputeNameKeys(context.Background())
	if err != nil || failures != nil {
		t.Fatalf("RecomputeNameKeys = %v, %v, want no failures", failures, err)
	}
	if !slices.Equal(repo.nameKeysUpdated, []string{tentID}) {
		t.Errorf("name keys updated for %v, want only %s", repo.nameKeysUpdated, tentID)
	}
	if got := repo.locations[tentID]; got.NameKey != "festzelt" || got.Name != tent().Name {
		t.Errorf("tent = %+v, want name key festzelt and the name unchanged", got)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: false, parkID: false, tentID: false})
}

func TestRecomputeNameKeysKeepsKeysOfCollidingLocationsAndMarksThem(t *testing.T) {
	tests := map[string]struct {
		locations []Location
		want      []string
	}{
		"pair, one already holding the key": {
			locations: []Location{hall(), renamed(tent(), "PAUL-METZ-HALLE", "alt")},
			want:      []string{hallID, tentID},
		},
		"three": {
			locations: []Location{
				renamed(hall(), "Festzelt", "a"),
				renamed(park(), "FESTZELT", "b"),
				renamed(tent(), "festzelt", "c"),
			},
			want: []string{hallID, parkID, tentID},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newFakeLocationRepo(tt.locations...)
			service := newLocationServiceOn(repo, newFakeEventRepo())

			failures, err := service.RecomputeNameKeys(context.Background())
			if err != nil {
				t.Fatalf("RecomputeNameKeys: %v", err)
			}
			if len(failures) != 1 || !slices.Equal(failures[0].LocationIDs, tt.want) {
				t.Fatalf("failures = %v, want one for %v", failureIDs(failures), tt.want)
			}
			key := NormalizeKey(tt.locations[0].Name)
			if !errors.Is(failures[0].Err, ErrConflict) || !strings.Contains(failures[0].Err.Error(), key) {
				t.Errorf("err = %v, want ErrConflict naming the key %q", failures[0].Err, key)
			}
			if len(repo.nameKeysUpdated) != 0 {
				t.Errorf("name keys updated for %v, want none", repo.nameKeysUpdated)
			}
			for _, location := range tt.locations {
				if got := repo.locations[location.ID].NameKey; got != location.NameKey {
					t.Errorf("%s: name key = %q, want the stored %q", location.ID, got, location.NameKey)
				}
			}
			want := map[string]bool{}
			for _, id := range tt.want {
				want[id] = true
			}
			assertLocationsNeedReview(t, service, want)
		})
	}
}

func TestRecomputeNameKeysMarksTheHolderWhenTheDatabaseReportsAConflict(t *testing.T) {
	// The tent still holds the new key of the hall and frees it only after
	// the hall has tried, since locations are recomputed in name order.
	stale := renamed(hall(), "Paul-Metz-Halle", "alt")
	holder := renamed(tent(), "Zeltplatz", "paul-metz-halle")
	repo := newFakeLocationRepo(stale, holder, park())
	service := newLocationServiceOn(repo, newFakeEventRepo())

	failures, err := service.RecomputeNameKeys(context.Background())
	if err != nil {
		t.Fatalf("RecomputeNameKeys: %v", err)
	}
	if got := failureIDs(failures); len(got) != 1 || !slices.Equal(got[0], []string{hallID, tentID}) {
		t.Fatalf("failures = %v, want one for %s and %s", got, hallID, tentID)
	}
	if !errors.Is(failures[0].Err, ErrConflict) || !strings.Contains(failures[0].Err.Error(), "paul-metz-halle") {
		t.Errorf("err = %v, want ErrConflict naming the key", failures[0].Err)
	}
	if got := repo.locations[hallID].NameKey; got != "alt" {
		t.Errorf("hall name key = %q, want the stored alt", got)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: true, tentID: true, parkID: false})
}

func TestRecomputeNameKeysMarksAloneALocationWhoseConflictHasNoHolderAnyMore(t *testing.T) {
	repo := newFakeLocationRepo(tent())
	repo.nameKeyErr = map[string]error{tentID: fmt.Errorf("update name key: %w", ErrConflict)}
	service := newLocationServiceOn(repo, newFakeEventRepo())

	failures, err := service.RecomputeNameKeys(context.Background())
	if err != nil {
		t.Fatalf("RecomputeNameKeys: %v", err)
	}
	if got := failureIDs(failures); len(got) != 1 || !slices.Equal(got[0], []string{tentID}) {
		t.Errorf("failures = %v, want one for %s", got, tentID)
	}
	assertLocationsNeedReview(t, service, map[string]bool{tentID: true})
}

func TestRecomputeNameKeysSkipsLocationsDeletedMeanwhile(t *testing.T) {
	repo := newFakeLocationRepo(tent())
	repo.nameKeyErr = map[string]error{tentID: fmt.Errorf("update name key: %w", ErrNotFound)}
	service := newLocationServiceOn(repo, newFakeEventRepo())

	failures, err := service.RecomputeNameKeys(context.Background())

	if err != nil || failures != nil {
		t.Errorf("RecomputeNameKeys = %v, %v, want no failures", failures, err)
	}
	assertLocationsNeedReview(t, service, map[string]bool{tentID: false})
}

func TestRecomputeNameKeysStopsAtFailuresWithoutMarking(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := map[string]struct {
		ctx    context.Context
		inject func(*fakeLocationRepo)
		want   error
	}{
		"list": {context.Background(), func(r *fakeLocationRepo) { r.listErr = errDatabaseDown }, errDatabaseDown},
		"store": {context.Background(), func(r *fakeLocationRepo) {
			r.nameKeyErr = map[string]error{tentID: errDatabaseDown}
		}, errDatabaseDown},
		"find the holder": {context.Background(), func(r *fakeLocationRepo) {
			r.nameKeyErr = map[string]error{tentID: ErrConflict}
			r.findErr = errDatabaseDown
		}, errDatabaseDown},
		"ended context": {canceled, func(*fakeLocationRepo) {}, context.Canceled},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Hall and park collide before the tent, whose key is stale, comes
			// up in name order.
			repo := newFakeLocationRepo(hall(), renamed(park(), "Paul-Metz-Halle", "bibertpark"), renamed(tent(), "Zeltplatz", "alt"))
			tt.inject(repo)
			service := newLocationServiceOn(repo, newFakeEventRepo())

			failures, err := service.RecomputeNameKeys(tt.ctx)

			if !errors.Is(err, tt.want) || failures != nil {
				t.Errorf("RecomputeNameKeys = %v, %v, want only %v", failureIDs(failures), err, tt.want)
			}
			repo.listErr = nil
			assertLocationsNeedReview(t, service, map[string]bool{hallID: false, parkID: false, tentID: false})
		})
	}
}

func TestListLocationEntriesReturnsSortedLocationsWithReviewMarks(t *testing.T) {
	repo := newFakeLocationRepo(hall(), renamed(park(), "PAUL-METZ-HALLE", "bibertpark"), tent())
	service := newLocationServiceOn(repo, newFakeEventRepo())
	if _, err := service.RecomputeNameKeys(context.Background()); err != nil {
		t.Fatalf("RecomputeNameKeys: %v", err)
	}

	entries, err := service.ListLocationEntries(context.Background())
	if err != nil {
		t.Fatalf("ListLocationEntries: %v", err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, fmt.Sprintf("%s %v", entry.Location.ID, entry.NeedsReview))
	}
	want := []string{tentID + " false", hallID + " true", parkID + " true"}
	if !slices.Equal(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
}

func TestListLocationEntriesPassesRepositoryFailureOn(t *testing.T) {
	repo := newFakeLocationRepo()
	repo.listErr = errDatabaseDown

	if _, err := newLocationServiceOn(repo, newFakeEventRepo()).ListLocationEntries(context.Background()); !errors.Is(err, errDatabaseDown) {
		t.Errorf("err = %v, want %v", err, errDatabaseDown)
	}
}

// collidingLocationService returns the use cases after a recomputation that
// marked hall and park as one collision group; the tent is not marked. An
// event refers to the hall.
func collidingLocationService(t *testing.T) (*LocationService, *fakeLocationRepo) {
	t.Helper()
	repo := newFakeLocationRepo(hall(), renamed(park(), "Paul-Metz-Halle", "bibertpark"), renamed(tent(), "Festzelt", "festzelt"))
	events := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))
	service := newLocationServiceOn(repo, events)
	if _, err := service.RecomputeNameKeys(context.Background()); err != nil {
		t.Fatalf("RecomputeNameKeys: %v", err)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: true, parkID: true, tentID: false})
	return service, repo
}

func TestSuccessfulSaveOfALocationClearsItsWholeCollisionGroup(t *testing.T) {
	service, _ := collidingLocationService(t)
	ctx := context.Background()
	in := validLocationInput()
	in.Name = "Bibertpark"

	in.PostalCode = "kaputt"
	if _, err := service.SaveLocation(ctx, parkID, in); err == nil {
		t.Fatal("SaveLocation accepted an invalid location")
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: true, parkID: true})

	in.PostalCode = "90513"
	in.Name = "Festzelt"
	var conflict *LocationConflictError
	if _, err := service.SaveLocation(ctx, parkID, in); !errors.As(err, &conflict) {
		t.Fatalf("SaveLocation err = %v, want *LocationConflictError", err)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: true, parkID: true})
	in.Name = "Bibertpark"

	in.PostalCode = "90513"
	if _, err := service.SaveLocation(ctx, strings.ToUpper(parkID), in); err != nil {
		t.Fatalf("SaveLocation: %v", err)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: false, parkID: false, tentID: false})
}

func TestSuccessfulDeleteOfALocationClearsItsWholeCollisionGroup(t *testing.T) {
	service, repo := collidingLocationService(t)
	ctx := context.Background()

	repo.deleteErr = errDatabaseDown
	if err := service.DeleteLocation(ctx, parkID); err == nil {
		t.Fatal("DeleteLocation succeeded despite the failure")
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: true, parkID: true})

	repo.deleteErr = nil
	var inUse *LocationInUseError
	if err := service.DeleteLocation(ctx, hallID); !errors.As(err, &inUse) {
		t.Fatalf("DeleteLocation err = %v, want *LocationInUseError", err)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: true, parkID: true})

	if err := service.DeleteLocation(ctx, strings.ToUpper(parkID)); err != nil {
		t.Fatalf("DeleteLocation: %v", err)
	}
	assertLocationsNeedReview(t, service, map[string]bool{hallID: false})
}

func TestOverlappingCollisionGroupsAreClearedTogether(t *testing.T) {
	service := newLocationServiceOn(newFakeLocationRepo(hall(), park(), tent()), newFakeEventRepo())
	service.markForReview([]string{hallID, parkID})
	service.markForReview([]string{parkID, tentID})

	service.clearReview(hallID)

	assertLocationsNeedReview(t, service, map[string]bool{hallID: false, parkID: false, tentID: false})
}

func TestLocationReviewMarksAreSafeForConcurrentUse(t *testing.T) {
	service := newLocationServiceOn(newFakeLocationRepo(), newFakeEventRepo())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			service.markForReview([]string{hallID, parkID})
			service.clearReview(parkID)
			_ = service.needsReview(hallID)
		})
	}
	wg.Wait()
}
