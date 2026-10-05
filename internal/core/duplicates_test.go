package core

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// archivedTwinID is an archived event a year earlier with another start.
const archivedTwinID = "0192f0b1-0000-7000-8000-000000000104"

func candidateIDs(candidates []Event) []string {
	var ids []string
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
	}
	return ids
}

func TestFindDuplicateCandidatesComparesTitleKeyStartDateAndLocation(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	otherDay := storedEvent(t, concertID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihMonday})
	otherPlace := storedEvent(t, newEventID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	otherPlace.LocationID = parkID
	otherTitle := storedEvent(t, archivedTwinID, "Flohmarkt", EventTimes{StartDate: kirchweihFriday})
	repo := newFakeEventRepo(market, otherDay, otherPlace, otherTitle)
	event, problems := newEvent(EventInput{
		Title: "  KIRCHWEIHmarkt ", Type: string(EventTypeMarket), LocationID: hallID,
		StartDate: "2026-10-16", StartTime: "20:00", Source: EventSource{Description: "Plakat"},
	})
	if problems != nil {
		t.Fatalf("newEvent problems = %v", problems)
	}

	candidates, err := FindDuplicateCandidates(context.Background(), repo, event)

	if err != nil || !slices.Equal(candidateIDs(candidates), []string{marketID}) {
		t.Errorf("FindDuplicateCandidates = %v, %v, want only %s", candidateIDs(candidates), err, marketID)
	}
}

func TestFindDuplicateCandidatesSkipsTheEventItselfAndOrdersByStart(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	evening := storedEvent(t, concertID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday, StartTime: localTime(19, 0)})
	morning := storedEvent(t, newEventID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday, StartTime: localTime(9, 0)})
	repo := newFakeEventRepo(market, evening, morning)

	candidates, err := FindDuplicateCandidates(context.Background(), repo, evening)

	if err != nil || !slices.Equal(candidateIDs(candidates), []string{marketID, newEventID}) {
		t.Errorf("FindDuplicateCandidates = %v, %v, want %s then %s", candidateIDs(candidates), err, marketID, newEventID)
	}
}

func TestFindDuplicateCandidatesPassesRepositoryFailuresOn(t *testing.T) {
	repo := newFakeEventRepo()
	repo.findErr = errDatabaseDown

	_, err := FindDuplicateCandidates(context.Background(), repo, Event{})

	if !errors.Is(err, errDatabaseDown) {
		t.Errorf("err = %v, want %v", err, errDatabaseDown)
	}
}

func TestSaveEventRejectsSuspectedDuplicateIncludingArchived(t *testing.T) {
	archived := storedEvent(t, archivedTwinID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	repo := newFakeEventRepo(archived)
	in := validEventInput()
	in.Title, in.StartTime = " kirchweihmarkt ", "20:00"

	_, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", in, RejectDuplicates)

	var suspect *DuplicateSuspectError
	if !errors.As(err, &suspect) || !errors.Is(err, ErrDuplicateSuspect) {
		t.Fatalf("err = %v, want *DuplicateSuspectError", err)
	}
	if !slices.Equal(candidateIDs(suspect.Candidates), []string{archivedTwinID}) {
		t.Errorf("candidates = %v, want %s", candidateIDs(suspect.Candidates), archivedTwinID)
	}
	if len(repo.created) != 0 {
		t.Error("a suspected duplicate was written")
	}
}

func TestSaveEventWritesDuplicateWhenAllowed(t *testing.T) {
	repo := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))

	saved, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", validEventInput(), AllowDuplicates)

	if err != nil || saved.ID != newEventID || repo.findCalls != 0 {
		t.Errorf("SaveEvent = %+v, %v, %d lookups, want a create without lookup", saved, err, repo.findCalls)
	}
}

func TestSaveEventWarnsWhenAnEditMakesAnotherEventsTwin(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	concert := storedEvent(t, concertID, "Konzert", EventTimes{StartDate: kirchweihMonday})
	repo := newFakeEventRepo(market, concert)
	service := newTestEventService(repo, hall())
	ctx := context.Background()

	if _, err := service.SaveEvent(ctx, marketID, validEventInput(), RejectDuplicates); err != nil {
		t.Errorf("resaving the market itself: %v", err)
	}
	_, err := service.SaveEvent(ctx, concertID, validEventInput(), RejectDuplicates)
	var suspect *DuplicateSuspectError
	if !errors.As(err, &suspect) || !slices.Equal(candidateIDs(suspect.Candidates), []string{marketID}) {
		t.Errorf("err = %v, want suspect %s", err, marketID)
	}
}

