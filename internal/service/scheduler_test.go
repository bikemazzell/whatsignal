package service

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	mockBridge.On("CleanupOldRecords", mock.Anything, 30).Return(nil).Maybe()

	done := make(chan struct{})
	go func() {
		scheduler.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

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

	mockBridge.On("CleanupOldRecords", mock.Anything, 30).Return(nil).Maybe()

	done := make(chan struct{})
	go func() {
		scheduler.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	scheduler.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Scheduler did not stop within timeout")
	}
}
