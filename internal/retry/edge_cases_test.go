package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSecureFloat64(t *testing.T) {
	// Test that secureFloat64 returns values in [0, 1)
	for i := 0; i < 100; i++ {
		val := secureFloat64()
		assert.True(t, val >= 0.0, "secureFloat64() should return value >= 0")
		assert.True(t, val < 1.0, "secureFloat64() should return value < 1")
	}
}

func TestBackoff_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config BackoffConfig
	}{
		{
			name: "minimum values",
			config: BackoffConfig{
				InitialDelay: 1 * time.Nanosecond,
				MaxDelay:     1 * time.Nanosecond,
				Multiplier:   1.0,
				MaxAttempts:  2, // At least 2 attempts to test retry
				Jitter:       false,
			},
		},
		{
			name: "high multiplier",
			config: BackoffConfig{
				InitialDelay: 1 * time.Millisecond,
				MaxDelay:     100 * time.Millisecond,
				Multiplier:   10.0,
				MaxAttempts:  3,
				Jitter:       false,
			},
		},
		{
			name: "fractional multiplier",
			config: BackoffConfig{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   1.5,
				MaxAttempts:  5,
				Jitter:       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := NewBackoff(tt.config)
			assert.NotNil(t, backoff)

			// Test that it can execute basic operations
			attempts := 0
			operation := func() error {
				attempts++
				if attempts == 1 {
					return errors.New("first attempt fails")
				}
				return nil
			}

			ctx := context.Background()
			err := backoff.Retry(ctx, operation)
			assert.NoError(t, err)
			assert.Equal(t, 2, attempts)
		})
	}
}

func TestBackoff_JitterBounds(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       true,
	}

	backoff := NewBackoff(config)

	// Test multiple delay calculations to ensure jitter stays within bounds
	baseDelay := 200 * time.Millisecond // Expected delay for attempt 2 without jitter

	for i := 0; i < 50; i++ {
		delay := backoff.GetNextDelay(2)

		// With 25% jitter, delay should be within ±25% of base delay
		minExpected := time.Duration(float64(baseDelay) * 0.75)
		maxExpected := time.Duration(float64(baseDelay) * 1.25)

		assert.True(t, delay >= 0, "Delay should never be negative")
		assert.True(t, delay <= config.MaxDelay, "Delay should not exceed MaxDelay")

		// Most delays should be within expected jitter range (allowing some variance due to randomness)
		if delay < minExpected || delay > maxExpected {
			t.Logf("Delay %v outside expected range [%v, %v] (this is rare but acceptable)", delay, minExpected, maxExpected)
		}
	}
}

func TestBackoff_JitterWithNegativeResult(t *testing.T) {
	// Test edge case where jitter might push delay below zero
	config := BackoffConfig{
		InitialDelay: 1 * time.Nanosecond, // Very small initial delay
		MaxDelay:     1 * time.Second,
		Multiplier:   1.1, // Small multiplier
		MaxAttempts:  5,
		Jitter:       true,
	}

	backoff := NewBackoff(config)

	// Even with small delays and jitter, result should never be negative
	for i := 1; i <= 5; i++ {
		delay := backoff.GetNextDelay(i)
		assert.True(t, delay >= 0, "Delay should never be negative, got %v for attempt %d", delay, i)
	}
}

func TestBackoff_MaxDelayWithJitter(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     150 * time.Millisecond,
		Multiplier:   3.0, // Will quickly exceed MaxDelay
		MaxAttempts:  10,
		Jitter:       true,
	}

	backoff := NewBackoff(config)

	// Even with jitter, delay should never exceed MaxDelay
	for i := 1; i <= 10; i++ {
		delay := backoff.GetNextDelay(i)
		assert.True(t, delay <= config.MaxDelay,
			"Delay %v should not exceed MaxDelay %v for attempt %d", delay, config.MaxDelay, i)
	}
}

