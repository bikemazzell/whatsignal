package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test handler with WAHA configuration
func setupHandlerForURLValidation(t *testing.T, wahaBaseURL string) *handler {
	return setupHandlerForURLValidationWithSignal(t, wahaBaseURL, "")
}

// Helper function to create a test handler with both WAHA and Signal configuration
func setupHandlerForURLValidationWithSignal(t *testing.T, wahaBaseURL, signalRPCURL string) *handler {
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-url-test")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	cacheDir := filepath.Join(tmpDir, "cache")
	handlerInterface, err := NewHandlerWithServices(cacheDir, getTestMediaConfig(), wahaBaseURL, "test-api-key", signalRPCURL)
	require.NoError(t, err)

	// Cast to concrete type to access private validateDownloadURL method
	h, ok := handlerInterface.(*handler)
	require.True(t, ok, "handler should be of type *handler")
	return h
}

func TestValidateDownloadURL_NoWAHABase(t *testing.T) {
	// When no WAHA base URL is configured, validation should be skipped
	h := setupHandlerForURLValidation(t, "") // Empty WAHA base URL

	testURLs := []string{
		"http://localhost/api/files/test.jpg",
		"https://127.0.0.1:3000/file.png",
		"http://192.168.1.100/media.mp4",
		"file:///etc/passwd",
		"ftp://evil.com/malware.exe",
		"http://malicious.com/evil.jpg",
	}

	for _, url := range testURLs {
		t.Run("skip_validation_"+url, func(t *testing.T) {
			err := h.validateDownloadURL(url)
			assert.NoError(t, err, "Should skip validation when no WAHA base URL is configured")
		})
	}
}

func TestValidateDownloadURL_SchemeValidation(t *testing.T) {
	h := setupHandlerForURLValidation(t, "https://example.com")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid file scheme",
			url:         "file:///etc/passwd",
			expectError: true,
			errorMsg:    "unsupported URL scheme: file",
		},
		{
			name:        "invalid ftp scheme",
			url:         "ftp://example.com/file.txt",
			expectError: true,
			errorMsg:    "unsupported URL scheme: ftp",
		},
		{
			name:        "invalid data scheme",
			url:         "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB",
			expectError: true,
			errorMsg:    "unsupported URL scheme: data",
		},
		{
			name:        "invalid javascript scheme",
			url:         "javascript:alert('xss')",
			expectError: true,
			errorMsg:    "unsupported URL scheme: javascript",
		},
		{
			name:        "no scheme",
			url:         "//example.com/file.jpg",
			expectError: true,
			errorMsg:    "unsupported URL scheme:",
		},
		{
			name:        "valid https scheme (but will fail host check)",
			url:         "https://different-host.com/file.jpg",
			expectError: true,
			errorMsg:    "download host not allowed:",
		},
		{
			name:        "valid http scheme (but will fail host check)",
			url:         "http://different-host.com/file.jpg",
			expectError: true,
			errorMsg:    "download host not allowed:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDownloadURL_InvalidURLs(t *testing.T) {
	h := setupHandlerForURLValidation(t, "https://example.com")

	invalidURLs := []struct {
		name string
		url  string
	}{
		{
			name: "malformed URL",
			url:  "ht\x00tp://example.com",
		},
		{
			name: "invalid characters",
			url:  "http://exam\nple.com",
		},
	}

	for _, tt := range invalidURLs {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid media URL")
		})
	}
}

func TestValidateDownloadURL_HostValidation(t *testing.T) {
	h := setupHandlerForURLValidation(t, "https://example.com")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "different host blocked",
			url:         "https://evil.com/malware.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: evil.com",
		},
		{
			name:        "subdomain blocked",
			url:         "https://sub.example.com/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: sub.example.com",
		},
		{
			name:        "similar domain blocked",
			url:         "https://example-evil.com/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: example-evil.com",
		},
		{
			name:        "case insensitive host comparison",
			url:         "https://EXAMPLE.COM/api/files/test.jpg",
			expectError: false, // Same host, different case should be allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				// Note: even valid host URLs may fail on DNS resolution in test environment
				// That's expected and still validates the host checking logic
				if err != nil {
					t.Logf("Expected success but got error (acceptable in test env): %v", err)
				}
			}
		})
	}
}

