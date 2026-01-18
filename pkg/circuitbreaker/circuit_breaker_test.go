package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		expected string
	}{
		{
			name:     "Closed state",
			state:    StateClosed,
			expected: "CLOSED",
		},
		{
			name:     "Open state",
			state:    StateOpen,
			expected: "OPEN",
		},
		{
			name:     "Half-open state",
			state:    StateHalfOpen,
			expected: "HALF_OPEN",
		},
		{
			name:     "Unknown state",
			state:    State(999),
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNew(t *testing.T) {
	cb := New("test-service", 3, time.Second*30)

	assert.NotNil(t, cb)
	assert.Equal(t, "test-service", cb.name)
	assert.Equal(t, uint32(3), cb.maxFailures)
	assert.Equal(t, time.Second*30, cb.timeout)
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, uint32(3), cb.halfOpenMaxCalls)
	assert.NotNil(t, cb.logger)
}

func TestNewWithLogger(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Quiet logging for tests

	cb := NewWithLogger("test-service", 5, time.Minute, logger)

	assert.NotNil(t, cb)
	assert.Equal(t, "test-service", cb.name)
	assert.Equal(t, uint32(5), cb.maxFailures)
	assert.Equal(t, time.Minute, cb.timeout)
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, logger, cb.logger)
}

func TestExecute_SuccessfulOperation(t *testing.T) {
	cb := New("test-service", 3, time.Second*30)
	ctx := context.Background()

	// Successful operation
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())

	stats := cb.GetStats()
	assert.Equal(t, uint32(1), stats.Requests)
	assert.Equal(t, uint32(1), stats.Successes)
	assert.Equal(t, uint32(0), stats.Failures)
}

func TestExecute_FailedOperation(t *testing.T) {
	cb := New("test-service", 3, time.Second*30)
	ctx := context.Background()
	expectedErr := errors.New("operation failed")

	// Failed operation
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return expectedErr
	})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, StateClosed, cb.GetState()) // Still closed after 1 failure

	stats := cb.GetStats()
	assert.Equal(t, uint32(1), stats.Requests)
	assert.Equal(t, uint32(0), stats.Successes)
	assert.Equal(t, uint32(1), stats.Failures)
}

func TestCircuitBreakerTripping(t *testing.T) {
	cb := New("test-service", 2, time.Second*30) // Trip after 2 failures
	ctx := context.Background()
	expectedErr := errors.New("operation failed")

	// First failure
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return expectedErr
	})
	assert.Error(t, err)
	assert.Equal(t, StateClosed, cb.GetState())

	// Second failure - should trip the circuit
	err = cb.Execute(ctx, func(ctx context.Context) error {
		return expectedErr
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())

	// Third attempt should be rejected
	err = cb.Execute(ctx, func(ctx context.Context) error {
		return nil // This function won't be called
	})
	assert.Error(t, err)
	assert.IsType(t, &CircuitBreakerError{}, err)

	stats := cb.GetStats()
	assert.Equal(t, uint32(2), stats.Requests) // Only 2 requests executed
	assert.Equal(t, uint32(2), stats.Failures)
	assert.Equal(t, StateOpen, stats.State)
}

func TestCircuitBreakerRecovery(t *testing.T) {
	cb := New("test-service", 2, time.Millisecond*100) // Short timeout for testing
	ctx := context.Background()

	// Trip the circuit breaker
	for i := 0; i < 2; i++ {
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout
	time.Sleep(time.Millisecond * 150)

	// Should transition to half-open on next state check
	state := cb.GetState()
	assert.Equal(t, StateHalfOpen, state)

	// Successful executions in half-open should close the circuit
	for i := 0; i < 3; i++ { // 3 successful calls to fully recover
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}

	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreakerHalfOpenLimits(t *testing.T) {
	cb := New("test-service", 1, time.Millisecond*100)
	ctx := context.Background()

	// Trip the circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("failure")
	})
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout
	time.Sleep(time.Millisecond * 150)

	// Force transition to half-open by checking state
	state := cb.GetState()
	assert.Equal(t, StateHalfOpen, state)

	// Make first two calls (should be allowed)
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, StateHalfOpen, cb.GetState()) // Should still be half-open
	}

	// Third call should close the circuit since we have 3 successful calls
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState()) // Should be closed now
}

