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
	"strconv"
	"strings"
	"sync"
	"time"
)

func verifySignatureWithSkew(r *http.Request, secretKey string, signatureHeaderName string, maxSkew time.Duration) ([]byte, error) {
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
		// WAHA: require timestamp and enforce skew
		timestampStr := r.Header.Get("X-Webhook-Timestamp")
		if timestampStr == "" {
			return nil, fmt.Errorf("missing X-Webhook-Timestamp header for WAHA webhook")
		}
		ts, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid X-Webhook-Timestamp: %w", err)
		}
		eventTime := time.Unix(ts, 0)
		if now := time.Now(); eventTime.Before(now.Add(-maxSkew)) || eventTime.After(now.Add(maxSkew)) {
			return nil, fmt.Errorf("timestamp out of acceptable range")
		}

		// Validate signature (body-only per current WAHA tests); if WAHA binds timestamp in the future, include it here
		expectedSignatureHex := signatureHeader
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
	requests     map[string][]time.Time
	mu           sync.RWMutex
	limit        int
	window       time.Duration
	lastCleanup  time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests:    make(map[string][]time.Time),
		limit:       limit,
		window:      window,
		lastCleanup: time.Now(),
	}
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Opportunistic global cleanup to prevent unbounded map growth
	if now.Sub(rl.lastCleanup) >= rl.window {
		for key, reqs := range rl.requests {
			var kept []time.Time
			for _, t := range reqs {
				if t.After(cutoff) {
					kept = append(kept, t)
				}
			}
			if len(kept) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = kept
			}
		}
		rl.lastCleanup = now
	}

	// Clean old requests for this IP and delete empty entries
	if requests, exists := rl.requests[ip]; exists {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		if len(validRequests) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = validRequests
		}
	}

	// Check if limit exceeded
	if rl.limit <= 0 {
		return false
	}
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
