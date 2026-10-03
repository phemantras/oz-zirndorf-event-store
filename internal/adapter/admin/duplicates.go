package admin

import (
	"net/url"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// The button that confirms the duplicate warning sends this field and
// value; any other form is saved only without suspected duplicates.
const (
	duplicatesField = "duplicates"
	duplicatesAllow = "allow"
)

// msgDuplicateSuspect introduces the events the input may duplicate and
// says where to decide, since the buttons sit after the plain save button.
const msgDuplicateSuspect = "Möglicherweise gibt es dieses Event schon: gleicher Titel am selben Tag am selben Ort. " +
	"„Trotzdem speichern“ und „Abbrechen“ stehen unten neben „Speichern“."

// duplicateWarning is the data of the warning on the event form: the
// events the input may duplicate and the field the confirm button sends.
type duplicateWarning struct {
	Message      string
	Candidates   []duplicateCandidate
	ConfirmField string
	ConfirmValue string
}

// duplicateCandidate is a stored event the warning links to.
type duplicateCandidate struct {
	Title string
	URL   string
	Start string
	// Status is statusArchived for an archived event, otherwise empty.
	Status string
}

// duplicatePolicyOf returns AllowDuplicates only when the form was sent
// with the confirm button of the warning.
func duplicatePolicyOf(form url.Values) core.DuplicatePolicy {
	if form.Get(duplicatesField) == duplicatesAllow {
		return core.AllowDuplicates
	}
	return core.RejectDuplicates
}

// duplicateWarningOf returns the warning for the candidates, their archive
// status at the time of clock; nil when there are none.
func duplicateWarningOf(candidates []core.Event, clock core.Clock) *duplicateWarning {
	if len(candidates) == 0 {
		return nil
	}
	warning := &duplicateWarning{Message: msgDuplicateSuspect, ConfirmField: duplicatesField, ConfirmValue: duplicatesAllow}
	for _, event := range candidates {
		candidate := duplicateCandidate{
			Title: event.Title,
			URL:   eventURL(event.ID),
			Start: formatMoment(event.Times.StartDate, event.Times.StartTime, event.Times.AllDay),
		}
		if event.Period.IsOver(clock) {
			candidate.Status = statusArchived
		}
		warning.Candidates = append(warning.Candidates, candidate)
	}
	return warning
}
