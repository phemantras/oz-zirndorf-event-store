package core

import (
	"errors"
	"slices"
	"testing"
)

// validLocationInput returns a complete input; tests change single fields.
func validLocationInput() LocationInput {
	return LocationInput{
		Name:       "Paul-Metz-Halle",
		Street:     "Volkhardtstraße 2",
		PostalCode: "90513",
		City:       "Zirndorf",
		Latitude:   "49.4424",
		Longitude:  "10.9539",
		Precision:  string(PrecisionBuilding),
		Note:       "Eingang an der Rückseite",
	}
}

func TestNewLocationAcceptsValidInputAndDerivesNameKey(t *testing.T) {
	in := validLocationInput()
	in.Name = "  Paul-Metz-Halle\u00a0"
	in.Street = " Volkhardtstraße 2 "
	in.PostalCode = " 90513 "
	in.City = "\u00a0Zirndorf "
	in.Latitude = " 49.4424 "
	in.Precision = " building "
	in.Note = "  "

	got, err := newLocation(in)
	if err != nil {
		t.Fatalf("newLocation: %v", err)
	}
	want := Location{
		Name:       "Paul-Metz-Halle",
		NameKey:    "paul-metz-halle",
		Street:     "Volkhardtstraße 2",
		PostalCode: "90513",
		City:       "Zirndorf",
		Latitude:   49.4424,
		Longitude:  10.9539,
		Precision:  PrecisionBuilding,
		Note:       "",
	}
	if got != want {
		t.Errorf("newLocation = %+v, want %+v", got, want)
	}
}

func TestNewLocationComposesTextsToNFC(t *testing.T) {
	in := validLocationInput()
	in.Name = decomposedOelmuehle
	in.Street = decomposedOelmuehle
	in.City = decomposedOelmuehle
	in.Note = decomposedOelmuehle

	got, err := newLocation(in)
	if err != nil {
		t.Fatalf("newLocation: %v", err)
	}
	if got.Name != composedOelmuehle || got.Street != composedOelmuehle || got.City != composedOelmuehle || got.Note != composedOelmuehle {
		t.Errorf("newLocation = %+v, want NFC texts %q", got, composedOelmuehle)
	}
}

func TestNewLocationAcceptsCoordinateBounds(t *testing.T) {
	bounds := [][2]string{{"-90", "-180"}, {"90", "180"}, {"0", "0"}}
	for _, coordinates := range bounds {
		in := validLocationInput()
		in.Latitude, in.Longitude = coordinates[0], coordinates[1]
		if _, err := newLocation(in); err != nil {
			t.Errorf("newLocation(%v) = %v, want no error", coordinates, err)
		}
	}
}

func TestNewLocationAcceptsEveryPrecisionCode(t *testing.T) {
	for _, precision := range LocationPrecisions() {
		in := validLocationInput()
		in.Precision = string(precision)
		got, err := newLocation(in)
		if err != nil || got.Precision != precision {
			t.Errorf("newLocation with %q = %+v, %v", precision, got, err)
		}
	}
}

func TestLocationPrecisionsAreTheFourCodesInOrder(t *testing.T) {
	want := []LocationPrecision{"building", "street", "area", "district"}
	if got := LocationPrecisions(); !slices.Equal(got, want) {
		t.Errorf("LocationPrecisions() = %v, want %v", got, want)
	}
}

