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
	"sort"
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

		expectedSignatureHex := signatureHeader
		bodyMAC := hmac.New(sha512.New, []byte(secretKey))
		bodyMAC.Write(body)
		computedBodySignatureHex := hex.EncodeToString(bodyMAC.Sum(nil))
		if hmac.Equal([]byte(computedBodySignatureHex), []byte(expectedSignatureHex)) {
			return body, nil
		}

		// Temporary compatibility path for older deployments that signed
		// "timestamp.body" instead of the documented raw-body WAHA format.
		legacyMAC := hmac.New(sha512.New, []byte(secretKey))
		legacyMAC.Write([]byte(timestampStr))
		legacyMAC.Write([]byte("."))
		legacyMAC.Write(body)
		computedLegacySignatureHex := hex.EncodeToString(legacyMAC.Sum(nil))
		if !hmac.Equal([]byte(computedLegacySignatureHex), []byte(expectedSignatureHex)) {
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

func requireProductionAdminToken(w http.ResponseWriter, r *http.Request) bool {
	if os.Getenv("WHATSIGNAL_ENV") != "production" {
		return true
	}

	adminToken := os.Getenv("WHATSIGNAL_ADMIN_TOKEN")
	authHeader := r.Header.Get("Authorization")
	const bearerPrefix = "Bearer "
	if adminToken == "" || !strings.HasPrefix(authHeader, bearerPrefix) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="whatsignal diagnostics"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	presentedToken := strings.TrimPrefix(authHeader, bearerPrefix)
	if !hmac.Equal([]byte(presentedToken), []byte(adminToken)) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="whatsignal diagnostics"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	return true
}

// RateLimiter implements a simple rate limiter for webhook endpoints
type RateLimiter struct {
	requests      map[string][]time.Time
	mu            sync.RWMutex
	limit         int
	window        time.Duration
	maxTrackedIPs int
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
		maxTrackedIPs: constants.DefaultMaxTrackedIPs,
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

// cleanup removes old entries from the request map and evicts least-active IPs if the cap is exceeded
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.cleanupLocked(time.Now())
}

func (rl *RateLimiter) cleanupLocked(now time.Time) {
	cutoff := now.Add(-rl.window)

	// Clean up expired timestamps for all IPs
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

	// Evict least-active IPs if map exceeds the cap
	if rl.maxTrackedIPs > 0 && len(rl.requests) > rl.maxTrackedIPs {
		type ipCount struct {
			ip    string
			count int
		}
		entries := make([]ipCount, 0, len(rl.requests))
		for ip, reqs := range rl.requests {
			entries = append(entries, ipCount{ip, len(reqs)})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].count < entries[j].count
		})
		toEvict := len(rl.requests) - rl.maxTrackedIPs
		for i := 0; i < toEvict; i++ {
			delete(rl.requests, entries[i].ip)
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
	if rl.cleanupPeriod > 0 && now.Sub(rl.lastCleanup) >= rl.cleanupPeriod {
		rl.cleanupLocked(now)
	}
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
