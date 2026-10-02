package admin

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// LocationUseCases are the core use cases behind the location pages. The
// admin never gets the repository, so every write goes through the core
// (AD-6).
type LocationUseCases interface {
	SaveLocation(ctx context.Context, id string, in core.LocationInput) (core.Location, error)
	GetLocation(ctx context.Context, id string) (core.Location, error)
	ListLocations(ctx context.Context) ([]core.Location, error)
}

// Routes of the location pages.
const (
	locationsPath       = adminPathPrefix + "/locations"
	newLocationPath     = locationsPath + "/new"
	locationIDParam     = "id"
	locationPathPattern = locationsPath + "/{" + locationIDParam + "}"
	// maxLocationFormBytes bounds the location form body; a real form with
	// a long note stays far below.
	maxLocationFormBytes = 16 << 10
)

// Admin input accepts a decimal comma; the core only knows the point.
const (
	decimalComma = ","
	decimalPoint = "."
)

// Coordinates are shown as float64 in the shortest form that reads back
// exactly, with a decimal point.
const (
	coordinateFormat    = 'f'
	coordinatePrecision = -1
	coordinateBitSize   = 64
)

// German texts of the location pages.
const (
	headingNewLocation  = "Neuer Ort"
	headingEditLocation = "Ort bearbeiten"
	msgNameConflict     = "Es gibt bereits einen Ort mit diesem Namen:"
	msgLocationNotFound = "Ort nicht gefunden."
	msgNoPersonalData   = "Bitte keine Privatpersonen, Kontaktpersonen oder Telefonnummern eintragen."
	// msgFieldInvalid covers a field problem without a specific message.
	msgFieldInvalid = "Bitte diese Angabe prüfen."
)

// logMsgLocationsFailed is logged when a location use case fails for a
// reason the admin cannot show as a field message.
const logMsgLocationsFailed = "admin location request failed"

// precisionLabels are the German names of the precision codes.
var precisionLabels = map[core.LocationPrecision]string{
	core.PrecisionBuilding: "Gebäude",
	core.PrecisionStreet:   "Platz/Straße",
	core.PrecisionArea:     "Bereich",
	core.PrecisionDistrict: "nur Ortsteil",
}

// fieldMessages are the German messages for the field problems the core
// reports for a location.
var fieldMessages = map[core.FieldError]string{
	{Field: core.LocationFieldName, Problem: core.ProblemMissing}:          "Bitte einen Namen angeben.",
	{Field: core.LocationFieldAddress, Problem: core.ProblemMissing}:       "Bitte eine Adresse angeben.",
	{Field: core.LocationFieldLatitude, Problem: core.ProblemMissing}:      "Bitte eine Breite angeben.",
	{Field: core.LocationFieldLatitude, Problem: core.ProblemNotANumber}:   "Die Breite ist keine Zahl.",
	{Field: core.LocationFieldLatitude, Problem: core.ProblemOutOfRange}:   "Die Breite muss zwischen −90 und 90 liegen.",
	{Field: core.LocationFieldLongitude, Problem: core.ProblemMissing}:     "Bitte eine Länge angeben.",
	{Field: core.LocationFieldLongitude, Problem: core.ProblemNotANumber}:  "Die Länge ist keine Zahl.",
	{Field: core.LocationFieldLongitude, Problem: core.ProblemOutOfRange}:  "Die Länge muss zwischen −180 und 180 liegen.",
	{Field: core.LocationFieldPrecision, Problem: core.ProblemMissing}:     "Bitte eine Ortsgenauigkeit auswählen.",
	{Field: core.LocationFieldPrecision, Problem: core.ProblemUnknownCode}: "Bitte eine der angebotenen Ortsgenauigkeiten auswählen.",
}

// locationListPage is the data of the location list.
type locationListPage struct {
	Locations []locationRow
}

type locationRow struct {
	Name      string
	Address   string
	Precision string
	Latitude  string
	Longitude string
	URL       string
}

// locationFormPage is the data of the form for a new or an existing
// location. Values hold the input exactly as entered, so nothing is lost
// on an error.
type locationFormPage struct {
	Heading    string
	Action     string
	Values     core.LocationInput
	Errors     map[string]string
	Conflict   *locationConflict
	Precisions []precisionOption
	// PrivacyHint is shown under name and note (NFR-4).
	PrivacyHint string
}

// locationConflict names the location that already has the entered name.
type locationConflict struct {
	Message string
	Name    string
	URL     string
}

type precisionOption struct {
	Code     string
	Label    string
	Selected bool
}

// notFoundPage is the data of the German 404 page.
type notFoundPage struct {
	Message string
	BackURL string
}

func (h *handler) showLocations(w http.ResponseWriter, r *http.Request) {
	locations, err := h.locations.ListLocations(r.Context())
	if err != nil {
		h.failLocationRequest(w, err)
		return
	}
	rows := make([]locationRow, 0, len(locations))
	for _, location := range locations {
		rows = append(rows, locationRow{
			Name:      location.Name,
			Address:   location.Address,
			Precision: precisionLabels[location.Precision],
			Latitude:  formatCoordinate(location.Latitude),
			Longitude: formatCoordinate(location.Longitude),
			URL:       locationURL(location.ID),
		})
	}
	h.render(w, locationsTemplate, http.StatusOK, locationListPage{Locations: rows})
}

