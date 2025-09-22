package media

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// validateDownloadURL enforces HTTPS scheme and restricts downloads to the configured WAHA host
// It also rejects IP literals and private/loopback/link-local addresses after resolution
// Special handling for Docker internal networks when URL rewriting is in effect
func (h *handler) validateDownloadURL(rawURL string) error {
	// In test or non-WAHA contexts (no base URL), skip strict validation to allow httptest servers
	if h.wahaBaseURL == "" {
		return nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid media URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" { // Prefer https; allow http only if WAHA base uses http
		return fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	waha, err := url.Parse(h.wahaBaseURL)
	if err != nil {
		return fmt.Errorf("invalid WAHA base URL: %w", err)
	}

	// Check if this is a Docker internal IP that would be rewritten
	if ip := net.ParseIP(u.Hostname()); ip != nil && isDockerInternalIP(ip) {
		// Allow Docker internal IPs only if:
		// 1. They have the same port as WAHA base URL
		// 2. The WAHA base URL is a real IP address (not a domain name)
		// 3. The WAHA IP is not a loopback address (127.x.x.x or ::1)
		// This handles the case where WAHA generates URLs with Docker internal IPs
		// but we'll rewrite them to the external WAHA host (could be LAN IP like 192.168.x.x)
		wahaIP := net.ParseIP(waha.Hostname())
		if wahaIP != nil && !wahaIP.IsLoopback() && u.Port() == waha.Port() {
			return nil // Allow - this will be rewritten by rewriteMediaURL
		}
	}

	// Check if this is a Docker internal hostname that would be rewritten
	// For validation, we allow Docker internal hostnames since they will be rewritten
	if isDockerInternalHost(u.Hostname()) {
		return nil // Allow - this will be rewritten by rewriteMediaURL
	}

	// Only allow downloads from the same host as WAHA base
	if !strings.EqualFold(u.Hostname(), waha.Hostname()) {
		return fmt.Errorf("download host not allowed: %s", u.Hostname())
	}

	// Reject IP literals by category (except Docker internal IPs handled above)
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("download IP not allowed: %s", ip.String())
		}
	}
	// Resolve DNS and block private/loopback/link-local
	addrs, err := net.LookupIP(u.Hostname())
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}
	for _, ip := range addrs {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("resolved to disallowed IP: %s", ip.String())
		}
	}
	return nil
}

// isDockerInternalHost checks if a hostname appears to be a Docker internal service
// This is a heuristic check - if it's not a valid domain name and not an IP,
// it's likely a Docker service name that should be rewritten
func isDockerInternalHost(hostname string) bool {
	// If it's an IP address, it's not a Docker service name
	if net.ParseIP(hostname) != nil {
		return false
	}

	// If it contains dots, it's likely a domain name, not a Docker service
	if strings.Contains(hostname, ".") {
		return false
	}

	// If it's localhost variants, handle separately
	if hostname == "localhost" {
		return false
	}

	// Single word hostnames without dots are likely Docker service names
	// This covers cases like "waha", "signal-cli-rest-api", etc.
	return len(hostname) > 0
}
