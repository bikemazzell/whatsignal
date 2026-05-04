//go:build e2e

package integration_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// WAHA webhook signing contract (per https://waha.devlike.pro/docs/how-to/webhooks/#hmac-authentication
// and signal-noise-cli/signal-cli-rest-api equivalents):
//
//	Header:           X-Webhook-Hmac
//	Header value:     lowercase hex of HMAC-SHA512(rawBody, hookKey)
//	Header:           X-Webhook-Timestamp
//	Header value:     milliseconds since unix epoch
//	Body:             raw HTTP request body (no transformation)
//
// This helper is the canonical reference. It is deliberately decoupled from
// any helper colocated with the verifier under test. If WAHA changes its
// signing format, update this helper and a corresponding upstream-docs link.
// Do NOT change this helper to chase a verifier change.
func signWAHAContractPayload(secret string, body []byte) string {
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestWAHAWebhookHMACContract_E2E builds the whatsignal binary, runs it with a
// known webhook secret, then sends a webhook signed using the WAHA contract
// helper above. The verifier under test (cmd/whatsignal/security.go) must
// accept the signature.
//
// This catches divergence between WAHA's actual signing format and our
// verification logic. v1.2.47 shipped with the verifier expecting
// `HMAC(timestamp.body)` while WAHA signs raw body; the unit-level signing
// helper colocated with the verifier mirrored the bug, so unit tests passed
// against 100% rejection in production. This test would have failed.
//
// Build tag `e2e` so it does not run by default. Run with:
//
//	go test -tags=e2e ./integration_test/ -run TestWAHAWebhookHMACContract_E2E -v
func TestWAHAWebhookHMACContract_E2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "whatsignal")

	// Build the binary from current source.
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/whatsignal")
	buildCmd.Dir = repoRoot
	buildCmd.Env = os.Environ()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Minimal config. whatsignal will warn loudly about Signal/WAHA being
	// unreachable but will still serve webhooks.
	dataDir := filepath.Join(tmpDir, "data")
	mediaDir := filepath.Join(tmpDir, "media")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, "config.json")
	configJSON := fmt.Sprintf(`{
  "whatsapp": {"api_base_url": "http://127.0.0.1:1"},
  "signal": {
    "rpc_url": "http://127.0.0.1:1",
    "intermediaryPhoneNumber": "+15555550100",
    "pollingEnabled": false,
    "attachmentsDir": "%s"
  },
  "channels": [
    {"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+15555550101"}
  ],
  "database": {"path": "%s/whatsignal.db"},
  "media": {"cache_dir": "%s"}
}`, mediaDir, dataDir, mediaDir)
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	// Allocate a port for the binary.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	const webhookSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "-config", configPath)
	cmd.Dir = repoRoot // so the binary can find scripts/migrations relative paths
	cmd.Env = append(os.Environ(),
		"PORT="+strconv.Itoa(port),
		"WHATSAPP_API_KEY=test-api-key",
		"WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET="+webhookSecret,
		"WHATSIGNAL_ENCRYPTION_SECRET=integration-test-encryption-secret-not-for-production-use",
		"WHATSIGNAL_ENCRYPTION_SALT=integration-test-salt-32-bytes-fixed-padding-here",
		"WHATSIGNAL_ENCRYPTION_LOOKUP_SALT=integration-test-lookup-salt-32-bytes-fixed-padding",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	logBuf := &syncBuffer{}
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start whatsignal: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("whatsignal logs:\n%s", logBuf.String())
		}
	})

	// Wait for /health to come up.
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	if !waitForHTTP200(healthURL, 30*time.Second) {
		t.Fatalf("whatsignal did not become healthy in time. logs:\n%s", logBuf.String())
	}

	// Send a webhook signed with the canonical WAHA contract.
	body := []byte(`{"event":"session.status","payload":{"name":"default","status":"WORKING"},"session":"default"}`)
	signature := signWAHAContractPayload(webhookSecret, body)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/webhook/whatsapp", port),
		bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Hmac", signature)
	req.Header.Set("X-Webhook-Timestamp", timestamp)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post webhook: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// 400 on body decode is acceptable — it means signature passed.
	// 401 means the verifier rejected our WAHA-format signature: contract drift.
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("verifier rejected a payload signed with the WAHA contract format.\n"+
			"This means cmd/whatsignal/security.go has diverged from WAHA's signing spec.\n"+
			"response body: %s\nwhatsignal logs:\n%s",
			string(respBody), logBuf.String())
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status %d, body=%s, logs:\n%s",
			resp.StatusCode, string(respBody), logBuf.String())
	}

	// Negative test: a bad signature must still be rejected.
	req.Header.Set("X-Webhook-Hmac", "00"+signature[2:])
	req.Body = io.NopCloser(bytes.NewReader(body))
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post webhook (bad sig): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("verifier accepted a bad signature; status=%d", resp2.StatusCode)
	}
}

// findRepoRoot walks up from the test directory until it finds go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func waitForHTTP200(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
