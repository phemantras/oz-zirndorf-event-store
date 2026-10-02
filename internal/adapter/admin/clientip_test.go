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
