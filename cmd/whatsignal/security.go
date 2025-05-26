package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The signature in the header is expected to be prefixed with "sha256=".
func verifySignature(r *http.Request, secretKey string, signatureHeaderName string) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	// Restore the body so it can be read again by subsequent handlers
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if secretKey == "" {
		// If no secret key is configured, skip verification. This might be desired for testing or specific setups.
		// For production, a secret key should always be configured.
		return body, nil
	}

	signatureHeader := r.Header.Get(signatureHeaderName)
	if signatureHeader == "" {
		return nil, fmt.Errorf("missing signature header: %s", signatureHeaderName)
	}

	// Signature is expected to be in the format "sha256=actualsignature"
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

	return body, nil
}
