package httputil

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the real client IP from the request.
// It checks X-Forwarded-For first (taking the first IP in the chain),
// then X-Real-IP, and finally falls back to RemoteAddr.
// Properly handles IPv6 addresses including bracketed notation.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			if ip := strings.TrimSpace(ips[0]); ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