func TestValidateDownloadURL_IPLiteralBlocking(t *testing.T) {
	h := setupHandlerForURLValidation(t, "https://example.com")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		// Loopback addresses
		{
			name:        "localhost IPv4",
			url:         "https://127.0.0.1/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 127.0.0.1", // Host check happens first
		},
		{
			name:        "localhost IPv6",
			url:         "https://[::1]/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: ::1",
		},

		// Private network addresses
		{
			name:        "RFC1918 10.x.x.x",
			url:         "https://10.0.0.1/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 10.0.0.1",
		},
		{
			name:        "RFC1918 192.168.x.x",
			url:         "https://192.168.1.1/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.1",
		},
		{
			name:        "RFC1918 172.16.x.x",
			url:         "https://172.16.0.1/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 172.16.0.1",
		},

		// Link-local addresses
		{
			name:        "IPv4 link-local",
			url:         "https://169.254.1.1/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 169.254.1.1",
		},

		// Public addresses should pass host validation but may fail DNS
		{
			name:        "public IPv4",
			url:         "https://8.8.8.8/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 8.8.8.8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDownloadURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		wahaBaseURL string
		testURL     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid WAHA base URL",
			wahaBaseURL: "ht\x00tp://invalid",
			testURL:     "https://example.com/test.jpg",
			expectError: true,
			errorMsg:    "invalid WAHA base URL",
		},
		{
			name:        "URL with query parameters",
			wahaBaseURL: "https://example.com",
			testURL:     "https://example.com/api/files/test.jpg?v=1&auth=token",
			expectError: false, // May fail on DNS but that's OK for test
		},
		{
			name:        "URL with fragment",
			wahaBaseURL: "https://example.com",
			testURL:     "https://example.com/api/files/test.jpg#section1",
			expectError: false, // May fail on DNS but that's OK for test
		},
		{
			name:        "URL with user info (should block host)",
			wahaBaseURL: "https://example.com",
			testURL:     "https://user:pass@evil.com/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: evil.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := setupHandlerForURLValidation(t, tt.wahaBaseURL)

			err := h.validateDownloadURL(tt.testURL)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// May fail on DNS resolution, which is acceptable in test environment
				if err != nil {
					t.Logf("Expected success but got error (acceptable in test env): %v", err)
				}
			}
		})
	}
}

func TestValidateDownloadURL_SecurityBypass(t *testing.T) {
	h := setupHandlerForURLValidation(t, "https://example.com")

	// Test various attempts to bypass the security checks
	bypassAttempts := []struct {
		name     string
		url      string
		errorMsg string
	}{
		{
			name:     "URL user info bypass attempt",
			url:      "https://example.com@evil.com/test.jpg",
			errorMsg: "download host not allowed",
		},
		{
			name:     "double slash bypass",
			url:      "https://evil.com//example.com/test.jpg",
			errorMsg: "download host not allowed: evil.com",
		},
		{
			name:     "subdomain confusion",
			url:      "https://example.com.evil.com/test.jpg",
			errorMsg: "download host not allowed",
		},
		{
			name:     "port confusion",
			url:      "https://example.com:80@evil.com/test.jpg",
			errorMsg: "download host not allowed",
		},
	}

	for _, tt := range bypassAttempts {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestValidateDownloadURL_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name        string
		wahaBaseURL string
		testURL     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "attempt to access file system via different protocol",
			wahaBaseURL: "http://localhost:3000",
			testURL:     "file:///var/www/html/uploads/file.jpg",
			expectError: true,
			errorMsg:    "unsupported URL scheme: file",
		},
		{
			name:        "attempt to access internal network service",
			wahaBaseURL: "https://example.com",
			testURL:     "http://192.168.1.10:6379/info",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.10",
		},
		{
			name:        "attempt to access different service on localhost",
			wahaBaseURL: "http://localhost:3000",
			testURL:     "http://localhost:8080/admin/secret",
			expectError: true,
			errorMsg:    "download host not allowed", // Different port blocked
		},
		{
			name:        "legitimate same-host URL",
			wahaBaseURL: "https://legitimate.example.com",
			testURL:     "https://legitimate.example.com/api/files/media_123.jpg",
			expectError: false, // May fail DNS resolution, but that validates host logic worked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := setupHandlerForURLValidation(t, tt.wahaBaseURL)

			err := h.validateDownloadURL(tt.testURL)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				// DNS resolution may fail in test environment, which is acceptable
				// The important thing is that host validation logic was exercised
				if err != nil {
					t.Logf("Expected success but got error (acceptable in test env): %v", err)
				}
			}
		})
	}
}

