package admin

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

const (
	testUser     = "andreas"
	testPassword = "richtig-und-lang"
	// wrongPassword must never show up in the log.
	wrongPassword = "geheim123"
	// testRemoteAddress is what Railway's edge looks like from the app.
	testRemoteAddress = "100.64.0.2:41000"
)

// testServer bundles a handler under test with its observable side effects.
type testServer struct {
	handler        *handler
	routes         http.Handler
	clock          *fakeClock
	logs           *bytes.Buffer
	passwordChecks int
	// locations and events are the in-memory storage behind the real core
	// use cases.
	locations    *memoryLocationRepo
	events       *memoryEventRepo
	eventService *core.EventService
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	// MinCost keeps the tests fast; the cost floor is enforced at startup.
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	ts := &testServer{
		clock:     &fakeClock{current: testLoginTime},
		logs:      &bytes.Buffer{},
		locations: newMemoryLocationRepo(),
		events:    newMemoryEventRepo(),
	}
	ts.eventService = core.NewEventService(memoryTx{repos: core.Repos{Events: ts.events, Locations: ts.locations}}, ts.events, ts.locations)
	ts.handler = newHandler(Config{
		User:          testUser,
		PasswordHash:  hash,
		SessionSecret: testSecret,
		Logger:        slog.New(slog.NewJSONHandler(ts.logs, nil)),
		Now:           ts.clock.now,
		Locations:     core.NewLocationService(ts.locations),
		Events:        ts.eventService,
		Clock:         ts.clock,
	})
	compare := ts.handler.comparePassword
	ts.handler.comparePassword = func(hash, password []byte) error {
		ts.passwordChecks++
		return compare(hash, password)
	}
	ts.routes = ts.handler.routes()
	return ts
}

func (ts *testServer) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	ts.routes.ServeHTTP(rec, req)
	return rec
}

func loginRequest(user, password string) *http.Request {
	form := url.Values{loginFieldUser: {user}, loginFieldPassword: {password}}
	req := httptest.NewRequest(http.MethodPost, loginPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Forwarded-For", testIP+", 198.51.100.4")
	req.RemoteAddr = testRemoteAddress
	return req
}

func withCookie(req *http.Request, cookie *http.Cookie) *http.Request {
	req.AddCookie(cookie)
	return req
}

func (ts *testServer) validCookie() *http.Cookie {
	now := ts.clock.now()
	return ts.handler.sessions.cookie(session{LoginAt: now, ActiveAt: now}, now)
}

func sessionCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	var found *http.Cookie
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			if found != nil {
				t.Fatal("response sets the session cookie more than once")
			}
			found = cookie
		}
	}
	return found
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, location string) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != location {
		t.Errorf("Location = %q, want %q", got, location)
	}
}

func TestLoginWithCorrectCredentialsSetsSessionAndRedirectsHome(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(loginRequest(testUser, testPassword))

	assertRedirect(t, rec, homePath)
	cookie := sessionCookieFrom(t, rec)
	if cookie == nil {
		t.Fatal("no session cookie set")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != adminPathPrefix {
		t.Errorf("cookie = %+v, want HttpOnly, Secure, SameSite=Strict, Path=%s", cookie, adminPathPrefix)
	}
	home := ts.do(withCookie(httptest.NewRequest(http.MethodGet, homePath, nil), cookie))
	if home.Code != http.StatusOK {
		t.Errorf("home with new cookie: status = %d, want %d", home.Code, http.StatusOK)
	}
}

func TestLoginWithWrongCredentialsShowsGermanErrorAndKeepsName(t *testing.T) {
	tests := map[string][2]string{
		"wrong password": {testUser, wrongPassword},
		"wrong user":     {"mallory", testPassword},
	}
	for name, credentials := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)

			rec := ts.do(loginRequest(credentials[0], credentials[1]))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			if !strings.Contains(body, msgWrongCredentials) {
				t.Errorf("body does not contain %q", msgWrongCredentials)
			}
			if !strings.Contains(body, `value="`+credentials[0]+`"`) {
				t.Errorf("body does not keep the user name %q", credentials[0])
			}
			if strings.Contains(body, credentials[1]) {
				t.Error("body echoes the password")
			}
			if sessionCookieFrom(t, rec) != nil {
				t.Error("failed login set a session cookie")
			}
		})
	}
}

