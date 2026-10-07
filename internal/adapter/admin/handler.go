package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// Routes of the admin interface. Everything lives below adminPathPrefix.
const (
	adminPathPrefix  = "/admin"
	homePath         = adminPathPrefix + "/"
	loginPath        = adminPathPrefix + "/login"
	logoutPath       = adminPathPrefix + "/logout"
	staticPathPrefix = adminPathPrefix + "/static/"
	htmxPath         = staticPathPrefix + "htmx.min.js"
)

// Static files of the map picker on the location form. Leaflet is vendored
// unchanged from the npm package leaflet@1.9.4.
const (
	leafletPathPrefix     = staticPathPrefix + "leaflet/"
	leafletScriptPath     = leafletPathPrefix + "leaflet.js"
	leafletStylePath      = leafletPathPrefix + "leaflet.css"
	leafletMarkerIconPath = leafletPathPrefix + "images/marker-icon.png"
	locationMapScriptPath = staticPathPrefix + "location-map.js"
)

// Names of the login form fields.
const (
	loginFieldUser     = "username"
	loginFieldPassword = "password"
	// maxLoginFormBytes bounds the login request body; real forms are tiny.
	maxLoginFormBytes = 4 << 10
)

// htmx headers: htmx 2 follows HX-Redirect on any status, including 401.
const (
	htmxRequestHeader  = "HX-Request"
	htmxRequestTrue    = "true"
	htmxRedirectHeader = "HX-Redirect"
)

// German messages shown on the login page.
const (
	msgWrongCredentials = "Benutzername oder Passwort ist falsch."
	msgLockedOut        = "Zu viele fehlgeschlagene Anmeldeversuche. Die Anmeldung ist für 15 Minuten gesperrt."
)

// Log messages. They never carry user names, passwords, client IPs or
// cookie values.
const (
	logMsgLoginSucceeded = "admin login succeeded"
	logMsgLoginFailed    = "admin login failed"
	logMsgLoginLocked    = "admin login rejected during lockout"
	logMsgRenderFailed   = "admin page render failed"
)

const (
	htmlContentType       = "text/html; charset=utf-8"
	setCookieHeader       = "Set-Cookie"
	sessionTimeResolution = time.Second
)

// Config is everything the admin interface needs from the outside. The
// startup code validates the credentials before building the handler.
type Config struct {
	// User is the name of the single admin account.
	User string
	// PasswordHash is the bcrypt hash of the admin password.
	PasswordHash []byte
	// SessionSecret is the HMAC-SHA256 key that signs session cookies.
	SessionSecret []byte
	Logger        *slog.Logger
	// Now returns the current time; injected so expiry is testable.
	Now func() time.Time
	// Locations are the core use cases behind the location pages.
	Locations LocationUseCases
	// Events are the core use cases behind the event pages.
	Events EventUseCases
	// Clock decides which events the list shows as archived.
	Clock core.Clock
	// Imports are the core use cases behind the import page.
	Imports ImportUseCases
}

// handler serves the admin interface.
type handler struct {
	userDigest      [sha256.Size]byte
	passwordHash    []byte
	sessions        sessionCodec
	lockout         *loginLockout
	logger          *slog.Logger
	now             func() time.Time
	comparePassword func(hash, password []byte) error
	locations       LocationUseCases
	events          EventUseCases
	clock           core.Clock
	imports         ImportUseCases
}

// NewHandler returns the admin interface for all paths below /admin/,
// wrapped in cross-origin protection.
func NewHandler(cfg Config) http.Handler {
	return newHandler(cfg).routes()
}

func newHandler(cfg Config) *handler {
	return &handler{
		userDigest:      sha256.Sum256([]byte(cfg.User)),
		passwordHash:    cfg.PasswordHash,
		sessions:        sessionCodec{secret: cfg.SessionSecret},
		lockout:         newLoginLockout(cfg.Now),
		logger:          cfg.Logger,
		now:             cfg.Now,
		comparePassword: bcrypt.CompareHashAndPassword,
		locations:       cfg.Locations,
		events:          cfg.Events,
		clock:           cfg.Clock,
		imports:         cfg.Imports,
	}
}

