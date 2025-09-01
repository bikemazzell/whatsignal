package service

import (
	"context"
	"sync"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

// SessionMonitor monitors WhatsApp session health and restarts it when needed
type SessionMonitor struct {
	waClient      types.WAClient
	logger        *logrus.Logger
	checkInterval time.Duration
	mu            sync.Mutex
	running       bool
	stopCh        chan struct{}
}

// NewSessionMonitor creates a new session monitor
func NewSessionMonitor(waClient types.WAClient, logger *logrus.Logger, checkInterval time.Duration) *SessionMonitor {
	if checkInterval <= 0 {
		checkInterval = time.Duration(constants.DefaultSessionHealthCheckSec) * time.Second
	}
	return &SessionMonitor{
		waClient:      waClient,
		logger:        logger,
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start begins monitoring the session
func (sm *SessionMonitor) Start(ctx context.Context) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		sm.logger.Warn("Session monitor is already running")
		return
	}

	// Reinitialize stopCh if it was closed
	if sm.stopCh == nil {
		sm.stopCh = make(chan struct{})
	}

	sm.running = true
	go sm.monitorLoop(ctx)
	sm.logger.Info("Session monitor started")
}

// Stop stops monitoring the session
func (sm *SessionMonitor) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return
	}

	if sm.stopCh != nil {
		close(sm.stopCh)
		sm.stopCh = nil
	}
	sm.running = false
	sm.logger.Info("Session monitor stopped")
}

func (sm *SessionMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(sm.checkInterval)
	defer ticker.Stop()

	// Initial check after a short delay
	time.Sleep(time.Duration(constants.DefaultSessionMonitorInitDelaySec) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.checkAndRecoverSession(ctx)
		}
	}
}

func (sm *SessionMonitor) checkAndRecoverSession(ctx context.Context) {
	// Create a timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(constants.DefaultHTTPTimeoutSec)*time.Second)
	defer cancel()

	// Get current session status
	status, err := sm.getSessionStatusFromAPI(checkCtx)
	if err != nil {
		sm.logger.WithError(err).Error("Failed to get session status")
		return
	}

	sm.logger.WithField("status", status).Debug("Session status check")

	// Check if session is in a bad state
	if sm.isSessionUnhealthy(status) {
		sm.logger.WithField("status", status).Warn("Session is in unhealthy state, attempting restart")

		// Try to restart the session
		if err := sm.restartSession(ctx); err != nil {
			sm.logger.WithError(err).Error("Failed to restart session")
		} else {
			sm.logger.Info("Session restart initiated successfully")
		}
	}
}

func (sm *SessionMonitor) getSessionStatusFromAPI(ctx context.Context) (string, error) {
	// Use the same method as WaitForSessionReady to get the actual WAHA status
	session, err := sm.waClient.GetSessionStatus(ctx)
	if err != nil {
		return "", err
	}
	return string(session.Status), nil
}

func (sm *SessionMonitor) isSessionUnhealthy(status string) bool {
	// WAHA statuses that indicate problems
	unhealthyStatuses := []string{
		"OPENING",      // Stuck in opening state
		"STOPPED",      // Session stopped
		"FAILED",       // Session failed
		"error",        // Error state
		"disconnected", // Disconnected from WhatsApp
	}

	for _, unhealthy := range unhealthyStatuses {
		if status == unhealthy {
			return true
		}
	}

	// Session is considered healthy if it's WORKING
	return status != "WORKING"
}

func (sm *SessionMonitor) restartSession(ctx context.Context) error {
	// Use the WAHA restart endpoint
	restartCtx, restartCancel := context.WithTimeout(ctx, time.Duration(constants.DefaultSessionRestartTimeoutSec)*time.Second)
	defer restartCancel()

	if err := sm.waClient.RestartSession(restartCtx); err != nil {
		return err
	}

	// Wait for session to be ready after restart
	waitTimeout := time.Duration(constants.DefaultSessionWaitTimeoutSec) * time.Second
	waitCtx, waitCancel := context.WithTimeout(ctx, waitTimeout)
	defer waitCancel()

	return sm.waClient.WaitForSessionReady(waitCtx, waitTimeout)
}
