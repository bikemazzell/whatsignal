package integration_test

import "os"

// The mock-backed integration suite drives the in-process database helpers,
// which derive their encryption key the same way production does. Secure mode
// (the default) requires WHATSIGNAL_ENCRYPTION_SALT to be set; these tests use
// the in-source default salt, so run them in development mode. The real
// secure-mode path is covered by the e2e contract test, which boots the binary
// with explicit salts.
func init() {
	_ = os.Setenv("WHATSIGNAL_ENV", "development")
}
