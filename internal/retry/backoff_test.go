package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBackoff_DefaultConfig(t *testing.T) {
	config := DefaultBackoffConfig()

	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected initial delay of 100ms, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected max delay of 30s, got %v", config.MaxDelay)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Expected multiplier of 2.0, got %v", config.Multiplier)
	}

	if config.MaxAttempts != 5 {
		t.Errorf("Expected max attempts of 5, got %v", config.MaxAttempts)
	}
}

func TestBackoff_SuccessFirstAttempt(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	attempts := 0
	operation := func() error {
		attempts++
		return nil // Success on first attempt
	}

	ctx := context.Background()
	err := backoff.Retry(ctx, operation)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestBackoff_SuccessAfterRetries(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil // Success on third attempt
	}

	ctx := context.Background()
	start := time.Now()
	err := backoff.Retry(ctx, operation)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Should have waited: 1ms + 2ms = 3ms minimum (with some tolerance)
	if duration < 2*time.Millisecond {
		t.Errorf("Expected at least 2ms duration, got %v", duration)
	}
}

func TestBackoff_FailureAfterMaxAttempts(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  2,
		Jitter:       false,
	})

	attempts := 0
	expectedError := errors.New("persistent error")
	operation := func() error {
		attempts++
		return expectedError
	}

	ctx := context.Background()
	err := backoff.Retry(ctx, operation)

	if err != expectedError {
		t.Errorf("Expected persistent error, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestBackoff_ContextCancellation(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	})

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("will be cancelled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := backoff.Retry(ctx, operation)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got %v", err)
	}

	// Should have made at least one attempt
	if attempts < 1 {
		t.Errorf("Expected at least 1 attempt, got %d", attempts)
	}
}

func TestBackoff_ExponentialIncrease(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	})

	// Test delay calculation for different attempts
	delay1 := backoff.GetNextDelay(1)
	delay2 := backoff.GetNextDelay(2)
	delay3 := backoff.GetNextDelay(3)

	expectedDelay1 := 10 * time.Millisecond
	expectedDelay2 := 20 * time.Millisecond
	expectedDelay3 := 40 * time.Millisecond

	if delay1 != expectedDelay1 {
		t.Errorf("Expected delay1 %v, got %v", expectedDelay1, delay1)
	}

	if delay2 != expectedDelay2 {
		t.Errorf("Expected delay2 %v, got %v", expectedDelay2, delay2)
	}

	if delay3 != expectedDelay3 {
		t.Errorf("Expected delay3 %v, got %v", expectedDelay3, delay3)
	}
}

func TestBackoff_MaxDelayConstraint(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     150 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  5,
		Jitter:       false,
	})

	// After several attempts, delay should be capped at MaxDelay
	delay5 := backoff.GetNextDelay(5)

	if delay5 > 150*time.Millisecond {
		t.Errorf("Expected delay capped at 150ms, got %v", delay5)
	}
}

func TestBackoff_WithPredicate_NonRetryableError(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	attempts := 0
	nonRetryableError := errors.New("non-retryable error")

	operation := func() error {
		attempts++
		return nonRetryableError
	}

	isRetryable := func(err error) bool {
		return err.Error() != "non-retryable error"
	}

	ctx := context.Background()
	err := backoff.RetryWithPredicate(ctx, operation, isRetryable)

	if err != nonRetryableError {
		t.Errorf("Expected non-retryable error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected only 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestBackoff_WithJitter(t *testing.T) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       true,
	})

	// With jitter, delays should vary (test multiple times)
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = backoff.GetNextDelay(2)
	}

	// Check that not all delays are identical (jitter should add variation)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to cause variation in delays, but all delays were identical")
	}
}
