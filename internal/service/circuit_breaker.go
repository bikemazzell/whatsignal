package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"whatsignal/internal/constants"
	"whatsignal/internal/errors"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern for external service calls
type CircuitBreaker struct {
	name             string
	maxFailures      uint32
	timeout          time.Duration
	halfOpenMaxCalls uint32

	mu              sync.RWMutex
	state           CircuitBreakerState
	failures        uint32
	lastFailureTime time.Time
	halfOpenCalls   uint32
	successCount    uint32
	requestCount    uint32

	logger *errors.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures uint32, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		timeout:          timeout,
		halfOpenMaxCalls: constants.CBHalfOpenMaxCalls,
		state:            StateClosed,
		logger:           errors.NewLogger(),
	}
}

// Execute wraps a function call with circuit breaker logic
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if circuit breaker allows the call
	if !cb.allowRequest() {
		return errors.New(errors.ErrCodeInternalError, "circuit breaker is open").
			WithContext("service", cb.name).
			WithUserMessage("Service is temporarily unavailable")
	}

	// Execute the function
	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Record the result
	if err != nil {
		cb.recordFailure()
		cb.logger.LogError(
			errors.Wrap(err, errors.ErrCodeInternalError, "circuit breaker recorded failure"),
			"Circuit breaker failure recorded",
			logrus.Fields{
				"service":     cb.name,
				"duration_ms": duration.Milliseconds(),
				"failures":    atomic.LoadUint32(&cb.failures),
			},
		)
	} else {
		cb.recordSuccess()
		cb.logger.WithFields(logrus.Fields{
			"service":     cb.name,
			"duration_ms": duration.Milliseconds(),
		}).Debug("Circuit breaker success recorded")
	}

	return err
}

// allowRequest checks if a request should be allowed based on circuit breaker state
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.halfOpenCalls = 0
			cb.logger.WithFields(logrus.Fields{
				"service": cb.name,
			}).Info("Circuit breaker transitioning to half-open")
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		return cb.halfOpenCalls < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// recordFailure records a failure and potentially opens the circuit
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddUint32(&cb.failures, 1)
	atomic.AddUint32(&cb.requestCount, 1)
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
			cb.logger.WithFields(logrus.Fields{
				"service":      cb.name,
				"failures":     cb.failures,
				"max_failures": cb.maxFailures,
			}).Warn("Circuit breaker opened due to failures")
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.logger.WithFields(logrus.Fields{
			"service": cb.name,
		}).Warn("Circuit breaker reopened from half-open state")
	}
}

// recordSuccess records a success and potentially closes the circuit
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddUint32(&cb.successCount, 1)
	atomic.AddUint32(&cb.requestCount, 1)

	switch cb.state {
	case StateHalfOpen:
		cb.halfOpenCalls++
		if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
			cb.state = StateClosed
			cb.failures = 0
			cb.logger.WithFields(logrus.Fields{
				"service": cb.name,
			}).Info("Circuit breaker closed after successful half-open tests")
		}
	case StateClosed:
		// Reset failure count on success
		if cb.failures > 0 {
			cb.failures = 0
		}
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":         cb.name,
		"state":        cb.state,
		"failures":     atomic.LoadUint32(&cb.failures),
		"successes":    atomic.LoadUint32(&cb.successCount),
		"requests":     atomic.LoadUint32(&cb.requestCount),
		"last_failure": cb.lastFailureTime,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenCalls = 0
	atomic.StoreUint32(&cb.successCount, 0)
	atomic.StoreUint32(&cb.requestCount, 0)

	cb.logger.WithFields(logrus.Fields{
		"service": cb.name,
	}).Info("Circuit breaker manually reset")
}
