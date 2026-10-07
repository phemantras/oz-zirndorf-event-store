package admin

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const (
	forwardedForHeader    = "X-Forwarded-For"
	forwardedForSeparator = ","
	// ipv6LockoutPrefixBits is the IPv6 network a single connection usually
	// owns as a whole, so the lockout counts it as one client.
	ipv6LockoutPrefixBits = 64
)

// clientIP returns the client address of the login request. Railway's
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

// lockoutKey returns what the login lockout counts ip against: an IPv4
// address as is, IPv4-mapped IPv6 as its IPv4 address and any other IPv6
// address as its /64 network without zone. A value that is no valid IP is
// returned unchanged.
func lockoutKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr.WithZone(""), ipv6LockoutPrefixBits).Masked().String()
}
