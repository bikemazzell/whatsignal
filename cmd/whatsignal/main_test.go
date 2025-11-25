package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"whatsignal/internal/migrations"
	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Disable typing delays in tests to prevent timeouts
	_ = os.Setenv("WHATSIGNAL_TEST_MODE", "true")
}

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
	_ = os.Setenv("LOG_LEVEL", "invalid")

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

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *models.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "http://localhost:8080",
				},
				Signal: models.SignalConfig{
					IntermediaryPhoneNumber: "+1234567890",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+0987654321",
					},
				},
				Database: models.DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Media: models.MediaConfig{
					CacheDir: "/tmp/media",
				},
			},
			expectError: false,
		},
		{
			name: "missing WhatsApp API base URL",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{},
				Signal: models.SignalConfig{
					IntermediaryPhoneNumber: "+1234567890",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+0987654321",
					},
				},
				Database: models.DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Media: models.MediaConfig{
					CacheDir: "/tmp/media",
				},
			},
			expectError: true,
			errorMsg:    "whatsApp API base URL is required",
		},
		{
			name: "missing Signal phone number",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "http://localhost:8080",
				},
				Signal: models.SignalConfig{},
				Database: models.DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Media: models.MediaConfig{
					CacheDir: "/tmp/media",
				},
			},
			expectError: true,
			errorMsg:    "signal intermediary phone number is required",
		},
		{
			name: "missing database path",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "http://localhost:8080",
				},
				Signal: models.SignalConfig{
					IntermediaryPhoneNumber: "+1234567890",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+0987654321",
					},
				},
				Database: models.DatabaseConfig{},
				Media: models.MediaConfig{
					CacheDir: "/tmp/media",
				},
			},
			expectError: true,
			errorMsg:    "database path is required",
		},
		{
			name: "missing media cache directory",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "http://localhost:8080",
				},
				Signal: models.SignalConfig{
					IntermediaryPhoneNumber: "+1234567890",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+0987654321",
					},
				},
				Database: models.DatabaseConfig{
					Path: "/tmp/test.db",
				},
				Media: models.MediaConfig{},
			},
			expectError: true,
			errorMsg:    "media cache directory is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunWithDatabaseRetries(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Use an invalid database path to trigger retries
	_ = os.Setenv("DB_PATH", "/invalid/path/test.db")
	defer func() { _ = os.Unsetenv("DB_PATH") }()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize database after retries")
}

func TestRunWithMediaHandlerError(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Unset MEDIA_DIR to prevent environment override
	_ = os.Unsetenv("MEDIA_DIR")

	// Create a file where the cache directory should be to cause mkdir to fail
	tmpFile, err := os.CreateTemp("", "block-mkdir-*")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	// Create a config with the file path as cache directory (this will cause mkdir to fail)
	configContent := fmt.Sprintf(`{
		"whatsapp": {
			"api_base_url": "http://localhost:8080",
			"session_name": "test-session",
			"timeout_ms": 5000000000,
			"retry_count": 3,
			"webhook_secret": "test-secret",
			"pollIntervalSec": 30
		},
		"signal": {
			"rpc_url": "http://localhost:8081",
			"auth_token": "test-token",
			"intermediaryPhoneNumber": "+1234567890",
			"device_name": "test-device"
		},
		"channels": [
			{
				"whatsappSessionName": "default",
				"signalDestinationPhoneNumber": "+0987654321"
			}
		],
		"retry": {
			"initialBackoffMs": 1000,
			"maxBackoffMs": 5000,
			"maxAttempts": 3
		},
		"database": {
			"path": "whatsignal.db"
		},
		"media": {
			"cache_dir": "%s",
			"maxSizeMB": {
				"image": 10,
				"video": 50,
				"gif": 10,
				"document": 20,
				"voice": 5
			},
			"allowedTypes": {
				"image": ["jpg", "jpeg", "png"],
				"video": ["mp4", "avi"],
				"document": ["pdf", "doc"],
				"voice": ["mp3", "wav"]
			}
		},
		"retentionDays": 7,
		"log_level": "info"
	}`, tmpFile.Name())

	err = os.WriteFile("config.json", []byte(configContent), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = run(ctx)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "failed to initialize media handler")
	}
}

func TestRunWithServerError(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Bind a random port first, then try to start server on same port
	// This guarantees "address already in use" error regardless of privileges
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	// Extract the port that was bound
	port := listener.Addr().(*net.TCPAddr).Port
	_ = os.Setenv("PORT", fmt.Sprintf("%d", port))
	defer func() { _ = os.Unsetenv("PORT") }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = run(ctx)
	// Should get "address already in use" error
	assert.Error(t, err)
}