func TestNewLocationReportsEveryInvalidField(t *testing.T) {
	tests := map[string]struct {
		change func(*LocationInput)
		want   []FieldError
	}{
		"all required fields missing or blank": {
			change: func(in *LocationInput) {
				*in = LocationInput{Name: "   ", Street: "\u00a0", PostalCode: "", City: " ", Latitude: "", Longitude: " ", Precision: ""}
			},
			want: []FieldError{
				{Field: LocationFieldName, Problem: ProblemMissing},
				{Field: LocationFieldStreet, Problem: ProblemMissing},
				{Field: LocationFieldPostalCode, Problem: ProblemMissing},
				{Field: LocationFieldCity, Problem: ProblemMissing},
				{Field: LocationFieldLatitude, Problem: ProblemMissing},
				{Field: LocationFieldLongitude, Problem: ProblemMissing},
				{Field: LocationFieldPrecision, Problem: ProblemMissing},
			},
		},
		"postal code with four digits": {
			change: func(in *LocationInput) { in.PostalCode = "9051" },
			want:   []FieldError{{Field: LocationFieldPostalCode, Problem: ProblemInvalidFormat}},
		},
		"postal code with six digits": {
			change: func(in *LocationInput) { in.PostalCode = "905130" },
			want:   []FieldError{{Field: LocationFieldPostalCode, Problem: ProblemInvalidFormat}},
		},
		"postal code with a letter": {
			change: func(in *LocationInput) { in.PostalCode = "90513a" },
			want:   []FieldError{{Field: LocationFieldPostalCode, Problem: ProblemInvalidFormat}},
		},
		"postal code with country prefix": {
			change: func(in *LocationInput) { in.PostalCode = "D-90513" },
			want:   []FieldError{{Field: LocationFieldPostalCode, Problem: ProblemInvalidFormat}},
		},
		"postal code with full-width digits": {
			change: func(in *LocationInput) { in.PostalCode = "\uff19\uff10\uff15\uff11\uff13" },
			want:   []FieldError{{Field: LocationFieldPostalCode, Problem: ProblemInvalidFormat}},
		},
		"postal code with inner space": {
			change: func(in *LocationInput) { in.PostalCode = "905 13" },
			want:   []FieldError{{Field: LocationFieldPostalCode, Problem: ProblemInvalidFormat}},
		},
		"latitude above range, longitude not a number": {
			change: func(in *LocationInput) { in.Latitude, in.Longitude = "91", "abc" },
			want: []FieldError{
				{Field: LocationFieldLatitude, Problem: ProblemOutOfRange},
				{Field: LocationFieldLongitude, Problem: ProblemNotANumber},
			},
		},
		"latitude below range, longitude above range": {
			change: func(in *LocationInput) { in.Latitude, in.Longitude = "-90.0001", "180.0001" },
			want: []FieldError{
				{Field: LocationFieldLatitude, Problem: ProblemOutOfRange},
				{Field: LocationFieldLongitude, Problem: ProblemOutOfRange},
			},
		},
		"longitude below range": {
			change: func(in *LocationInput) { in.Longitude = "-181" },
			want:   []FieldError{{Field: LocationFieldLongitude, Problem: ProblemOutOfRange}},
		},
		"NaN and Inf are not numbers": {
			change: func(in *LocationInput) { in.Latitude, in.Longitude = "NaN", "Inf" },
			want: []FieldError{
				{Field: LocationFieldLatitude, Problem: ProblemNotANumber},
				{Field: LocationFieldLongitude, Problem: ProblemNotANumber},
			},
		},
		"negative infinity is not a number": {
			change: func(in *LocationInput) { in.Latitude = "-Infinity" },
			want:   []FieldError{{Field: LocationFieldLatitude, Problem: ProblemNotANumber}},
		},
		"overflowing number is out of range": {
			change: func(in *LocationInput) { in.Latitude = "1e400" },
			want:   []FieldError{{Field: LocationFieldLatitude, Problem: ProblemOutOfRange}},
		},
		"decimal comma is not a number in the core": {
			change: func(in *LocationInput) { in.Latitude = "49,4424" },
			want:   []FieldError{{Field: LocationFieldLatitude, Problem: ProblemNotANumber}},
		},
		"unknown precision": {
			change: func(in *LocationInput) { in.Precision = "city" },
			want:   []FieldError{{Field: LocationFieldPrecision, Problem: ProblemUnknownCode}},
		},
		"precision codes are case sensitive": {
			change: func(in *LocationInput) { in.Precision = "Building" },
			want:   []FieldError{{Field: LocationFieldPrecision, Problem: ProblemUnknownCode}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := validLocationInput()
			tt.change(&in)

			_, err := newLocation(in)

			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if !slices.Equal(validation.Fields, tt.want) {
				t.Errorf("fields = %v, want %v", validation.Fields, tt.want)
			}
		})
	}
}

