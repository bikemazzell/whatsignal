package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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

// RateLimiter implements a simple rate limiter for webhook endpoints
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old requests
	if requests, exists := rl.requests[ip]; exists {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		rl.requests[ip] = validRequests
	}

	// Check if limit exceeded
	if len(rl.requests[ip]) >= rl.limit {
		return false
	}

	// Add current request
	rl.requests[ip] = append(rl.requests[ip], now)
	return true
}

// GetClientIP extracts the real client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