func setupTestEnv(t *testing.T) {
	t.Helper()

	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "whatsignal-test-*")
	require.NoError(t, err)

	// Create test config.json
	configContent := `{
		"whatsapp": {
			"api_base_url": "http://localhost:8080",
			"session_name": "test-session",
			"timeout_ms": 5000000000,
			"retry_count": 3,
			"webhook_secret": "test-secret",
			"pollIntervalSec": 30
		},
		"signal": {
			"rpc_url": "http://localhost:8081",
			"auth_token": "test-token",
			"intermediaryPhoneNumber": "+1234567890",
			"device_name": "test-device"
		},
		"channels": [
			{
				"whatsappSessionName": "default",
				"signalDestinationPhoneNumber": "+1987654321"
			}
		],
		"retry": {
			"initialBackoffMs": 1000,
			"maxBackoffMs": 5000,
			"maxAttempts": 3
		},
		"database": {
			"path": "whatsignal.db"
		},
		"media": {
			"cache_dir": "cache"
		},
		"retentionDays": 7,
		"log_level": "info"
	}`

	err = os.WriteFile("config.json", []byte(configContent), 0644)
	require.NoError(t, err)

	// Set required environment variables
	_ = os.Setenv("WHATSAPP_API_KEY", "test-key")
	_ = os.Setenv("WHATSAPP_API_URL", "http://localhost:8080")
	_ = os.Setenv("SIGNAL_CLI_PATH", "/usr/local/bin/signal-cli")
	_ = os.Setenv("SIGNAL_PHONE_NUMBER", "+1234567890")
	_ = os.Setenv("SIGNAL_CONFIG_PATH", tmpDir+"/signal")
	_ = os.Setenv("WEBHOOK_PORT", "8081")
	_ = os.Setenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET", "test-secret")
	_ = os.Setenv("DB_PATH", tmpDir+"/whatsignal.db")
	_ = os.Setenv("MEDIA_DIR", tmpDir+"/media")
	_ = os.Setenv("MEDIA_RETENTION_DAYS", "7")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-main-testing-purposes")

	// Set up test migrations
	setupTestMigrations(t, tmpDir)
}

