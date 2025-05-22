package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	// Set up test environment
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx)
	}()

	// Wait for either context cancellation or server error
	select {
	case err := <-errCh:
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		}
	case <-ctx.Done():
		// Expected case: context timeout
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	}
}

func TestRunWithInvalidConfig(t *testing.T) {
	// Test with missing required environment variables
	ctx := context.Background()
	err := run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")
}

func TestRunWithInvalidLogLevel(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Set invalid log level
	os.Setenv("LOG_LEVEL", "invalid")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := run(ctx)
	assert.NoError(t, err) // Should not error, just warn and use default level
}

func TestGracefulShutdown(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create a context that we'll cancel to trigger shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Start the server
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx)
	}()

	// Give it a moment to start up
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown
	cancel()

	// Wait for shutdown to complete
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out")
	}
}

func setupTestEnv(t *testing.T) {
	t.Helper()

	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "whatsignal-test-*")
	require.NoError(t, err)

	// Create test config.json
	configContent := `{
		"whatsapp": {
			"apiBaseUrl": "http://localhost:8080",
			"apiKey": "test-key",
			"sessionName": "test-session",
			"timeout": 5000000000,
			"retryCount": 3,
			"webhookSecret": "test-secret",
			"pollIntervalSec": 30
		},
		"signal": {
			"rpcUrl": "http://localhost:8081",
			"authToken": "test-token",
			"phoneNumber": "+1234567890",
			"deviceName": "test-device"
		},
		"retry": {
			"initialBackoffMs": 1000,
			"maxBackoffMs": 5000,
			"maxAttempts": 3
		},
		"database": {
			"path": "whatsignal.db"
		},
		"media": {
			"cacheDir": "cache"
		},
		"retentionDays": 7,
		"logLevel": "info"
	}`

	err = os.WriteFile("config.json", []byte(configContent), 0644)
	require.NoError(t, err)

	// Set required environment variables
	os.Setenv("WHATSAPP_API_KEY", "test-key")
	os.Setenv("WHATSAPP_BASE_URL", "http://localhost:8080")
	os.Setenv("SIGNAL_CLI_PATH", "/usr/local/bin/signal-cli")
	os.Setenv("SIGNAL_PHONE_NUMBER", "+1234567890")
	os.Setenv("SIGNAL_CONFIG_PATH", tmpDir+"/signal")
	os.Setenv("WEBHOOK_PORT", "8081")
	os.Setenv("WEBHOOK_SECRET", "test-secret")
	os.Setenv("DB_PATH", tmpDir+"/whatsignal.db")
	os.Setenv("MEDIA_DIR", tmpDir+"/media")
	os.Setenv("MEDIA_RETENTION_DAYS", "7")
}

func cleanupTestEnv(t *testing.T) {
	t.Helper()

	// Remove test config file
	os.Remove("config.json")

	vars := []string{
		"WHATSAPP_API_KEY",
		"WHATSAPP_BASE_URL",
		"SIGNAL_CLI_PATH",
		"SIGNAL_PHONE_NUMBER",
		"SIGNAL_CONFIG_PATH",
		"WEBHOOK_PORT",
		"WEBHOOK_SECRET",
		"DB_PATH",
		"MEDIA_DIR",
		"MEDIA_RETENTION_DAYS",
		"LOG_LEVEL",
	}

	for _, v := range vars {
		os.Unsetenv(v)
	}

	// Cleanup temporary directories
	if path := os.Getenv("SIGNAL_CONFIG_PATH"); path != "" {
		os.RemoveAll(path[:len(path)-7]) // Remove the parent temp dir
	}
}
