package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/models"

	"github.com/sirupsen/logrus"
)

// ContainerRestarter defines the interface for restarting WAHA containers
type ContainerRestarter interface {
	RestartContainer(ctx context.Context) error
}

// WebhookContainerRestarter restarts containers via webhook
type WebhookContainerRestarter struct {
	webhookURL    string
	containerName string
	client        *http.Client
	logger        *logrus.Logger
}

// NewWebhookContainerRestarter creates a new webhook-based container restarter
func NewWebhookContainerRestarter(config models.ContainerRestartConfig, logger *logrus.Logger) *WebhookContainerRestarter {
	timeout := time.Duration(constants.DefaultContainerRestartWebhookTimeoutSec) * time.Second
	return &WebhookContainerRestarter{
		webhookURL:    config.WebhookURL,
		containerName: config.ContainerName,
		client:        &http.Client{Timeout: timeout},
		logger:        logger,
	}
}

// RestartContainer sends a webhook request to restart the container
func (w *WebhookContainerRestarter) RestartContainer(ctx context.Context) error {
	if w.webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	payload := map[string]string{
		"action":         "restart",
		"container_name": w.containerName,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "whatsignal-container-restarter")

	w.logger.WithFields(logrus.Fields{
		"webhook_url":    w.webhookURL,
		"container_name": w.containerName,
	}).Info("Sending container restart webhook request")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-success status: %d", resp.StatusCode)
	}

	w.logger.WithFields(logrus.Fields{
		"status_code":    resp.StatusCode,
		"container_name": w.containerName,
	}).Info("Container restart webhook request successful")

	return nil
}

// NoOpContainerRestarter is a no-op implementation (when feature is disabled)
type NoOpContainerRestarter struct{}

// NewNoOpContainerRestarter creates a new no-op container restarter
func NewNoOpContainerRestarter() *NoOpContainerRestarter {
	return &NoOpContainerRestarter{}
}

// RestartContainer does nothing
func (n *NoOpContainerRestarter) RestartContainer(ctx context.Context) error {
	return nil
}
