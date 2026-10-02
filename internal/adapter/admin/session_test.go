package admin

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	testSecret  = []byte("0123456789abcdef0123456789abcdef")
	otherSecret = []byte("fedcba9876543210fedcba9876543210")
	// testLoginTime is a fixed instant so expiry arithmetic is deterministic.
	testLoginTime = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
)

func TestSessionCodecRoundTripsTimestamps(t *testing.T) {
	codec := sessionCodec{secret: testSecret}
	want := session{LoginAt: testLoginTime, ActiveAt: testLoginTime.Add(time.Hour)}

	got, ok := codec.decode(codec.encode(want))
	if !ok {
		t.Fatal("decode rejected a freshly encoded session")
	}
	if !got.LoginAt.Equal(want.LoginAt) || !got.ActiveAt.Equal(want.ActiveAt) {
		t.Errorf("decoded = %+v, want %+v", got, want)
	}
}

func TestSessionCodecEncodesLoginActivityAndMAC(t *testing.T) {
	value := sessionCodec{secret: testSecret}.encode(session{LoginAt: testLoginTime, ActiveAt: testLoginTime})

	parts := strings.Split(value, ".")
	if len(parts) != sessionValueParts {
		t.Fatalf("value %q has %d parts, want %d", value, len(parts), sessionValueParts)
	}
	unix := strconv.FormatInt(testLoginTime.Unix(), 10)
	if parts[0] != unix || parts[1] != unix {
		t.Errorf("timestamps = %q, %q, want %q", parts[0], parts[1], unix)
	}
}

func TestSessionCodecRejectsTamperedOrMalformedValues(t *testing.T) {
	codec := sessionCodec{secret: testSecret}
	valid := codec.encode(session{LoginAt: testLoginTime, ActiveAt: testLoginTime})
	parts := strings.Split(valid, ".")
	later := strconv.FormatInt(testLoginTime.Add(time.Hour).Unix(), 10)

	tests := map[string]string{
		"empty":              "",
		"too few parts":      parts[0] + "." + parts[1],
		"too many parts":     valid + ".x",
		"login not a number": "abc." + parts[1] + "." + parts[2],
		"activity not a num": parts[0] + ".abc." + parts[2],
		"mac not base64":     parts[0] + "." + parts[1] + ".!!!",
		"activity changed":   parts[0] + "." + later + "." + parts[2],
		"mac of other key":   sessionCodec{secret: otherSecret}.encode(session{LoginAt: testLoginTime, ActiveAt: testLoginTime}),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if _, ok := codec.decode(value); ok {
				t.Errorf("decode accepted %q", value)
			}
		})
	}
}

func TestSessionValidityHonoursIdleAndMaximumLifetime(t *testing.T) {
	tests := []struct {
		name     string
		activeAt time.Time
		now      time.Time
		want     bool
	}{
		{name: "fresh", activeAt: testLoginTime, now: testLoginTime, want: true},
		{name: "just before idle timeout", activeAt: testLoginTime, now: testLoginTime.Add(sessionIdleTimeout - time.Second), want: true},
		{name: "at idle timeout", activeAt: testLoginTime, now: testLoginTime.Add(sessionIdleTimeout), want: false},
		{name: "active but just before max lifetime", activeAt: testLoginTime.Add(sessionMaxLifetime - time.Hour), now: testLoginTime.Add(sessionMaxLifetime - time.Second), want: true},
		{name: "active but at max lifetime", activeAt: testLoginTime.Add(sessionMaxLifetime - time.Hour), now: testLoginTime.Add(sessionMaxLifetime), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := session{LoginAt: testLoginTime, ActiveAt: tt.activeAt}
			if got := s.isValidAt(tt.now); got != tt.want {
				t.Errorf("isValidAt = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionExpiresAtEarlierOfIdleAndMaximumLifetime(t *testing.T) {
	idle := session{LoginAt: testLoginTime, ActiveAt: testLoginTime}
	if got, want := idle.expiresAt(), testLoginTime.Add(sessionIdleTimeout); !got.Equal(want) {
		t.Errorf("idle expiresAt = %v, want %v", got, want)
	}
	capped := session{LoginAt: testLoginTime, ActiveAt: testLoginTime.Add(sessionMaxLifetime - time.Hour)}
	if got, want := capped.expiresAt(), testLoginTime.Add(sessionMaxLifetime); !got.Equal(want) {
		t.Errorf("capped expiresAt = %v, want %v", got, want)
	}
}

func TestSessionCookieIsHardenedAndScopedToAdmin(t *testing.T) {
	s := session{LoginAt: testLoginTime, ActiveAt: testLoginTime}
	cookie := sessionCodec{secret: testSecret}.cookie(s, testLoginTime)

	if cookie.Name != sessionCookieName || cookie.Path != adminPathPrefix {
		t.Errorf("name/path = %q/%q, want %q/%q", cookie.Name, cookie.Path, sessionCookieName, adminPathPrefix)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie = %+v, want HttpOnly, Secure, SameSite=Strict", cookie)
	}
	if want := int(sessionIdleTimeout / time.Second); cookie.MaxAge != want {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, want)
	}
}

func TestSessionCookieMaxAgeRoundsUpAndStaysPositive(t *testing.T) {
	s := session{LoginAt: testLoginTime, ActiveAt: testLoginTime}
	now := s.expiresAt().Add(-time.Millisecond)

	if got := (sessionCodec{secret: testSecret}).cookie(s, now).MaxAge; got != 1 {
		t.Errorf("MaxAge = %d, want 1", got)
	}
}

func TestExpiredSessionCookieDeletesTheSession(t *testing.T) {
	cookie := expiredSessionCookie()
	if cookie.Name != sessionCookieName || cookie.Path != adminPathPrefix || cookie.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want %s on %s with negative MaxAge", cookie, sessionCookieName, adminPathPrefix)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie = %+v, want HttpOnly, Secure, SameSite=Strict", cookie)
	}
}
