package media

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func (h *handler) validateDownloadURL(rawURL string) error {
	if h.wahaBaseURL == "" {
		return nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid media URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	waha, err := url.Parse(h.wahaBaseURL)
	if err != nil {
		return fmt.Errorf("invalid WAHA base URL: %w", err)
	}

	var signal *url.URL
	if h.signalRPCURL != "" {
		signal, err = url.Parse(h.signalRPCURL)
		if err != nil {
			return fmt.Errorf("invalid Signal RPC URL: %w", err)
		}
	}

	if endpointMatches(u, waha) || (signal != nil && endpointMatches(u, signal)) {
		return nil
	}

	if err := rejectDisallowedResolvedIP(u.Hostname()); err != nil {
		return err
	}

	if ip := net.ParseIP(u.Hostname()); ip != nil && isDockerInternalIP(ip) {
		wahaIP := net.ParseIP(waha.Hostname())
		if wahaIP != nil && !wahaIP.IsLoopback() && u.Port() == waha.Port() {
			return nil
		}
	}

	if isDockerInternalHost(waha.Hostname()) && u.Port() == waha.Port() {
		if net.ParseIP(u.Hostname()) != nil {
			return nil
		}
	}

	if signal != nil && isDockerInternalHost(signal.Hostname()) && u.Port() == signal.Port() {
		if net.ParseIP(u.Hostname()) != nil {
			return nil
		}
	}

	return fmt.Errorf("download host not allowed: %s", u.Hostname())
}

func endpointMatches(candidate, allowed *url.URL) bool {
	return strings.EqualFold(candidate.Hostname(), allowed.Hostname()) && candidate.Port() == allowed.Port()
}

func rejectDisallowedResolvedIP(hostname string) error {
	if ip := parseLegacyIPv4(hostname); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("hostname %q resolves to disallowed IP %s", hostname, ip.String())
	}

	if net.ParseIP(hostname) != nil {
		return nil
	}

	addrs, err := net.LookupHost(hostname) // #nosec G704 -- Defensive SSRF check; every resolved IP is rejected if blocked before any fetch.
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && isBlockedIP(ip) {
			return fmt.Errorf("hostname %q resolves to disallowed IP %s", hostname, ip.String())
		}
	}
	return nil
}

func parseLegacyIPv4(hostname string) net.IP {
	if hostname == "" || strings.Contains(hostname, ".") || strings.Contains(hostname, ":") {
		return nil
	}
	value, err := strconv.ParseUint(hostname, 0, 32)
	if err != nil {
		return nil
	}
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], uint32(value))
	return net.IPv4(raw[0], raw[1], raw[2], raw[3])
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

func isDockerInternalHost(hostname string) bool {
	if net.ParseIP(hostname) != nil {
		return false
	}
	if strings.Contains(hostname, ".") {
		return false
	}
	if hostname == "localhost" {
		return false
	}
	return len(hostname) > 0
}