func TestLoginAlwaysRunsBcryptEvenForUnknownUser(t *testing.T) {
	ts := newTestServer(t)

	ts.do(loginRequest("mallory", wrongPassword))

	if ts.passwordChecks != 1 {
		t.Errorf("password checks = %d, want 1", ts.passwordChecks)
	}
}

func TestFailedLoginLogNeverContainsPasswordNameOrIP(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins + 1 {
		ts.do(loginRequest("mallory", wrongPassword))
	}

	logs := ts.logs.String()
	if !strings.Contains(logs, logMsgLoginFailed) {
		t.Errorf("log %q does not record the failed login", logs)
	}
	for _, secret := range []string{wrongPassword, "mallory", testIP, "100.64.0.2", string(testSecret)} {
		if strings.Contains(logs, secret) {
			t.Errorf("log contains %q", secret)
		}
	}
}

func TestLoginIsLockedAfterMaximumFailuresWithoutCheckingPassword(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins {
		ts.do(loginRequest(testUser, wrongPassword))
	}
	checksBefore := ts.passwordChecks

	rec := ts.do(loginRequest(testUser, testPassword))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if !strings.Contains(rec.Body.String(), msgLockedOut) {
		t.Errorf("body does not contain %q", msgLockedOut)
	}
	if ts.passwordChecks != checksBefore {
		t.Error("bcrypt ran during the lockout")
	}
	if sessionCookieFrom(t, rec) != nil {
		t.Error("locked login set a session cookie")
	}
}

func TestLoginLockoutIsKeyedOnForwardedClientIP(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins {
		ts.do(loginRequest(testUser, wrongPassword))
	}

	other := loginRequest(testUser, testPassword)
	other.Header.Set("X-Forwarded-For", otherTestIP+", 198.51.100.4")

	assertRedirect(t, ts.do(other), homePath)
}

