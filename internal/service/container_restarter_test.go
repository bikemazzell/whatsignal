package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"whatsignal/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookContainerRestarter_Success(t *testing.T) {
	// Create a test server that simulates the webhook endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "whatsignal-container-restarter", r.Header.Get("User-Agent"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := models.ContainerRestartConfig{
		WebhookURL:    server.URL,
		ContainerName: "waha",
	}

	restarter := NewWebhookContainerRestarter(config, logger)

	ctx := context.Background()
	err := restarter.RestartContainer(ctx)

	require.NoError(t, err)
}

func TestWebhookContainerRestarter_Failure(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := models.ContainerRestartConfig{
		WebhookURL:    server.URL,
		ContainerName: "waha",
	}

	restarter := NewWebhookContainerRestarter(config, logger)

	ctx := context.Background()
	err := restarter.RestartContainer(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-success status")
}

func TestWebhookContainerRestarter_Timeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := models.ContainerRestartConfig{
		WebhookURL:    server.URL,
		ContainerName: "waha",
	}

	restarter := NewWebhookContainerRestarter(config, logger)

	// Use a short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := restarter.RestartContainer(ctx)

	require.Error(t, err)
}

func TestWebhookContainerRestarter_NoURL(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := models.ContainerRestartConfig{
		WebhookURL:    "", // Empty URL
		ContainerName: "waha",
	}

	restarter := NewWebhookContainerRestarter(config, logger)

	ctx := context.Background()
	err := restarter.RestartContainer(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook URL not configured")
}

func TestNoOpContainerRestarter(t *testing.T) {
	restarter := NewNoOpContainerRestarter()

	ctx := context.Background()
	err := restarter.RestartContainer(ctx)

	require.NoError(t, err, "NoOp restarter should always succeed")
}
