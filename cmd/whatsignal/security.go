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

func verifySignature(r *http.Request, secretKey string, signatureHeaderName string) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	if secretKey == "" {
		if os.Getenv("WHATSIGNAL_ENV") == "production" {
			return nil, fmt.Errorf("webhook secret is required in production mode")
		}
		return body, nil
	}

	signatureHeader := r.Header.Get(signatureHeaderName)
	if signatureHeader == "" {
		return nil, fmt.Errorf("missing signature header: %s", signatureHeaderName)
	}

	if signatureHeaderName == "X-Webhook-Hmac" {
		expectedSignatureHex := signatureHeader
		
		timestamp := r.Header.Get("X-Webhook-Timestamp")
		if timestamp == "" {
			return nil, fmt.Errorf("missing X-Webhook-Timestamp header for WAHA webhook")
		}
		
		mac := hmac.New(sha512.New, []byte(secretKey))
		mac.Write(body)
		computedMAC := mac.Sum(nil)
		computedSignatureHex := hex.EncodeToString(computedMAC)

		if !hmac.Equal([]byte(computedSignatureHex), []byte(expectedSignatureHex)) {
			return nil, fmt.Errorf("signature mismatch")
		}
	} else {
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