// routes wires login and static files as public routes and everything else
// behind the session check. Cross-origin protection wraps all of it,
// including the login form.
func (h *handler) routes() http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc(http.MethodGet+" "+homePath+"{$}", h.showHome)
	protected.HandleFunc(http.MethodPost+" "+logoutPath, h.logout)
	protected.HandleFunc(http.MethodGet+" "+locationsPath, h.showLocations)
	protected.HandleFunc(http.MethodGet+" "+newLocationPath, h.showNewLocation)
	protected.HandleFunc(http.MethodPost+" "+locationsPath, h.createLocation)
	protected.HandleFunc(http.MethodGet+" "+locationPathPattern, h.showLocation)
	protected.HandleFunc(http.MethodPost+" "+locationPathPattern, h.updateLocation)
	protected.HandleFunc(http.MethodPost+" "+locationDeletePathPattern, h.deleteLocation)
	protected.HandleFunc(http.MethodGet+" "+eventsPath, h.showEvents)
	protected.HandleFunc(http.MethodGet+" "+newEventPath, h.showNewEvent)
	protected.HandleFunc(http.MethodPost+" "+eventsPath, h.createEvent)
	protected.HandleFunc(http.MethodGet+" "+eventPathPattern, h.showEvent)
	protected.HandleFunc(http.MethodPost+" "+eventPathPattern, h.updateEvent)
	protected.HandleFunc(http.MethodPost+" "+eventDeletePathPattern, h.deleteEvent)
	protected.HandleFunc(http.MethodPost+" "+eventImportKeyRemovePathPattern, h.removeImportKey)
	protected.HandleFunc(http.MethodGet+" "+inlineLocationPath, h.openInlineLocation)
	protected.HandleFunc(http.MethodGet+" "+inlineLocationCancelPath, h.cancelInlineLocation)
	protected.HandleFunc(http.MethodPost+" "+inlineLocationPath, h.createInlineLocation)
	protected.HandleFunc(http.MethodGet+" "+timetableEntryPath, h.showTimetableEntry)
	protected.HandleFunc(http.MethodGet+" "+importPath, h.showImport)
	protected.HandleFunc(http.MethodPost+" "+importPath, h.checkImport)
	protected.HandleFunc(http.MethodPost+" "+importCommitPath, h.commitImport)

	mux := http.NewServeMux()
	mux.HandleFunc(http.MethodGet+" "+loginPath, h.showLogin)
	mux.HandleFunc(http.MethodPost+" "+loginPath, h.submitLogin)
	mux.Handle(http.MethodGet+" "+staticPathPrefix,
		http.StripPrefix(adminPathPrefix, http.FileServerFS(staticFiles)))
	mux.Handle(homePath, h.requireSession(protected))

	return http.NewCrossOriginProtection().Handler(mux)
}

// requireSession lets only requests with a valid session through and
// renews their activity time. Others go to the login page.
func (h *handler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current, ok := h.currentSession(r)
		if !ok {
			redirectToLogin(w, r)
			return
		}
		now := h.currentTime()
		current.ActiveAt = now
		http.SetCookie(w, h.sessions.cookie(current, now))
		next.ServeHTTP(w, r)
	})
}

// currentSession returns the request's session if it is authentic and not
// expired.
func (h *handler) currentSession(r *http.Request) (session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return session{}, false
	}
	current, ok := h.sessions.decode(cookie.Value)
	if !ok || !current.isValidAt(h.now()) {
		return session{}, false
	}
	return current, true
}

// currentTime returns now at the resolution the session cookie stores.
func (h *handler) currentTime() time.Time {
	return h.now().Truncate(sessionTimeResolution)
}

// redirectToLogin answers htmx requests with HX-Redirect, so the login page
// is not swapped into a fragment, and everything else with 303.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	redirectTo(w, r, loginPath, http.StatusUnauthorized)
}

