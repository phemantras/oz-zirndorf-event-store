package admin

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
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

// The location list shows the address parts as "Straße, PLZ Ort".
const (
	streetSeparator     = ", "
	postalCodeSeparator = " "
)

// German texts of the location pages.
const (
	headingNewLocation  = "Neuer Ort"
	headingEditLocation = "Ort bearbeiten"
	msgNameConflict     = "Es gibt bereits einen Ort mit diesem Namen:"
	msgLocationNotFound = "Ort nicht gefunden."
	backToLocationList  = "Zurück zur Ortsliste"
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

// locationFieldMessages are the German messages for the field problems the
// core reports for a location.
var locationFieldMessages = map[core.FieldError]string{
	{Field: core.LocationFieldName, Problem: core.ProblemMissing}:             "Bitte einen Namen angeben.",
	{Field: core.LocationFieldStreet, Problem: core.ProblemMissing}:           "Bitte Straße und Hausnummer angeben.",
	{Field: core.LocationFieldPostalCode, Problem: core.ProblemMissing}:       "Bitte eine PLZ angeben.",
	{Field: core.LocationFieldPostalCode, Problem: core.ProblemInvalidFormat}: "Die PLZ muss aus genau fünf Ziffern bestehen.",
	{Field: core.LocationFieldCity, Problem: core.ProblemMissing}:             "Bitte einen Ort angeben.",
	{Field: core.LocationFieldLatitude, Problem: core.ProblemMissing}:         "Bitte eine Breite angeben.",
	{Field: core.LocationFieldLatitude, Problem: core.ProblemNotANumber}:      "Die Breite ist keine Zahl.",
	{Field: core.LocationFieldLatitude, Problem: core.ProblemOutOfRange}:      "Die Breite muss zwischen −90 und 90 liegen.",
	{Field: core.LocationFieldLongitude, Problem: core.ProblemMissing}:        "Bitte eine Länge angeben.",
	{Field: core.LocationFieldLongitude, Problem: core.ProblemNotANumber}:     "Die Länge ist keine Zahl.",
	{Field: core.LocationFieldLongitude, Problem: core.ProblemOutOfRange}:     "Die Länge muss zwischen −180 und 180 liegen.",
	{Field: core.LocationFieldPrecision, Problem: core.ProblemMissing}:        "Bitte eine Ortsgenauigkeit auswählen.",
	{Field: core.LocationFieldPrecision, Problem: core.ProblemUnknownCode}:    "Bitte eine der angebotenen Ortsgenauigkeiten auswählen.",
}

// locationListPage is the data of the location list.
type locationListPage struct {
	Locations []locationRow
}

type locationRow struct {
	Name string
	// Address joins the address parts for display; empty for a location
	// stored before the address was split.
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
	Precisions []selectOption
	// PrivacyHint is shown under name and note (NFR-4).
	PrivacyHint string
}

// locationConflict names the location that already has the entered name.
type locationConflict struct {
	Message string
	Name    string
	URL     string
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
			Address:   formatAddress(location),
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
		page.Errors = fieldErrorMessages(validation.Fields, locationFieldMessages)
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
	h.render(w, notFoundTemplate, http.StatusNotFound, notFoundPage{
		Message: msgLocationNotFound, BackURL: locationsPath, BackLabel: backToLocationList,
	})
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
		page.Precisions = append(page.Precisions, selectOption{
			Value:    string(precision),
			Label:    precisionLabels[precision],
			Selected: string(precision) == values.Precision,
		})
	}
	return page
}

// inputFromForm reads the location fields exactly as entered.
func inputFromForm(form url.Values) core.LocationInput {
	return core.LocationInput{
		Name:       form.Get(core.LocationFieldName),
		Street:     form.Get(core.LocationFieldStreet),
		PostalCode: form.Get(core.LocationFieldPostalCode),
		City:       form.Get(core.LocationFieldCity),
		Latitude:   form.Get(core.LocationFieldLatitude),
		Longitude:  form.Get(core.LocationFieldLongitude),
		Precision:  form.Get(core.LocationFieldPrecision),
		Note:       form.Get(core.LocationFieldNote),
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
		Name:       location.Name,
		Street:     location.Street,
		PostalCode: location.PostalCode,
		City:       location.City,
		Latitude:   formatCoordinate(location.Latitude),
		Longitude:  formatCoordinate(location.Longitude),
		Precision:  string(location.Precision),
		Note:       location.Note,
	}
}

// formatAddress returns the address as "Straße, PLZ Ort", leaving out empty
// parts, so a location stored before the address was split shows an empty
// address.
func formatAddress(location core.Location) string {
	place := joinNonEmpty(postalCodeSeparator, location.PostalCode, location.City)
	return joinNonEmpty(streetSeparator, location.Street, place)
}

// joinNonEmpty joins the parts that are not empty with separator.
func joinNonEmpty(separator string, parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(part string) bool { return part == "" }), separator)
}

func formatCoordinate(degrees float64) string {
	return strconv.FormatFloat(degrees, coordinateFormat, coordinatePrecision, coordinateBitSize)
}

func locationURL(id string) string {
	return locationsPath + "/" + url.PathEscape(id)
}
