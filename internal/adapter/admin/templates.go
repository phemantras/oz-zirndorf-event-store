package admin

import (
	"embed"
	"html/template"
)

// layoutTemplate is the entry point of every page; pages define "title"
// and "content".
const layoutTemplate = "layout"

const (
	templateDir      = "templates/"
	layoutFile       = templateDir + "layout.html"
	loginPageFile    = templateDir + "login.html"
	homePageFile     = templateDir + "home.html"
	locationsFile    = templateDir + "locations.html"
	locationFormFile = templateDir + "location_form.html"
	notFoundFile     = templateDir + "not_found.html"
	eventsFile       = templateDir + "events.html"
	eventFormFile    = templateDir + "event_form.html"
	// The partials hold the location fields, shared by the location form
	// and the inline input on the event form, and the fragments of the
	// inline input.
	locationFieldsFile = templateDir + "location_fields.html"
	newLocationFile    = templateDir + "new_location.html"
)

//go:embed templates
var templateFiles embed.FS

// staticFiles holds htmx, the vendored Leaflet and the map picker script,
// served below /admin/static/ instead of from a CDN.
//
//go:embed static
var staticFiles embed.FS

var (
	loginTemplate        = parsePage(loginPageFile)
	homeTemplate         = parsePage(homePageFile)
	locationsTemplate    = parsePage(locationsFile)
	locationFormTemplate = parsePage(locationFormFile, locationFieldsFile)
	notFoundTemplate     = parsePage(notFoundFile)
	eventsTemplate       = parsePage(eventsFile)
	eventFormTemplate    = parsePage(eventFormFile, locationFieldsFile, newLocationFile)
	// newLocationTemplate holds the fragments that htmx swaps into the
	// event form.
	newLocationTemplate = parseFragments(newLocationFile, locationFieldsFile)
)

// loginPage is the data of the login form.
type loginPage struct {
	Username string
	Error    string
}

// homePage is the data of the admin start page.
type homePage struct{}

// parsePage combines the layout with one page and the partials it uses. The
// templates are embedded, so a parse error is a programming error caught by
// every test run.
func parsePage(file string, partials ...string) *template.Template {
	return parseFragments(append([]string{layoutFile, file}, partials...)...)
}

// parseFragments parses templates that are rendered by name, without the
// layout.
func parseFragments(files ...string) *template.Template {
	return template.Must(template.ParseFS(templateFiles, files...))
}
