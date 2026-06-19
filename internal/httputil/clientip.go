package httputil

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the real client IP from the request.
// It honors X-Forwarded-For and X-Real-IP only when RemoteAddr belongs to a
// configured trusted proxy CIDR. Otherwise, it falls back to RemoteAddr.
// Properly handles IPv6 addresses including bracketed notation.
func GetClientIP(r *http.Request, trustedProxyCIDRs ...string) string {
	remoteIP := remoteAddrIP(r.RemoteAddr)
	if !isTrustedProxy(remoteIP, trustedProxyCIDRs) {
		return remoteIP
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, rawIP := range strings.Split(xff, ",") {
			if ip := strings.TrimSpace(rawIP); ip != "" {
				return ip
			}
		}
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	return remoteIP
}

// ValidateTrustedProxyCIDRs validates that every configured trusted proxy entry
// is an explicit CIDR, such as 10.0.0.0/24 or 2001:db8::/32.
func ValidateTrustedProxyCIDRs(cidrs []string) error {
	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid trusted proxy CIDR %q: %w", cidr, err)
		}
	}
	return nil
}

func remoteAddrIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}

func isTrustedProxy(remoteIP string, trustedProxyCIDRs []string) bool {
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}

	for _, cidr := range trustedProxyCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
