package service

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestScheduler_RunCleanup(t *testing.T) {
	mockBridge := &mockBridge{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	scheduler := NewScheduler(mockBridge, 30, 24, logger)

	ctx := context.Background()

	mockBridge.On("CleanupOldRecords", ctx, 30).Return(nil).Once()

	scheduler.runCleanup(ctx)

	mockBridge.AssertExpectations(t)
}

func TestScheduler_RunCleanupError(t *testing.T) {
	mockBridge := &mockBridge{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	scheduler := NewScheduler(mockBridge, 30, 24, logger)

	ctx := context.Background()

	mockBridge.On("CleanupOldRecords", ctx, 30).Return(assert.AnError).Once()

	scheduler.runCleanup(ctx)

	mockBridge.AssertExpectations(t)
}

func TestScheduler_StartStop(t *testing.T) {
	mockBridge := &mockBridge{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	scheduler := NewScheduler(mockBridge, 30, 24, logger)

	ctx, cancel := context.WithCancel(context.Background())

	cleanupStarted := make(chan struct{}, 1)
	mockBridge.On("CleanupOldRecords", mock.Anything, 30).
		Run(func(mock.Arguments) {
			cleanupStarted <- struct{}{}
		}).
		Return(nil).
		Maybe()

	done := make(chan struct{})
	go func() {
		scheduler.Start(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-cleanupStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Scheduler did not stop within timeout")
	}
}

func TestScheduler_StopSignal(t *testing.T) {
	mockBridge := &mockBridge{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	scheduler := NewScheduler(mockBridge, 30, 24, logger)

	ctx := context.Background()

	cleanupStarted := make(chan struct{}, 1)
	mockBridge.On("CleanupOldRecords", mock.Anything, 30).
		Run(func(mock.Arguments) {
			cleanupStarted <- struct{}{}
		}).
		Return(nil).
		Maybe()

	done := make(chan struct{})
	go func() {
		scheduler.Start(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-cleanupStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	scheduler.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Scheduler did not stop within timeout")
	}
}

func TestScheduler_StopIsIdempotent(t *testing.T) {
	mockBridge := &mockBridge{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	scheduler := NewScheduler(mockBridge, 30, 24, logger)
	cleanupStarted := make(chan struct{}, 1)
	mockBridge.On("CleanupOldRecords", mock.Anything, 30).
		Run(func(mock.Arguments) {
			cleanupStarted <- struct{}{}
		}).
		Return(nil).
		Maybe()

	done := make(chan struct{})
	go func() {
		scheduler.Start(context.Background())
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-cleanupStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	scheduler.Stop()
	scheduler.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Scheduler did not stop within timeout")
	}
}
