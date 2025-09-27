package config

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"whatsignal/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) {
	return f(p)
}

func TestNewConfigWatcher(t *testing.T) {
	logger := logrus.New()
	configPath := "/path/to/config.json"

	watcher := NewConfigWatcher(configPath, logger)

	assert.NotNil(t, watcher)
	assert.Equal(t, configPath, watcher.configPath)
	assert.Equal(t, logger, watcher.logger)
	assert.NotNil(t, watcher.callbacks)
	assert.Len(t, watcher.callbacks, 0)
}

func TestConfigWatcher_Start_InvalidPath(t *testing.T) {
	logger := logrus.New()
	watcher := NewConfigWatcher("/nonexistent/config.json", logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	assert.Error(t, err)
}

func TestConfigWatcher_Start_ValidConfig(t *testing.T) {
	// Create temporary directory and config file
	tmpDir, err := os.MkdirTemp("", "watcher-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configContent := `{
		"whatsapp": {
			"api_base_url": "https://whatsapp.example.com",
			"webhook_secret": "secret123"
		},
		"signal": {
			"rpc_url": "https://signal.example.com"
		},
		"database": {
			"path": "/path/to/db.sqlite"
		},
		"media": {
			"cache_dir": "/path/to/cache"
		},
		"channels": [
			{
				"whatsappSessionName": "default",
				"signalDestinationPhoneNumber": "+1234567890"
			}
		],
		"retentionDays": 30
	}`

	configPath := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logrus.New()
	watcher := NewConfigWatcher(configPath, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = watcher.Start(ctx)
	assert.NoError(t, err) // Should exit gracefully when context is cancelled

	// Verify config was loaded
	config := watcher.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "https://whatsapp.example.com", config.WhatsApp.APIBaseURL)
}

func TestConfigWatcher_GetConfig(t *testing.T) {
	logger := logrus.New()
	watcher := NewConfigWatcher("/path/to/config.json", logger)

	// Initially should return nil
	config := watcher.GetConfig()
	assert.Nil(t, config)

	// Set a config manually for testing
	testConfig := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL: "https://test.com",
		},
	}

	watcher.mu.Lock()
	watcher.config = testConfig
	watcher.mu.Unlock()

	config = watcher.GetConfig()
	assert.Equal(t, testConfig, config)
}

func TestConfigWatcher_OnConfigChange(t *testing.T) {
	logger := logrus.New()
	watcher := NewConfigWatcher("/path/to/config.json", logger)

	callbackCalled := false
	var receivedConfig *models.Config

	callback := func(config *models.Config) {
		callbackCalled = true
		receivedConfig = config
	}

	watcher.OnConfigChange(callback)

	assert.Len(t, watcher.callbacks, 1)

	// Test callback is called during reload
	testConfig := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL: "https://test.com",
		},
	}

	// Simulate a config reload by setting the config and calling callbacks
	watcher.mu.Lock()
	watcher.config = testConfig
	callbacks := make([]func(*models.Config), len(watcher.callbacks))
	copy(callbacks, watcher.callbacks)
	watcher.mu.Unlock()

	for _, cb := range callbacks {
		cb(testConfig)
	}

	assert.True(t, callbackCalled)
	assert.Equal(t, testConfig, receivedConfig)
}

func TestConfigWatcher_ReloadConfig_FileChanged(t *testing.T) {
	// Create temporary directory and config file
	tmpDir, err := os.MkdirTemp("", "watcher-reload-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	initialConfig := `{
		"whatsapp": {
			"api_base_url": "https://whatsapp.example.com",
			"webhook_secret": "secret123"
		},
		"signal": {
			"rpc_url": "https://signal.example.com"
		},
		"database": {
			"path": "/path/to/db.sqlite"
		},
		"media": {
			"cache_dir": "/path/to/cache"
		},
		"channels": [
			{
				"whatsappSessionName": "default",
				"signalDestinationPhoneNumber": "+1234567890"
			}
		],
		"retentionDays": 30
	}`

	configPath := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	var logOutput strings.Builder
	logger := logrus.New()
	logger.SetOutput(&logOutput)

	watcher := NewConfigWatcher(configPath, logger)

	// Load initial config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	watcher.mu.Lock()
	watcher.config = config
	watcher.mu.Unlock()

	// Set up callback to track changes
	var mu sync.Mutex
	callbackCalled := false
	var newConfig *models.Config
	watcher.OnConfigChange(func(config *models.Config) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		newConfig = config
	})

	// Modify the config file
	updatedConfig := strings.Replace(initialConfig, `"retentionDays": 30`, `"retentionDays": 60`, 1)
	err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	// Trigger reload
	watcher.reloadConfig()

	// Give callbacks time to execute
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, callbackCalled)
	assert.NotNil(t, newConfig)
	assert.Equal(t, 60, newConfig.RetentionDays)

	// Check that log shows configuration reloaded
	logStr := logOutput.String()
	assert.Contains(t, logStr, "Configuration reloaded successfully")
}

func TestConfigWatcher_ReloadConfig_InvalidFile(t *testing.T) {
	// Create temporary directory and config file
	tmpDir, err := os.MkdirTemp("", "watcher-invalid-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	validConfig := `{
		"whatsapp": {
			"api_base_url": "https://whatsapp.example.com",
			"webhook_secret": "secret123"
		},
		"signal": {
			"rpc_url": "https://signal.example.com"
		},
		"database": {
			"path": "/path/to/db.sqlite"
		},
		"media": {
			"cache_dir": "/path/to/cache"
		},
		"channels": [
			{
				"whatsappSessionName": "default",
				"signalDestinationPhoneNumber": "+1234567890"
			}
		],
		"retentionDays": 30
	}`

	configPath := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	var logOutput strings.Builder
	logger := logrus.New()
	logger.SetOutput(&logOutput)

	watcher := NewConfigWatcher(configPath, logger)

	// Load initial config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	watcher.mu.Lock()
	watcher.config = config
	watcher.mu.Unlock()

	// Write invalid JSON
	err = os.WriteFile(configPath, []byte(`invalid json`), 0644)
	require.NoError(t, err)

	// Trigger reload
	watcher.reloadConfig()

	// Check that log shows reload failure
	logStr := logOutput.String()
	assert.Contains(t, logStr, "Failed to reload configuration")

	// Config should remain unchanged
	currentConfig := watcher.GetConfig()
	assert.Equal(t, config, currentConfig)
}

func TestConfigWatcher_CallbackPanic(t *testing.T) {
	// Create temporary directory and config file
	tmpDir, err := os.MkdirTemp("", "watcher-panic-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	validConfig := `{
		"whatsapp": {
			"api_base_url": "https://whatsapp.example.com",
			"webhook_secret": "secret123"
		},
		"signal": {
			"rpc_url": "https://signal.example.com"
		},
		"database": {
			"path": "/path/to/db.sqlite"
		},
		"media": {
			"cache_dir": "/path/to/cache"
		},
		"channels": [
			{
				"whatsappSessionName": "default",
				"signalDestinationPhoneNumber": "+1234567890"
			}
		],
		"retentionDays": 30
	}`

	configPath := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	var logMu sync.Mutex
	var logOutput strings.Builder

	safeWriter := struct {
		io.Writer
	}{
		Writer: writerFunc(func(p []byte) (int, error) {
			logMu.Lock()
			defer logMu.Unlock()
			return logOutput.Write(p)
		}),
	}

	logger := logrus.New()
	logger.SetOutput(safeWriter)

	watcher := NewConfigWatcher(configPath, logger)

	// Load initial config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	watcher.mu.Lock()
	watcher.config = config
	watcher.mu.Unlock()

	// Add a callback that panics
	watcher.OnConfigChange(func(config *models.Config) {
		panic("test panic")
	})

	// Trigger reload
	watcher.reloadConfig()

	// Give callbacks time to execute and panic
	time.Sleep(10 * time.Millisecond)

	// Check that panic was handled and logged
	logMu.Lock()
	logStr := logOutput.String()
	logMu.Unlock()
	assert.Contains(t, logStr, "Config change callback panicked")
}

func TestConfigWatcher_LogConfigChanges(t *testing.T) {
	var logOutput strings.Builder
	logger := logrus.New()
	logger.SetOutput(&logOutput)

	watcher := NewConfigWatcher("/path/to/config.json", logger)

	oldConfig := &models.Config{
		RetentionDays: 30,
		Server: models.ServerConfig{
			CleanupIntervalHours: 24,
		},
		Channels: []models.Channel{
			{WhatsAppSessionName: "session1"},
		},
	}

	newConfig := &models.Config{
		RetentionDays: 60,
		Server: models.ServerConfig{
			CleanupIntervalHours: 12,
		},
		Channels: []models.Channel{
			{WhatsAppSessionName: "session1"},
			{WhatsAppSessionName: "session2"},
		},
	}

	watcher.logConfigChanges(oldConfig, newConfig)

	logStr := logOutput.String()
	assert.Contains(t, logStr, "Retention days changed")
	assert.Contains(t, logStr, "Cleanup interval changed")
	assert.Contains(t, logStr, "Number of channels changed")
}

func TestConfigWatcher_LogConfigChanges_NilOldConfig(t *testing.T) {
	var logOutput strings.Builder
	logger := logrus.New()
	logger.SetOutput(&logOutput)

	watcher := NewConfigWatcher("/path/to/config.json", logger)

	newConfig := &models.Config{
		RetentionDays: 60,
	}

	// Should not log anything when old config is nil
	watcher.logConfigChanges(nil, newConfig)

	logStr := logOutput.String()
	assert.Equal(t, "", logStr)
}

func TestConfigWatcher_Start_FileStatError(t *testing.T) {
	// Skip this test as it's timing dependent and may be flaky
	t.Skip("Skipping timing-dependent test for now")
}
