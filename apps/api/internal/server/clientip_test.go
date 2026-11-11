package server

import (
	"net/http/httptest"
	"testing"

	"github.com/granborg/wcs_workshops/apps/api/internal/config"
)

// The audit trail must not take an address the client wrote. X-Forwarded-For is never used, and
// Fly-Client-IP only behind Fly Proxy, which sets it.
func TestClientIP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		onFly  bool
		header map[string]string
		want   string
	}{
		{"connection address", false, nil, "203.0.113.7"},
		{"forged X-Forwarded-For", false, map[string]string{"X-Forwarded-For": "1.2.3.4"}, "203.0.113.7"},
		{"forged X-Forwarded-For on Fly", true, map[string]string{"X-Forwarded-For": "1.2.3.4", "Fly-Client-IP": "198.51.100.9"}, "198.51.100.9"},
		{"Fly-Client-IP off Fly", false, map[string]string{"Fly-Client-IP": "1.2.3.4"}, "203.0.113.7"},
		{"Fly-Client-IP not an address", true, map[string]string{"Fly-Client-IP": "evil"}, "203.0.113.7"},
		{"IPv6 on Fly", true, map[string]string{"Fly-Client-IP": "2001:db8::1"}, "2001:db8::1"},
	} {
		s := &Server{cfg: config.Config{BehindFlyProxy: tc.onFly}}
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.RemoteAddr = "203.0.113.7:51234"
		for k, v := range tc.header {
			r.Header.Set(k, v)
		}
		if got := *s.clientIP(r); got != tc.want {
			t.Errorf("%s: clientIP = %q, want %q", tc.name, got, tc.want)
		}
	}
}