func TestLoginRejectsSixthAttemptWhileEarlierAttemptsAreInFlight(t *testing.T) {
	ts := newTestServer(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	ts.handler.comparePassword = func(_, _ []byte) error {
		// Only the first attempts block, so a lockout bypass fails the test
		// instead of hanging it.
		if calls.Add(1) <= maxFailedLogins {
			entered <- struct{}{}
			<-release
		}
		return bcrypt.ErrMismatchedHashAndPassword
	}
	done := make(chan struct{})
	for range maxFailedLogins {
		go func() {
			ts.do(loginRequest(testUser, wrongPassword))
			done <- struct{}{}
		}()
	}
	for range maxFailedLogins {
		<-entered
	}

	rec := ts.do(loginRequest(testUser, testPassword))

	close(release)
	for range maxFailedLogins {
		<-done
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginIgnoresSurroundingWhitespaceInUserName(t *testing.T) {
	ts := newTestServer(t)

	assertRedirect(t, ts.do(loginRequest(" "+testUser+" ", testPassword)), homePath)
}

func TestLoginLockoutEndsAfterDuration(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins {
		ts.do(loginRequest(testUser, wrongPassword))
	}

	ts.clock.advance(lockoutDuration)

	assertRedirect(t, ts.do(loginRequest(testUser, testPassword)), homePath)
}

func TestSuccessfulLoginResetsFailureCount(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins - 1 {
		ts.do(loginRequest(testUser, wrongPassword))
	}
	ts.do(loginRequest(testUser, testPassword))
	for range maxFailedLogins - 1 {
		ts.do(loginRequest(testUser, wrongPassword))
	}

	assertRedirect(t, ts.do(loginRequest(testUser, testPassword)), homePath)
}

func TestLoginPageRendersGermanFormWithLocalHtmx(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(httptest.NewRequest(http.MethodGet, loginPath, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{`lang="de"`, `<script src="` + htmxPath + `"`, `action="` + loginPath + `"`, "Anmelden", `name="` + loginFieldUser + `"`, `name="` + loginFieldPassword + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("login page does not contain %q", want)
		}
	}
	if ct := rec.Header().Get("Content-Type"); ct != htmlContentType {
		t.Errorf("Content-Type = %q, want %q", ct, htmlContentType)
	}
}

func TestLoginPageRedirectsHomeWhenSignedIn(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(withCookie(httptest.NewRequest(http.MethodGet, loginPath, nil), ts.validCookie()))

	assertRedirect(t, rec, homePath)
}

func TestHomeLinksLocationsAndShowsLogoutButton(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(withCookie(httptest.NewRequest(http.MethodGet, homePath, nil), ts.validCookie()))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"<h1>", `href="` + locationsPath + `"`, ">Orte<", `action="` + logoutPath + `"`, "Abmelden"} {
		if !strings.Contains(body, want) {
			t.Errorf("home page does not contain %q", want)
		}
	}
}

func TestAuthenticatedRequestRenewsActivityButNotLoginTime(t *testing.T) {
	ts := newTestServer(t)
	cookie := ts.validCookie()
	ts.clock.advance(time.Hour)

	rec := ts.do(withCookie(httptest.NewRequest(http.MethodGet, homePath, nil), cookie))

	renewed := sessionCookieFrom(t, rec)
	if renewed == nil {
		t.Fatal("authenticated request did not renew the session cookie")
	}
	s, ok := ts.handler.sessions.decode(renewed.Value)
	if !ok {
		t.Fatal("renewed cookie does not decode")
	}
	if !s.LoginAt.Equal(testLoginTime) || !s.ActiveAt.Equal(testLoginTime.Add(time.Hour)) {
		t.Errorf("renewed session = %+v, want login %v, activity %v", s, testLoginTime, testLoginTime.Add(time.Hour))
	}
}

func TestRequestsWithoutValidSessionAreRedirectedToLogin(t *testing.T) {
	requests := map[string]func() *http.Request{
		"GET home":          func() *http.Request { return httptest.NewRequest(http.MethodGet, homePath, nil) },
		"GET unknown page":  func() *http.Request { return httptest.NewRequest(http.MethodGet, "/admin/locations", nil) },
		"POST logout":       func() *http.Request { return httptest.NewRequest(http.MethodPost, logoutPath, nil) },
		"POST unknown page": func() *http.Request { return httptest.NewRequest(http.MethodPost, "/admin/locations", nil) },
		"POST static":       func() *http.Request { return httptest.NewRequest(http.MethodPost, htmxPath, nil) },
	}
	for name, newRequest := range requests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)

			rec := ts.do(newRequest())

			assertRedirect(t, rec, loginPath)
			if strings.Contains(rec.Body.String(), "Abmelden") {
				t.Error("protected handler ran without a session")
			}
		})
	}
}

func TestInvalidSessionsAreTreatedAsMissing(t *testing.T) {
	ts := newTestServer(t)
	login := testLoginTime.Add(-sessionMaxLifetime)
	cookies := map[string]*http.Cookie{
		"idle too long":           ts.handler.sessions.cookie(session{LoginAt: testLoginTime.Add(-sessionIdleTimeout), ActiveAt: testLoginTime.Add(-sessionIdleTimeout)}, testLoginTime),
		"older than max lifetime": ts.handler.sessions.cookie(session{LoginAt: login, ActiveAt: testLoginTime.Add(-time.Minute)}, testLoginTime),
		"wrong mac":               {Name: sessionCookieName, Value: ts.validCookie().Value + "x"},
		"broken format":           {Name: sessionCookieName, Value: "not-a-session"},
		"old secret":              sessionCodec{secret: otherSecret}.cookie(session{LoginAt: testLoginTime, ActiveAt: testLoginTime}, testLoginTime),
	}
	for name, cookie := range cookies {
		t.Run(name, func(t *testing.T) {
			rec := ts.do(withCookie(httptest.NewRequest(http.MethodGet, homePath, nil), cookie))
			assertRedirect(t, rec, loginPath)
		})
	}
}