func TestSaveEventReportsFieldProblemsBeforeDuplicates(t *testing.T) {
	repo := newFakeEventRepo(storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday}))
	in := validEventInput()
	in.Source.Description = ""

	_, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", in, RejectDuplicates)

	if !errors.Is(err, ErrValidation) || errors.Is(err, ErrDuplicateSuspect) || repo.findCalls != 0 {
		t.Errorf("err = %v after %d lookups, want only the field problem", err, repo.findCalls)
	}
}

func TestSaveEventPassesDuplicateLookupFailuresOn(t *testing.T) {
	repo := newFakeEventRepo()
	repo.findErr = errDatabaseDown

	_, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", validEventInput(), RejectDuplicates)

	if !errors.Is(err, errDatabaseDown) || len(repo.created) != 0 {
		t.Errorf("err = %v, created %d, want %v and nothing written", err, len(repo.created), errDatabaseDown)
	}
}

func TestDuplicateSuspectErrorNamesTheCandidates(t *testing.T) {
	err := &DuplicateSuspectError{Candidates: []Event{{ID: marketID}, {ID: concertID}}}

	want := "suspected duplicate of events " + marketID + ", " + concertID
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestNewEventDerivesTheTitleKey(t *testing.T) {
	in := validEventInput()
	in.Title = " Kirchweih  MARKT "

	event, problems := newEvent(in)

	if problems != nil || event.TitleKey != "kirchweih markt" || event.Title != "Kirchweih  MARKT" {
		t.Errorf("newEvent = title %q, key %q, %v", event.Title, event.TitleKey, problems)
	}
}

func TestRecomputeDerivedWritesAMissingTitleKey(t *testing.T) {
	keyless := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	keyless.TitleKey = ""
	repo := newFakeEventRepo(keyless)

	failures, err := newTestEventService(repo, hall()).RecomputeDerived(context.Background())

	if err != nil || failures != nil {
		t.Fatalf("RecomputeDerived = %v, %v, want no failures", failures, err)
	}
	stored := repo.events[marketID]
	if stored.TitleKey != "kirchweihmarkt" || !stored.Period.End.Equal(keyless.Period.End) {
		t.Errorf("stored = key %q, period %+v, want key kirchweihmarkt and the same period", stored.TitleKey, stored.Period)
	}
	if !slices.Equal(repo.derivedUpdated, []string{marketID}) {
		t.Errorf("derived values updated for %v, want %s", repo.derivedUpdated, marketID)
	}
}

func TestStoredEventsWithEqualKeysButOtherYearsAreNoCandidates(t *testing.T) {
	lastYear := storedEvent(t, archivedTwinID, "Kirchweihmarkt", EventTimes{StartDate: LocalDate{2025, time.October, 16}})
	repo := newFakeEventRepo(lastYear)

	if _, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), "", validEventInput(), RejectDuplicates); err != nil {
		t.Errorf("SaveEvent: %v, want no warning for another year", err)
	}
}

func TestEditingAConfirmedTwinWithoutChangingItsKeyDoesNotWarnAgain(t *testing.T) {
	market := storedEvent(t, marketID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday})
	twin := storedEvent(t, concertID, "Kirchweihmarkt", EventTimes{StartDate: kirchweihFriday, StartTime: localTime(20, 0)})
	repo := newFakeEventRepo(market, twin)
	in := validEventInput()
	in.StartTime, in.Note = "20:00", "Zweite Vorstellung"

	_, err := newTestEventService(repo, hall()).SaveEvent(context.Background(), concertID, in, RejectDuplicates)

	if err != nil || repo.findCalls != 0 || repo.events[concertID].Note != "Zweite Vorstellung" {
		t.Errorf("SaveEvent err = %v after %d lookups, want the note saved without a check", err, repo.findCalls)
	}
}

func TestDuplicateConfirmationOfChangesOnlyWithTheDuplicateKeyInput(t *testing.T) {
	base := validEventInput()
	confirmation := DuplicateConfirmationOf(base)
	if confirmation == "" {
		t.Fatal("confirmation is empty")
	}

	sameKey := base
	sameKey.Title, sameKey.Note, sameKey.StartTime = "  KIRCHWEIHmarkt ", "anders", "19:00"
	if got := DuplicateConfirmationOf(sameKey); got != confirmation {
		t.Errorf("confirmation for the same duplicate key = %q, want %q", got, confirmation)
	}

	changes := map[string]func(*EventInput){
		"title":    func(in *EventInput) { in.Title = "Flohmarkt" },
		"date":     func(in *EventInput) { in.StartDate = "2026-10-19" },
		"location": func(in *EventInput) { in.LocationID = parkID },
		// The parts are separated unambiguously, so text cannot move between them.
		"shifted": func(in *EventInput) { in.Title, in.StartDate = base.Title+"2026", "-10-16" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			changed := base
			change(&changed)
			if DuplicateConfirmationOf(changed) == confirmation {
				t.Errorf("confirmation did not change with the %s", name)
			}
		})
	}
}
