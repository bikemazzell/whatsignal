package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/signal"

	"github.com/sirupsen/logrus"
)

// SignalPoller handles automatic polling of Signal messages
type SignalPoller struct {
	signalClient   signal.Client
	messageService MessageService
	config         models.SignalConfig
	retryConfig    models.RetryConfig
	logger         *logrus.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	running        bool
	mu             sync.RWMutex
}

// NewSignalPoller creates a new Signal polling service
func NewSignalPoller(signalClient signal.Client, messageService MessageService, signalConfig models.SignalConfig, retryConfig models.RetryConfig, logger *logrus.Logger) *SignalPoller {
	return &SignalPoller{
		signalClient:   signalClient,
		messageService: messageService,
		config:         signalConfig,
		retryConfig:    retryConfig,
		logger:         logger,
	}
}

// Start begins the background polling process
func (sp *SignalPoller) Start(ctx context.Context) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.running {
		return fmt.Errorf("signal poller is already running")
	}

	if !sp.config.PollingEnabled {
		sp.logger.Info("Signal polling is disabled in configuration")
		return nil
	}

	// Test connection first
	if err := sp.testConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to Signal CLI before starting poller: %w", err)
	}

	sp.ctx, sp.cancel = context.WithCancel(ctx)
	sp.running = true

	sp.wg.Add(1)
	go sp.pollLoop()

	sp.logger.WithFields(logrus.Fields{
		"interval": sp.config.PollIntervalSec,
	}).Info("Signal poller started successfully")

	return nil
}

// Stop gracefully stops the polling process
func (sp *SignalPoller) Stop() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if !sp.running {
		return
	}

	sp.logger.Info("Stopping Signal poller...")
	sp.cancel()
	sp.wg.Wait()
	sp.running = false
	sp.logger.Info("Signal poller stopped")
}

// IsRunning returns whether the poller is currently active
func (sp *SignalPoller) IsRunning() bool {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.running
}

// testConnection verifies Signal CLI is accessible
func (sp *SignalPoller) testConnection(ctx context.Context) error {
	return sp.signalClient.InitializeDevice(ctx)
}

// pollLoop runs the main polling logic with exponential backoff retry
func (sp *SignalPoller) pollLoop() {
	defer sp.wg.Done()

	ticker := time.NewTicker(time.Duration(sp.config.PollIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sp.ctx.Done():
			return
		case <-ticker.C:
			sp.pollWithRetry()
		}
	}
}

// pollWithRetry executes a single poll attempt with exponential backoff on failure
func (sp *SignalPoller) pollWithRetry() {
	ctx, cancel := context.WithTimeout(sp.ctx, 30*time.Second)
	defer cancel()

	backoff := time.Duration(sp.retryConfig.InitialBackoffMs) * time.Millisecond
	maxBackoff := time.Duration(sp.retryConfig.MaxBackoffMs) * time.Millisecond

	for attempt := 0; attempt < sp.retryConfig.MaxAttempts; attempt++ {
		err := sp.messageService.PollSignalMessages(ctx)
		if err == nil {
			// Success - reset for next poll cycle
			return
		}

		if IsVerboseLogging(ctx) {
			sp.logger.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"error":   err,
				"backoff": backoff,
			}).Warn("Signal polling failed, retrying with backoff")
		} else {
			sp.logger.Warn("Signal polling failed, retrying")
		}

		// Don't sleep on the last attempt
		if attempt < sp.retryConfig.MaxAttempts-1 {
			select {
			case <-sp.ctx.Done():
				return
			case <-time.After(backoff):
				// Exponential backoff with jitter
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}

	sp.logger.Error("Signal polling failed after all retry attempts")
}