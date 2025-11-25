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
	waClient               types.WAClient
	logger                 *logrus.Logger
	checkInterval          time.Duration
	startupTimeout         time.Duration
	sessionStateTimestamps map[string]time.Time // Track when sessions entered their current state
	lastKnownStatus        map[string]string    // Track last known status for each session
	sessionName            string               // Cached session name
	mu                     sync.Mutex
	running                bool
	stopCh                 chan struct{}
	unhealthyStatusSet     map[string]struct{} // Pre-computed set for O(1) lookup
}

// NewSessionMonitor creates a new session monitor
func NewSessionMonitor(waClient types.WAClient, logger *logrus.Logger, checkInterval time.Duration) *SessionMonitor {
	return NewSessionMonitorWithStartupTimeout(waClient, logger, checkInterval, 0)
}

// NewSessionMonitorWithStartupTimeout creates a new session monitor with custom startup timeout
func NewSessionMonitorWithStartupTimeout(waClient types.WAClient, logger *logrus.Logger, checkInterval time.Duration, startupTimeout time.Duration) *SessionMonitor {
	if checkInterval <= 0 {
		checkInterval = time.Duration(constants.DefaultSessionHealthCheckSec) * time.Second
	}
	if startupTimeout <= 0 {
		startupTimeout = time.Duration(constants.DefaultSessionStartupTimeoutSec) * time.Second
	}

	// Pre-compute unhealthy status set for O(1) lookup
	unhealthyStatusSet := map[string]struct{}{
		"OPENING":      {},
		"STOPPED":      {},
		"FAILED":       {},
		"error":        {},
		"disconnected": {},
	}

	return &SessionMonitor{
		waClient:               waClient,
		logger:                 logger,
		checkInterval:          checkInterval,
		startupTimeout:         startupTimeout,
		sessionStateTimestamps: make(map[string]time.Time),
		lastKnownStatus:        make(map[string]string),
		sessionName:            waClient.GetSessionName(),
		stopCh:                 make(chan struct{}),
		unhealthyStatusSet:     unhealthyStatusSet,
	}
}

// Start begins monitoring the session
func (sm *SessionMonitor) Start(ctx context.Context) {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		sm.logger.Warn("Session monitor is already running")
		return
	}

	// Reinitialize stopCh if it was closed
	if sm.stopCh == nil {
		sm.stopCh = make(chan struct{})
	}

	sm.running = true
	sm.mu.Unlock()

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
	initDelay := time.NewTimer(time.Duration(constants.DefaultSessionMonitorInitDelaySec) * time.Second)
	defer initDelay.Stop()

	select {
	case <-ctx.Done():
		return
	case <-sm.getStopCh():
		return
	case <-initDelay.C:
		// Continue to main loop
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.getStopCh():
			return
		case <-ticker.C:
			sm.checkAndRecoverSession(ctx)
		}
	}
}

// getStopCh safely retrieves the stop channel
func (sm *SessionMonitor) getStopCh() <-chan struct{} {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.stopCh == nil {
		// Return a closed channel to prevent blocking
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return sm.stopCh
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

	// Update state tracking and check if session is stuck in STARTING
	stuckInStarting, startingDuration := sm.updateAndCheckStartingTimeout(sm.sessionName, status)

	// Check if session is stuck in STARTING status (check this first)
	if stuckInStarting {
		sm.logger.WithFields(logrus.Fields{
			"status":   status,
			"duration": startingDuration.Seconds(),
			"timeout":  sm.startupTimeout.Seconds(),
		}).Warn("Session stuck in STARTING status, attempting restart")

		sm.handleSessionRestart(ctx, sm.sessionName, "STARTING timeout")
		return
	}

	// Check if session is in a bad state
	if sm.isSessionUnhealthy(status) {
		sm.logger.WithField("status", status).Warn("Session is in unhealthy state, attempting restart")
		sm.handleSessionRestart(ctx, sm.sessionName, "unhealthy state")
	}
}

// handleSessionRestart encapsulates the restart logic to avoid duplication
func (sm *SessionMonitor) handleSessionRestart(ctx context.Context, sessionName, reason string) {
	if err := sm.restartSession(ctx); err != nil {
		sm.logger.WithError(err).WithField("reason", reason).Error("Failed to restart session")
	} else {
		sm.logger.WithField("reason", reason).Info("Session restart initiated successfully")
		sm.resetSessionTracking(sessionName)
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
	// Check pre-computed set for O(1) lookup
	if _, exists := sm.unhealthyStatusSet[status]; exists {
		return true
	}

	// STARTING is handled separately, so don't treat it as unhealthy here
	if status == "STARTING" {
		return false
	}

	// Session is considered healthy if it's WORKING
	return status != "WORKING"
}

// updateAndCheckStartingTimeout updates state tracking and checks if session is stuck in STARTING
// Returns (isStuck, duration) where isStuck indicates if timeout exceeded
func (sm *SessionMonitor) updateAndCheckStartingTimeout(sessionName, currentStatus string) (bool, time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get last known status
	lastStatus, exists := sm.lastKnownStatus[sessionName]

	// If status changed or first time seeing this session, update timestamp
	if !exists || lastStatus != currentStatus {
		sm.sessionStateTimestamps[sessionName] = time.Now()
		sm.lastKnownStatus[sessionName] = currentStatus
		return false, 0 // Not stuck, just transitioned
	}

	// Status hasn't changed - check if we're stuck in STARTING
	if currentStatus != "STARTING" {
		return false, 0 // Not in STARTING status
	}

	// Check how long we've been in STARTING state
	timestamp := sm.sessionStateTimestamps[sessionName]
	duration := time.Since(timestamp)

	// Return whether we've exceeded the timeout
	return duration > sm.startupTimeout, duration
}

// resetSessionTracking clears tracking data for a session (e.g., after restart)
func (sm *SessionMonitor) resetSessionTracking(sessionName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessionStateTimestamps, sessionName)
	delete(sm.lastKnownStatus, sessionName)
}

func (sm *SessionMonitor) restartSession(ctx context.Context) error {
	// Create a single context for the entire restart operation
	// Use the longer of the two timeouts to ensure we don't cut off prematurely
	restartTimeout := time.Duration(constants.DefaultSessionRestartTimeoutSec) * time.Second
	waitTimeout := time.Duration(constants.DefaultSessionWaitTimeoutSec) * time.Second
	totalTimeout := restartTimeout + waitTimeout

	restartCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	// Restart the session
	if err := sm.waClient.RestartSession(restartCtx); err != nil {
		return err
	}

	// Wait for session to be ready after restart
	return sm.waClient.WaitForSessionReady(restartCtx, waitTimeout)
}