func TestSortLocationsOrdersByFoldedNameThenID(t *testing.T) {
	locations := []Location{
		{ID: "3", Name: "Zirndorfer Ölmühle"},
		{ID: "2", Name: "alte Feuerwache"},
		{ID: "1", Name: "Bibertpark"},
		{ID: "6", Name: "Ölmühle"},
		{ID: "5", Name: "Oberasbach"},
		{ID: "b", Name: "Same"},
		{ID: "a", Name: "same"},
		{ID: "4", Name: "Straße am Bach"},
		{ID: "7", Name: "Strasse"},
	}

	SortLocations(locations)

	var got []string
	for _, location := range locations {
		got = append(got, location.ID)
	}
	// Folded keys: alte feuerwache, bibertpark, oberasbach, olmuhle, same (a),
	// same (b), strasse, strasse am bach, zirndorfer olmuhle.
	want := []string{"2", "1", "5", "6", "a", "b", "7", "4", "3"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSortLocationsMatchesAcceptanceExample(t *testing.T) {
	locations := []Location{
		{ID: "1", Name: "Zirndorfer Ölmühle"},
		{ID: "2", Name: "alte Feuerwache"},
		{ID: "3", Name: "Bibertpark"},
	}

	SortLocations(locations)

	want := []string{"alte Feuerwache", "Bibertpark", "Zirndorfer Ölmühle"}
	for i, name := range want {
		if locations[i].Name != name {
			t.Errorf("position %d = %q, want %q", i, locations[i].Name, name)
		}
	}
}

func TestNewLocationChecksTheLimitsOfENT24(t *testing.T) {
	tests := map[string]struct {
		change func(*LocationInput)
		want   []FieldError
	}{
		"name": {
			change: func(in *LocationInput) { in.Name = overLimit(MaxLocationNameLength) },
			want:   []FieldError{{Field: LocationFieldName, Problem: ProblemTooLong, Limit: MaxLocationNameLength}},
		},
		"street": {
			change: func(in *LocationInput) { in.Street = overLimit(MaxStreetLength) },
			want:   []FieldError{{Field: LocationFieldStreet, Problem: ProblemTooLong, Limit: MaxStreetLength}},
		},
		"city": {
			change: func(in *LocationInput) { in.City = overLimit(MaxCityLength) },
			want:   []FieldError{{Field: LocationFieldCity, Problem: ProblemTooLong, Limit: MaxCityLength}},
		},
		"note": {
			change: func(in *LocationInput) { in.Note = overLimit(MaxLocationNoteLength) },
			want:   []FieldError{{Field: LocationFieldNote, Problem: ProblemTooLong, Limit: MaxLocationNoteLength}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := validLocationInput()
			tt.change(&in)

			_, err := newLocation(in)

			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if !slices.Equal(validation.Fields, tt.want) {
				t.Errorf("fields = %v, want %v", validation.Fields, tt.want)
			}
		})
	}
}

func TestNewLocationAcceptsTextsExactlyAtTheirLimits(t *testing.T) {
	in := validLocationInput()
	in.Name = overLimit(MaxLocationNameLength - 1)
	in.Street = overLimit(MaxStreetLength - 1)
	in.City = overLimit(MaxCityLength - 1)
	in.Note = overLimit(MaxLocationNoteLength - 1)

	if _, err := newLocation(in); err != nil {
		t.Errorf("newLocation = %v, want no error", err)
	}
}
