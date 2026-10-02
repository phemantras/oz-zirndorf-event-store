package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookieName = "eventstore_admin_session"
	// sessionIdleTimeout ends a session after this long without a request.
	sessionIdleTimeout = 8 * time.Hour
	// sessionMaxLifetime ends a session this long after the login, however
	// active it is.
	sessionMaxLifetime = 7 * 24 * time.Hour

	sessionValueSeparator = "."
	// sessionValueParts are login time, activity time and MAC.
	sessionValueParts = 3
	timestampBase     = 10
	timestampBits     = 64
)

// session is the stateless admin session carried in the signed cookie.
type session struct {
	LoginAt  time.Time
	ActiveAt time.Time
}

// expiresAt returns the earlier of idle timeout and maximum lifetime.
func (s session) expiresAt() time.Time {
	idleEnd := s.ActiveAt.Add(sessionIdleTimeout)
	lifetimeEnd := s.LoginAt.Add(sessionMaxLifetime)
	if lifetimeEnd.Before(idleEnd) {
		return lifetimeEnd
	}
	return idleEnd
}

// isValidAt reports whether the session has not expired at now.
func (s session) isValidAt(now time.Time) bool {
	return now.Before(s.expiresAt())
}

// sessionCodec signs and verifies session cookie values with HMAC-SHA256.
type sessionCodec struct {
	secret []byte
}

// encode returns "<login_unix>.<activity_unix>.<base64url(mac)>".
func (c sessionCodec) encode(s session) string {
	payload := strconv.FormatInt(s.LoginAt.Unix(), timestampBase) +
		sessionValueSeparator +
		strconv.FormatInt(s.ActiveAt.Unix(), timestampBase)
	return payload + sessionValueSeparator + base64.RawURLEncoding.EncodeToString(c.mac(payload))
}

// decode returns the session in value and whether its format and MAC are
// valid. It does not check expiry.
func (c sessionCodec) decode(value string) (session, bool) {
	parts := strings.Split(value, sessionValueSeparator)
	if len(parts) != sessionValueParts {
		return session{}, false
	}
	loginUnix, err := strconv.ParseInt(parts[0], timestampBase, timestampBits)
	if err != nil {
		return session{}, false
	}
	activeUnix, err := strconv.ParseInt(parts[1], timestampBase, timestampBits)
	if err != nil {
		return session{}, false
	}
	gotMAC, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return session{}, false
	}
	if !hmac.Equal(gotMAC, c.mac(parts[0]+sessionValueSeparator+parts[1])) {
		return session{}, false
	}
	return session{LoginAt: time.Unix(loginUnix, 0), ActiveAt: time.Unix(activeUnix, 0)}, true
}

func (c sessionCodec) mac(payload string) []byte {
	m := hmac.New(sha256.New, c.secret)
	m.Write([]byte(payload)) // hash.Hash.Write never returns an error.
	return m.Sum(nil)
}

// cookie returns the signed session cookie, valid from now until the
// session expires. Max-Age is rounded up so it never reaches zero, which
// net/http would omit.
func (c sessionCodec) cookie(s session, now time.Time) *http.Cookie {
	remaining := s.expiresAt().Sub(now)
	maxAge := int((remaining + time.Second - 1) / time.Second)
	cookie := hardenedSessionCookie()
	cookie.Value = c.encode(s)
	cookie.MaxAge = maxAge
	return cookie
}

// expiredSessionCookie returns a cookie that makes the browser delete the
// session.
func expiredSessionCookie() *http.Cookie {
	cookie := hardenedSessionCookie()
	cookie.MaxAge = -1
	return cookie
}

func hardenedSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Path:     adminPathPrefix,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}