// redirectAfterDelete sends the browser to the list at target once a record
// is deleted: htmx with HX-Redirect, so the whole page is replaced,
// everything else with 303.
func redirectAfterDelete(w http.ResponseWriter, r *http.Request, target string) {
	redirectTo(w, r, target, http.StatusNoContent)
}

// redirectTo answers htmx requests with HX-Redirect to target and
// htmxStatus, everything else with 303 to target.
func redirectTo(w http.ResponseWriter, r *http.Request, target string, htmxStatus int) {
	if r.Header.Get(htmxRequestHeader) == htmxRequestTrue {
		w.Header().Set(htmxRedirectHeader, target)
		w.WriteHeader(htmxStatus)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (h *handler) showLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentSession(r); ok {
		http.Redirect(w, r, homePath, http.StatusSeeOther)
		return
	}
	h.render(w, loginTemplate, http.StatusOK, loginPage{})
}

// submitLogin checks the credentials unless the client IP is locked out.
// Every allowed attempt counts as a failure until it succeeds.
func (h *handler) submitLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.lockout.beginAttempt(ip) {
		h.logger.Warn(logMsgLoginLocked)
		h.render(w, loginTemplate, http.StatusTooManyRequests, loginPage{Error: msgLockedOut})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLoginFormBytes)
	// ADMIN_USER is trimmed at startup; mobile keyboards often append a space.
	user := strings.TrimSpace(r.PostFormValue(loginFieldUser))
	if !h.credentialsMatch(user, r.PostFormValue(loginFieldPassword)) {
		h.logger.Warn(logMsgLoginFailed)
		h.render(w, loginTemplate, http.StatusOK, loginPage{Username: user, Error: msgWrongCredentials})
		return
	}

	h.lockout.reset(ip)
	now := h.currentTime()
	http.SetCookie(w, h.sessions.cookie(session{LoginAt: now, ActiveAt: now}, now))
	h.logger.Info(logMsgLoginSucceeded)
	http.Redirect(w, r, homePath, http.StatusSeeOther)
}

// credentialsMatch compares the user name in constant time and always runs
// bcrypt, so neither timing nor work reveals whether the name was right.
func (h *handler) credentialsMatch(user, password string) bool {
	userDigest := sha256.Sum256([]byte(user))
	userMatches := subtle.ConstantTimeCompare(userDigest[:], h.userDigest[:]) == 1
	passwordMatches := h.comparePassword(h.passwordHash, []byte(password)) == nil
	return userMatches && passwordMatches
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	// requireSession has already renewed the cookie; drop that header so the
	// browser only receives the deletion.
	w.Header().Del(setCookieHeader)
	http.SetCookie(w, expiredSessionCookie())
	http.Redirect(w, r, loginPath, http.StatusSeeOther)
}

func (h *handler) showHome(w http.ResponseWriter, _ *http.Request) {
	h.render(w, homeTemplate, http.StatusOK, homePage{})
}

// logRequestFailure logs err of a failed request under msg. A request that
// ran out of its deadline met a slow database or an exhausted pool, which
// is no fault of the code, so it is logged as Warn; anything else as Error.
func (h *handler) logRequestFailure(msg string, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		h.logger.Warn(msg, "error", err)
		return
	}
	h.logger.Error(msg, "error", err)
}

// render executes page within the layout.
func (h *handler) render(w http.ResponseWriter, page *template.Template, status int, data any) {
	h.renderFragment(w, page, layoutTemplate, status, data)
}

// renderFragment executes the template name into a buffer first, so a
// template error yields a clean 500 instead of a half-written response.
func (h *handler) renderFragment(w http.ResponseWriter, templates *template.Template, name string, status int, data any) {
	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, name, data); err != nil {
		h.logger.Error(logMsgRenderFailed, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", htmlContentType)
	w.WriteHeader(status)
	// A failed write means the client went away; there is nobody to tell.
	_, _ = body.WriteTo(w)
}
