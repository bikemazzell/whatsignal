package whatsapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestWhatsAppClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
		errorMessage string
	}{
		{
			name:         "successful health check",
			statusCode:   200,
			responseBody: `{"name": "test-session", "status": "WORKING"}`,
			expectError:  false,
		},
		{
			name:         "successful health check with different status",
			statusCode:   200,
			responseBody: `{"name": "test-session", "status": "SCAN_QR_CODE"}`,
			expectError:  false,
		},
		{
			name:         "server error",
			statusCode:   500,
			responseBody: `{"error": "internal server error"}`,
			expectError:  true,
			errorMessage: "WhatsApp API health check returned status 500",
		},
		{
			name:         "not found is ok",
			statusCode:   404,
			responseBody: `{"error": "session not found"}`,
			expectError:  false, // 404 is treated as OK in health check
		},
		{
			name:         "unauthorized error",
			statusCode:   401,
			responseBody: `{"error": "unauthorized"}`,
			expectError:  true,
			errorMessage: "WhatsApp API health check returned status 401",
		},
		{
			name:         "invalid JSON response",
			statusCode:   200,
			responseBody: `invalid json`,
			expectError:  false, // HealthCheck doesn't parse response, just checks status code
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request details
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/api/sessions/test-session")
				assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			logger := logrus.New()
			logger.SetLevel(logrus.FatalLevel) // Silence logs during test

			client := &WhatsAppClient{
				baseURL:     server.URL,
				sessionName: "test-session",
				apiKey:      "test-api-key",
				client:      &http.Client{Timeout: 10 * time.Second},
				logger:      logger,
			}

			ctx := context.Background()
			err := client.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWhatsAppClient_HealthCheck_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status": "WORKING"}`))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	client := &WhatsAppClient{
		baseURL:     server.URL,
		sessionName: "test-session",
		apiKey:      "test-api-key",
		client:      &http.Client{Timeout: 1 * time.Second},
		logger:      logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := client.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WhatsApp API health check failed")
}

func TestWhatsAppClient_HealthCheck_NetworkError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	client := &WhatsAppClient{
		baseURL:     "http://localhost:9999", // Non-existent server
		sessionName: "test-session",
		apiKey:      "test-api-key",
		client:      &http.Client{Timeout: 100 * time.Millisecond},
		logger:      logger,
	}

	ctx := context.Background()
	err := client.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WhatsApp API health check failed")
}

func TestWhatsAppClient_HealthCheck_InvalidURL(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	client := &WhatsAppClient{
		baseURL:     "://invalid-url", // Invalid URL
		sessionName: "test-session",
		apiKey:      "test-api-key",
		client:      &http.Client{},
		logger:      logger,
	}

	ctx := context.Background()
	err := client.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create health check request")
}
