package media

import (
	"fmt"
	"net"
	"net/url"
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

	if ip := net.ParseIP(u.Hostname()); ip != nil && isDockerInternalIP(ip) {
		wahaIP := net.ParseIP(waha.Hostname())
		if wahaIP != nil && !wahaIP.IsLoopback() && u.Port() == waha.Port() {
			return nil
		}
	}

	if isDockerInternalHost(u.Hostname()) {
		return nil
	}

	if strings.EqualFold(u.Hostname(), waha.Hostname()) && u.Port() == waha.Port() {
		return nil
	}

	if signal != nil && strings.EqualFold(u.Hostname(), signal.Hostname()) && u.Port() == signal.Port() {
		return nil
	}

	if isDockerInternalHost(waha.Hostname()) && u.Port() == waha.Port() {
		if net.ParseIP(u.Hostname()) != nil || isDockerInternalHost(u.Hostname()) {
			return nil
		}
	}

	if signal != nil && isDockerInternalHost(signal.Hostname()) && u.Port() == signal.Port() {
		if net.ParseIP(u.Hostname()) != nil || isDockerInternalHost(u.Hostname()) {
			return nil
		}
	}

	return fmt.Errorf("download host not allowed: %s", u.Hostname())
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
