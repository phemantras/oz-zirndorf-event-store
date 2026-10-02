package admin

import (
	"embed"
	"html/template"
)

// layoutTemplate is the entry point of every page; pages define "title"
// and "content".
const layoutTemplate = "layout"

const (
	templateDir   = "templates/"
	layoutFile    = templateDir + "layout.html"
	loginPageFile = templateDir + "login.html"
	homePageFile  = templateDir + "home.html"
)

//go:embed templates
var templateFiles embed.FS

// staticFiles holds htmx and later Leaflet, served below /admin/static/
// instead of from a CDN.
//
//go:embed static
var staticFiles embed.FS

var (
	loginTemplate = parsePage(loginPageFile)
	homeTemplate  = parsePage(homePageFile)
)

// loginPage is the data of the login form.
type loginPage struct {
	Username string
	Error    string
}

// homePage is the data of the admin start page.
type homePage struct{}

// parsePage combines the layout with one page. The templates are embedded,
// so a parse error is a programming error caught by every test run.
func parsePage(file string) *template.Template {
	return template.Must(template.ParseFS(templateFiles, layoutFile, file))
}
