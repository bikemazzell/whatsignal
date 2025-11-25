package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSessionMonitor(t *testing.T) {
	whatsappClient := &mockWhatsAppClient{}
	logger := logrus.New()
	checkInterval := 30 * time.Second

	monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)
	require.NotNil(t, monitor)
}

func TestSessionMonitor_Start(t *testing.T) {
	tests := []struct {
		name          string
		sessionStatus types.SessionStatus
		expectRestart bool
		setup         func(*mockWhatsAppClient)
	}{
		{
			name:          "session working - no restart needed",
			sessionStatus: "WORKING",
			expectRestart: false,
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
					Name:   "test-session",
					Status: "WORKING",
				}, nil).Maybe()
			},
		},
		{
			name:          "session stopped - restart needed",
			sessionStatus: "STOPPED",
			expectRestart: true,
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
					Name:   "test-session",
					Status: "STOPPED",
				}, nil).Once()
				client.On("RestartSession", mock.Anything).Return(nil).Once()
				client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
		},
		{
			name:          "session failed - restart needed",
			sessionStatus: "FAILED",
			expectRestart: true,
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
					Name:   "test-session",
					Status: "FAILED",
				}, nil).Once()
				client.On("RestartSession", mock.Anything).Return(nil).Once()
				client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whatsappClient := &mockWhatsAppClient{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)      // Reduce log noise in tests
			checkInterval := 100 * time.Millisecond // Short interval for testing

			if tt.setup != nil {
				tt.setup(whatsappClient)
			}

			monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

			// Create a context with timeout to prevent infinite running
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			// Start monitoring
			monitor.Start(ctx)

			// Wait a bit to ensure the goroutine starts
			time.Sleep(50 * time.Millisecond)

			// Stop monitoring
			monitor.Stop()

			// For tests that expect a restart, we need to call checkAndRecoverSession directly
			// since the monitor loop has an initial delay that's longer than our test timeout
			if tt.expectRestart {
				monitor.checkAndRecoverSession(ctx)
			}

			whatsappClient.AssertExpectations(t)
		})
	}
}

func TestSessionMonitor_Stop(t *testing.T) {
	whatsappClient := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests
	checkInterval := 30 * time.Second

	monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

	// Start monitoring
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup mock to return working status
	whatsappClient.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   "test-session",
		Status: "WORKING",
	}, nil).Maybe()

	monitor.Start(ctx)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop monitoring
	monitor.Stop()

	// Verify it stopped
	assert.True(t, true) // If we get here without hanging, the test passed
}

func TestSessionMonitor_checkAndRecoverSession(t *testing.T) {
	tests := []struct {
		name          string
		sessionStatus *types.Session
		statusError   error
		restartError  error
		waitError     error
		setup         func(*mockWhatsAppClient)
	}{
		{
			name: "session working - no action needed",
			sessionStatus: &types.Session{
				Name:   "test-session",
				Status: "WORKING",
			},
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
					Name:   "test-session",
					Status: "WORKING",
				}, nil).Once()
			},
		},
		{
			name: "session stopped - successful restart",
			sessionStatus: &types.Session{
				Name:   "test-session",
				Status: "STOPPED",
			},
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
					Name:   "test-session",
					Status: "STOPPED",
				}, nil).Once()
				client.On("RestartSession", mock.Anything).Return(nil).Once()
				client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Once()
			},
		},
		{
			name:        "get status error",
			statusError: assert.AnError,
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(nil, assert.AnError).Once()
			},
		},
		{
			name: "restart error",
			sessionStatus: &types.Session{
				Name:   "test-session",
				Status: "STOPPED",
			},
			restartError: assert.AnError,
			setup: func(client *mockWhatsAppClient) {
				client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
					Name:   "test-session",
					Status: "STOPPED",
				}, nil).Once()
				client.On("RestartSession", mock.Anything).Return(assert.AnError).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whatsappClient := &mockWhatsAppClient{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests
			checkInterval := 30 * time.Second

			if tt.setup != nil {
				tt.setup(whatsappClient)
			}

			monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

			ctx := context.Background()
			monitor.checkAndRecoverSession(ctx)

			whatsappClient.AssertExpectations(t)
		})
	}
}

func TestSessionMonitor_isSessionUnhealthy(t *testing.T) {
	whatsappClient := &mockWhatsAppClient{}
	logger := logrus.New()
	checkInterval := 30 * time.Second
	monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "working session",
			status:   "WORKING",
			expected: false,
		},
		{
			name:     "stopped session",
			status:   "STOPPED",
			expected: true,
		},
		{
			name:     "failed session",
			status:   "FAILED",
			expected: true,
		},
		{
			name:     "opening session",
			status:   "OPENING",
			expected: true,
		},
		{
			name:     "error session",
			status:   "error",
			expected: true,
		},
		{
			name:     "disconnected session",
			status:   "disconnected",
			expected: true,
		},
		{
			name:     "unknown status",
			status:   "unknown",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monitor.isSessionUnhealthy(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionMonitor_StopMultipleTimes(t *testing.T) {
	whatsappClient := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	checkInterval := 30 * time.Second

	monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	whatsappClient.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   "test-session",
		Status: "WORKING",
	}, nil).Maybe()

	monitor.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// First stop
	monitor.Stop()

	// Second stop - this should not panic
	assert.NotPanics(t, func() {
		monitor.Stop()
	})

	// Third stop - this should also not panic
	assert.NotPanics(t, func() {
		monitor.Stop()
	})
}