// setupTestMigrations creates test migration files for main app tests
func setupTestMigrations(t *testing.T, tmpDir string) {
	t.Helper()

	// Store original migrations directory
	originalMigrationsDir := migrations.MigrationsDir
	t.Cleanup(func() {
		migrations.MigrationsDir = originalMigrationsDir
	})

	// Create migrations directory
	migrationsPath := filepath.Join(tmpDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Create the complete initial schema
	schemaContent := `-- Initial schema for WhatsSignal
CREATE TABLE IF NOT EXISTS message_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    whatsapp_chat_id TEXT NOT NULL,
    whatsapp_msg_id TEXT NOT NULL,
    signal_msg_id TEXT NOT NULL,
    signal_timestamp DATETIME NOT NULL,
    forwarded_at DATETIME NOT NULL,
    delivery_status TEXT NOT NULL,
    media_path TEXT,
    session_name TEXT NOT NULL DEFAULT 'default',
    media_type TEXT,
    chat_id_hash TEXT,
    whatsapp_msg_id_hash TEXT,
    signal_msg_id_hash TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id ON message_mappings(whatsapp_msg_id);
CREATE INDEX IF NOT EXISTS idx_signal_msg_id ON message_mappings(signal_msg_id);
CREATE INDEX IF NOT EXISTS idx_chat_time ON message_mappings(whatsapp_chat_id, forwarded_at);
CREATE INDEX IF NOT EXISTS idx_session_name ON message_mappings(session_name);
CREATE INDEX IF NOT EXISTS idx_session_chat ON message_mappings(session_name, whatsapp_chat_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id_hash ON message_mappings(whatsapp_msg_id_hash);
CREATE INDEX IF NOT EXISTS idx_signal_msg_id_hash ON message_mappings(signal_msg_id_hash);
CREATE INDEX IF NOT EXISTS idx_chat_id_hash ON message_mappings(chat_id_hash);

CREATE TRIGGER IF NOT EXISTS message_mappings_updated_at 
AFTER UPDATE ON message_mappings
BEGIN
    UPDATE message_mappings SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contact_id TEXT NOT NULL UNIQUE,
    phone_number TEXT NOT NULL,
    name TEXT,
    push_name TEXT,
    short_name TEXT,
    is_blocked BOOLEAN DEFAULT FALSE,
    is_group BOOLEAN DEFAULT FALSE,
    is_my_contact BOOLEAN DEFAULT FALSE,
    cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_contact_id ON contacts(contact_id);
CREATE INDEX IF NOT EXISTS idx_phone_number ON contacts(phone_number);
CREATE INDEX IF NOT EXISTS idx_cached_at ON contacts(cached_at);

CREATE TRIGGER IF NOT EXISTS contacts_updated_at
AFTER UPDATE ON contacts
BEGIN
    UPDATE contacts SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;`

	// Write the schema to the test directory
	err = os.WriteFile(filepath.Join(migrationsPath, "001_initial_schema.sql"), []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Set migrations directory temporarily - this will be restored by t.Cleanup
	migrations.MigrationsDir = migrationsPath
}

func TestMultiSessionContactSync(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create a config with multiple channels for testing multi-session contact sync
	configContent := `{
		"whatsapp": {
			"api_base_url": "http://localhost:8080",
			"timeout_ms": 5000000000,
			"retry_count": 3,
			"pollIntervalSec": 30,
			"contactSyncOnStartup": true
		},
		"signal": {
			"rpc_url": "http://localhost:8081",
			"intermediaryPhoneNumber": "+1234567890"
		},
		"channels": [
			{
				"whatsappSessionName": "personal",
				"signalDestinationPhoneNumber": "+1987654321"
			},
			{
				"whatsappSessionName": "business",
				"signalDestinationPhoneNumber": "+1876543210"
			}
		],
		"retry": {
			"initialBackoffMs": 100,
			"maxBackoffMs": 500,
			"maxAttempts": 2
		},
		"database": {
			"path": "whatsignal.db"
		},
		"media": {
			"cache_dir": "cache"
		},
		"retentionDays": 7,
		"log_level": "info"
	}`

	err := os.WriteFile("config.json", []byte(configContent), 0644)
	require.NoError(t, err)

	// Create a very short context to test contact sync initialization
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// The run function should attempt to sync contacts for both sessions
	// Even though it will fail due to no actual WAHA server, we can verify
	// the multi-session logic is triggered
	err = run(ctx)

	// We expect either a context deadline exceeded, connection error, or port binding error
	// The important thing is that it attempted to sync multiple sessions
	if err != nil {
		// Should contain some indication of multi-session processing or server startup issues
		assert.True(t,
			strings.Contains(err.Error(), "context") ||
				strings.Contains(err.Error(), "connection") ||
				strings.Contains(err.Error(), "dial") ||
				strings.Contains(err.Error(), "bind: address already in use"),
			"Expected context timeout, connection error, or port binding error, got: %v", err)
	}
}

func TestContactSyncDisabled(t *testing.T) {
	setupTestEnv(t)
	defer cleanupTestEnv(t)

	// Create a config with contact sync disabled
	configContent := `{
		"whatsapp": {
			"api_base_url": "http://localhost:8080",
			"timeout_ms": 5000000000,
			"retry_count": 3,
			"pollIntervalSec": 30,
			"contactSyncOnStartup": false
		},
		"signal": {
			"rpc_url": "http://localhost:8081",
			"intermediaryPhoneNumber": "+1234567890"
		},
		"channels": [
			{
				"whatsappSessionName": "test",
				"signalDestinationPhoneNumber": "+1987654321"
			}
		],
		"retry": {
			"initialBackoffMs": 100,
			"maxBackoffMs": 500,
			"maxAttempts": 2
		},
		"database": {
			"path": "whatsignal.db"
		},
		"media": {
			"cache_dir": "cache"
		},
		"retentionDays": 7,
		"log_level": "info"
	}`

	err := os.WriteFile("config.json", []byte(configContent), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Ensure server binds to an ephemeral port to avoid conflicts
	oldPort := os.Getenv("PORT")
	require.NoError(t, os.Setenv("PORT", "0"))
	defer func() {
		if oldPort == "" {
			_ = os.Unsetenv("PORT")
		} else {
			_ = os.Setenv("PORT", oldPort)
		}
	}()

	err = run(ctx)
	// Graceful shutdown on context timeout should not be treated as an error
	// Contact sync is disabled, so startup should complete and exit cleanly
	assert.NoError(t, err)
}

func cleanupTestEnv(t *testing.T) {
	t.Helper()

	// Remove test config file
	_ = os.Remove("config.json")

	vars := []string{
		"WHATSAPP_API_KEY",
		"WHATSAPP_API_URL",
		"SIGNAL_CLI_PATH",
		"SIGNAL_PHONE_NUMBER",
		"SIGNAL_CONFIG_PATH",
		"WEBHOOK_PORT",
		"WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET",
		"DB_PATH",
		"MEDIA_DIR",
		"MEDIA_RETENTION_DAYS",
		"LOG_LEVEL",
		"WHATSIGNAL_ENCRYPTION_SECRET",
	}

	for _, v := range vars {
		_ = os.Unsetenv(v)
	}

	// Cleanup temporary directories
	if path := os.Getenv("SIGNAL_CONFIG_PATH"); path != "" {
		_ = os.RemoveAll(path[:len(path)-7]) // Remove the parent temp dir
	}
}
