package admin

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// memoryLocationRepo is an in-memory core.LocationRepo, so the admin tests
// run against the real core use cases and rules.
type memoryLocationRepo struct {
	locations map[string]core.Location
	created   int
	// raceWinner is inserted right before the next write, simulating a
	// concurrent save that slipped past the core's name pre-check.
	raceWinner *core.Location
}

func newMemoryLocationRepo() *memoryLocationRepo {
	return &memoryLocationRepo{locations: map[string]core.Location{}}
}

func (r *memoryLocationRepo) List(context.Context) ([]core.Location, error) {
	all := make([]core.Location, 0, len(r.locations))
	for _, location := range r.locations {
		all = append(all, location)
	}
	// Deliberately reverse name order, so a missing core sort always fails.
	slices.SortFunc(all, func(a, b core.Location) int { return strings.Compare(b.NameKey, a.NameKey) })
	return all, nil
}

func (r *memoryLocationRepo) Get(_ context.Context, id string) (core.Location, error) {
	// Like PostgreSQL's uuid type, the lookup ignores the case of the id.
	location, ok := r.locations[strings.ToLower(id)]
	if !ok {
		return core.Location{}, core.ErrNotFound
	}
	return location, nil
}

func (r *memoryLocationRepo) FindByNameKey(_ context.Context, nameKey string) (core.Location, error) {
	for _, location := range r.locations {
		if location.NameKey == nameKey {
			return location, nil
		}
	}
	return core.Location{}, core.ErrNotFound
}

func (r *memoryLocationRepo) Create(ctx context.Context, location core.Location) (core.Location, error) {
	if err := r.checkUnique(location); err != nil {
		return core.Location{}, err
	}
	r.created++
	location.ID = fmt.Sprintf("0192f0b1-0000-7000-8000-%012d", r.created)
	r.locations[location.ID] = location
	return location, nil
}

func (r *memoryLocationRepo) Update(ctx context.Context, location core.Location) (core.Location, error) {
	if err := r.checkUnique(location); err != nil {
		return core.Location{}, err
	}
	r.locations[location.ID] = location
	return location, nil
}

// checkUnique enforces the unique name_key like the database does.
func (r *memoryLocationRepo) checkUnique(location core.Location) error {
	if r.raceWinner != nil {
		r.locations[r.raceWinner.ID] = *r.raceWinner
		r.raceWinner = nil
	}
	for _, existing := range r.locations {
		if existing.NameKey == location.NameKey && existing.ID != location.ID {
			return core.ErrConflict
		}
	}
	return nil
}

// failingLocations answers every use case with err, for failures the core
// rules cannot produce with valid storage.
type failingLocations struct{ err error }

func (f failingLocations) SaveLocation(context.Context, string, core.LocationInput) (core.Location, error) {
	return core.Location{}, f.err
}

func (f failingLocations) GetLocation(context.Context, string) (core.Location, error) {
	return core.Location{}, f.err
}

func (f failingLocations) ListLocations(context.Context) ([]core.Location, error) {
	return nil, f.err
}

var errStorageDown = errors.New("storage down")

const (
	hallName    = "Paul-Metz-Halle"
	hallAddress = "Volkhardtstraße 2, 90513 Zirndorf"
	// unknownLocationID is a well-formed id that no test stores.
	unknownLocationID = "0192f0b1-0000-7000-8000-0000000000ff"
)

// hallForm is a complete, valid location form with a decimal comma.
func hallForm() url.Values {
	return url.Values{
		core.LocationFieldName:      {hallName},
		core.LocationFieldAddress:   {hallAddress},
		core.LocationFieldLatitude:  {"49,4424"},
		core.LocationFieldLongitude: {"10,9539"},
		core.LocationFieldPrecision: {string(core.PrecisionBuilding)},
		core.LocationFieldNote:      {"Eingang hinten"},
	}
}

func (ts *testServer) get(path string) *httptest.ResponseRecorder {
	return ts.do(withCookie(httptest.NewRequest(http.MethodGet, path, nil), ts.validCookie()))
}

func (ts *testServer) post(path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return ts.do(withCookie(req, ts.validCookie()))
}