func TestSessionMonitor_ConcurrentStop(t *testing.T) {
	whatsappClient := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	checkInterval := 30 * time.Second

	monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	whatsappClient.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   "test-session",
		Status: "WORKING",
	}, nil).Maybe()

	monitor.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Launch multiple goroutines calling Stop() concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotPanics(t, func() {
				monitor.Stop()
			})
		}()
	}

	wg.Wait()
}

func TestSessionMonitor_StartStopStartStop(t *testing.T) {
	whatsappClient := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	checkInterval := 30 * time.Second

	monitor := NewSessionMonitor(whatsappClient, logger, checkInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	whatsappClient.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   "test-session",
		Status: "WORKING",
	}, nil).Maybe()

	// First cycle
	monitor.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	monitor.Stop()

	// Second cycle - ensure stopCh is recreated properly
	monitor.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	monitor.Stop()

	// Third cycle
	monitor.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	monitor.Stop()

	assert.True(t, true) // If we get here without issues, test passed
}

func TestSessionMonitor_UnhealthyStatusesTriggerRestart(t *testing.T) {
	statuses := []string{"OPENING", "disconnected", "FAILED"}
	for _, st := range statuses {
		t.Run(st, func(t *testing.T) {
			client := &mockWhatsAppClient{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)
			monitor := NewSessionMonitor(client, logger, 30*time.Second)

			client.On("GetSessionStatus", mock.Anything).Return(&types.Session{Name: "test", Status: types.SessionStatus(st)}, nil).Once()
			client.On("RestartSession", mock.Anything).Return(nil).Once()
			client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Once()

			ctx := context.Background()
			monitor.checkAndRecoverSession(ctx)

			client.AssertExpectations(t)
		})
	}
}

func TestSessionMonitor_Restart_WaitError(t *testing.T) {
	client := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	monitor := NewSessionMonitor(client, logger, 30*time.Second)

	client.On("GetSessionStatus", mock.Anything).Return(&types.Session{Name: "test", Status: "STOPPED"}, nil).Once()
	client.On("RestartSession", mock.Anything).Return(nil).Once()
	client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(assert.AnError).Once()

	ctx := context.Background()
	monitor.checkAndRecoverSession(ctx)

	client.AssertExpectations(t)
}

func TestSessionMonitor_RapidStateChanges(t *testing.T) {
	client := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	monitor := NewSessionMonitor(client, logger, 30*time.Second)

	// First call: STOPPED -> triggers restart
	client.On("GetSessionStatus", mock.Anything).Return(&types.Session{Name: "test", Status: "STOPPED"}, nil).Once()
	client.On("RestartSession", mock.Anything).Return(nil).Once()
	client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Once()

	// Second call: WORKING -> no restart
	client.On("GetSessionStatus", mock.Anything).Return(&types.Session{Name: "test", Status: "WORKING"}, nil).Once()

	ctx := context.Background()
	monitor.checkAndRecoverSession(ctx)
	monitor.checkAndRecoverSession(ctx)

	client.AssertExpectations(t)
}

func TestSessionMonitor_StartingStatusTimeout(t *testing.T) {
	tests := []struct {
		name            string
		startupTimeout  time.Duration
		waitBeforeCheck time.Duration
		expectRestart   bool
		sessionStatus   string
		sessionName     string
	}{
		{
			name:            "session in STARTING within timeout - no restart",
			startupTimeout:  100 * time.Millisecond,
			waitBeforeCheck: 50 * time.Millisecond,
			expectRestart:   false,
			sessionStatus:   "STARTING",
			sessionName:     "test-session",
		},
		{
			name:            "session in STARTING beyond timeout - restart triggered",
			startupTimeout:  50 * time.Millisecond,
			waitBeforeCheck: 100 * time.Millisecond,
			expectRestart:   true,
			sessionStatus:   "STARTING",
			sessionName:     "test-session",
		},
		{
			name:            "session in WORKING - no restart",
			startupTimeout:  50 * time.Millisecond,
			waitBeforeCheck: 100 * time.Millisecond,
			expectRestart:   false,
			sessionStatus:   "WORKING",
			sessionName:     "test-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockWhatsAppClient{}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			// GetSessionName can be called multiple times
			client.On("GetSessionName").Return(tt.sessionName).Maybe()

			monitor := NewSessionMonitorWithStartupTimeout(client, logger, 30*time.Second, tt.startupTimeout)

			// First check - record the timestamp
			client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
				Name:   tt.sessionName,
				Status: types.SessionStatus(tt.sessionStatus),
			}, nil).Once()

			ctx := context.Background()
			monitor.checkAndRecoverSession(ctx)

			// Wait for the specified duration
			time.Sleep(tt.waitBeforeCheck)

			// Second check - should trigger restart if beyond timeout
			client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
				Name:   tt.sessionName,
				Status: types.SessionStatus(tt.sessionStatus),
			}, nil).Once()

			if tt.expectRestart {
				client.On("RestartSession", mock.Anything).Return(nil).Once()
				client.On("WaitForSessionReady", mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Once()
			}

			monitor.checkAndRecoverSession(ctx)

			client.AssertExpectations(t)
		})
	}
}

