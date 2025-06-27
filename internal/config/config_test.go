package config

import (
	"os"
	"path/filepath"
	"testing"
	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "whatsignal-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a valid config file
	validConfig := `{
		"whatsapp": {
			"api_base_url": "https://whatsapp.example.com",
			"webhook_secret": "secret123",
			"pollIntervalSec": 30
		},
		"signal": {
			"rpc_url": "https://signal.example.com"
		},
		"retry": {
			"initialBackoffMs": 1000,
			"maxBackoffMs": 5000,
			"maxAttempts": 3
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

	validConfigPath := filepath.Join(tmpDir, "valid_config.json")
	err = os.WriteFile(validConfigPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Create an invalid config file
	invalidConfig := `{
		"whatsapp": {},
		"signal": {},
		"retry": {},
		"database": {},
		"media": {}
	}`

	invalidConfigPath := filepath.Join(tmpDir, "invalid_config.json")
	err = os.WriteFile(invalidConfigPath, []byte(invalidConfig), 0644)
	require.NoError(t, err)

	tests := []struct {
		name      string
		path      string
		setEnv    map[string]string
		wantError bool
		validate  func(*testing.T, interface{})
	}{
		{
			name: "valid config",
			path: validConfigPath,
			validate: func(t *testing.T, cfg interface{}) {
				config := cfg.(*models.Config)
				assert.Equal(t, "https://whatsapp.example.com", config.WhatsApp.APIBaseURL)
				assert.Equal(t, "secret123", config.WhatsApp.WebhookSecret)
				assert.Equal(t, 30, config.WhatsApp.PollIntervalSec)
				assert.Equal(t, "https://signal.example.com", config.Signal.RPCURL)
				assert.Equal(t, 1000, config.Retry.InitialBackoffMs)
				assert.Equal(t, 5000, config.Retry.MaxBackoffMs)
				assert.Equal(t, 3, config.Retry.MaxAttempts)
				assert.Equal(t, "/path/to/db.sqlite", config.Database.Path)
				assert.Equal(t, "/path/to/cache", config.Media.CacheDir)
				assert.Equal(t, 30, config.RetentionDays)
			},
		},
		{
			name: "environment overrides",
			path: validConfigPath,
			setEnv: map[string]string{
				"WHATSAPP_API_URL":        "https://wa.override.com",
				"WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET": "override_secret",
				"SIGNAL_RPC_URL":          "https://signal.override.com",
				"DB_PATH":                 "/override/path/to/db.sqlite",
				"MEDIA_DIR":               "/override/path/to/cache",
			},
			validate: func(t *testing.T, cfg interface{}) {
				config := cfg.(*models.Config)
				assert.Equal(t, "https://wa.override.com", config.WhatsApp.APIBaseURL)
				assert.Equal(t, "override_secret", config.WhatsApp.WebhookSecret)
				assert.Equal(t, "https://signal.override.com", config.Signal.RPCURL)
				assert.Equal(t, "/override/path/to/db.sqlite", config.Database.Path)
				assert.Equal(t, "/override/path/to/cache", config.Media.CacheDir)
			},
		},
		{
			name:      "invalid config",
			path:      invalidConfigPath,
			wantError: true,
		},
		{
			name:      "nonexistent file",
			path:      "/nonexistent/config.json",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.setEnv != nil {
				for k, v := range tt.setEnv {
					os.Setenv(k, v)
				}
				defer func() {
					for k := range tt.setEnv {
						os.Unsetenv(k)
					}
				}()
			}

			config, err := LoadConfig(tt.path)
			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)

			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestValidateDefaults(t *testing.T) {
	config := &models.Config{}
	err := validate(config)
	assert.Error(t, err)
	assert.Equal(t, ErrMissingWhatsAppURL, err)

	config.WhatsApp.APIBaseURL = "https://whatsapp.example.com"
	err = validate(config)
	assert.Error(t, err)
	assert.Equal(t, ErrMissingSignalURL, err)

	config.Signal.RPCURL = "https://signal.example.com"
	err = validate(config)
	assert.Error(t, err)
	assert.Equal(t, ErrMissingDBPath, err)

	config.Database.Path = "/path/to/db.sqlite"
	err = validate(config)
	assert.Error(t, err)
	assert.Equal(t, ErrMissingMediaDir, err)

	config.Media.CacheDir = "/path/to/cache"
	err = validate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channels array is required")

	// Add required channels
	config.Channels = []models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	err = validate(config)
	assert.NoError(t, err)
	assert.Equal(t, 30, config.RetentionDays)            // Default value
	assert.Equal(t, 30, config.WhatsApp.PollIntervalSec) // Default value
}

func TestValidateSecurity(t *testing.T) {
	// Store original environment value
	originalEnv := os.Getenv("WHATSIGNAL_ENV")
	defer func() {
		if originalEnv != "" {
			os.Setenv("WHATSIGNAL_ENV", originalEnv)
		} else {
			os.Unsetenv("WHATSIGNAL_ENV")
		}
	}()

	tests := []struct {
		name        string
		config      *models.Config
		environment string
		expectError bool
		errorMsg    string
	}{
		{
			name: "development environment - no webhook secret",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "",
				},
			},
			environment: "",
			expectError: false,
		},
		{
			name: "development environment - with webhook secret",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "test-secret-123",
				},
			},
			environment: "",
			expectError: false,
		},
		{
			name: "production environment - missing webhook secret",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "",
				},
			},
			environment: "production",
			expectError: true,
			errorMsg:    "WhatsApp webhook secret is required in production",
		},
		{
			name: "production environment - short webhook secret",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "short",
				},
			},
			environment: "production",
			expectError: true,
			errorMsg:    "WhatsApp webhook secret must be at least 32 characters long",
		},
		{
			name: "production environment - valid webhook secret",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "this-is-a-very-long-webhook-secret-that-meets-requirements",
				},
			},
			environment: "production",
			expectError: false,
		},
		{
			name: "production environment - debug logging enabled",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "this-is-a-very-long-webhook-secret-that-meets-requirements",
				},
				LogLevel: "debug",
			},
			environment: "production",
			expectError: true,
			errorMsg:    "debug logging should not be used in production",
		},
		{
			name: "production environment - info logging allowed",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "this-is-a-very-long-webhook-secret-that-meets-requirements",
				},
				LogLevel: "info",
			},
			environment: "production",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment
			if tt.environment != "" {
				os.Setenv("WHATSIGNAL_ENV", tt.environment)
			} else {
				os.Unsetenv("WHATSIGNAL_ENV")
			}

			err := validateSecurity(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
