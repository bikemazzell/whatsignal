package service

import (
	"context"
	"time"

	"whatsignal/internal/metrics"

	"github.com/sirupsen/logrus"
)

type StaleMessageCounter interface {
	GetStaleMessageCount(ctx context.Context, threshold time.Duration) (int, error)
}

type DeliveryMonitor struct {
	db             StaleMessageCounter
	checkInterval  time.Duration
	staleThreshold time.Duration
	logger         *logrus.Logger
	stopCh         chan struct{}
}

func NewDeliveryMonitor(db StaleMessageCounter, checkInterval, staleThreshold time.Duration, logger *logrus.Logger) *DeliveryMonitor {
	return &DeliveryMonitor{
		db:             db,
		checkInterval:  checkInterval,
		staleThreshold: staleThreshold,
		logger:         logger,
		stopCh:         make(chan struct{}),
	}
}

func (m *DeliveryMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	m.logger.WithFields(logrus.Fields{
		"check_interval":  m.checkInterval,
		"stale_threshold": m.staleThreshold,
	}).Info("Starting delivery monitor")

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkStaleMessages(ctx)
		}
	}
}

func (m *DeliveryMonitor) Stop() {
	close(m.stopCh)
}

func (m *DeliveryMonitor) checkStaleMessages(ctx context.Context) {
	count, err := m.db.GetStaleMessageCount(ctx, m.staleThreshold)
	if err != nil {
		m.logger.WithError(err).Error("Failed to check for stale messages")
		return
	}
	metrics.SetGauge("delivery_stale_messages", float64(count), nil, "Messages stuck in sent status")
	if count > 0 {
		m.logger.WithFields(logrus.Fields{
			"stale_count": count,
			"threshold":   m.staleThreshold,
		}).Warn("Messages stuck in 'sent' status without delivery confirmation")
	}
}