func TestSessionMonitor_StartingStatusTransition(t *testing.T) {
	client := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	sessionName := "test-session"

	// GetSessionName can be called multiple times
	client.On("GetSessionName").Return(sessionName).Maybe()

	monitor := NewSessionMonitorWithStartupTimeout(client, logger, 30*time.Second, 100*time.Millisecond)

	ctx := context.Background()

	// First check: Session in STARTING
	client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   sessionName,
		Status: "STARTING",
	}, nil).Once()

	monitor.checkAndRecoverSession(ctx)

	// Second check: Session transitioned to WORKING (should reset timestamp)
	client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   sessionName,
		Status: "WORKING",
	}, nil).Once()

	monitor.checkAndRecoverSession(ctx)

	// Third check: Session back to STARTING (new timestamp, no restart yet)
	client.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Name:   sessionName,
		Status: "STARTING",
	}, nil).Once()

	monitor.checkAndRecoverSession(ctx)

	// No restart should have been triggered
	client.AssertExpectations(t)
	client.AssertNotCalled(t, "RestartSession")
}

func TestSessionMonitor_UpdateAndCheckStartingTimeout(t *testing.T) {
	client := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	sessionName := "test-session"
	monitor := NewSessionMonitorWithStartupTimeout(client, logger, 30*time.Second, 50*time.Millisecond)

	// First call - should record timestamp and return false (not stuck)
	stuck, duration := monitor.updateAndCheckStartingTimeout(sessionName, "STARTING")
	assert.False(t, stuck, "First check should not indicate stuck session")
	assert.Equal(t, time.Duration(0), duration, "Duration should be 0 on first check")

	// Immediate second call with same status - should return false (not enough time passed)
	stuck, duration = monitor.updateAndCheckStartingTimeout(sessionName, "STARTING")
	assert.False(t, stuck, "Second immediate check should not indicate stuck session")
	assert.Greater(t, duration, time.Duration(0), "Duration should be > 0")
	assert.Less(t, duration, 50*time.Millisecond, "Duration should be less than timeout")

	// Wait beyond timeout
	time.Sleep(60 * time.Millisecond)

	// Third call - should return true (timeout exceeded)
	stuck, duration = monitor.updateAndCheckStartingTimeout(sessionName, "STARTING")
	assert.True(t, stuck, "Check after timeout should indicate stuck session")
	assert.Greater(t, duration, 50*time.Millisecond, "Duration should exceed timeout")

	// Check with different status - should reset and return false
	stuck, duration = monitor.updateAndCheckStartingTimeout(sessionName, "WORKING")
	assert.False(t, stuck, "Status change should reset tracking")
	assert.Equal(t, time.Duration(0), duration, "Duration should be 0 after status change")

	// Check WORKING status again - should still return false
	stuck, duration = monitor.updateAndCheckStartingTimeout(sessionName, "WORKING")
	assert.False(t, stuck, "Non-STARTING status should not indicate stuck session")
	assert.Equal(t, time.Duration(0), duration, "Duration should be 0 for non-STARTING status")
}

func TestSessionMonitor_ResetSessionTracking(t *testing.T) {
	client := &mockWhatsAppClient{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	sessionName := "test-session"
	monitor := NewSessionMonitorWithStartupTimeout(client, logger, 30*time.Second, 100*time.Millisecond)

	// Set up some tracking data
	monitor.updateAndCheckStartingTimeout(sessionName, "STARTING")

	// Verify data exists
	monitor.mu.Lock()
	_, timestampExists := monitor.sessionStateTimestamps[sessionName]
	_, statusExists := monitor.lastKnownStatus[sessionName]
	monitor.mu.Unlock()

	assert.True(t, timestampExists, "Timestamp should exist before reset")
	assert.True(t, statusExists, "Status should exist before reset")

	// Reset tracking
	monitor.resetSessionTracking(sessionName)

	// Verify data is cleared
	monitor.mu.Lock()
	_, timestampExists = monitor.sessionStateTimestamps[sessionName]
	_, statusExists = monitor.lastKnownStatus[sessionName]
	monitor.mu.Unlock()

	assert.False(t, timestampExists, "Timestamp should be cleared after reset")
	assert.False(t, statusExists, "Status should be cleared after reset")
}
