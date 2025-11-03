// Package service provides core business logic services for WhatSignal.
//
// The SignalPoller component handles automatic polling of Signal messages
// from the Signal CLI REST API. It implements:
//
//   - Configurable polling intervals with automatic retry
//   - Exponential backoff with jitter for failed attempts
//   - Smart error classification (retryable vs non-retryable)
//   - Graceful shutdown handling with context cancellation
//   - Comprehensive metrics and structured logging
//   - Graceful degradation on persistent failures
//
// Basic usage:
//
//	poller := NewSignalPoller(signalClient, messageService, config, retryConfig, logger)
//	if err := poller.Start(ctx); err != nil {
//		log.Fatal(err)
//	}
//	defer poller.Stop()
//
// The poller runs in a background goroutine and polls at the configured interval.
// Each poll attempt includes retry logic with exponential backoff for transient failures.
package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"whatsignal/internal/metrics"
	"whatsignal/internal/models"
	"whatsignal/pkg/signal"

	"github.com/sirupsen/logrus"
)

const (
	// metricsLabelAttempt is the label key for retry attempt number in metrics
	metricsLabelAttempt = "attempt"
	// metricsLabelStatus is the label key for operation status in metrics
	metricsLabelStatus = "status"
	// connectionTestTimeoutSec is the timeout for testing Signal CLI connection
	connectionTestTimeoutSec = 10
)

// SignalPoller handles automatic polling of Signal messages.
// It runs in a background goroutine and polls the Signal CLI REST API
// at configured intervals, with retry logic for transient failures.
type SignalPoller struct {
	signalClient        signal.Client
	messageService      MessageService
	config              models.SignalConfig
	retryConfig         models.RetryConfig
	logger              *logrus.Logger
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	running             bool
	mu                  sync.RWMutex
	consecutiveFailures int
	lastSuccessTime     time.Time
}

// NewSignalPoller creates a new Signal polling service.
// If logger is nil, a default logger with WARN level is created.
// The poller is created in a stopped state and must be started with Start().
func NewSignalPoller(signalClient signal.Client, messageService MessageService, signalConfig models.SignalConfig, retryConfig models.RetryConfig, logger *logrus.Logger) *SignalPoller {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}
	return &SignalPoller{
		signalClient:   signalClient,
		messageService: messageService,
		config:         signalConfig,
		retryConfig:    retryConfig,
		logger:         logger,
	}
}

// validateConfig checks if the poller configuration is valid.
// It returns an error if any configuration values are invalid or out of range.
func (sp *SignalPoller) validateConfig() error {
	if sp.config.PollIntervalSec <= 0 {
		return fmt.Errorf("poll interval must be positive, got %d", sp.config.PollIntervalSec)
	}

	if sp.config.PollTimeoutSec < 0 {
		return fmt.Errorf("poll timeout cannot be negative, got %d", sp.config.PollTimeoutSec)
	}

	if sp.retryConfig.MaxAttempts <= 0 {
		return fmt.Errorf("max retry attempts must be positive, got %d", sp.retryConfig.MaxAttempts)
	}

	if sp.retryConfig.InitialBackoffMs < 0 {
		return fmt.Errorf("initial backoff cannot be negative, got %d", sp.retryConfig.InitialBackoffMs)
	}

	if sp.retryConfig.MaxBackoffMs < sp.retryConfig.InitialBackoffMs {
		return fmt.Errorf("max backoff (%d ms) must be >= initial backoff (%d ms)",
			sp.retryConfig.MaxBackoffMs, sp.retryConfig.InitialBackoffMs)
	}

	return nil
}

// Start begins the background polling process.
// It validates the configuration, tests the connection to Signal CLI,
// and starts a background goroutine that polls at the configured interval.
//
// Returns an error if:
//   - The poller is already running
//   - Configuration validation fails
//   - Connection test to Signal CLI fails
//
// If polling is disabled in configuration, this is a no-op that returns nil.
func (sp *SignalPoller) Start(ctx context.Context) error {
	sp.mu.Lock()

	if sp.running {
		sp.mu.Unlock()
		return fmt.Errorf("signal poller is already running")
	}

	// Validate configuration before starting
	if err := sp.validateConfig(); err != nil {
		sp.mu.Unlock()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if !sp.config.PollingEnabled {
		sp.mu.Unlock()
		sp.logger.WithFields(sp.logFields()).Info("Signal polling is disabled in configuration")
		return nil
	}

	// Release lock before potentially long-running connection test
	sp.mu.Unlock()

	// Test connection first
	if err := sp.testConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to Signal CLI before starting poller: %w", err)
	}

	// Re-acquire lock to update state
	sp.mu.Lock()

	// Create context for polling loop
	pollCtx, cancel := context.WithCancel(ctx)
	sp.ctx = pollCtx
	sp.cancel = cancel
	sp.running = true
	sp.lastSuccessTime = time.Now()

	sp.mu.Unlock()

	// Start polling goroutine
	sp.wg.Add(1)
	go sp.pollLoop()

	sp.logger.WithFields(sp.logFields()).Info("Signal poller started successfully")

	return nil
}

