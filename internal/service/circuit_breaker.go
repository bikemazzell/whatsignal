package service

import (
	"context"
	"time"

	appErrors "whatsignal/internal/errors"
	pkgCircuitBreaker "whatsignal/pkg/circuitbreaker"

	"github.com/sirupsen/logrus"
)

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState = pkgCircuitBreaker.State

const (
	StateClosed   = pkgCircuitBreaker.StateClosed
	StateOpen     = pkgCircuitBreaker.StateOpen
	StateHalfOpen = pkgCircuitBreaker.StateHalfOpen
)

// CircuitBreaker adapts the shared circuitbreaker package to service-layer errors.
type CircuitBreaker struct {
	breaker *pkgCircuitBreaker.CircuitBreaker
	log     *logrus.Logger
	name    string
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, maxFailures uint32, timeout time.Duration) *CircuitBreaker {
	logger := appErrors.NewLogger()
	return NewCircuitBreakerWithLogger(name, maxFailures, timeout, logger.Logger)
}

// NewCircuitBreakerWithLogger creates a new circuit breaker with a configured logger.
func NewCircuitBreakerWithLogger(name string, maxFailures uint32, timeout time.Duration, logger *logrus.Logger) *CircuitBreaker {
	if logger == nil {
		logger = logrus.New()
	}
	return &CircuitBreaker{
		breaker: pkgCircuitBreaker.NewWithLogger(name, maxFailures, timeout, logger),
		log:     logger,
		name:    name,
	}
}

func (cb *CircuitBreaker) logger() *logrus.Logger {
	return cb.log
}

// Execute wraps a function call with circuit breaker logic.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	err := cb.breaker.Execute(ctx, fn)
	if pkgCircuitBreaker.IsCircuitBreakerError(err) {
		return appErrors.New(appErrors.ErrCodeInternalError, "circuit breaker is open").
			WithContext("service", cb.name).
			WithUserMessage("Service is temporarily unavailable")
	}
	return err
}

// GetState returns the current circuit breaker state without changing it.
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return cb.breaker.GetState()
}

// MaybeTransition advances an open circuit to half-open after its timeout elapses.
func (cb *CircuitBreaker) MaybeTransition() CircuitBreakerState {
	return cb.breaker.MaybeTransition()
}

// GetStats returns current circuit breaker statistics.
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	stats := cb.breaker.GetStats()
	return map[string]interface{}{
		"name":         stats.Name,
		"state":        stats.State,
		"failures":     stats.Failures,
		"successes":    stats.Successes,
		"requests":     stats.Requests,
		"last_failure": stats.LastFailureTime,
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.breaker.Reset()
}
