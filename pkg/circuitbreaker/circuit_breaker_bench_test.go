package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func BenchmarkCircuitBreaker_Success(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during benchmarking

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)

	operation := func(ctx context.Context) error {
		return nil // Always succeeds
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cb.Execute(ctx, operation)
	}
}

func BenchmarkCircuitBreaker_Failure(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)

	operation := func(ctx context.Context) error {
		return errors.New("always fails")
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cb.Execute(ctx, operation)
	}
}

func BenchmarkCircuitBreaker_OpenState(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 1, 30*time.Second, logger) // Very low threshold

	// Trigger the circuit breaker to open by causing a failure
	failOperation := func(ctx context.Context) error {
		return errors.New("trigger failure")
	}
	_ = cb.Execute(context.Background(), failOperation)
	_ = cb.Execute(context.Background(), failOperation)

	// Now benchmark operations on open circuit breaker
	operation := func(ctx context.Context) error {
		return nil
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cb.Execute(ctx, operation)
	}
}

func BenchmarkCircuitBreaker_GetState(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cb.GetState()
	}
}

func BenchmarkCircuitBreaker_GetStats(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)

	// Add some operations to make stats meaningful
	operation := func(ctx context.Context) error {
		return nil
	}
	for i := 0; i < 10; i++ {
		_ = cb.Execute(context.Background(), operation)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = cb.GetStats()
	}
}

func BenchmarkCircuitBreaker_ConcurrentAccess(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)

	operation := func(ctx context.Context) error {
		return nil
	}

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.Execute(ctx, operation)
		}
	})
}

func BenchmarkCircuitBreaker_ConcurrentStateCheck(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.GetState()
		}
	})
}

func BenchmarkCircuitBreaker_MixedOperations(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cb := NewWithLogger("test-cb", 5, 30*time.Second, logger)

	successOperation := func(ctx context.Context) error {
		return nil
	}

	failOperation := func(ctx context.Context) error {
		return errors.New("failure")
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Mix of successful and failing operations
		if i%4 == 0 {
			_ = cb.Execute(ctx, failOperation)
		} else {
			_ = cb.Execute(ctx, successOperation)
		}

		// Occasionally check state
		if i%10 == 0 {
			_ = cb.GetState()
		}
	}
}

func BenchmarkCircuitBreaker_StateTransition(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	failOperation := func(ctx context.Context) error {
		return errors.New("failure")
	}

	successOperation := func(ctx context.Context) error {
		return nil
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create a new circuit breaker for each iteration to test initialization cost
		cb := NewWithLogger("test-cb", 2, 1*time.Millisecond, logger)

		// Cause failures to open the circuit
		_ = cb.Execute(ctx, failOperation)
		_ = cb.Execute(ctx, failOperation)
		_ = cb.Execute(ctx, failOperation) // Should be rejected due to open circuit

		// Wait for timeout to allow half-open state
		time.Sleep(2 * time.Millisecond)

		// Try to recover
		_ = cb.Execute(ctx, successOperation)
		_ = cb.Execute(ctx, successOperation)
		_ = cb.Execute(ctx, successOperation)
	}
}
