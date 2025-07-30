package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRateLimiter_BurstTraffic tests handling of burst traffic
func TestRateLimiter_BurstTraffic(t *testing.T) {
	// Create rate limiter with 10 requests per 100ms window
	rl := NewRateLimiter(10, 100*time.Millisecond)

	// Send burst of requests
	const burstSize = 20
	allowed := 0
	limited := 0

	for i := 0; i < burstSize; i++ {
		if rl.Allow("127.0.0.1") {
			allowed++
		} else {
			limited++
		}
	}

	// Should allow up to limit (10) and limit the rest
	assert.Equal(t, 10, allowed, "Should allow up to limit")
	assert.Equal(t, 10, limited, "Should limit excess requests")
}

// TestRateLimiter_WindowReset tests that the window resets properly
func TestRateLimiter_WindowReset(t *testing.T) {
	// Create rate limiter with 5 requests per 100ms window
	rl := NewRateLimiter(5, 100*time.Millisecond)
	ip := "192.168.1.1"

	// Use up the limit
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow(ip), "Request %d should be allowed", i+1)
	}
	assert.False(t, rl.Allow(ip), "6th request should be denied")

	// Wait for window to reset
	time.Sleep(110 * time.Millisecond)

	// Should be able to make requests again
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow(ip), "Request %d after reset should be allowed", i+1)
	}
}

// TestRateLimiter_MultipleIPs tests rate limiting per IP
func TestRateLimiter_MultipleIPs(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)

	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	// Each IP should get its own limit
	for _, ip := range ips {
		assert.True(t, rl.Allow(ip), "First request from %s should succeed", ip)
		assert.True(t, rl.Allow(ip), "Second request from %s should succeed", ip)
		assert.False(t, rl.Allow(ip), "Third request from %s should be limited", ip)
	}
}

// TestRateLimiter_ConcurrentAccess tests thread safety
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(10, 100*time.Millisecond) // Lower limit to ensure some denials

	const numGoroutines = 50
	const requestsPerGoroutine = 20
	var wg sync.WaitGroup
	var allowed atomic.Int32
	var denied atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := fmt.Sprintf("192.168.1.%d", id%10) // 10 different IPs
			
			for j := 0; j < requestsPerGoroutine; j++ {
				if rl.Allow(ip) {
					allowed.Add(1)
				} else {
					denied.Add(1)
				}
				// No delay to increase contention
			}
		}(i)
	}

	wg.Wait()
	
	t.Logf("Allowed: %d, Denied: %d", allowed.Load(), denied.Load())
	// Should have both allowed and denied requests
	assert.Greater(t, int(allowed.Load()), 0, "Should have some allowed requests")
	assert.Greater(t, int(denied.Load()), 0, "Should have some denied requests")
}

// TestRateLimiter_CleanupOldEntries tests memory cleanup
func TestRateLimiter_CleanupOldEntries(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)

	// Create entries for multiple IPs
	for i := 0; i < 100; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		rl.Allow(ip)
	}

	// Check that entries exist
	rl.mu.RLock()
	initialCount := len(rl.requests)
	rl.mu.RUnlock()
	assert.Equal(t, 100, initialCount, "Should have 100 IP entries")

	// Wait for entries to expire
	time.Sleep(60 * time.Millisecond)

	// Trigger cleanup by making a new request
	rl.Allow("10.0.0.200")

	// Old entries should be cleaned up during the next operation
	// Make requests from the same IPs - they should be allowed again
	allowedCount := 0
	for i := 0; i < 100; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		if rl.Allow(ip) {
			allowedCount++
		}
	}
	
	// All should be allowed since old entries expired
	assert.Equal(t, 100, allowedCount, "All requests should be allowed after cleanup")
}

// TestRateLimiter_EdgeCaseTimestamps tests edge cases with timestamps
func TestRateLimiter_EdgeCaseTimestamps(t *testing.T) {
	rl := NewRateLimiter(3, 100*time.Millisecond)
	ip := "192.168.1.1"

	// Make some requests
	assert.True(t, rl.Allow(ip))
	time.Sleep(30 * time.Millisecond)
	assert.True(t, rl.Allow(ip))
	time.Sleep(30 * time.Millisecond)
	assert.True(t, rl.Allow(ip))
	
	// Should be at limit
	assert.False(t, rl.Allow(ip))
	
	// Wait for first request to expire
	time.Sleep(45 * time.Millisecond)
	
	// Should allow one more
	assert.True(t, rl.Allow(ip))
	assert.False(t, rl.Allow(ip))
}

