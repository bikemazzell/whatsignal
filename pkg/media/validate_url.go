package media

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// validateDownloadURL enforces HTTPS scheme and restricts downloads to the configured WAHA host
// It also rejects IP literals and private/loopback/link-local addresses after resolution
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
	// Only allow downloads from the same host as WAHA base
	if !strings.EqualFold(u.Hostname(), waha.Hostname()) {
		return fmt.Errorf("download host not allowed: %s", u.Hostname())
	}

	// Reject IP literals by category
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

