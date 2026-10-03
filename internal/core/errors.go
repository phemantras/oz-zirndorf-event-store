package core

import (
	"errors"
	"strings"
)

// Typed errors of the core. Only adapters translate them into HTTP status
// codes, problem details or German messages.
var (
	// ErrValidation means the input breaks a domain rule.
	ErrValidation = errors.New("validation failed")
	// ErrNotFound means the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means the input collides with an existing entity.
	ErrConflict = errors.New("conflict")
)

// FieldProblem is the machine-readable reason a field was rejected.
type FieldProblem string

// Field problems reported in a ValidationError.
const (
	ProblemMissing     FieldProblem = "missing"
	ProblemNotANumber  FieldProblem = "notANumber"
	ProblemOutOfRange  FieldProblem = "outOfRange"
	ProblemUnknownCode FieldProblem = "unknownCode"
	// ProblemInvalidFormat means the text does not have the required shape,
	// such as a postal code that is not five digits.
	ProblemInvalidFormat FieldProblem = "invalidFormat"
	// ProblemConflictsWithAllDay means a time was given for an all-day event.
	ProblemConflictsWithAllDay FieldProblem = "conflictsWithAllDay"
	// ProblemNotAfterStart means the effective end is not after the
	// effective start.
	ProblemNotAfterStart FieldProblem = "notAfterStart"
	// ProblemNotAfterStartRepeatedHour is ProblemNotAfterStart where start or
	// end lies in the hour that repeats when daylight saving time ends; the
	// earlier offset was taken, which may not be what was meant.
	ProblemNotAfterStartRepeatedHour FieldProblem = "notAfterStartRepeatedHour"
	// ProblemNonexistentTime means the local time falls into the gap when
	// daylight saving time starts.
	ProblemNonexistentTime FieldProblem = "nonexistentTime"
	// ProblemNotFound means the field refers to an entity that does not
	// exist, such as an unknown location of an event.
	ProblemNotFound FieldProblem = "notFound"
)

// FieldError names one rejected field and why it was rejected.
type FieldError struct {
	Field   string
	Problem FieldProblem
}

// ValidationError lists every rejected field of an input. It matches
// ErrValidation with errors.Is.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	problems := make([]string, 0, len(e.Fields))
	for _, field := range e.Fields {
		problems = append(problems, field.Field+" "+string(field.Problem))
	}
	return ErrValidation.Error() + ": " + strings.Join(problems, ", ")
}

func (e *ValidationError) Unwrap() error { return ErrValidation }

// LocationConflictError reports that another location already has the same
// name key. It matches ErrConflict with errors.Is.
type LocationConflictError struct {
	Existing Location
}

func (e *LocationConflictError) Error() string {
	return "location name conflicts with existing location " + e.Existing.ID
}

func (e *LocationConflictError) Unwrap() error { return ErrConflict }