// seed stores a location through the core use case and returns it.
func (ts *testServer) seed(t *testing.T, name string) core.Location {
	t.Helper()
	form := hallForm()
	form.Set(core.LocationFieldName, name)
	rec := ts.post(locationsPath, form)
	assertRedirect(t, rec, locationsPath)
	location, err := ts.locations.FindByNameKey(context.Background(), core.NormalizeKey(name))
	if err != nil {
		t.Fatalf("seeded location %q not stored: %v", name, err)
	}
	return location
}

func locationPath(id string) string {
	return locationsPath + "/" + id
}

func assertStatusCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body:\n%s", rec.Code, want, rec.Body.String())
	}
}

// assertBodyContains checks for texts as the browser shows them, so
// escaping by html/template does not matter.
func assertBodyContains(t *testing.T, rec *httptest.ResponseRecorder, wants ...string) {
	t.Helper()
	body := html.UnescapeString(rec.Body.String())
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func assertBodyLacks(t *testing.T, rec *httptest.ResponseRecorder, unwanted ...string) {
	t.Helper()
	body := html.UnescapeString(rec.Body.String())
	for _, text := range unwanted {
		if strings.Contains(body, text) {
			t.Errorf("body contains %q", text)
		}
	}
}

func TestCreateLocationReadsDecimalCommaAndRedirectsToList(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.post(locationsPath, hallForm())

	assertRedirect(t, rec, locationsPath)
	stored, err := ts.locations.FindByNameKey(context.Background(), "paul-metz-halle")
	if err != nil {
		t.Fatalf("location not stored: %v", err)
	}
	if stored.Latitude != 49.4424 || stored.Longitude != 10.9539 {
		t.Errorf("coordinates = %v, %v, want 49.4424, 10.9539", stored.Latitude, stored.Longitude)
	}
	list := ts.get(locationsPath)
	assertStatusCode(t, list, http.StatusOK)
	assertBodyContains(t, list, hallName, hallAddress, "Gebäude", "49.4424, 10.9539", `href="`+locationPath(stored.ID)+`"`)
}

func TestLocationListIsSortedByCoreOrder(t *testing.T) {
	ts := newTestServer(t)
	for _, name := range []string{"Zirndorfer Ölmühle", "alte Feuerwache", "Bibertpark"} {
		ts.seed(t, name)
	}

	rec := ts.get(locationsPath)

	assertStatusCode(t, rec, http.StatusOK)
	body := html.UnescapeString(rec.Body.String())
	first := strings.Index(body, "alte Feuerwache")
	second := strings.Index(body, "Bibertpark")
	third := strings.Index(body, "Zirndorfer Ölmühle")
	if first < 0 || first > second || second > third {
		t.Errorf("positions = %d, %d, %d, want alte Feuerwache, Bibertpark, Zirndorfer Ölmühle", first, second, third)
	}
}

func TestLocationListWithoutLocationsLinksNewForm(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(locationsPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, "Noch keine Orte angelegt.", `href="`+newLocationPath+`"`)
}

func TestNewLocationFormOffersPrecisionsWithGermanLabelsAndPrivacyHints(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newLocationPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`action="`+locationsPath+`"`,
		`<option value="building">Gebäude</option>`,
		`<option value="street">Platz/Straße</option>`,
		`<option value="area">Bereich</option>`,
		`<option value="district">nur Ortsteil</option>`,
	)
	body := html.UnescapeString(rec.Body.String())
	if got := strings.Count(body, msgNoPersonalData); got != 2 {
		t.Errorf("privacy hint appears %d times, want 2 (name and note)", got)
	}
	if got := strings.Count(body, "<option"); got != len(core.LocationPrecisions())+1 {
		t.Errorf("form has %d options, want the four precisions and a prompt", got)
	}
}

func TestCreateLocationWithMissingFieldsShowsGermanMessagesAndKeepsInput(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldName, "   ")
	form.Set(core.LocationFieldLongitude, "")
	form.Set(core.LocationFieldPrecision, "")

	rec := ts.post(locationsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec,
		"Bitte einen Namen angeben.", "Bitte eine Länge angeben.", "Bitte eine Ortsgenauigkeit auswählen.",
		`value="`+hallAddress+`"`, `value="49,4424"`, "Eingang hinten",
	)
	if len(ts.locations.locations) != 0 {
		t.Error("an invalid location was stored")
	}
}

