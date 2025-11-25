package httputil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func() *http.Request
		expectedIP string
	}{
		{
			name: "X-Forwarded-For single IPv4",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Forwarded-For", "203.0.113.5")
				return r
			},
			expectedIP: "203.0.113.5",
		},
		{
			name: "X-Forwarded-For multiple IPs (take first)",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Forwarded-For", "198.51.100.7, 203.0.113.9, 192.0.2.1")
				return r
			},
			expectedIP: "198.51.100.7",
		},
		{
			name: "X-Forwarded-For IPv6",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Forwarded-For", "2001:db8::1, 203.0.113.9")
				return r
			},
			expectedIP: "2001:db8::1",
		},
		{
			name: "X-Forwarded-For with spaces",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Forwarded-For", "  203.0.113.10  ,  198.51.100.2  ")
				return r
			},
			expectedIP: "203.0.113.10",
		},
		{
			name: "X-Real-IP takes effect when no XFF",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Real-IP", "203.0.113.12")
				return r
			},
			expectedIP: "203.0.113.12",
		},
		{
			name: "X-Real-IP IPv6",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Real-IP", "2001:db8::2")
				return r
			},
			expectedIP: "2001:db8::2",
		},
		{
			name: "Fallback to RemoteAddr IPv4",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.RemoteAddr = "192.0.2.55:54321"
				return r
			},
			expectedIP: "192.0.2.55",
		},
		{
			name: "Fallback to RemoteAddr IPv6 (bracketed)",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.RemoteAddr = "[2001:db8::5]:8443"
				return r
			},
			expectedIP: "2001:db8::5",
		},
		{
			name: "Malformed RemoteAddr returns raw",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.RemoteAddr = "not_an_ip_port"
				return r
			},
			expectedIP: "not_an_ip_port",
		},
		{
			name: "XFF takes precedence over X-Real-IP",
			setupReq: func() *http.Request {
				r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
				r.Header.Set("X-Forwarded-For", "198.51.100.77, 203.0.113.1")
				r.Header.Set("X-Real-IP", "203.0.113.200")
				return r
			},
			expectedIP: "198.51.100.77",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			got := GetClientIP(req)
			assert.Equal(t, tt.expectedIP, got)
		})
	}
}
