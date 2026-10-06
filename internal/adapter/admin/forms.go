package admin

import (
	"fmt"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// German texts shared by the forms and pages.
const (
	msgNoPersonalData = "Bitte keine Privatpersonen, Kontaktpersonen oder Telefonnummern eintragen."
	// msgFieldInvalid covers a field problem without a specific message.
	msgFieldInvalid = "Bitte diese Angabe prüfen."
	// msgTooLongFormat names the limit of a text that is too long.
	msgTooLongFormat = "Höchstens %d Zeichen."
	// msgTooManyFormat names the limit of the timetable, the only list the
	// core limits.
	msgTooManyFormat = "Höchstens %d Programmpunkte."
)

// selectOption is one option of a select field.
type selectOption struct {
	Value    string
	Label    string
	Selected bool
}

// deletePathSuffix turns the path of a record into the path that deletes
// it.
const deletePathSuffix = "/delete"

// logKeyID is the log attribute that names the deleted record.
const logKeyID = "id"

// deleteForm is the data of the delete button on the edit page of a record:
// its own form after the edit form, so Enter in the edit form never
// deletes, and a confirmation question before htmx sends it.
type deleteForm struct {
	Action  string
	Confirm string
	// Include selects the form whose unsaved input htmx sends along, so a
	// refused delete shows it again; empty when nothing is shown again.
	Include string
}

// notFoundPage is the data of the German 404 page.
type notFoundPage struct {
	Message   string
	BackURL   string
	BackLabel string
}

// fieldErrorMessages returns the German message per rejected field from the
// messages of one form. Each form has its own messages, because the same
// field name, such as note, means different things on different forms.
func fieldErrorMessages(fields []core.FieldError, messages map[core.FieldError]string) map[string]string {
	byField := make(map[string]string, len(fields))
	for _, field := range fields {
		byField[field.Field] = fieldErrorMessage(field, messages)
	}
	return byField
}

// fieldErrorMessage returns the German message for one rejected field:
// the form's own message for field and problem, whatever the limit; else
// for a limit the message that names it; else the generic one.
func fieldErrorMessage(field core.FieldError, messages map[core.FieldError]string) string {
	if message, ok := messages[core.FieldError{Field: field.Field, Problem: field.Problem}]; ok {
		return message
	}
	switch field.Problem {
	case core.ProblemTooLong:
		return fmt.Sprintf(msgTooLongFormat, field.Limit)
	case core.ProblemTooMany:
		return fmt.Sprintf(msgTooManyFormat, field.Limit)
	default:
		return msgFieldInvalid
	}
}