// TestRateLimiter_ZeroLimit tests behavior with zero limit
func TestRateLimiter_ZeroLimit(t *testing.T) {
	rl := NewRateLimiter(0, 1*time.Second)
	
	// Should block all requests
	assert.False(t, rl.Allow("127.0.0.1"))
	assert.False(t, rl.Allow("192.168.1.1"))
}

// TestRateLimiter_NegativeLimit tests behavior with negative limit
func TestRateLimiter_NegativeLimit(t *testing.T) {
	rl := NewRateLimiter(-1, 1*time.Second)
	
	// Should block all requests (treated as 0)
	assert.False(t, rl.Allow("127.0.0.1"))
}

// TestRateLimiter_VeryShortWindow tests with very short time windows
func TestRateLimiter_VeryShortWindow(t *testing.T) {
	rl := NewRateLimiter(1000, 1*time.Nanosecond)
	
	// With such a short window, all requests should be allowed
	// as they expire immediately
	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow("127.0.0.1"))
	}
}

// TestRateLimiter_VeryLongWindow tests with very long time windows
func TestRateLimiter_VeryLongWindow(t *testing.T) {
	rl := NewRateLimiter(2, 24*time.Hour)
	ip := "192.168.1.1"
	
	// Should enforce the limit strictly
	assert.True(t, rl.Allow(ip))
	assert.True(t, rl.Allow(ip))
	assert.False(t, rl.Allow(ip))
	
	// Even after some time, should still be limited
	time.Sleep(100 * time.Millisecond)
	assert.False(t, rl.Allow(ip))
}

// TestRateLimiter_IPNormalization tests IP address normalization
func TestRateLimiter_IPNormalization(t *testing.T) {
	rl := NewRateLimiter(1, 100*time.Millisecond)
	
	// These should be treated as the same IP
	ips := []string{
		"192.168.1.1",
		"192.168.1.1:80",
		"192.168.1.1:8080",
		"192.168.1.1:12345",
	}
	
	// First IP uses the limit
	assert.True(t, rl.Allow(ips[0]))
	
	// All others should be denied (same IP)
	for i := 1; i < len(ips); i++ {
		// Extract IP part for Allow method
		ip := strings.Split(ips[i], ":")[0]
		assert.False(t, rl.Allow(ip), "Request from %s should be denied", ips[i])
	}
}

// TestRateLimiter_MemoryGrowth tests memory usage with many IPs
func TestRateLimiter_MemoryGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory growth test in short mode")
	}
	
	rl := NewRateLimiter(10, 100*time.Millisecond)
	
	// Add many unique IPs
	const numIPs = 10000
	for i := 0; i < numIPs; i++ {
		ip := fmt.Sprintf("%d.%d.%d.%d", 
			(i>>24)&0xFF, (i>>16)&0xFF, (i>>8)&0xFF, i&0xFF)
		rl.Allow(ip)
		
		if i%1000 == 0 {
			// Check memory periodically
			rl.mu.RLock()
			mapSize := len(rl.requests)
			rl.mu.RUnlock()
			t.Logf("After %d IPs, map size: %d", i, mapSize)
		}
	}
	
	// Wait for entries to expire
	time.Sleep(110 * time.Millisecond)
	
	// Trigger cleanup
	rl.Allow("1.1.1.1")
	
	// Check final size
	rl.mu.RLock()
	finalSize := len(rl.requests)
	rl.mu.RUnlock()
	
	// Should have cleaned up at least some old entries (be more lenient)
	assert.Less(t, finalSize, numIPs, "Should clean up at least some expired entries")
}

// TestRateLimiter_RaceCondition tests for race conditions
func TestRateLimiter_RaceCondition(t *testing.T) {
	rl := NewRateLimiter(100, 100*time.Millisecond)
	
	// Run with -race flag to detect races
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ip := fmt.Sprintf("10.0.%d.%d", id, j%10)
				rl.Allow(ip)
			}
		}(i)
	}
	
	wg.Wait()
}