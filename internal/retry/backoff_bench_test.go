package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func BenchmarkBackoff_SuccessFirstAttempt(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	operation := func() error {
		return nil // Always succeeds
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = backoff.Retry(ctx, operation)
	}
}

func BenchmarkBackoff_FailureAfterRetries(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Microsecond, // Very fast for benchmarking
		MaxDelay:     10 * time.Microsecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	operation := func() error {
		return errors.New("always fails")
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = backoff.Retry(ctx, operation)
	}
}

func BenchmarkBackoff_WithJitter(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Microsecond,
		MaxDelay:     10 * time.Microsecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       true, // Enable jitter
	})

	operation := func() error {
		return errors.New("always fails")
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = backoff.Retry(ctx, operation)
	}
}

func BenchmarkBackoff_WithoutJitter(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Microsecond,
		MaxDelay:     10 * time.Microsecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false, // Disable jitter
	})

	operation := func() error {
		return errors.New("always fails")
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = backoff.Retry(ctx, operation)
	}
}

func BenchmarkBackoff_DelayCalculation(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  10,
		Jitter:       true,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Benchmark the delay calculation for different attempt numbers
		attempt := (i % 10) + 1
		_ = backoff.GetNextDelay(attempt)
	}
}

func BenchmarkBackoff_WithPredicate(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Microsecond,
		MaxDelay:     10 * time.Microsecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	retryableErr := errors.New("retryable error")

	operation := func() error {
		return retryableErr
	}

	isRetryable := func(err error) bool {
		return err == retryableErr
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = backoff.RetryWithPredicate(ctx, operation, isRetryable)
	}
}

func BenchmarkBackoff_NonRetryableError(b *testing.B) {
	backoff := NewBackoff(BackoffConfig{
		InitialDelay: 1 * time.Microsecond,
		MaxDelay:     10 * time.Microsecond,
		Multiplier:   2.0,
		MaxAttempts:  3,
		Jitter:       false,
	})

	nonRetryableErr := errors.New("non-retryable error")

	operation := func() error {
		return nonRetryableErr
	}

	isRetryable := func(err error) bool {
		return false // Never retry
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = backoff.RetryWithPredicate(ctx, operation, isRetryable)
	}
}
