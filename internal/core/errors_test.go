package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidationErrorIsErrValidationAndNamesFields(t *testing.T) {
	err := fmt.Errorf("save: %w", &ValidationError{Fields: []FieldError{
		{Field: LocationFieldName, Problem: ProblemMissing},
		{Field: LocationFieldLatitude, Problem: ProblemOutOfRange},
	}})

	if !errors.Is(err, ErrValidation) {
		t.Errorf("errors.Is(%v, ErrValidation) = false, want true", err)
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
		t.Errorf("%v also matches another sentinel", err)
	}
	var validation *ValidationError
	if !errors.As(err, &validation) || len(validation.Fields) != 2 {
		t.Fatalf("errors.As did not recover the field errors from %v", err)
	}
	for _, part := range []string{"name", "missing", "latitude", "outOfRange"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("message %q does not mention %q", err.Error(), part)
		}
	}
}

func TestLocationConflictErrorIsErrConflictAndCarriesExistingLocation(t *testing.T) {
	existing := Location{ID: "0192f0b1-0000-7000-8000-000000000001", Name: "Paul-Metz-Halle"}
	err := fmt.Errorf("save: %w", &LocationConflictError{Existing: existing})

	if !errors.Is(err, ErrConflict) {
		t.Errorf("errors.Is(%v, ErrConflict) = false, want true", err)
	}
	if errors.Is(err, ErrValidation) {
		t.Errorf("%v also matches ErrValidation", err)
	}
	var conflict *LocationConflictError
	if !errors.As(err, &conflict) || conflict.Existing != existing {
		t.Fatalf("errors.As did not recover the existing location from %v", err)
	}
	if !strings.Contains(err.Error(), existing.ID) {
		t.Errorf("message %q does not name the existing location id", err.Error())
	}
}

func TestFieldProblemCodesOfTheTimeModel(t *testing.T) {
	codes := map[FieldProblem]string{
		ProblemConflictsWithAllDay:       "conflictsWithAllDay",
		ProblemNotAfterStart:             "notAfterStart",
		ProblemNotAfterStartRepeatedHour: "notAfterStartRepeatedHour",
		ProblemNonexistentTime:           "nonexistentTime",
	}
	for problem, want := range codes {
		if string(problem) != want {
			t.Errorf("problem code = %q, want %q", problem, want)
		}
	}
}

func TestFieldProblemNotFoundCode(t *testing.T) {
	if ProblemNotFound != "notFound" {
		t.Errorf("ProblemNotFound = %q, want notFound", ProblemNotFound)
	}
}

func TestLocationInUseErrorIsErrConflictAndCarriesEventCount(t *testing.T) {
	err := fmt.Errorf("delete: %w", &LocationInUseError{EventCount: 3})

	if !errors.Is(err, ErrConflict) {
		t.Errorf("errors.Is(%v, ErrConflict) = false, want true", err)
	}
	if errors.Is(err, ErrValidation) || errors.Is(err, ErrNotFound) {
		t.Errorf("%v also matches another sentinel", err)
	}
	var inUse *LocationInUseError
	if !errors.As(err, &inUse) || inUse.EventCount != 3 {
		t.Fatalf("errors.As did not recover the event count from %v", err)
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("message %q does not name the event count", err.Error())
	}
}

func TestValidationErrorNamesTheExceededLimit(t *testing.T) {
	err := &ValidationError{Fields: []FieldError{
		{Field: EventFieldTitle, Problem: ProblemTooLong, Limit: MaxTitleLength},
		{Field: EventFieldNote, Problem: ProblemMissing},
	}}

	want := "validation failed: title tooLong 200, note missing"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