// Stop gracefully stops the polling process.
// This method is idempotent and can be called multiple times safely.
// It waits for the polling goroutine to complete before returning.
func (sp *SignalPoller) Stop() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if !sp.running {
		return
	}

	sp.logger.WithFields(sp.logFields()).Info("Stopping Signal poller...")

	// Cancel context if it exists
	if sp.cancel != nil {
		sp.cancel()
	}

	sp.wg.Wait()
	sp.running = false

	// Clear context and cancel function for idempotency
	sp.ctx = nil
	sp.cancel = nil

	sp.logger.WithFields(sp.logFields()).Info("Signal poller stopped")
}

// IsRunning returns whether the poller is currently active.
func (sp *SignalPoller) IsRunning() bool {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.running
}

// testConnection verifies Signal CLI is accessible.
// It uses a shorter timeout to fail fast if the service is unavailable.
func (sp *SignalPoller) testConnection(ctx context.Context) error {
	testCtx, cancel := context.WithTimeout(ctx, connectionTestTimeoutSec*time.Second)
	defer cancel()

	if err := sp.signalClient.InitializeDevice(testCtx); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	return nil
}

// logFields returns common structured logging fields for this poller instance.
func (sp *SignalPoller) logFields() logrus.Fields {
	return logrus.Fields{
		"component":         "signal_poller",
		"phone_number":      sp.config.IntermediaryPhoneNumber,
		"poll_interval_sec": sp.config.PollIntervalSec,
		"poll_timeout_sec":  sp.config.PollTimeoutSec,
		"polling_enabled":   sp.config.PollingEnabled,
	}
}

// pollLoop runs the main polling logic.
// It polls at the configured interval, resetting the ticker after each poll
// to ensure consistent intervals regardless of poll duration.
func (sp *SignalPoller) pollLoop() {
	defer sp.wg.Done()

	interval := time.Duration(sp.config.PollIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sp.ctx.Done():
			sp.logger.WithFields(sp.logFields()).Debug("Poll loop context cancelled, exiting")
			return
		case <-ticker.C:
			sp.pollWithRetry()
			// Reset ticker after poll completes to ensure consistent intervals
			ticker.Reset(interval)
		}
	}
}

// isRetryableError determines if an error should be retried.
// It returns false for context errors, authentication errors, and validation errors.
// It returns true for network errors and other transient failures.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Treat context.Canceled as non-retryable (shutdown), but retry context.DeadlineExceeded (HTTP/client timeout)
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	errStr := strings.ToLower(err.Error())

	// Retry network errors and transient failures
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "no route to host") {
		return true
	}

	// Don't retry authentication/authorization errors
	if strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "not authorized") {
		return false
	}

	// Don't retry validation errors
	if strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "bad request") {
		return false
	}

	// Default to retrying unknown errors (conservative approach)
	return true
}

