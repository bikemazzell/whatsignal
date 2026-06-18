package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDeliveryMonitor(t *testing.T) {
	db := &mockDatabaseService{}
	logger := logrus.New()

	monitor := NewDeliveryMonitor(db, 30*time.Second, 5*time.Minute, logger)

	require.NotNil(t, monitor)
	assert.Equal(t, db, monitor.db)
	assert.Equal(t, 30*time.Second, monitor.checkInterval)
	assert.Equal(t, 5*time.Minute, monitor.staleThreshold)
	assert.Equal(t, logger, monitor.logger)
	assert.NotNil(t, monitor.stopCh)
	assert.Equal(t, 0, monitor.lastStaleCount)
}

func TestDeliveryMonitor_checkStaleMessages(t *testing.T) {
	tests := []struct {
		name              string
		setup             func(*mockDatabaseService)
		priorCount        int
		expectedLastCount int
	}{
		{
			name: "success with increasing count warns",
			setup: func(db *mockDatabaseService) {
				db.On("GetStaleMessageCount", mock.Anything, 5*time.Minute).Return(5, nil).Once()
			},
			priorCount:        2,
			expectedLastCount: 5,
		},
		{
			name: "success with decreasing count debugs",
			setup: func(db *mockDatabaseService) {
				db.On("GetStaleMessageCount", mock.Anything, 5*time.Minute).Return(2, nil).Once()
			},
			priorCount:        5,
			expectedLastCount: 2,
		},
		{
			name: "db error does not update lastStaleCount",
			setup: func(db *mockDatabaseService) {
				db.On("GetStaleMessageCount", mock.Anything, 5*time.Minute).Return(0, errors.New("db failure")).Once()
			},
			priorCount:        3,
			expectedLastCount: 3,
		},
		{
			name: "zero count no warning",
			setup: func(db *mockDatabaseService) {
				db.On("GetStaleMessageCount", mock.Anything, 5*time.Minute).Return(0, nil).Once()
			},
			priorCount:        0,
			expectedLastCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mockDatabaseService{}
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)

			tt.setup(db)

			monitor := NewDeliveryMonitor(db, time.Minute, 5*time.Minute, logger)
			monitor.lastStaleCount = tt.priorCount

			monitor.checkStaleMessages(context.Background())

			assert.Equal(t, tt.expectedLastCount, monitor.lastStaleCount)
			db.AssertExpectations(t)
		})
	}
}

func TestDeliveryMonitor_StartStop(t *testing.T) {
	db := &mockDatabaseService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	db.On("GetStaleMessageCount", mock.Anything, 5*time.Minute).Return(0, nil)

	monitor := NewDeliveryMonitor(db, 50*time.Millisecond, 5*time.Minute, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return len(db.Calls) > 0
	}, 2*time.Second, 10*time.Millisecond)

	monitor.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop")
	}

	db.AssertExpectations(t)
}

func TestDeliveryMonitor_StopViaContext(t *testing.T) {
	db := &mockDatabaseService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	db.On("GetStaleMessageCount", mock.Anything, 5*time.Minute).Return(0, nil)

	monitor := NewDeliveryMonitor(db, 50*time.Millisecond, 5*time.Minute, logger)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		monitor.Start(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return len(db.Calls) > 0
	}, 2*time.Second, 10*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	db.AssertExpectations(t)
}