func TestHtmxRequestsWithoutSessionGetHXRedirect(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/locations", nil)
	req.Header.Set(htmxRequestHeader, "true")

	rec := ts.do(req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get(htmxRedirectHeader); got != loginPath {
		t.Errorf("%s = %q, want %q", htmxRedirectHeader, got, loginPath)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q, want none", got)
	}
}

func TestCrossSiteFormPostsAreRejected(t *testing.T) {
	ts := newTestServer(t)
	for _, path := range []string{loginPath, logoutPath} {
		t.Run(path, func(t *testing.T) {
			req := loginRequest(testUser, testPassword)
			req.URL.Path = path
			req.Header.Set("Sec-Fetch-Site", "cross-site")

			rec := ts.do(withCookie(req, ts.validCookie()))

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}
}

func TestLogoutDeletesCookieAndRedirectsToLogin(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(withCookie(httptest.NewRequest(http.MethodPost, logoutPath, nil), ts.validCookie()))

	assertRedirect(t, rec, loginPath)
	cookie := sessionCookieFrom(t, rec)
	if cookie == nil || cookie.MaxAge >= 0 || cookie.Value != "" {
		t.Errorf("cookie = %+v, want a deletion cookie", cookie)
	}
}

func TestStaticFilesAreServedWithoutSession(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(httptest.NewRequest(http.MethodGet, htmxPath, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.HasPrefix(rec.Body.String(), "var htmx=") {
		t.Error("body is not htmx")
	}
}

func TestMapFilesAreServedWithoutSession(t *testing.T) {
	tests := map[string]struct {
		// contentType is matched as a substring: the MIME type of .js
		// depends on the OS (text/ on Linux, application/ on Windows).
		path, contentType, prefix string
	}{
		"Leaflet script": {leafletScriptPath, "javascript", "/* @preserve\n * Leaflet 1.9.4,"},
		"Leaflet styles": {leafletStylePath, "text/css", "/* required styles */"},
		"marker image":   {leafletMarkerIconPath, "image/png", "\x89PNG"},
		"map script":     {locationMapScriptPath, "javascript", "// location-map.js couples"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)

			rec := ts.do(httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Errorf("Content-Type = %q, want %q", got, tt.contentType)
			}
			if !strings.HasPrefix(rec.Body.String(), tt.prefix) {
				t.Errorf("body does not start with %q", tt.prefix)
			}
		})
	}
}

func TestAdminResponsesCarryNoCORSHeaders(t *testing.T) {
	ts := newTestServer(t)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, loginPath, nil),
		httptest.NewRequest(http.MethodOptions, loginPath, nil),
		httptest.NewRequest(http.MethodGet, htmxPath, nil),
		withCookie(httptest.NewRequest(http.MethodGet, homePath, nil), ts.validCookie()),
		httptest.NewRequest(http.MethodGet, homePath, nil),
		loginRequest(testUser, testPassword),
	}
	for _, req := range requests {
		req.Header.Set("Origin", "https://example.org")
		rec := ts.do(req)
		for name := range rec.Header() {
			if strings.HasPrefix(http.CanonicalHeaderKey(name), "Access-Control-") {
				t.Errorf("%s %s: response has header %s", req.Method, req.URL.Path, name)
			}
		}
	}
}

func TestRenderFailureAnswersInternalServerErrorAndLogs(t *testing.T) {
	ts := newTestServer(t)
	broken := template.Must(template.New(layoutTemplate).Parse("{{.Missing}}"))
	rec := httptest.NewRecorder()

	ts.handler.render(rec, broken, http.StatusOK, loginPage{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(ts.logs.String(), logMsgRenderFailed) {
		t.Errorf("log %q does not record the render failure", ts.logs.String())
	}
}

func TestNewHandlerServesLoginPage(t *testing.T) {
	ts := newTestServer(t)
	public := NewHandler(Config{
		User:          testUser,
		PasswordHash:  ts.handler.passwordHash,
		SessionSecret: testSecret,
		Logger:        slog.New(slog.DiscardHandler),
		Now:           ts.clock.now,
	})

	rec := httptest.NewRecorder()
	public.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, loginPath, nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
