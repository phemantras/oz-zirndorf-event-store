package core

import (
	"cmp"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
)

// LocationPrecision says how exactly a location's coordinates mark the
// place.
type LocationPrecision string

// Location precision codes. The OpenAPI spec (api/v1/openapi.yaml) defines
// them in the schema LocationPrecision; these constants mirror it, and a
// test of the public API compares both (AD-9).
const (
	PrecisionBuilding LocationPrecision = "building"
	PrecisionStreet   LocationPrecision = "street"
	PrecisionArea     LocationPrecision = "area"
	PrecisionDistrict LocationPrecision = "district"
)

// LocationPrecisions returns every precision code, from most to least exact.
func LocationPrecisions() []LocationPrecision {
	return []LocationPrecision{PrecisionBuilding, PrecisionStreet, PrecisionArea, PrecisionDistrict}
}

// Field names of a location, used in FieldError. The OpenAPI spec
// (api/v1/openapi.yaml) defines them in the schemas EventLocation and
// EventInputLocation, street, postalCode and city as the fields of the
// schema Address; these constants mirror it (AD-9).
const (
	LocationFieldName       = "name"
	LocationFieldStreet     = "street"
	LocationFieldPostalCode = "postalCode"
	LocationFieldCity       = "city"
	LocationFieldLatitude   = "latitude"
	LocationFieldLongitude  = "longitude"
	LocationFieldPrecision  = "precision"
	LocationFieldNote       = "note"
	// LocationFieldAddress is the object of the import that holds street,
	// postalCode and city.
	LocationFieldAddress = "address"
)

// Coordinate bounds in degrees, both inclusive.
const (
	maxLatitude  = 90
	maxLongitude = 180
)

// coordinateBitSize parses coordinates as float64.
const coordinateBitSize = 64

// postalCodeDigits is the length of a German postal code.
const postalCodeDigits = 5

// Location is a place events refer to. Its ID stays the same when it is
// renamed.
type Location struct {
	ID string
	// Name is unique after NormalizeKey; NameKey stores that key.
	Name    string
	NameKey string
	// Street holds the street with house number if there is one, or an
	// area such as "Marktplatz bis Schulsportplatz". Street, PostalCode and
	// City are empty only for locations stored before Story 1.12.
	Street     string
	PostalCode string
	City       string
	Latitude   float64
	Longitude  float64
	Precision  LocationPrecision
	// Note is optional; empty means none.
	Note string
}

// LocationInput is a location as entered by a person or an import.
// Coordinates arrive as text so the core decides what counts as missing
// (empty) and what is not a number.
type LocationInput struct {
	Name       string
	Street     string
	PostalCode string
	City       string
	Latitude   string
	Longitude  string
	Precision  string
	Note       string
}

// normalized returns the input with every text normalized by
// normalizeText.
func (in LocationInput) normalized() LocationInput {
	return LocationInput{
		Name:       normalizeText(in.Name),
		Street:     normalizeText(in.Street),
		PostalCode: normalizeText(in.PostalCode),
		City:       normalizeText(in.City),
		Latitude:   normalizeText(in.Latitude),
		Longitude:  normalizeText(in.Longitude),
		Precision:  normalizeText(in.Precision),
		Note:       normalizeText(in.Note),
	}
}

// newLocation canonicalizes and validates input and returns the location
// without ID. It reports every rejected field at once as *ValidationError.
func newLocation(in LocationInput) (Location, error) {
	var problems []FieldError
	report := func(field string, problem FieldProblem) {
		problems = append(problems, FieldError{Field: field, Problem: problem})
	}

	location := Location{
		Name:   normalizeText(in.Name),
		Street: normalizeText(in.Street),
		City:   normalizeText(in.City),
		Note:   normalizeText(in.Note),
	}
	location.NameKey = NormalizeKey(location.Name)
	if location.Name == "" {
		report(LocationFieldName, ProblemMissing)
	}
	problems = append(problems, checkLength(LocationFieldName, location.Name, MaxLocationNameLength)...)
	if location.Street == "" {
		report(LocationFieldStreet, ProblemMissing)
	}
	problems = append(problems, checkLength(LocationFieldStreet, location.Street, MaxStreetLength)...)
	var problem FieldProblem
	if location.PostalCode, problem = parsePostalCode(in.PostalCode); problem != "" {
		report(LocationFieldPostalCode, problem)
	}
	if location.City == "" {
		report(LocationFieldCity, ProblemMissing)
	}
	problems = append(problems, checkLength(LocationFieldCity, location.City, MaxCityLength)...)
	if location.Latitude, problem = parseCoordinate(in.Latitude, maxLatitude); problem != "" {
		report(LocationFieldLatitude, problem)
	}
	if location.Longitude, problem = parseCoordinate(in.Longitude, maxLongitude); problem != "" {
		report(LocationFieldLongitude, problem)
	}
	if location.Precision, problem = parsePrecision(in.Precision); problem != "" {
		report(LocationFieldPrecision, problem)
	}
	problems = append(problems, checkLength(LocationFieldNote, location.Note, MaxLocationNoteLength)...)

	if problems != nil {
		return Location{}, &ValidationError{Fields: problems}
	}
	return location, nil
}

// parseCoordinate reads a coordinate in degrees that must lie within
// [-limit, limit]. Empty text is missing, never 0; NaN and infinities are
// not numbers. It returns an empty problem when the value is valid.
func parseCoordinate(text string, limit float64) (float64, FieldProblem) {
	text = normalizeText(text)
	if text == "" {
		return 0, ProblemMissing
	}
	value, err := strconv.ParseFloat(text, coordinateBitSize)
	if errors.Is(err, strconv.ErrRange) {
		return 0, ProblemOutOfRange
	}
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, ProblemNotANumber
	}
	if value < -limit || value > limit {
		return 0, ProblemOutOfRange
	}
	return value, ""
}

// parsePostalCode accepts exactly five ASCII digits after trimming; other
// digits such as full-width ones are rejected.
func parsePostalCode(text string) (string, FieldProblem) {
	postalCode := normalizeText(text)
	if postalCode == "" {
		return "", ProblemMissing
	}
	if len(postalCode) != postalCodeDigits || strings.ContainsFunc(postalCode, isNotASCIIDigit) {
		return "", ProblemInvalidFormat
	}
	return postalCode, ""
}

func isNotASCIIDigit(r rune) bool {
	return r < '0' || r > '9'
}

// parsePrecision accepts exactly one of the precision codes.
func parsePrecision(text string) (LocationPrecision, FieldProblem) {
	precision := LocationPrecision(normalizeText(text))
	if precision == "" {
		return "", ProblemMissing
	}
	if !slices.Contains(LocationPrecisions(), precision) {
		return "", ProblemUnknownCode
	}
	return precision, ""
}

// SortLocations orders locations by name, ignoring case and treating ä, ö,
// ü and ß as a, o, u and ss; equal names are ordered by ID so the order is
// stable across requests.
func SortLocations(locations []Location) {
	slices.SortFunc(locations, func(a, b Location) int {
		return cmp.Or(cmp.Compare(sortKey(a.Name), sortKey(b.Name)), cmp.Compare(a.ID, b.ID))
	})
}