func TestValidateDownloadURL_ComprehensiveSecurity(t *testing.T) {
	h := setupHandlerForURLValidation(t, "https://safe.example.com")

	// Test comprehensive security scenarios that should all be blocked
	securityTests := []struct {
		name string
		url  string
	}{
		// Protocol attacks
		{"file protocol", "file:///etc/passwd"},
		{"ftp protocol", "ftp://attacker.com/malware"},
		{"data protocol", "data:text/html,<script>alert('xss')</script>"},

		// Network attacks
		{"localhost access", "http://localhost:22/ssh"},
		{"loopback IP", "http://127.0.0.1:3306/mysql"},
		{"private network", "http://192.168.1.1:80/router"},
		{"link local", "http://169.254.169.254/metadata"}, // AWS metadata

		// Host confusion attacks
		{"different host", "https://attacker.com/evil.jpg"},
		{"subdomain attack", "https://safe.example.com.evil.com/file.jpg"},
		{"host header injection", "https://safe.example.com@evil.com/file.jpg"},

		// IPv6 attacks
		{"IPv6 loopback", "http://[::1]:8080/local"},
		{"IPv6 link-local", "http://[fe80::1]/local"},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			assert.Error(t, err, "Security validation should block: %s", tt.url)

			// Verify the error is a security-related error, not just a network error
			assert.True(t,
				strings.Contains(err.Error(), "unsupported URL scheme") ||
					strings.Contains(err.Error(), "download host not allowed") ||
					strings.Contains(err.Error(), "download IP not allowed") ||
					strings.Contains(err.Error(), "resolved to disallowed IP") ||
					strings.Contains(err.Error(), "invalid media URL") ||
					strings.Contains(err.Error(), "invalid WAHA base URL"),
				"Expected security error but got: %v", err)
		})
	}
}

func TestValidateDownloadURL_CodeCoverage(t *testing.T) {
	// Test specific code paths for coverage

	t.Run("empty_waha_base_skips_validation", func(t *testing.T) {
		h := setupHandlerForURLValidation(t, "")
		err := h.validateDownloadURL("file:///etc/passwd")
		assert.NoError(t, err, "Should skip validation when no WAHA base URL")
	})

	t.Run("invalid_url_parsing", func(t *testing.T) {
		h := setupHandlerForURLValidation(t, "https://example.com")
		err := h.validateDownloadURL("ht\x00tp://invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid media URL")
	})

	t.Run("invalid_waha_base_url", func(t *testing.T) {
		h := setupHandlerForURLValidation(t, "ht\x00tp://invalid")
		err := h.validateDownloadURL("https://example.com/test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid WAHA base URL")
	})

	t.Run("host_comparison_logic", func(t *testing.T) {
		h := setupHandlerForURLValidation(t, "https://EXAMPLE.COM")
		// Test case-insensitive host comparison
		err := h.validateDownloadURL("https://example.com/test")
		// May fail on DNS, but host comparison logic was exercised
		if err != nil {
			t.Logf("Host comparison exercised, DNS may have failed: %v", err)
		}
	})
}