func TestHalfOpenMaxCallsLimit(t *testing.T) {
	// Create a circuit breaker with manual control to test half-open limits
	cb := New("test-service", 1, time.Hour) // Long timeout to prevent auto-reset

	// Manually set to half-open state
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.halfOpenCalls = 0
	cb.mu.Unlock()

	// First 3 calls should be allowed
	for i := 0; i < 3; i++ {
		assert.True(t, cb.allowRequest(), "Call %d should be allowed", i+1)
		cb.mu.Lock()
		cb.halfOpenCalls++
		cb.mu.Unlock()
	}

	// Fourth call should be rejected
	assert.False(t, cb.allowRequest(), "Fourth call should be rejected in half-open")
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cb := New("test-service", 1, time.Millisecond*100)
	ctx := context.Background()

	// Trip the circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("failure")
	})

	// Wait for timeout
	time.Sleep(time.Millisecond * 150)
	cb.GetState() // Transition to half-open

	// Failure in half-open should immediately trip the circuit
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("failure in half-open")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestGetStats(t *testing.T) {
	cb := New("test-service", 3, time.Second*30)
	ctx := context.Background()

	// Execute some operations
	_ = cb.Execute(ctx, func(ctx context.Context) error { return nil })
	_ = cb.Execute(ctx, func(ctx context.Context) error { return errors.New("failure") })
	_ = cb.Execute(ctx, func(ctx context.Context) error { return nil })

	stats := cb.GetStats()
	assert.Equal(t, "test-service", stats.Name)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, uint32(3), stats.Requests)
	assert.Equal(t, uint32(2), stats.Successes)
	assert.Equal(t, uint32(1), stats.Failures)
	assert.False(t, stats.LastFailureTime.IsZero())
}

func TestCircuitBreakerError(t *testing.T) {
	err := &CircuitBreakerError{
		Name:  "test-service",
		State: StateOpen,
	}

	expectedMsg := "circuit breaker 'test-service' is OPEN"
	assert.Equal(t, expectedMsg, err.Error())

	// Test IsCircuitBreakerError
	assert.True(t, IsCircuitBreakerError(err))
	assert.False(t, IsCircuitBreakerError(errors.New("regular error")))
	assert.False(t, IsCircuitBreakerError(nil))
}

func TestConcurrentAccess(t *testing.T) {
	cb := New("test-service", 20, time.Second*30) // Higher threshold to avoid tripping
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Launch many concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_ = cb.Execute(ctx, func(ctx context.Context) error {
				if id%10 == 0 { // 10% failure rate
					return errors.New("failure")
				}
				return nil
			})
		}(i)
	}

	wg.Wait()

	stats := cb.GetStats()
	// Some requests might be rejected if circuit trips, so check for reasonable values
	assert.True(t, stats.Requests > 0)
	assert.True(t, stats.Failures > 0)
	assert.True(t, stats.Successes > 0)
}

func TestShouldAttemptReset(t *testing.T) {
	cb := New("test-service", 1, time.Millisecond*100)

	// Set last failure time to now
	cb.mu.Lock()
	cb.lastFailureTime = time.Now()
	cb.mu.Unlock()

	// Should not reset immediately
	assert.False(t, cb.shouldAttemptReset())

	// Wait for timeout
	time.Sleep(time.Millisecond * 150)
	assert.True(t, cb.shouldAttemptReset())
}

func TestAllowRequestInDifferentStates(t *testing.T) {
	cb := New("test-service", 2, time.Second*30)

	// Closed state should allow requests
	assert.True(t, cb.allowRequest())

	// Trip to open state
	cb.mu.Lock()
	cb.state = StateOpen
	cb.lastFailureTime = time.Now()
	cb.mu.Unlock()

	// Open state should not allow requests (timeout not reached)
	assert.False(t, cb.allowRequest())

	// Transition to half-open
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.halfOpenCalls = 0
	cb.mu.Unlock()

	// Half-open should allow limited requests
	assert.True(t, cb.allowRequest())

	// Fill up half-open calls
	cb.mu.Lock()
	cb.halfOpenCalls = cb.halfOpenMaxCalls
	cb.mu.Unlock()

	// Should not allow more requests in half-open
	assert.False(t, cb.allowRequest())
}