func TestCreateLocationWithInvalidCoordinatesShowsFieldMessages(t *testing.T) {
	tests := map[string]struct {
		latitude, longitude string
		messages            []string
	}{
		"out of range and not a number": {"91", "abc", []string{
			"Die Breite muss zwischen −90 und 90 liegen.", "Die Länge ist keine Zahl.",
		}},
		"NaN and Inf":            {"NaN", "Inf", []string{"Die Breite ist keine Zahl.", "Die Länge ist keine Zahl."}},
		"longitude out of range": {"49,4", "-180,5", []string{"Die Länge muss zwischen −180 und 180 liegen."}},
		"latitude missing":       {" ", "10", []string{"Bitte eine Breite angeben."}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			form := hallForm()
			form.Set(core.LocationFieldLatitude, tt.latitude)
			form.Set(core.LocationFieldLongitude, tt.longitude)

			rec := ts.post(locationsPath, form)

			assertStatusCode(t, rec, http.StatusUnprocessableEntity)
			assertBodyContains(t, rec, tt.messages...)
			assertBodyContains(t, rec, `value="`+tt.longitude+`"`)
		})
	}
}

func TestCreateLocationWithUnknownPrecisionShowsFieldMessage(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldPrecision, "city")

	rec := ts.post(locationsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, "Bitte eine der angebotenen Ortsgenauigkeiten auswählen.")
}

func TestCreateLocationWithMissingAddressShowsFieldMessage(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldAddress, "")

	rec := ts.post(locationsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, "Bitte eine Adresse angeben.")
}

func TestCreateLocationKeepsSelectedPrecisionAfterError(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldName, "")
	form.Set(core.LocationFieldPrecision, string(core.PrecisionArea))

	rec := ts.post(locationsPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, `<option value="area" selected>Bereich</option>`)
}

func TestCreateLocationWithNormalizedDuplicateNameShowsConflictWithLink(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := hallForm()
	form.Set(core.LocationFieldName, " paul-metz-halle ")

	rec := ts.post(locationsPath, form)

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec,
		"Es gibt bereits einen Ort mit diesem Namen: ", `href="`+locationPath(hall.ID)+`">`+hallName+`</a>`,
		`value=" paul-metz-halle "`,
	)
	if len(ts.locations.locations) != 1 {
		t.Errorf("stored %d locations, want 1", len(ts.locations.locations))
	}
}

func TestRenamingLocationIntoAnotherNameShowsConflict(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	park := ts.seed(t, "Bibertpark")
	form := hallForm()
	form.Set(core.LocationFieldName, "PAUL-METZ-HALLE")

	rec := ts.post(locationPath(park.ID), form)

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, msgNameConflict, `href="`+locationPath(hall.ID)+`"`, `action="`+locationPath(park.ID)+`"`)
	if stored := ts.locations.locations[park.ID]; stored.Name != "Bibertpark" {
		t.Errorf("park renamed to %q despite conflict", stored.Name)
	}
}

func TestConcurrentDuplicateFromStorageShowsSameConflictMessage(t *testing.T) {
	ts := newTestServer(t)
	winner := core.Location{ID: unknownLocationID, Name: hallName, NameKey: "paul-metz-halle"}
	ts.locations.raceWinner = &winner

	rec := ts.post(locationsPath, hallForm())

	assertStatusCode(t, rec, http.StatusConflict)
	assertBodyContains(t, rec, msgNameConflict, `href="`+locationPath(winner.ID)+`">`+hallName+`</a>`)
}

func TestSavingLocationWithOwnNameRedirects(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)

	rec := ts.post(locationPath(hall.ID), hallForm())

	assertRedirect(t, rec, locationsPath)
}

