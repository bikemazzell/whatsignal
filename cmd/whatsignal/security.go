package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// The signature in the header is expected to be prefixed with "sha256=" or just the hex value for WAHA.
func verifySignature(r *http.Request, secretKey string, signatureHeaderName string) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	// Restore the body so it can be read again by subsequent handlers
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if secretKey == "" {
		// Check if we're in production mode
		if os.Getenv("WHATSIGNAL_ENV") == "production" {
			return nil, fmt.Errorf("webhook secret is required in production mode")
		}
		// Allow empty secrets only in development/testing
		return body, nil
	}

	signatureHeader := r.Header.Get(signatureHeaderName)
	if signatureHeader == "" {
		return nil, fmt.Errorf("missing signature header: %s", signatureHeaderName)
	}

	// Check if it's WAHA webhook (uses SHA512)
	if signatureHeaderName == "X-Webhook-Hmac" {
		// WAHA uses SHA512 and provides the signature directly (no prefix)
		expectedSignatureHex := signatureHeader
		
		// Get timestamp from header for WAHA webhooks
		timestamp := r.Header.Get("X-Webhook-Timestamp")
		if timestamp == "" {
			return nil, fmt.Errorf("missing X-Webhook-Timestamp header for WAHA webhook")
		}
		
		// WAHA uses SHA512 HMAC over just the body
		mac := hmac.New(sha512.New, []byte(secretKey))
		mac.Write(body)
		computedMAC := mac.Sum(nil)
		computedSignatureHex := hex.EncodeToString(computedMAC)

		if !hmac.Equal([]byte(computedSignatureHex), []byte(expectedSignatureHex)) {
			return nil, fmt.Errorf("signature mismatch")
		}
	} else {
		// Original logic for other webhooks (SHA256 with prefix)
		parts := strings.SplitN(signatureHeader, "=", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "sha256" {
			return nil, fmt.Errorf("invalid signature format in header %s", signatureHeaderName)
		}
		expectedSignatureHex := parts[1]

		mac := hmac.New(sha256.New, []byte(secretKey))
		mac.Write(body)
		computedMAC := mac.Sum(nil)
		computedSignatureHex := hex.EncodeToString(computedMAC)

		if !hmac.Equal([]byte(computedSignatureHex), []byte(expectedSignatureHex)) {
			return nil, fmt.Errorf("signature mismatch")
		}
	}

	return body, nil
}
