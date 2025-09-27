package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_EdgeCases(t *testing.T) {
	t.Run("circuit breaker opens after threshold failures", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 2, 100*time.Millisecond) // threshold: 2, timeout: 100ms
		ctx := context.Background()

		testErr := errors.New("test error")
		failingOperation := func(ctx context.Context) error {
			return testErr
		}

		// First failure
		err := cb.Execute(ctx, failingOperation)
		assert.Error(t, err)
		assert.Equal(t, testErr, err)

		// Second failure - should open circuit
		err = cb.Execute(ctx, failingOperation)
		assert.Error(t, err)
		assert.Equal(t, testErr, err)

		// Third call - circuit should be open
		err = cb.Execute(ctx, failingOperation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
	})

	t.Run("circuit breaker resets after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 10*time.Millisecond) // threshold: 1, timeout: 10ms
		ctx := context.Background()

		// Cause failure to open circuit
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure")
		})
		assert.Error(t, err)

		// Should be open now
		err = cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")

		// Wait for timeout
		time.Sleep(15 * time.Millisecond)

		// Should allow one request (half-open state)
		successCalled := false
		err = cb.Execute(ctx, func(ctx context.Context) error {
			successCalled = true
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, successCalled)
	})

	t.Run("multiple successes reset failure count", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 3, 100*time.Millisecond) // threshold: 3
		ctx := context.Background()

		// Two failures
		_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail1") })
		_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail2") })

		// Success should reset the count
		err := cb.Execute(ctx, func(ctx context.Context) error { return nil })
		assert.NoError(t, err)

		// Now we need 3 more failures to open
		_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail3") })
		_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail4") })

		// Circuit should still be closed
		err = cb.Execute(ctx, func(ctx context.Context) error { return nil })
		assert.NoError(t, err)
	})
}

func TestCircuitBreaker_AllowRequest(t *testing.T) {
	t.Run("closed state allows requests", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 2, 100*time.Millisecond)
		ctx := context.Background()

		// Should allow request when closed
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("open state blocks requests", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 100*time.Millisecond)
		ctx := context.Background()

		// Open the circuit
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure")
		})

		// Should block subsequent requests
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
	})
}

func TestCircuitBreaker_States(t *testing.T) {
	t.Run("test all circuit breaker states", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 2, 100*time.Millisecond)
		ctx := context.Background()

		// Should start closed
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)

		// Record failures to open
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure 1")
		})
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure 2")
		})

		// Now should be open
		err = cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
	})
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	t.Run("half-open allows limited calls", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)
		ctx := context.Background()

		// Open the circuit
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure")
		})

		// Wait for timeout to enter half-open
		time.Sleep(15 * time.Millisecond)

		// Should allow calls in half-open state
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("half-open failure reopens circuit", func(t *testing.T) {
		cb := NewCircuitBreaker("test", 1, 10*time.Millisecond)
		ctx := context.Background()

		// Open the circuit
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure")
		})

		// Wait for timeout to enter half-open
		time.Sleep(15 * time.Millisecond)

		// Fail in half-open state
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("half-open failure")
		})
		assert.Error(t, err)
		assert.Equal(t, "half-open failure", err.Error())

		// Should be open again
		err = cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
	})
}

func TestCircuitBreaker_GetState(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 100*time.Millisecond)
	ctx := context.Background()

	// Initially should be closed
	assert.Equal(t, StateClosed, cb.GetState())

	// After failures, should be open
	_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail1") })
	_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail2") })
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_GetStats(t *testing.T) {
	cb := NewCircuitBreaker("test-stats", 2, 100*time.Millisecond)
	ctx := context.Background()

	// Initial stats
	stats := cb.GetStats()
	assert.Equal(t, "test-stats", stats["name"])
	assert.Equal(t, StateClosed, stats["state"])
	assert.Equal(t, uint32(0), stats["failures"])
	assert.Equal(t, uint32(0), stats["successes"])
	assert.Equal(t, uint32(0), stats["requests"])

	// After success
	_ = cb.Execute(ctx, func(ctx context.Context) error { return nil })
	stats = cb.GetStats()
	assert.Equal(t, uint32(0), stats["failures"])
	assert.Equal(t, uint32(1), stats["successes"])
	assert.Equal(t, uint32(1), stats["requests"])

	// After failure
	_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail") })
	stats = cb.GetStats()
	assert.Equal(t, uint32(1), stats["failures"])
	assert.Equal(t, uint32(1), stats["successes"])
	assert.Equal(t, uint32(2), stats["requests"])
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker("test-reset", 1, 100*time.Millisecond)
	ctx := context.Background()

	// Open the circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail") })
	assert.Equal(t, StateOpen, cb.GetState())

	// Reset should close the circuit
	cb.Reset()
	assert.Equal(t, StateClosed, cb.GetState())

	// Stats should be reset
	stats := cb.GetStats()
	assert.Equal(t, uint32(0), stats["failures"])
	assert.Equal(t, uint32(0), stats["successes"])
	assert.Equal(t, uint32(0), stats["requests"])

	// Should allow requests again
	err := cb.Execute(ctx, func(ctx context.Context) error { return nil })
	assert.NoError(t, err)
}

func TestCircuitBreaker_HalfOpenSuccess(t *testing.T) {
	cb := NewCircuitBreaker("test-half-open", 1, 10*time.Millisecond)
	ctx := context.Background()

	// Open the circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("fail") })
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout to enter half-open
	time.Sleep(15 * time.Millisecond)

	// Multiple successes in half-open should close circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func(ctx context.Context) error { return nil })
		assert.NoError(t, err)
	}

	// Should be closed now
	assert.Equal(t, StateClosed, cb.GetState())

	// Verify stats show successful operations
	stats := cb.GetStats()
	assert.Equal(t, uint32(0), stats["failures"])
	assert.Greater(t, stats["successes"], uint32(0))
}
