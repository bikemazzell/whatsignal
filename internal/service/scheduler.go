package service

import (
	"context"
	"time"

	"whatsignal/internal/constants"

	"github.com/sirupsen/logrus"
)

type Scheduler struct {
	cleaner       RecordCleaner
	retentionDays int
	intervalHours int
	logger        *logrus.Logger
	stopCh        chan struct{}
}

// NewScheduler creates a new cleanup scheduler.
// It accepts RecordCleaner (rather than full MessageBridge) following the Interface Segregation Principle.
func NewScheduler(cleaner RecordCleaner, retentionDays, intervalHours int, logger *logrus.Logger) *Scheduler {
	if intervalHours <= 0 {
		intervalHours = constants.CleanupSchedulerIntervalHours
	}
	return &Scheduler{
		cleaner:       cleaner,
		retentionDays: retentionDays,
		intervalHours: intervalHours,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.intervalHours) * time.Hour)
	defer ticker.Stop()

	s.logger.Info("Starting cleanup scheduler")

	s.runCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler context cancelled, stopping")
			return
		case <-s.stopCh:
			s.logger.Info("Scheduler stop signal received, stopping")
			return
		case <-ticker.C:
			s.runCleanup(ctx)
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) runCleanup(ctx context.Context) {
	s.logger.WithField("retentionDays", s.retentionDays).Info("Running scheduled cleanup")

	if err := s.cleaner.CleanupOldRecords(ctx, s.retentionDays); err != nil {
		s.logger.WithError(err).Error("Failed to cleanup old records")
	} else {
		s.logger.Info("Successfully completed cleanup")
	}
}