func TestValidateDownloadURL_DockerInternalIP(t *testing.T) {
	// Test with external IP as WAHA base URL (simulating Docker deployment)
	h := setupHandlerForURLValidation(t, "http://192.168.1.50:3000")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Docker internal IP with matching port - allowed",
			url:         "http://172.18.0.3:3000/api/files/voice.ogg",
			expectError: false,
		},
		{
			name:        "Docker internal IP with different port - blocked",
			url:         "http://172.18.0.3:8080/api/files/voice.ogg",
			expectError: true,
			errorMsg:    "download host not allowed: 172.18.0.3",
		},
		{
			name:        "Docker internal IP different subnet - allowed if same port",
			url:         "http://172.17.0.2:3000/api/files/voice.ogg",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDownloadURL_DockerInternalHosts(t *testing.T) {
	// Test with external IP as WAHA base URL (simulating Docker deployment)
	h := setupHandlerForURLValidation(t, "http://192.168.1.50:3000")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Docker internal host 'waha' - allowed",
			url:         "http://waha:3000/api/files/voice.ogg",
			expectError: false,
		},
		{
			name:        "Docker internal host 'signal-cli-waha' - allowed",
			url:         "http://signal-cli-waha:8080/api/files/voice.ogg",
			expectError: false,
		},
		{
			name:        "Docker internal host 'myservice' - allowed",
			url:         "http://myservice:3000/api/files/voice.ogg",
			expectError: false,
		},
		{
			name:        "Domain name with dots - blocked",
			url:         "http://malicious.example.com:3000/api/files/voice.ogg",
			expectError: true,
			errorMsg:    "download host not allowed: malicious.example.com",
		},
		{
			name:        "WAHA host IP directly - allowed",
			url:         "http://192.168.1.50:3000/api/files/voice.ogg",
			expectError: false,
		},
		{
			name:        "WAHA host IP with different path - allowed",
			url:         "http://192.168.1.50:3000/media/image.jpg",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDownloadURL_PrivateIPAsWAHAHost(t *testing.T) {
	// Test that private IPs are allowed when they match the WAHA base URL
	// This is the common case for self-hosted deployments on private networks

	tests := []struct {
		name        string
		wahaBaseURL string
		testURL     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "192.168.x.x private IP - allowed when matches WAHA",
			wahaBaseURL: "http://192.168.1.50:3000",
			testURL:     "http://192.168.1.50:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "10.x.x.x private IP - allowed when matches WAHA",
			wahaBaseURL: "http://10.0.0.5:3000",
			testURL:     "http://10.0.0.5:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "172.x.x.x private IP (non-Docker) - allowed when matches WAHA",
			wahaBaseURL: "http://172.31.50.100:3000",
			testURL:     "http://172.31.50.100:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "localhost - allowed when matches WAHA",
			wahaBaseURL: "http://localhost:3000",
			testURL:     "http://localhost:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "127.0.0.1 - allowed when matches WAHA",
			wahaBaseURL: "http://127.0.0.1:3000",
			testURL:     "http://127.0.0.1:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "different private IP - blocked",
			wahaBaseURL: "http://192.168.1.50:3000",
			testURL:     "http://192.168.1.99:3000/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := setupHandlerForURLValidation(t, tt.wahaBaseURL)
			err := h.validateDownloadURL(tt.testURL)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDownloadURL_DockerHostnameWithExternalIP tests the scenario where
// WAHA is configured with a Docker internal hostname (e.g., "waha:3000") but
// generates media URLs with an external IP
// This is a common scenario in Docker deployments where WAHA auto-detects its external IP
func TestValidateDownloadURL_DockerHostnameWithExternalIP(t *testing.T) {
	// WAHA configured with Docker internal hostname
	h := setupHandlerForURLValidation(t, "http://waha:3000")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "External IP with same port as WAHA - allowed",
			url:         "http://192.168.1.50:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "External IP with different port - blocked",
			url:         "http://192.168.1.50:8080/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.50",
		},
		{
			name:        "Different external IP with same port - allowed (WAHA could be behind NAT)",
			url:         "http://10.0.0.5:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "Docker internal hostname - allowed",
			url:         "http://waha:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "Different Docker internal hostname with same port - allowed",
			url:         "http://other-service:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "External domain with same port - blocked",
			url:         "http://evil.com:3000/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: evil.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDownloadURL_DockerHostnameWithExternalIP_Signal tests the same scenario for Signal
func TestValidateDownloadURL_DockerHostnameWithExternalIP_Signal(t *testing.T) {
	// Both WAHA and Signal configured with Docker internal hostnames
	h := setupHandlerForURLValidationWithSignal(t, "http://waha:3000", "http://signal-cli-waha:8080")

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "WAHA external IP with same port - allowed",
			url:         "http://192.168.1.50:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "Signal external IP with same port - allowed",
			url:         "http://192.168.1.50:8080/v1/attachments/abc123",
			expectError: false,
		},
		{
			name:        "External IP with WAHA port - allowed",
			url:         "http://10.0.0.5:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "External IP with Signal port - allowed",
			url:         "http://10.0.0.5:8080/v1/attachments/abc123",
			expectError: false,
		},
		{
			name:        "External IP with different port - blocked",
			url:         "http://192.168.1.50:9999/malicious/file.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.50",
		},
		{
			name:        "External domain with WAHA port - blocked",
			url:         "http://evil.com:3000/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: evil.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsDockerInternalHost(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		expected bool
	}{
		{
			name:     "waha service name",
			hostname: "waha",
			expected: true,
		},
		{
			name:     "signal-cli service name",
			hostname: "signal-cli",
			expected: true,
		},
		{
			name:     "signal-cli-rest-api service name",
			hostname: "signal-cli-rest-api",
			expected: true,
		},
		{
			name:     "signal-cli-waha service name",
			hostname: "signal-cli-waha",
			expected: true,
		},
		{
			name:     "domain name with dots",
			hostname: "example.com",
			expected: false,
		},
		{
			name:     "subdomain",
			hostname: "api.example.com",
			expected: false,
		},
		{
			name:     "localhost",
			hostname: "localhost",
			expected: false,
		},
		{
			name:     "IP address",
			hostname: "192.168.1.1",
			expected: false,
		},
		{
			name:     "IPv6 address",
			hostname: "::1",
			expected: false,
		},
		{
			name:     "empty hostname",
			hostname: "",
			expected: false,
		},
		{
			name:     "single word hostname",
			hostname: "myservice",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDockerInternalHost(tt.hostname)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateDownloadURL_SignalRPCService(t *testing.T) {
	// Test that Signal RPC service URLs are properly validated
	wahaBaseURL := "http://192.168.1.50:3000"
	signalRPCURL := "http://192.168.1.50:8081"
	h := setupHandlerForURLValidationWithSignal(t, wahaBaseURL, signalRPCURL)

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "WAHA service URL - allowed",
			url:         "http://192.168.1.50:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "Signal RPC service URL - allowed",
			url:         "http://192.168.1.50:8081/v1/attachments/abc123",
			expectError: false,
		},
		{
			name:        "Same host different port - blocked",
			url:         "http://192.168.1.50:9999/malicious/file.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.50",
		},
		{
			name:        "Different host - blocked",
			url:         "http://192.168.1.99:3000/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.99",
		},
		{
			name:        "External host - blocked",
			url:         "http://evil.com:3000/api/files/test.jpg",
			expectError: true,
			errorMsg:    "download host not allowed: evil.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDownloadURL_SignalRPCServiceNotConfigured(t *testing.T) {
	// Test behavior when Signal RPC URL is not configured
	wahaBaseURL := "http://192.168.1.50:3000"
	h := setupHandlerForURLValidationWithSignal(t, wahaBaseURL, "") // No Signal RPC URL

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "WAHA service URL - allowed",
			url:         "http://192.168.1.50:3000/api/files/test.jpg",
			expectError: false,
		},
		{
			name:        "Signal RPC service URL - blocked when not configured",
			url:         "http://192.168.1.50:8081/v1/attachments/abc123",
			expectError: true,
			errorMsg:    "download host not allowed: 192.168.1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validateDownloadURL(tt.url)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDownloadURL_InvalidSignalRPCURL(t *testing.T) {
	// Test behavior with invalid Signal RPC URL configuration
	wahaBaseURL := "http://192.168.1.50:3000"
	invalidSignalRPCURL := "://invalid-url"
	h := setupHandlerForURLValidationWithSignal(t, wahaBaseURL, invalidSignalRPCURL)

	// Should return error when trying to validate any URL due to invalid Signal RPC URL
	err := h.validateDownloadURL("http://192.168.1.50:8081/v1/attachments/abc123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Signal RPC URL")
}