// pollWithRetry executes a single poll attempt with exponential backoff on failure.
// It implements smart retry logic with:
//   - Error classification (retryable vs non-retryable)
//   - Exponential backoff with jitter
//   - Context cancellation checks
//   - Comprehensive metrics and logging
//   - Graceful degradation tracking
//
// The method uses the parent context directly (no additional timeout) to avoid
// conflicts with the retry logic. The parent context is managed by pollLoop and
// will be cancelled when Stop() is called.
func (sp *SignalPoller) pollWithRetry() {
	// Use parent context directly - no additional timeout
	// This prevents "context deadline exceeded" errors when retries take longer than expected
	ctx := sp.ctx

	startTime := time.Now()

	// Check verbose logging once at start
	verbose := IsVerboseLogging(ctx)

	// Record polling attempt
	metrics.IncrementCounter("signal_poll_attempts_total", nil, "Total Signal polling attempts")

	backoff := time.Duration(sp.retryConfig.InitialBackoffMs) * time.Millisecond
	maxBackoff := time.Duration(sp.retryConfig.MaxBackoffMs) * time.Millisecond

	for attempt := 0; attempt < sp.retryConfig.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			sp.logger.WithFields(sp.logFields()).Debug("Context cancelled, stopping retry attempts")
			metrics.IncrementCounter("signal_poll_cancelled_total", nil, "Cancelled Signal polling operations")
			return
		default:
		}

		attemptStart := time.Now()
		err := sp.messageService.PollSignalMessages(ctx)
		attemptDuration := time.Since(attemptStart)

		// Record timing for this attempt
		metrics.RecordTimer("signal_poll_attempt_duration", attemptDuration, map[string]string{
			metricsLabelAttempt: fmt.Sprintf("%d", attempt+1),
		}, "Signal polling attempt duration")

		if err == nil {
			// Success - record success metrics and reset failure tracking
			totalDuration := time.Since(startTime)
			metrics.IncrementCounter("signal_poll_success_total", nil, "Successful Signal polling operations")
			metrics.RecordTimer("signal_poll_total_duration", totalDuration, map[string]string{
				metricsLabelStatus: "success",
			}, "Total Signal polling operation duration")

			// Reset failure tracking on success
			sp.mu.Lock()
			sp.consecutiveFailures = 0
			sp.lastSuccessTime = time.Now()
			sp.mu.Unlock()

			return
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			sp.logger.WithFields(sp.logFields()).WithError(err).Error("Non-retryable error in Signal polling")
			metrics.IncrementCounter("signal_poll_non_retryable_errors_total", nil, "Non-retryable Signal polling errors")

			// Record as failure
			totalDuration := time.Since(startTime)
			metrics.RecordTimer("signal_poll_total_duration", totalDuration, map[string]string{
				metricsLabelStatus: "non_retryable_error",
			}, "Total Signal polling operation duration")

			return
		}

		// Record attempt failure
		metrics.IncrementCounter("signal_poll_attempt_failures_total", map[string]string{
			metricsLabelAttempt: fmt.Sprintf("%d", attempt+1),
		}, "Failed Signal polling attempts")

		// Log failure with appropriate detail level
		if verbose {
			sp.logger.WithFields(logrus.Fields{
				LogFieldAttempt:  attempt + 1,
				"error":          err,
				"backoff_ms":     backoff.Milliseconds(),
				LogFieldDuration: attemptDuration.Milliseconds(),
			}).Warn("Signal polling failed, retrying with backoff")
		} else {
			sp.logger.WithFields(sp.logFields()).Warn("Signal polling failed, retrying")
		}

		// Don't sleep on the last attempt
		if attempt < sp.retryConfig.MaxAttempts-1 {
			// Add jitter to backoff (±25% randomization)
			// #nosec G404 - Using math/rand for backoff jitter, not cryptographic purposes
			jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
			backoffWithJitter := backoff - backoff/4 + jitter

			select {
			case <-ctx.Done():
				sp.logger.WithFields(sp.logFields()).Debug("Context cancelled during backoff, stopping retry attempts")
				metrics.IncrementCounter("signal_poll_cancelled_total", nil, "Cancelled Signal polling operations")
				return
			case <-time.After(backoffWithJitter):
				// Exponential backoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}

	// All retries exhausted - record final failure
	totalDuration := time.Since(startTime)
	metrics.IncrementCounter("signal_poll_failures_total", nil, "Failed Signal polling operations (all retries exhausted)")
	metrics.RecordTimer("signal_poll_total_duration", totalDuration, map[string]string{
		metricsLabelStatus: "failure",
	}, "Total Signal polling operation duration")

	// Track consecutive failures for graceful degradation
	sp.mu.Lock()
	sp.consecutiveFailures++
	failures := sp.consecutiveFailures
	lastSuccess := sp.lastSuccessTime
	sp.mu.Unlock()

	// Alert on failure thresholds
	if failures == 10 || failures == 50 || failures == 100 {
		sp.logger.WithFields(logrus.Fields{
			"consecutive_failures": failures,
			"last_success":         lastSuccess,
			"duration_since":       time.Since(lastSuccess),
		}).Error("Signal polling experiencing persistent failures")
	}

	sp.logger.WithFields(sp.logFields()).Error("Signal polling failed after all retry attempts")
}
