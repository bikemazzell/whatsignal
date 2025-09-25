package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// State represents the state of a circuit breaker
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern for external service calls
type CircuitBreaker struct {
	name             string
	maxFailures      uint32
	timeout          time.Duration
	halfOpenMaxCalls uint32

	mu              sync.RWMutex
	state           State
	failures        uint32
	lastFailureTime time.Time
	halfOpenCalls   uint32
	successCount    uint32
	requestCount    uint32

	logger *logrus.Logger
}

// New creates a new circuit breaker
func New(name string, maxFailures uint32, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		timeout:          timeout,
		halfOpenMaxCalls: 3, // Allow 3 test calls in half-open state
		state:            StateClosed,
		logger:           logrus.New(),
	}
}

// NewWithLogger creates a new circuit breaker with a custom logger
func NewWithLogger(name string, maxFailures uint32, timeout time.Duration, logger *logrus.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		timeout:          timeout,
		halfOpenMaxCalls: 3,
		state:            StateClosed,
		logger:           logger,
	}
}

// Execute executes the given function if the circuit breaker is in a state that allows it
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if !cb.allowRequest() {
		return &CircuitBreakerError{
			Name:  cb.name,
			State: cb.GetState(),
		}
	}

	// Track request
	atomic.AddUint32(&cb.requestCount, 1)

	// Execute the function
	err := fn(ctx)

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// allowRequest determines if a request should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		return cb.shouldAttemptReset()
	case StateHalfOpen:
		return cb.halfOpenCalls < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// shouldAttemptReset checks if the circuit breaker should attempt to reset
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	return time.Since(cb.lastFailureTime) >= cb.timeout
}

// onSuccess handles successful requests
func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.halfOpenCalls++
		cb.successCount++

		// If we have enough successful calls in half-open, close the circuit
		if cb.successCount >= cb.halfOpenMaxCalls {
			cb.reset()
			cb.logger.WithFields(logrus.Fields{
				"circuit_breaker": cb.name,
				"state":           "CLOSED",
			}).Info("Circuit breaker closed after successful recovery")
		}
	case StateClosed:
		atomic.AddUint32(&cb.successCount, 1)
	}
}

// onFailure handles failed requests
func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddUint32(&cb.failures, 1)
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.trip()
		}
	case StateHalfOpen:
		cb.trip()
	}
}

// trip transitions the circuit breaker to the open state
func (cb *CircuitBreaker) trip() {
	cb.state = StateOpen
	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"failures":        cb.failures,
		"state":           "OPEN",
	}).Warn("Circuit breaker opened due to failures")
}

// reset resets the circuit breaker to the closed state
func (cb *CircuitBreaker) reset() {
	cb.state = StateClosed
	cb.failures = 0
	cb.successCount = 0
	cb.halfOpenCalls = 0
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Check if we should transition from open to half-open
	if cb.state == StateOpen && cb.shouldAttemptReset() {
		cb.mu.RUnlock()
		cb.mu.Lock()
		// Double-check pattern
		if cb.state == StateOpen && cb.shouldAttemptReset() {
			cb.state = StateHalfOpen
			cb.halfOpenCalls = 0
			cb.successCount = 0
			cb.logger.WithFields(logrus.Fields{
				"circuit_breaker": cb.name,
				"state":           "HALF_OPEN",
			}).Info("Circuit breaker transitioned to half-open")
		}
		cb.mu.Unlock()
		cb.mu.RLock()
	}

	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return Stats{
		Name:            cb.name,
		State:           cb.state,
		Failures:        cb.failures,
		Requests:        cb.requestCount,
		Successes:       cb.successCount,
		LastFailureTime: cb.lastFailureTime,
	}
}

// Stats represents circuit breaker statistics
type Stats struct {
	Name            string
	State           State
	Failures        uint32
	Requests        uint32
	Successes       uint32
	LastFailureTime time.Time
}

// CircuitBreakerError represents an error when the circuit breaker is open
type CircuitBreakerError struct {
	Name  string
	State State
}

func (e *CircuitBreakerError) Error() string {
	return fmt.Sprintf("circuit breaker '%s' is %s", e.Name, e.State)
}

// IsCircuitBreakerError checks if an error is a circuit breaker error
func IsCircuitBreakerError(err error) bool {
	_, ok := err.(*CircuitBreakerError)
	return ok
}
