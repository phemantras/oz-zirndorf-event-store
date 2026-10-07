package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
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
	// testEdgeIP is the Railway edge node that follows the client IP in
	// X-Forwarded-For.
	testEdgeIP = "192.0.2.254"
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
	locations       *memoryLocationRepo
	events          *memoryEventRepo
	eventService    *core.EventService
	locationService *core.LocationService
	importService   *core.ImportService
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
	tx := memoryTx{repos: core.Repos{Events: ts.events, Locations: ts.locations}}
	ts.eventService = core.NewEventService(tx, ts.events, ts.locations)
	ts.locationService = core.NewLocationService(tx, ts.locations)
	ts.importService = core.NewImportService(ts.eventService)
	ts.handler = newHandler(Config{
		User:          testUser,
		PasswordHash:  hash,
		SessionSecret: testSecret,
		Logger:        slog.New(slog.NewJSONHandler(ts.logs, nil)),
		Now:           ts.clock.now,
		Locations:     ts.locationService,
		Events:        ts.eventService,
		Clock:         ts.clock,
		Imports:       ts.importService,
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
	req.Header.Set("X-Forwarded-For", testIP+", "+testEdgeIP)
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
	other.Header.Set("X-Forwarded-For", otherTestIP+", "+testEdgeIP)

	assertRedirect(t, ts.do(other), homePath)
}

func TestLoginCountsAttemptsAsFailuresWhileInFlight(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins - maxConcurrentPasswordChecks {
		ts.do(loginRequest(testUser, wrongPassword))
	}
	held := holdPasswordChecks(ts, maxConcurrentPasswordChecks, func() *http.Request {
		return loginRequest(testUser, wrongPassword)
	})

	// Both slots are taken now, so a further attempt would get 503 before the
	// lockout check; the map shows that the running attempts already count.
	countInFlight := ts.lockoutFailures(testIP)
	held.release()
	rec := ts.do(loginRequest(testUser, testPassword))

	if countInFlight != maxFailedLogins {
		t.Errorf("failure count while in flight = %d, want %d", countInFlight, maxFailedLogins)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

// lockoutFailures returns the failure count stored under key.
func (ts *testServer) lockoutFailures(key string) int {
	ts.handler.lockout.mu.Lock()
	defer ts.handler.lockout.mu.Unlock()
	return ts.handler.lockout.failures[key].count
}

// hasLockoutEntry reports whether the lockout stores a record under key.
func (ts *testServer) hasLockoutEntry(key string) bool {
	ts.handler.lockout.mu.Lock()
	defer ts.handler.lockout.mu.Unlock()
	_, ok := ts.handler.lockout.failures[key]
	return ok
}

// heldLogins are logins whose password comparisons block until release.
type heldLogins struct {
	calls   *atomic.Int32
	release func()
}

// holdPasswordChecks starts count logins from newRequest and returns once
// exactly their count password comparisons block. Comparisons after those
// fail at once; every comparison is counted. release lets the held
// comparisons finish and waits for their logins.
func holdPasswordChecks(ts *testServer, count int, newRequest func() *http.Request) *heldLogins {
	calls := &atomic.Int32{}
	entered := make(chan struct{})
	unblock := make(chan struct{})
	ts.handler.comparePassword = func(_, _ []byte) error {
		// Only the held calls block, so an unexpected extra check fails the
		// test instead of hanging it.
		if int(calls.Add(1)) <= count {
			entered <- struct{}{}
			<-unblock
		}
		return bcrypt.ErrMismatchedHashAndPassword
	}
	done := make(chan struct{}, count)
	for range count {
		go func() {
			ts.do(newRequest())
			done <- struct{}{}
		}()
	}
	for range count {
		<-entered
	}
	return &heldLogins{calls: calls, release: func() {
		close(unblock)
		for range count {
			<-done
		}
	}}
}

// countPasswordChecks replaces the password comparison of ts with bcrypt
// and counts its calls safely across goroutines.
func countPasswordChecks(ts *testServer) *atomic.Int32 {
	calls := &atomic.Int32{}
	ts.handler.comparePassword = func(hash, password []byte) error {
		calls.Add(1)
		return bcrypt.CompareHashAndPassword(hash, password)
	}
	return calls
}

func loginRequestFrom(ip, user, password string) *http.Request {
	req := loginRequest(user, password)
	req.Header.Set("X-Forwarded-For", ip+", "+testEdgeIP)
	return req
}

// stalledBody is a login form body whose first read signals on reading and
// then blocks until resume is closed, like a client that sends slowly.
type stalledBody struct {
	reading chan<- struct{}
	resume  <-chan struct{}
	form    *strings.Reader
	started bool
}

func (b *stalledBody) Read(p []byte) (int, error) {
	if !b.started {
		b.started = true
		b.reading <- struct{}{}
		<-b.resume
	}
	return b.form.Read(p)
}

func TestLoginRejectsAttemptBeyondConcurrentPasswordChecksWithoutBcrypt(t *testing.T) {
	ts := newTestServer(t)
	held := holdPasswordChecks(ts, maxConcurrentPasswordChecks, func() *http.Request {
		return loginRequest(testUser, wrongPassword)
	})

	rec := ts.do(loginRequestFrom(otherTestIP, testUser, testPassword))

	held.release()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := rec.Body.String()
	if !strings.Contains(body, msgTooManyLogins) {
		t.Errorf("body does not contain %q", msgTooManyLogins)
	}
	if !strings.Contains(body, `value="`+testUser+`"`) {
		t.Errorf("body does not keep the user name %q", testUser)
	}
	if strings.Contains(body, msgLockedOut) {
		t.Errorf("body shows the lockout message %q", msgLockedOut)
	}
	if got := held.calls.Load(); got != maxConcurrentPasswordChecks {
		t.Errorf("password checks = %d, want %d", got, maxConcurrentPasswordChecks)
	}
	if ts.hasLockoutEntry(otherTestIP) {
		t.Error("rejected attempt created a lockout entry")
	}
	if sessionCookieFrom(t, rec) != nil {
		t.Error("rejected login set a session cookie")
	}
}

func TestLoginRejectedForBusyPasswordChecksDoesNotCountAsFailure(t *testing.T) {
	ts := newTestServer(t)
	held := holdPasswordChecks(ts, maxConcurrentPasswordChecks, func() *http.Request {
		return loginRequest(testUser, wrongPassword)
	})

	ts.do(loginRequest(testUser, wrongPassword))

	held.release()
	if got := ts.lockoutFailures(testIP); got != maxConcurrentPasswordChecks {
		t.Errorf("failure count = %d, want %d", got, maxConcurrentPasswordChecks)
	}
}

func TestLoginAnswersLockedKeyWithBusyStatusWhileSlotsAreTaken(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins {
		ts.do(loginRequest(testUser, wrongPassword))
	}
	held := holdPasswordChecks(ts, maxConcurrentPasswordChecks, func() *http.Request {
		return loginRequestFrom(otherTestIP, testUser, wrongPassword)
	})

	rec := ts.do(loginRequest(testUser, testPassword))

	held.release()
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := ts.lockoutFailures(testIP); got != maxFailedLogins {
		t.Errorf("failure count = %d, want %d", got, maxFailedLogins)
	}
}

func TestLoginChecksPasswordAgainOnceSlotsAreFree(t *testing.T) {
	ts := newTestServer(t)
	held := holdPasswordChecks(ts, maxConcurrentPasswordChecks, func() *http.Request {
		return loginRequest(testUser, wrongPassword)
	})
	held.release()

	rec := ts.do(loginRequestFrom(otherTestIP, testUser, wrongPassword))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := held.calls.Load(); got != maxConcurrentPasswordChecks+1 {
		t.Errorf("password checks = %d, want %d", got, maxConcurrentPasswordChecks+1)
	}
}

func TestLoginFreesSlotsAfterLockedAndSuccessfulAttempts(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins {
		ts.do(loginRequest(testUser, wrongPassword))
	}
	for range maxConcurrentPasswordChecks {
		if rec := ts.do(loginRequest(testUser, testPassword)); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("locked attempt: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}
	}
	for range maxConcurrentPasswordChecks {
		assertRedirect(t, ts.do(loginRequestFrom(otherTestIP, testUser, testPassword)), homePath)
	}
	calls := countPasswordChecks(ts)

	rec := ts.do(loginRequestFrom(otherTestIP, testUser, wrongPassword))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("password checks = %d, want 1", got)
	}
}

func TestLoginReadsFormBeforeTakingPasswordCheckSlot(t *testing.T) {
	ts := newTestServer(t)
	calls := countPasswordChecks(ts)
	reading := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan struct{}, maxConcurrentPasswordChecks)
	form := url.Values{loginFieldUser: {testUser}, loginFieldPassword: {wrongPassword}}.Encode()
	for range maxConcurrentPasswordChecks {
		req := loginRequest(testUser, wrongPassword)
		req.Body = io.NopCloser(&stalledBody{reading: reading, resume: resume, form: strings.NewReader(form)})
		go func() {
			ts.do(req)
			done <- struct{}{}
		}()
	}
	for range maxConcurrentPasswordChecks {
		<-reading
	}

	rec := ts.do(loginRequestFrom(otherTestIP, testUser, wrongPassword))

	close(resume)
	for range maxConcurrentPasswordChecks {
		<-done
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := calls.Load(); got != maxConcurrentPasswordChecks+1 {
		t.Errorf("password checks = %d, want %d", got, maxConcurrentPasswordChecks+1)
	}
}

func TestLoginRejectedForBusyPasswordChecksLogsWarningWithoutSecrets(t *testing.T) {
	ts := newTestServer(t)
	held := holdPasswordChecks(ts, maxConcurrentPasswordChecks, func() *http.Request {
		return loginRequest("mallory", wrongPassword)
	})

	ts.do(loginRequestFrom(otherTestIP, "trudy", wrongPassword))

	held.release()
	assertLoggedAt(t, ts, slog.LevelWarn, logMsgLoginBusy)
	logs := ts.logs.String()
	for _, secret := range []string{wrongPassword, "mallory", "trudy", testIP, otherTestIP, testEdgeIP, "100.64.0.2"} {
		if strings.Contains(logs, secret) {
			t.Errorf("log contains %q", secret)
		}
	}
}

func TestLoginLockoutCoversWholeIPv6Network(t *testing.T) {
	ts := newTestServer(t)
	for i := range maxFailedLogins {
		ts.do(loginRequestFrom(fmt.Sprintf("2001:db8:1:2::%d", i+1), testUser, wrongPassword))
	}

	sameNetwork := ts.do(loginRequestFrom("2001:db8:1:2::ffff", testUser, testPassword))
	otherNetwork := ts.do(loginRequestFrom("2001:db8:1:3::1", testUser, testPassword))

	if sameNetwork.Code != http.StatusTooManyRequests {
		t.Errorf("same /64: status = %d, want %d", sameNetwork.Code, http.StatusTooManyRequests)
	}
	assertRedirect(t, otherNetwork, homePath)
}

func TestSuccessfulLoginResetsFailureCountOfWholeIPv6Network(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins - 1 {
		ts.do(loginRequestFrom("2001:db8:1:2::1", testUser, wrongPassword))
	}

	assertRedirect(t, ts.do(loginRequestFrom("2001:db8:1:2::2", testUser, testPassword)), homePath)

	if ts.hasLockoutEntry("2001:db8:1:2::/64") {
		t.Error("successful login kept the failures of its /64 network")
	}
}

func TestLoginLockoutKeepsIPv4AddressesApart(t *testing.T) {
	ts := newTestServer(t)
	for range maxFailedLogins {
		ts.do(loginRequestFrom("203.0.113.7", testUser, wrongPassword))
	}

	assertRedirect(t, ts.do(loginRequestFrom("203.0.113.8", testUser, testPassword)), homePath)
}

func TestLoginLockoutCountsInvalidClientIPUnchanged(t *testing.T) {
	ts := newTestServer(t)

	ts.do(loginRequestFrom("unknown", testUser, wrongPassword))

	if got := ts.lockoutFailures("unknown"); got != 1 {
		t.Errorf("failure count for %q = %d, want 1", "unknown", got)
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

// errRequestTimedOut is a failure of a use case whose request ran out of
// its deadline, as with a slow database or an exhausted pool.
var errRequestTimedOut = fmt.Errorf("use case: %w", context.DeadlineExceeded)

// errRequestCancelled is a failure of a use case whose request the client
// cancelled.
var errRequestCancelled = fmt.Errorf("use case: %w", context.Canceled)

// errStatementCanceled is how pgx reports a query the database server
// cancelled at the request deadline (SQLSTATE 57014). It wraps neither
// context error, so only the ended request context tells what happened.
var errStatementCanceled = errors.New("use case: ERROR: canceling statement due to user request (SQLSTATE 57014)")

// withExpiredDeadline returns req with a request context whose deadline
// has passed.
func withExpiredDeadline(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	ctx, cancel := context.WithDeadline(req.Context(), time.Unix(0, 0))
	t.Cleanup(cancel)
	return req.WithContext(ctx)
}

// withCancelledContext returns req with a request context the client has
// cancelled.
func withCancelledContext(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	return req.WithContext(ctx)
}

// assertFailurePage checks that rec is the German failure page with status
// and want's message and link back, without the English status text.
func assertFailurePage(t *testing.T, rec *httptest.ResponseRecorder, status int, want failurePage) {
	t.Helper()
	assertStatusCode(t, rec, status)
	assertBodyContains(t, rec, "<h1>"+want.Message+"</h1>", `href="`+want.BackURL+`">`+want.BackLabel+`</a>`)
	assertBodyLacks(t, rec, http.StatusText(http.StatusInternalServerError), http.StatusText(http.StatusServiceUnavailable))
}

// logLine is the level and message of a JSON log line.
type logLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// logLinesOf decodes the JSON log lines of ts.
func logLinesOf(t *testing.T, ts *testServer) []logLine {
	t.Helper()
	var lines []logLine
	for _, raw := range strings.Split(strings.TrimSpace(ts.logs.String()), "\n") {
		var line logLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("decode log line %q: %v", raw, err)
		}
		lines = append(lines, line)
	}
	return lines
}

// assertLoggedAt checks that the logs of ts record msg at level.
func assertLoggedAt(t *testing.T, ts *testServer, level slog.Level, msg string) {
	t.Helper()
	if want := (logLine{Level: level.String(), Msg: msg}); !slices.Contains(logLinesOf(t, ts), want) {
		t.Errorf("log %q has no %s %q", ts.logs.String(), want.Level, msg)
	}
}

// assertLoggedAsWarningOnly checks that the logs of ts record msg as a
// warning and hold no error.
func assertLoggedAsWarningOnly(t *testing.T, ts *testServer, msg string) {
	t.Helper()
	assertLoggedWithoutError(t, ts, slog.LevelWarn, msg)
}

// assertLoggedAsInfoOnly checks that the logs of ts record msg as info and
// hold no error.
func assertLoggedAsInfoOnly(t *testing.T, ts *testServer, msg string) {
	t.Helper()
	assertLoggedWithoutError(t, ts, slog.LevelInfo, msg)
}

// assertLoggedWithoutError checks that the logs of ts record msg at level
// and hold no error.
func assertLoggedWithoutError(t *testing.T, ts *testServer, level slog.Level, msg string) {
	t.Helper()
	assertLoggedAt(t, ts, level, msg)
	for _, line := range logLinesOf(t, ts) {
		if line.Level == slog.LevelError.String() {
			t.Errorf("log line %+v reports a failure that is no fault of the code as an error", line)
		}
	}
}
