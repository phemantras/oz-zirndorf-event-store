package admin

import "github.com/phemantras/oz-zirndorf-event-store/internal/core"

// German texts shared by the forms and pages.
const (
	msgNoPersonalData = "Bitte keine Privatpersonen, Kontaktpersonen oder Telefonnummern eintragen."
	// msgFieldInvalid covers a field problem without a specific message.
	msgFieldInvalid = "Bitte diese Angabe prüfen."
)

// selectOption is one option of a select field.
type selectOption struct {
	Value    string
	Label    string
	Selected bool
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
		message, ok := messages[field]
		if !ok {
			message = msgFieldInvalid
		}
		byField[field.Field] = message
	}
	return byField
}