func TestEditingLocationKeepsID(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := hallForm()
	form.Set(core.LocationFieldName, "Paul-Metz-Halle Zirndorf")
	form.Set(core.LocationFieldAddress, "Volkhardtstraße 2a, 90513 Zirndorf")

	rec := ts.post(locationPath(hall.ID), form)

	assertRedirect(t, rec, locationsPath)
	stored := ts.locations.locations[hall.ID]
	if stored.Name != "Paul-Metz-Halle Zirndorf" || stored.Address != "Volkhardtstraße 2a, 90513 Zirndorf" {
		t.Errorf("stored = %+v, want new name and address under id %s", stored, hall.ID)
	}
	if len(ts.locations.locations) != 1 {
		t.Errorf("stored %d locations, want 1", len(ts.locations.locations))
	}
}

func TestEditFormShowsStoredValues(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)

	rec := ts.get(locationPath(hall.ID))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`action="`+locationPath(hall.ID)+`"`, `value="`+hallName+`"`, `value="`+hallAddress+`"`,
		`value="49.4424"`, `value="10.9539"`, `<option value="building" selected>Gebäude</option>`, "Eingang hinten",
	)
}

func TestEditFormActionUsesStoredIDForDifferentlySpelledID(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)

	rec := ts.get(locationPath(strings.ToUpper(hall.ID)))

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec, `action="`+locationPath(hall.ID)+`"`)
}

func TestUnknownOrMalformedLocationIDShowsGermanNotFoundPage(t *testing.T) {
	for _, id := range []string{unknownLocationID, "kaputt"} {
		t.Run(id, func(t *testing.T) {
			ts := newTestServer(t)

			for _, rec := range []*httptest.ResponseRecorder{ts.get(locationPath(id)), ts.post(locationPath(id), hallForm())} {
				assertStatusCode(t, rec, http.StatusNotFound)
				assertBodyContains(t, rec, msgLocationNotFound, `href="`+locationsPath+`"`)
			}
			if len(ts.locations.locations) != 0 {
				t.Error("a location was stored for an unknown id")
			}
		})
	}
}

func TestLocationPagesRequireSession(t *testing.T) {
	ts := newTestServer(t)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, locationsPath, nil),
		httptest.NewRequest(http.MethodGet, newLocationPath, nil),
		httptest.NewRequest(http.MethodGet, locationPath(unknownLocationID), nil),
		httptest.NewRequest(http.MethodPost, locationsPath, strings.NewReader(hallForm().Encode())),
		httptest.NewRequest(http.MethodPost, locationPath(unknownLocationID), strings.NewReader(hallForm().Encode())),
	}
	for _, req := range requests {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		assertRedirect(t, ts.do(req), loginPath)
	}
	if len(ts.locations.locations) != 0 {
		t.Error("a location was stored without session")
	}
}

func TestLocationPagesAnswerStorageFailureWithServerErrorAndLog(t *testing.T) {
	requests := map[string]func(*testServer) *httptest.ResponseRecorder{
		"list":   func(ts *testServer) *httptest.ResponseRecorder { return ts.get(locationsPath) },
		"edit":   func(ts *testServer) *httptest.ResponseRecorder { return ts.get(locationPath(unknownLocationID)) },
		"create": func(ts *testServer) *httptest.ResponseRecorder { return ts.post(locationsPath, hallForm()) },
		"update": func(ts *testServer) *httptest.ResponseRecorder {
			return ts.post(locationPath(unknownLocationID), hallForm())
		},
	}
	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.handler.locations = failingLocations{err: errStorageDown}

			rec := request(ts)

			assertStatusCode(t, rec, http.StatusInternalServerError)
			if !strings.Contains(ts.logs.String(), logMsgLocationsFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
				t.Errorf("log %q does not record the failure", ts.logs.String())
			}
		})
	}
}

func TestUnexpectedFieldProblemFallsBackToGenericMessage(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.locations = failingLocations{err: &core.ValidationError{Fields: []core.FieldError{
		{Field: core.LocationFieldName, Problem: core.ProblemOutOfRange},
	}}}

	rec := ts.post(locationsPath, hallForm())

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, msgFieldInvalid)
}

func TestOversizedLocationFormIsRejected(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldNote, strings.Repeat("x", maxLocationFormBytes))

	rec := ts.post(locationsPath, form)

	assertStatusCode(t, rec, http.StatusBadRequest)
	assertBodyLacks(t, rec, "xxxxxxxx")
	if len(ts.locations.locations) != 0 {
		t.Error("an oversized form was stored")
	}
}
