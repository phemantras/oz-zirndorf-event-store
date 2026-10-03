package admin

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// Routes of the inline input for a new location on the event form (FR-15).
// htmx swaps their fragments into the event form, which itself is never
// sent or reloaded.
const (
	inlineLocationPath       = eventsPath + "/new-location"
	inlineLocationCancelPath = inlineLocationPath + "/cancel"
)

// Element ids the fragments rely on. The inline fields belong to a separate,
// empty form after the event form, so saving the event never sends them.
const (
	newLocationAreaID      = "new-location"
	locationChoiceID       = "location-choice"
	inlineLocationFormID   = "new-location-form"
	inlineLocationIDPrefix = newLocationAreaID + "-"
)

// Names of the fragment templates in new_location.html.
const (
	newLocationOpenTemplate   = "newLocationOpen"
	newLocationClosedTemplate = "newLocationClosed"
	newLocationSavedTemplate  = "newLocationSaved"
)

// msgLocationCreatedFormat confirms the new location, which is now selected.
const msgLocationCreatedFormat = "Ort „%s“ angelegt und ausgewählt."

// newLocationArea is the data of the inline input, open with its fields or
// closed with an optional notice.
type newLocationArea struct {
	Fields   locationFields
	Conflict *locationConflict
	Notice   string
}

// newLocationSaved is the answer to a saved location: the closed input and
// the location choice with the new location selected, swapped out of band.
type newLocationSaved struct {
	Area   newLocationArea
	Choice locationChoice
}

func (h *handler) openInlineLocation(w http.ResponseWriter, _ *http.Request) {
	h.renderFragment(w, newLocationTemplate, newLocationOpenTemplate, http.StatusOK, openNewLocationArea(core.LocationInput{}))
}

func (h *handler) cancelInlineLocation(w http.ResponseWriter, _ *http.Request) {
	h.renderFragment(w, newLocationTemplate, newLocationClosedTemplate, http.StatusOK, newLocationArea{})
}

// createInlineLocation saves the location through the same use case and
// with the same messages as the location form: field messages (422), name
// conflict (409). On success it selects the new location in the choice.
func (h *handler) createInlineLocation(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLocationFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	values := inputFromForm(r.PostForm)
	saved, err := h.locations.SaveLocation(r.Context(), "", withDecimalPoints(values))

	area := openNewLocationArea(values)
	var validation *core.ValidationError
	var conflict *core.LocationConflictError
	switch {
	case err == nil:
		h.renderSavedLocation(w, r, saved)
	case errors.As(err, &validation):
		area.Fields.Errors = fieldErrorMessages(validation.Fields, locationFieldMessages)
		h.renderFragment(w, newLocationTemplate, newLocationOpenTemplate, http.StatusUnprocessableEntity, area)
	case errors.As(err, &conflict):
		area.Conflict = locationConflictOf(conflict)
		h.renderFragment(w, newLocationTemplate, newLocationOpenTemplate, http.StatusConflict, area)
	default:
		h.failLocationRequest(w, err)
	}
}

// renderSavedLocation closes the input with a notice and replaces the
// location choice, with the saved location selected.
func (h *handler) renderSavedLocation(w http.ResponseWriter, r *http.Request, saved core.Location) {
	locations, err := h.locations.ListLocations(r.Context())
	if err != nil {
		h.failLocationRequest(w, err)
		return
	}
	choice := locationChoiceOf(locations, saved.ID, "")
	choice.OutOfBand = true
	h.renderFragment(w, newLocationTemplate, newLocationSavedTemplate, http.StatusOK, newLocationSaved{
		Area:   newLocationArea{Notice: fmt.Sprintf(msgLocationCreatedFormat, saved.Name)},
		Choice: choice,
	})
}

// openNewLocationArea returns the open input filled with values.
func openNewLocationArea(values core.LocationInput) newLocationArea {
	return newLocationArea{Fields: newLocationFields(locationFields{
		IDPrefix: inlineLocationIDPrefix,
		FormID:   inlineLocationFormID,
		Values:   values,
	})}
}
