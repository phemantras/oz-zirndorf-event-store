package admin

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPPrefersLeftmostForwardedForEntry(t *testing.T) {
	tests := []struct {
		name          string
		forwardedFor  []string
		realIP        string
		remoteAddress string
		want          string
	}{
		{name: "railway chain", forwardedFor: []string{" 203.0.113.7 , 198.51.100.4"}, remoteAddress: "100.64.0.2:1234", want: "203.0.113.7"},
		{name: "single entry", forwardedFor: []string{"203.0.113.7"}, remoteAddress: "100.64.0.2:1234", want: "203.0.113.7"},
		{name: "several header lines", forwardedFor: []string{"203.0.113.7", "198.51.100.4"}, remoteAddress: "100.64.0.2:1234", want: "203.0.113.7"},
		{name: "no header", remoteAddress: "192.0.2.1:5678", want: "192.0.2.1"},
		{name: "blank header", forwardedFor: []string{"  "}, remoteAddress: "192.0.2.1:5678", want: "192.0.2.1"},
		{name: "ipv6 remote", remoteAddress: "[2001:db8::1]:5678", want: "2001:db8::1"},
		{name: "remote without port", remoteAddress: "192.0.2.1", want: "192.0.2.1"},
		{name: "x-real-ip ignored", realIP: "203.0.113.99", remoteAddress: "192.0.2.1:5678", want: "192.0.2.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", loginPath, nil)
			req.RemoteAddr = tt.remoteAddress
			for _, value := range tt.forwardedFor {
				req.Header.Add("X-Forwarded-For", value)
			}
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			if got := clientIP(req); got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLockoutKeyGroupsIPv6ByNetworkAndKeepsIPv4Exact(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{name: "ipv4", ip: "203.0.113.7", want: "203.0.113.7"},
		{name: "neighbouring ipv4", ip: "203.0.113.8", want: "203.0.113.8"},
		{name: "ipv6", ip: "2001:db8:1:2::1", want: "2001:db8:1:2::/64"},
		{name: "same ipv6 network", ip: "2001:db8:1:2::ffff", want: "2001:db8:1:2::/64"},
		{name: "full ipv6 address", ip: "2001:db8:1:2:aaaa:bbbb:cccc:dddd", want: "2001:db8:1:2::/64"},
		{name: "other ipv6 network", ip: "2001:db8:1:3::1", want: "2001:db8:1:3::/64"},
		{name: "ipv4 in ipv6", ip: "::ffff:203.0.113.7", want: "203.0.113.7"},
		{name: "ipv6 zone dropped", ip: "fe80::1%eth0", want: "fe80::/64"},
		{name: "invalid", ip: "unknown", want: "unknown"},
		{name: "empty", ip: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lockoutKey(tt.ip); got != tt.want {
				t.Errorf("lockoutKey(%q) = %q, want %q", tt.ip, got, tt.want)
			}
		})
	}
}