func TestReset(t *testing.T) {
	cb := New("test-service", 2, time.Second*30)

	// Set some values
	cb.mu.Lock()
	cb.state = StateOpen
	cb.failures = 5
	cb.successCount = 10
	cb.halfOpenCalls = 2
	cb.mu.Unlock()

	// Reset should clear all counters and set state to closed
	cb.reset()

	cb.mu.RLock()
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, uint32(0), cb.failures)
	assert.Equal(t, uint32(0), cb.successCount)
	assert.Equal(t, uint32(0), cb.halfOpenCalls)
	cb.mu.RUnlock()
}

func TestOnSuccessInDifferentStates(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Silence logs in tests
	cb := NewWithLogger("test-service", 2, time.Second*30, logger)

	// Test success in closed state
	cb.onSuccess()
	assert.Equal(t, StateClosed, cb.GetState())

	// Test success in half-open state
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.halfOpenCalls = 0
	cb.successCount = 0
	cb.mu.Unlock()

	// First success in half-open
	cb.onSuccess()
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Second success
	cb.onSuccess()
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Third success should close the circuit
	cb.onSuccess()
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestOnFailureInDifferentStates(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Silence logs in tests
	cb := NewWithLogger("test-service", 2, time.Second*30, logger)

	// Test failure in closed state (should not trip yet)
	cb.onFailure()
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, uint32(1), cb.failures)

	// Second failure should trip
	cb.onFailure()
	assert.Equal(t, StateOpen, cb.GetState())

	// Test failure in half-open state (should immediately trip)
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.failures = 0 // Reset for clean test
	cb.mu.Unlock()

	cb.onFailure()
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestConcurrentStateTransition(t *testing.T) {
	// This test verifies that concurrent GetState() calls during the
	// OPEN -> HALF_OPEN transition don't cause race conditions.
	// Run with -race flag: go test -race ./pkg/circuitbreaker/...
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Silence logs in tests
	cb := NewWithLogger("test-service", 1, time.Millisecond*50, logger)
	ctx := context.Background()

	// Trip the circuit breaker
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("failure")
	})
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout to be almost ready
	time.Sleep(time.Millisecond * 40)

	// Launch many goroutines that will call GetState() concurrently
	// during the transition window
	var wg sync.WaitGroup
	numGoroutines := 100
	statesSeen := make([]State, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Small sleep to spread out the calls around the transition time
			time.Sleep(time.Millisecond * time.Duration(idx%20))
			statesSeen[idx] = cb.GetState()
		}(i)
	}

	wg.Wait()

	// All states should be valid (either OPEN or HALF_OPEN after timeout)
	for i, state := range statesSeen {
		assert.True(t, state == StateOpen || state == StateHalfOpen,
			"goroutine %d saw invalid state: %s", i, state)
	}

	// Final state should be HALF_OPEN (timeout has definitely elapsed)
	time.Sleep(time.Millisecond * 20)
	assert.Equal(t, StateHalfOpen, cb.GetState())
}

func TestConcurrentExecuteDuringRecovery(t *testing.T) {
	// This test verifies that concurrent Execute() calls during recovery
	// (HALF_OPEN state) are handled correctly without race conditions.
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	cb := NewWithLogger("test-service", 1, time.Millisecond*50, logger)
	ctx := context.Background()

	// Trip the circuit breaker
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("failure")
	})

	// Wait for timeout
	time.Sleep(time.Millisecond * 60)

	// Verify we're in half-open
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Launch concurrent successful requests
	var wg sync.WaitGroup
	var successCount int32
	var rejectedCount int32
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Execute(ctx, func(ctx context.Context) error {
				return nil
			})
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if IsCircuitBreakerError(err) {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}

	wg.Wait()

	// Should have processed exactly 3 (halfOpenMaxCalls) before closing
	// and then allowed the rest after closing
	finalState := cb.GetState()
	assert.Equal(t, StateClosed, finalState, "Circuit should be closed after successful recovery")

	// Total should be all goroutines
	totalProcessed := successCount + rejectedCount
	assert.Equal(t, int32(numGoroutines), totalProcessed, "All goroutines should have completed")

	// At least halfOpenMaxCalls should have succeeded (the recovery calls)
	assert.True(t, successCount >= 3, "At least 3 calls should have succeeded")
}
