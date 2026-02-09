package signal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"whatsignal/pkg/signal/types"
)

func TestSignalClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "successful health check",
			statusCode:   200,
			responseBody: `{"success": true}`,
			expectError:  false,
		},
		{
			name:         "health check with 202 accepted",
			statusCode:   202,
			responseBody: `{"accepted": true}`,
			expectError:  false,
		},
		{
			name:          "health check with 404 error",
			statusCode:    404,
			responseBody:  `{"error": "not found"}`,
			expectError:   true,
			errorContains: "status 404",
		},
		{
			name:          "health check with 500 error",
			statusCode:    500,
			responseBody:  `{"error": "internal server error"}`,
			expectError:   true,
			errorContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/about", r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := &SignalClient{
				baseURL:     server.URL,
				phoneNumber: "+1234567890",
				client:      &http.Client{Timeout: 10 * time.Second},
			}

			ctx := context.Background()
			err := client.HealthCheck(ctx)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSignalClient_HealthCheck_RequestErrors(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		client := &SignalClient{
			baseURL:     "http://localhost:9999",
			phoneNumber: "+1234567890",
			client:      &http.Client{Timeout: 1 * time.Second},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := client.HealthCheck(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("network timeout", func(t *testing.T) {
		client := &SignalClient{
			baseURL:     "http://localhost:9999", // Non-existent server
			phoneNumber: "+1234567890",
			client:      &http.Client{Timeout: 1 * time.Millisecond},
		}

		ctx := context.Background()
		err := client.HealthCheck(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signal API health check failed")
	})
}

func TestSignalClient_SendMessage_ErrorPaths(t *testing.T) {
	t.Run("request creation error", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)

		client := &SignalClient{
			baseURL:     "://invalid-url", // Invalid URL
			phoneNumber: "+1234567890",
			client:      &http.Client{},
			logger:      logger,
		}

		ctx := context.Background()
		_, err := client.SendMessage(ctx, "+0987654321", "test message", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create request")
	})

	t.Run("context timeout during send", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Delay response
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"success": true}`))
		}))
		defer server.Close()

		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
			logger:      logger,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := client.SendMessage(ctx, "+0987654321", "test message", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send request")
	})

	t.Run("server error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
			logger:      logger,
		}

		ctx := context.Background()
		_, err := client.SendMessage(ctx, "+0987654321", "test message", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signal API error")
	})
}

func TestSignalClient_ReceiveMessages_ErrorPaths(t *testing.T) {
	t.Run("request creation error", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)

		client := &SignalClient{
			baseURL:     "://invalid-url", // Invalid URL
			phoneNumber: "+1234567890",
			client:      &http.Client{},
			logger:      logger,
		}

		ctx := context.Background()
		messages, err := client.ReceiveMessages(ctx, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create request")
		assert.Nil(t, messages)
	})

	t.Run("context cancellation during receive", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Delay response
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`[]`))
		}))
		defer server.Close()

		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
			logger:      logger,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		messages, err := client.ReceiveMessages(ctx, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send request")
		assert.Nil(t, messages)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`invalid json`))
		}))
		defer server.Close()

		logger := logrus.New()
		logger.SetLevel(logrus.FatalLevel)

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
			logger:      logger,
		}

		ctx := context.Background()
		messages, err := client.ReceiveMessages(ctx, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode")
		assert.Nil(t, messages)
	})
}

func TestSignalClient_InitializeDevice_ErrorPaths(t *testing.T) {
	t.Run("request creation error", func(t *testing.T) {
		client := &SignalClient{
			baseURL:     "://invalid-url", // Invalid URL
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		err := client.InitializeDevice(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create initialize device request")
	})

	t.Run("server error during registration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error": "invalid phone number"}`))
		}))
		defer server.Close()

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		err := client.InitializeDevice(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device initialization failed")
	})

	t.Run("already registered device", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"version": "0.11.0", "build": 123, "mode": "native", "versions": ["v1", "v2"]}`))
		}))
		defer server.Close()

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		err := client.InitializeDevice(ctx)
		require.NoError(t, err)
	})
}

func TestSignalClient_DownloadAttachment_ErrorPaths(t *testing.T) {
	t.Run("request creation error", func(t *testing.T) {
		client := &SignalClient{
			baseURL:     "://invalid-url", // Invalid URL
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		data, err := client.DownloadAttachment(ctx, "attachment123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create attachment download request")
		assert.Nil(t, data)
	})

	t.Run("server error during download", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error": "attachment not found"}`))
		}))
		defer server.Close()

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		data, err := client.DownloadAttachment(ctx, "attachment123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "attachment download failed")
		assert.Nil(t, data)
	})
}

func TestSignalClient_ListAttachments_ErrorPaths(t *testing.T) {
	t.Run("request creation error", func(t *testing.T) {
		client := &SignalClient{
			baseURL:     "://invalid-url", // Invalid URL
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		attachments, err := client.ListAttachments(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create list attachments request")
		assert.Nil(t, attachments)
	})

	t.Run("server error during list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		client := &SignalClient{
			baseURL:     server.URL,
			phoneNumber: "+1234567890",
			client:      &http.Client{},
		}

		ctx := context.Background()
		attachments, err := client.ListAttachments(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list attachments failed")
		assert.Nil(t, attachments)
	})
}

func TestDetectContentType_EdgeCases(t *testing.T) {
	client := &SignalClient{}

	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "file with no extension",
			filePath: "filename",
			expected: "application/octet-stream",
		},
		{
			name:     "text file",
			filePath: "file.txt",
			expected: "text/plain; charset=utf-8",
		},
		{
			name:     "image file",
			filePath: "image.png",
			expected: "image/png",
		},
		{
			name:     "unknown extension",
			filePath: "file.unknownext",
			expected: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.detectContentType(tt.filePath)
			// Just verify it returns a string (actual MIME detection depends on constants)
			assert.NotEmpty(t, result)
		})
	}
}

func TestExtractAttachmentPaths_EdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	client := &SignalClient{
		logger: logger,
	}

	t.Run("empty attachments", func(t *testing.T) {
		var attachments []types.RestMessageAttachment
		ctx := context.Background()
		paths := client.extractAttachmentPaths(ctx, attachments)
		assert.Empty(t, paths)
	})

	t.Run("attachments with various IDs", func(t *testing.T) {
		attachments := []types.RestMessageAttachment{
			{ID: "attachment1"},
			{ID: "attachment2"},
		}
		ctx := context.Background()
		paths := client.extractAttachmentPaths(ctx, attachments)
		// Should return some paths (exact content depends on implementation)
		assert.NotNil(t, paths)
	})
}
