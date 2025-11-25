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
	"strconv"
	"strings"
	"sync"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/httputil"
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
		// WAHA sends timestamp in milliseconds, convert to seconds
		eventTime := time.Unix(ts/1000, (ts%1000)*1e6)
		now := time.Now()
		if eventTime.Before(now.Add(-maxSkew)) || eventTime.After(now.Add(maxSkew)) {
			timeDiff := now.Sub(eventTime)
			return nil, fmt.Errorf("timestamp out of acceptable range (webhook time: %s, server time: %s, difference: %v, max allowed: %v)",
				eventTime.Format(time.RFC3339), now.Format(time.RFC3339), timeDiff, maxSkew)
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
	requests      map[string][]time.Time
	mu            sync.RWMutex
	limit         int
	window        time.Duration
	lastCleanup   time.Time
	cleanupTicker *time.Ticker
	cleanupStop   chan struct{}
	cleanupPeriod time.Duration
	stopOnce      sync.Once
	cleanupWg     sync.WaitGroup
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration, cleanupPeriod time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests:      make(map[string][]time.Time),
		limit:         limit,
		window:        window,
		lastCleanup:   time.Now(),
		cleanupPeriod: cleanupPeriod,
		cleanupStop:   make(chan struct{}),
	}

	// Start background cleanup goroutine
	rl.startBackgroundCleanup()

	return rl
}

// startBackgroundCleanup starts a background goroutine that periodically cleans up old entries
func (rl *RateLimiter) startBackgroundCleanup() {
	if rl.cleanupPeriod <= 0 {
		// Default to configured cleanup interval if not specified
		rl.cleanupPeriod = time.Duration(constants.RateLimiterCleanupMinutes) * time.Minute
	}

	rl.cleanupTicker = time.NewTicker(rl.cleanupPeriod)
	rl.cleanupWg.Add(1)

	go func() {
		defer rl.cleanupWg.Done()
		for {
			select {
			case <-rl.cleanupTicker.C:
				rl.cleanup()
			case <-rl.cleanupStop:
				return
			}
		}
	}()
}

// Stop stops the background cleanup goroutine
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		if rl.cleanupTicker != nil {
			rl.cleanupTicker.Stop()
		}
		close(rl.cleanupStop)
		rl.cleanupWg.Wait() // Wait for goroutine to complete
	})
}

// cleanup removes old entries from the request map
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean up all IPs
	for ip, requests := range rl.requests {
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

	rl.lastCleanup = now
}

// RateLimitInfo contains information about rate limit status
type RateLimitInfo struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetTime time.Time
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	info := rl.AllowWithInfo(ip)
	return info.Allowed
}

// AllowWithInfo checks if a request from the given IP is allowed and returns detailed info
func (rl *RateLimiter) AllowWithInfo(ip string) RateLimitInfo {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old requests for this IP
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

	currentCount := len(rl.requests[ip])

	// Calculate reset time (when oldest request will expire)
	resetTime := now.Add(rl.window)
	if currentCount > 0 {
		oldestRequest := rl.requests[ip][0]
		resetTime = oldestRequest.Add(rl.window)
	}

	// Check if limit exceeded
	if rl.limit <= 0 {
		return RateLimitInfo{
			Allowed:   false,
			Limit:     rl.limit,
			Remaining: 0,
			ResetTime: resetTime,
		}
	}

	if currentCount >= rl.limit {
		return RateLimitInfo{
			Allowed:   false,
			Limit:     rl.limit,
			Remaining: 0,
			ResetTime: resetTime,
		}
	}

	// Add current request
	rl.requests[ip] = append(rl.requests[ip], now)

	return RateLimitInfo{
		Allowed:   true,
		Limit:     rl.limit,
		Remaining: rl.limit - currentCount - 1,
		ResetTime: resetTime,
	}
}

// GetClientIP extracts the real client IP from the request
func GetClientIP(r *http.Request) string {
	return httputil.GetClientIP(r)
}