func (h *handler) showNewLocation(w http.ResponseWriter, _ *http.Request) {
	h.render(w, locationFormTemplate, http.StatusOK, newLocationFormPage("", core.LocationInput{}))
}

func (h *handler) showLocation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue(locationIDParam)
	location, err := h.locations.GetLocation(r.Context(), id)
	if errors.Is(err, core.ErrNotFound) {
		h.renderLocationNotFound(w)
		return
	}
	if err != nil {
		h.failLocationRequest(w, err)
		return
	}
	h.render(w, locationFormTemplate, http.StatusOK, newLocationFormPage(location.ID, inputFromLocation(location)))
}

func (h *handler) createLocation(w http.ResponseWriter, r *http.Request) {
	h.saveLocation(w, r, "")
}

func (h *handler) updateLocation(w http.ResponseWriter, r *http.Request) {
	h.saveLocation(w, r, r.PathValue(locationIDParam))
}

// saveLocation hands the form to the core and translates its typed errors:
// field messages (422), name conflict (409), unknown location (404).
func (h *handler) saveLocation(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLocationFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	values := inputFromForm(r.PostForm)
	_, err := h.locations.SaveLocation(r.Context(), id, withDecimalPoints(values))

	page := newLocationFormPage(id, values)
	var validation *core.ValidationError
	var conflict *core.LocationConflictError
	switch {
	case err == nil:
		http.Redirect(w, r, locationsPath, http.StatusSeeOther)
	case errors.As(err, &validation):
		page.Errors = fieldErrorMessages(validation.Fields)
		h.render(w, locationFormTemplate, http.StatusUnprocessableEntity, page)
	case errors.As(err, &conflict):
		page.Conflict = &locationConflict{
			Message: msgNameConflict,
			Name:    conflict.Existing.Name,
			URL:     locationURL(conflict.Existing.ID),
		}
		h.render(w, locationFormTemplate, http.StatusConflict, page)
	case errors.Is(err, core.ErrNotFound):
		h.renderLocationNotFound(w)
	default:
		h.failLocationRequest(w, err)
	}
}

func (h *handler) renderLocationNotFound(w http.ResponseWriter) {
	h.render(w, notFoundTemplate, http.StatusNotFound, notFoundPage{Message: msgLocationNotFound, BackURL: locationsPath})
}

func (h *handler) failLocationRequest(w http.ResponseWriter, err error) {
	h.logger.Error(logMsgLocationsFailed, "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// newLocationFormPage returns the form for a new location (empty id) or for
// the location with id, filled with values.
func newLocationFormPage(id string, values core.LocationInput) locationFormPage {
	page := locationFormPage{
		Heading:     headingNewLocation,
		Action:      locationsPath,
		Values:      values,
		PrivacyHint: msgNoPersonalData,
	}
	if id != "" {
		page.Heading = headingEditLocation
		page.Action = locationURL(id)
	}
	for _, precision := range core.LocationPrecisions() {
		page.Precisions = append(page.Precisions, precisionOption{
			Code:     string(precision),
			Label:    precisionLabels[precision],
			Selected: string(precision) == values.Precision,
		})
	}
	return page
}

// inputFromForm reads the location fields exactly as entered.
func inputFromForm(form url.Values) core.LocationInput {
	return core.LocationInput{
		Name:      form.Get(core.LocationFieldName),
		Address:   form.Get(core.LocationFieldAddress),
		Latitude:  form.Get(core.LocationFieldLatitude),
		Longitude: form.Get(core.LocationFieldLongitude),
		Precision: form.Get(core.LocationFieldPrecision),
		Note:      form.Get(core.LocationFieldNote),
	}
}

// withDecimalPoints accepts a decimal comma in the coordinates. Everything
// else, including what counts as missing, is left to the core.
func withDecimalPoints(in core.LocationInput) core.LocationInput {
	in.Latitude = strings.ReplaceAll(in.Latitude, decimalComma, decimalPoint)
	in.Longitude = strings.ReplaceAll(in.Longitude, decimalComma, decimalPoint)
	return in
}

// inputFromLocation fills the edit form with a stored location.
func inputFromLocation(location core.Location) core.LocationInput {
	return core.LocationInput{
		Name:      location.Name,
		Address:   location.Address,
		Latitude:  formatCoordinate(location.Latitude),
		Longitude: formatCoordinate(location.Longitude),
		Precision: string(location.Precision),
		Note:      location.Note,
	}
}

// fieldErrorMessages returns the German message per rejected field.
func fieldErrorMessages(fields []core.FieldError) map[string]string {
	messages := make(map[string]string, len(fields))
	for _, field := range fields {
		message, ok := fieldMessages[field]
		if !ok {
			message = msgFieldInvalid
		}
		messages[field.Field] = message
	}
	return messages
}

func formatCoordinate(degrees float64) string {
	return strconv.FormatFloat(degrees, coordinateFormat, coordinatePrecision, coordinateBitSize)
}

func locationURL(id string) string {
	return locationsPath + "/" + url.PathEscape(id)
}