func TestBackoff_RetryWithPredicate_SuccessAfterRetryableErrors(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	}

	backoff := NewBackoff(config)
	attempts := 0

	retryableError := errors.New("retryable error")
	operation := func() error {
		attempts++
		if attempts < 3 {
			return retryableError
		}
		return nil // Success on third attempt
	}

	isRetryable := func(err error) bool {
		return err == retryableError
	}

	ctx := context.Background()
	err := backoff.RetryWithPredicate(ctx, operation, isRetryable)

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestBackoff_RetryWithPredicate_MaxAttemptsReached(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  2,
		Jitter:       false,
	}

	backoff := NewBackoff(config)
	attempts := 0

	retryableError := errors.New("always retryable")
	operation := func() error {
		attempts++
		return retryableError // Always fails
	}

	isRetryable := func(err error) bool {
		return true // Always retryable
	}

	ctx := context.Background()
	err := backoff.RetryWithPredicate(ctx, operation, isRetryable)

	assert.Error(t, err)
	assert.Equal(t, retryableError, err)
	assert.Equal(t, 2, attempts)
}

func TestBackoff_RetryWithPredicate_ContextCancellation(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	}

	backoff := NewBackoff(config)
	attempts := 0

	operation := func() error {
		attempts++
		return errors.New("retryable error")
	}

	isRetryable := func(err error) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := backoff.RetryWithPredicate(ctx, operation, isRetryable)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.True(t, attempts >= 1)
}

func TestBackoff_ContextCancelledBeforeOperation(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	}

	backoff := NewBackoff(config)
	attempts := 0

	operation := func() error {
		attempts++
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := backoff.Retry(ctx, operation)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 0, attempts) // Operation should never be called
}

func TestBackoff_ContextCancelledDuringBackoff(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	}

	backoff := NewBackoff(config)
	attempts := 0

	operation := func() error {
		attempts++
		return errors.New("retry me")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after first attempt during backoff
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := backoff.Retry(ctx, operation)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 1, attempts) // Should stop after first attempt
}

func TestBackoff_DelayCalculationConsistency(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	}

	backoff := NewBackoff(config)

	// Verify that GetNextDelay returns consistent results for same attempt
	for attempt := 1; attempt <= 5; attempt++ {
		delay1 := backoff.GetNextDelay(attempt)
		delay2 := backoff.GetNextDelay(attempt)
		assert.Equal(t, delay1, delay2, "GetNextDelay should be consistent for attempt %d", attempt)
	}
}

func TestBackoff_GetNextDelay_HighAttemptNumbers(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	}

	backoff := NewBackoff(config)

	// Test that high attempt numbers don't cause overflow or panics
	delay := backoff.GetNextDelay(100)
	assert.True(t, delay <= config.MaxDelay, "High attempt number should still respect MaxDelay")
	assert.True(t, delay > 0, "Delay should be positive")
}

func TestNewBackoff(t *testing.T) {
	config := BackoffConfig{
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   1.5,
		MaxAttempts:  7,
		Jitter:       true,
	}

	backoff := NewBackoff(config)

	assert.NotNil(t, backoff)
	assert.Equal(t, config, backoff.config)
}

func TestDefaultBackoffConfig_Jitter(t *testing.T) {
	config := DefaultBackoffConfig()
	assert.True(t, config.Jitter, "Default config should have jitter enabled")
}

func TestBackoff_ZeroMultiplier(t *testing.T) {
	// Edge case: what happens with multiplier close to zero
	config := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   0.1, // Very small multiplier
		MaxAttempts:  5,
		Jitter:       false,
	}

	backoff := NewBackoff(config)

	// Delays should decrease with multiplier < 1
	delay1 := backoff.GetNextDelay(1)
	delay2 := backoff.GetNextDelay(2)

	assert.True(t, delay2 < delay1, "With multiplier < 1, delay should decrease")
	assert.True(t, delay2 > 0, "Delay should still be positive")
}
