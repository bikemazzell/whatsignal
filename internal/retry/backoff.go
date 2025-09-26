package retry

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"time"
)

// BackoffConfig contains configuration for exponential backoff
type BackoffConfig struct {
	InitialDelay time.Duration `json:"initial_delay" validate:"min=10ms,max=10s"`
	MaxDelay     time.Duration `json:"max_delay" validate:"min=100ms,max=5m"`
	Multiplier   float64       `json:"multiplier" validate:"min=1.0,max=10.0"`
	MaxAttempts  int           `json:"max_attempts" validate:"min=1,max=20"`
	Jitter       bool          `json:"jitter"`
}

// DefaultBackoffConfig returns a sensible default configuration
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       true,
	}
}

// Backoff implements exponential backoff with optional jitter
type Backoff struct {
	config BackoffConfig
}

// NewBackoff creates a new exponential backoff instance
func NewBackoff(config BackoffConfig) *Backoff {
	return &Backoff{
		config: config,
	}
}

// Retry executes the operation with exponential backoff retry logic
func (b *Backoff) Retry(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= b.config.MaxAttempts; attempt++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt == b.config.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := b.calculateDelay(attempt)

		// Wait for the calculated delay
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateDelay computes the delay for the given attempt with exponential backoff and optional jitter
func (b *Backoff) calculateDelay(attempt int) time.Duration {
	// Calculate exponential delay
	delay := float64(b.config.InitialDelay)
	for i := 1; i < attempt; i++ {
		delay *= b.config.Multiplier
	}

	// Cap at maximum delay
	if delay > float64(b.config.MaxDelay) {
		delay = float64(b.config.MaxDelay)
	}

	// Add jitter if enabled (±25% randomness)
	if b.config.Jitter {
		jitter := delay * 0.25
		// Use cryptographically secure random number generator
		randomValue := secureFloat64()
		delay += (randomValue - 0.5) * 2 * jitter

		// Ensure delay doesn't go negative or exceed max
		if delay < 0 {
			delay = float64(b.config.InitialDelay)
		}
		if delay > float64(b.config.MaxDelay) {
			delay = float64(b.config.MaxDelay)
		}
	}

	return time.Duration(delay)
}

// RetryWithPredicate executes the operation with exponential backoff, using a predicate to determine if errors are retryable
func (b *Backoff) RetryWithPredicate(ctx context.Context, operation func() error, isRetryable func(error) bool) error {
	var lastErr error

	for attempt := 1; attempt <= b.config.MaxAttempts; attempt++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err) {
			return err // Non-retryable error, fail immediately
		}

		// Don't wait after the last attempt
		if attempt == b.config.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := b.calculateDelay(attempt)

		// Wait for the calculated delay
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// GetNextDelay returns the delay that would be used for the given attempt (for testing/monitoring)
func (b *Backoff) GetNextDelay(attempt int) time.Duration {
	return b.calculateDelay(attempt)
}

// secureFloat64 generates a cryptographically secure float64 between 0 and 1
func secureFloat64() float64 {
	// Generate a random 64-bit integer
	max := big.NewInt(0).SetUint64(math.MaxUint64)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback to time-based value if crypto/rand fails
		// This is extremely unlikely but provides a safety net
		return float64(time.Now().UnixNano()%1000000) / 1000000.0
	}

	// Convert to float64 in range [0, 1)
	return float64(n.Uint64()) / float64(math.MaxUint64)
}
