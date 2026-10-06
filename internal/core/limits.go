package core

import "unicode/utf8"

// Limits of the texts and lists of an event and a location (ENT-24). Texts
// count code points after normalizeText. They hold for the admin forms and
// the import alike. The OpenAPI spec (api/v1/openapi.yaml) defines them as
// maxLength and maxItems; these constants mirror it, and a test of the
// public API compares both (AD-9).
const (
	MaxTitleLength                = 200
	MaxImportKeyLength            = 200
	MaxSourceDescriptionLength    = 500
	MaxSourceURLLength            = 2000
	MaxNoteLength                 = 2000
	MaxTimetableEntries           = 100
	MaxTimetableDescriptionLength = 500
	MaxLocationNameLength         = 200
	MaxStreetLength               = 200
	MaxCityLength                 = 100
	MaxLocationNoteLength         = 2000
)

// checkLength reports field as tooLong when the normalized text has more
// than limit code points, and nothing otherwise.
func checkLength(field, text string, limit int) []FieldError {
	if utf8.RuneCountInString(text) <= limit {
		return nil
	}
	return []FieldError{{Field: field, Problem: ProblemTooLong, Limit: limit}}
}
