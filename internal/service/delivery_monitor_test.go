package service

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type signalingStaleMessageCounter struct {
	checked chan struct{}
}

func (s signalingStaleMessageCounter) GetStaleMessageCount(context.Context, time.Duration) (int, error) {
	select {
	case s.checked <- struct{}{}:
	default:
	}
	return 0, nil
}

func TestDeliveryMonitor_StopIsIdempotent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	checked := make(chan struct{}, 1)
	monitor := NewDeliveryMonitor(signalingStaleMessageCounter{checked: checked}, time.Millisecond, time.Minute, logger)

	done := make(chan struct{})
	go func() {
		monitor.Start(context.Background())
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-checked:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	monitor.Stop()
	monitor.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Delivery monitor did not stop within timeout")
	}
}
