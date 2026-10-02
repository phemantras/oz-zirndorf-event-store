package admin

import (
	"net"
	"net/http"
	"strings"
)

const (
	forwardedForHeader    = "X-Forwarded-For"
	forwardedForSeparator = ","
)

// clientIP returns the address the login lockout counts against. Railway's
// edge discards client-sent X-Forwarded-For values and puts the real client
// first (measured in Story 1.2, see README), so the leftmost entry is
// trustworthy there. Without the header it falls back to RemoteAddr without
// port. X-Real-IP is deliberately ignored.
func clientIP(r *http.Request) string {
	leftmost, _, _ := strings.Cut(r.Header.Get(forwardedForHeader), forwardedForSeparator)
	if ip := strings.TrimSpace(leftmost); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
