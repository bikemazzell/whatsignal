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
	// Save and restore environment variables that might interfere with tests
	originalWebhookSecret := os.Getenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
	defer func() {
		if originalWebhookSecret != "" {
			os.Setenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET", originalWebhookSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
		}
	}()

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
				"WHATSAPP_API_URL":                   "https://wa.override.com",
				"WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET": "override_secret",
				"SIGNAL_RPC_URL":                     "https://signal.override.com",
				"DB_PATH":                            "/override/path/to/db.sqlite",
				"MEDIA_DIR":                          "/override/path/to/cache",
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
			// Clear environment variables that would override config file values
			// unless the test explicitly sets them
			if tt.setEnv == nil || tt.setEnv["WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET"] == "" {
				os.Unsetenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
			}

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

func TestValidateBounds(t *testing.T) {
	tests := []struct {
		name        string
		config      *models.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid retry configuration",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "https://whatsapp.example.com",
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Retry: models.RetryConfig{
					InitialBackoffMs: 100,
					MaxBackoffMs:     5000,
					MaxAttempts:      3,
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: false,
		},
		{
			name: "initial backoff too small",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "https://whatsapp.example.com",
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Retry: models.RetryConfig{
					InitialBackoffMs: 5, // Too small
					MaxBackoffMs:     5000,
					MaxAttempts:      3,
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "initial backoff milliseconds too small",
		},
		{
			name: "max backoff too large",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "https://whatsapp.example.com",
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Retry: models.RetryConfig{
					InitialBackoffMs: 100,
					MaxBackoffMs:     70000, // Too large
					MaxAttempts:      3,
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "max backoff milliseconds too large",
		},
		{
			name: "max backoff less than initial backoff",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "https://whatsapp.example.com",
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Retry: models.RetryConfig{
					InitialBackoffMs: 5000,
					MaxBackoffMs:     1000, // Less than initial
					MaxAttempts:      3,
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "max backoff must be greater than or equal to initial backoff",
		},
		{
			name: "retry attempts too many",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL: "https://whatsapp.example.com",
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Retry: models.RetryConfig{
					InitialBackoffMs: 100,
					MaxBackoffMs:     5000,
					MaxAttempts:      15, // Too many
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "max retry attempts too large",
		},
		{
			name: "contact cache hours too large",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL:        "https://whatsapp.example.com",
					ContactCacheHours: 200, // Too large
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "contact cache hours too large",
		},
		{
			name: "session health check too large",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL:            "https://whatsapp.example.com",
					SessionHealthCheckSec: 5000, // Too large
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "session health check interval too large",
		},
		{
			name: "whatsapp poll interval too large",
			config: &models.Config{
				WhatsApp: models.WhatsAppConfig{
					APIBaseURL:      "https://whatsapp.example.com",
					PollIntervalSec: 5000, // Too large
				},
				Signal: models.SignalConfig{
					RPCURL: "https://signal.example.com",
				},
				Database: models.DatabaseConfig{
					Path: "/path/to/db.sqlite",
				},
				Media: models.MediaConfig{
					CacheDir: "/path/to/cache",
				},
				Channels: []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				},
			},
			expectError: true,
			errorMsg:    "WhatsApp poll interval too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First validate basic config
			err := validate(tt.config)
			require.NoError(t, err)

			// Then test bounds validation
			err = validateBounds(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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

func TestLoadConfig_EdgeCases(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "whatsignal-config-edge-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		configJSON  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty file",
			configJSON:  "",
			expectError: true,
			errorMsg:    "unexpected end of JSON input",
		},
		{
			name:        "invalid JSON syntax",
			configJSON:  `{"whatsapp": {`,
			expectError: true,
			errorMsg:    "unexpected end of JSON input",
		},
		{
			name: "missing required whatsapp config",
			configJSON: `{
				"signal": {"rpc_url": "https://signal.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": [{"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+1234567890"}]
			}`,
			expectError: true,
			errorMsg:    "missing WhatsApp API URL",
		},
		{
			name: "missing required signal config",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": [{"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+1234567890"}]
			}`,
			expectError: true,
			errorMsg:    "missing Signal RPC URL",
		},
		{
			name: "missing required database config",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"signal": {"rpc_url": "https://signal.example.com"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": [{"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+1234567890"}]
			}`,
			expectError: true,
			errorMsg:    "missing database path",
		},
		{
			name: "missing required media config",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"signal": {"rpc_url": "https://signal.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"channels": [{"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+1234567890"}]
			}`,
			expectError: true,
			errorMsg:    "missing media cache directory",
		},
		{
			name: "empty channels array",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"signal": {"rpc_url": "https://signal.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": []
			}`,
			expectError: true,
			errorMsg:    "channels array is required",
		},
		{
			name: "invalid channel - missing whatsapp session",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"signal": {"rpc_url": "https://signal.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": [{"signalDestinationPhoneNumber": "+1234567890"}]
			}`,
			expectError: true,
			errorMsg:    "empty WhatsApp session name",
		},
		{
			name: "invalid channel - missing signal phone",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"signal": {"rpc_url": "https://signal.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": [{"whatsappSessionName": "default"}]
			}`,
			expectError: true,
			errorMsg:    "empty Signal destination",
		},
		{
			name: "valid minimal config with defaults",
			configJSON: `{
				"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
				"signal": {"rpc_url": "https://signal.example.com"},
				"database": {"path": "/path/to/db.sqlite"},
				"media": {"cache_dir": "/path/to/cache"},
				"channels": [{"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+1234567890"}]
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tt.name+".json")
			err = os.WriteFile(configPath, []byte(tt.configJSON), 0644)
			require.NoError(t, err)

			config, err := LoadConfig(configPath)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)

				// Verify defaults were applied
				assert.Equal(t, 30, config.RetentionDays)
				assert.Equal(t, 30, config.WhatsApp.PollIntervalSec)
			}
		})
	}
}

func TestValidateAppliesDefaults(t *testing.T) {
	config := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL: "https://whatsapp.example.com",
		},
		Signal: models.SignalConfig{
			RPCURL: "https://signal.example.com",
		},
		Database: models.DatabaseConfig{
			Path: "/path/to/db.sqlite",
		},
		Media: models.MediaConfig{
			CacheDir: "/path/to/cache",
		},
		Channels: []models.Channel{
			{
				WhatsAppSessionName:          "default",
				SignalDestinationPhoneNumber: "+1234567890",
			},
		},
	}

	err := validate(config)
	require.NoError(t, err)

	// Test that defaults are applied by validate function
	assert.Equal(t, 30, config.RetentionDays)
	assert.Equal(t, 30, config.WhatsApp.PollIntervalSec)
	assert.Equal(t, 5, config.Media.MaxSizeMB.Image)
	assert.Equal(t, 100, config.Media.MaxSizeMB.Video)
	assert.Equal(t, 100, config.Media.MaxSizeMB.Document)
	assert.Equal(t, 16, config.Media.MaxSizeMB.Voice)
	assert.Equal(t, "whatsignal", config.Tracing.ServiceName)
	assert.Equal(t, "dev", config.Tracing.ServiceVersion)
	assert.Equal(t, "development", config.Tracing.Environment)
	assert.Equal(t, 0.1, config.Tracing.SampleRate)
	assert.True(t, config.Tracing.UseStdout)
}

func TestValidateDoesNotOverrideExisting(t *testing.T) {
	config := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL:      "https://whatsapp.example.com",
			PollIntervalSec: 15,
		},
		Signal: models.SignalConfig{
			RPCURL: "https://signal.example.com",
		},
		Database: models.DatabaseConfig{
			Path: "/path/to/db.sqlite",
		},
		Media: models.MediaConfig{
			CacheDir: "/path/to/cache",
			MaxSizeMB: models.MediaSizeLimits{
				Image: 10,
			},
		},
		Channels: []models.Channel{
			{
				WhatsAppSessionName:          "default",
				SignalDestinationPhoneNumber: "+1234567890",
			},
		},
		RetentionDays: 60,
		Tracing: models.TracingConfig{
			ServiceName: "custom-service",
		},
	}

	err := validate(config)
	require.NoError(t, err)

	// Existing values should not be overridden
	assert.Equal(t, 60, config.RetentionDays)
	assert.Equal(t, 15, config.WhatsApp.PollIntervalSec)
	assert.Equal(t, 10, config.Media.MaxSizeMB.Image)
	assert.Equal(t, "custom-service", config.Tracing.ServiceName)

	// But missing values should get defaults
	assert.Equal(t, 100, config.Media.MaxSizeMB.Video)
	assert.Equal(t, "dev", config.Tracing.ServiceVersion)
}

func TestLoadConfig_FilePermissions(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "whatsignal-config-perm-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configContent := `{
		"whatsapp": {"api_base_url": "https://whatsapp.example.com"},
		"signal": {"rpc_url": "https://signal.example.com"},
		"database": {"path": "/path/to/db.sqlite"},
		"media": {"cache_dir": "/path/to/cache"},
		"channels": [{"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+1234567890"}]
	}`

	configPath := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Make file unreadable
	err = os.Chmod(configPath, 0000)
	require.NoError(t, err)

	// Restore permissions for cleanup
	defer func() { _ = os.Chmod(configPath, 0644) }()

	_, err = LoadConfig(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestApplyEnvironmentOverrides(t *testing.T) {
	// Store original environment values
	originalWhatsAppURL := os.Getenv("WHATSAPP_API_URL")
	originalWebhookSecret := os.Getenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
	originalSignalURL := os.Getenv("SIGNAL_RPC_URL")
	originalDBPath := os.Getenv("DB_PATH")
	originalMediaDir := os.Getenv("MEDIA_DIR")

	defer func() {
		// Restore original environment
		if originalWhatsAppURL != "" {
			os.Setenv("WHATSAPP_API_URL", originalWhatsAppURL)
		} else {
			os.Unsetenv("WHATSAPP_API_URL")
		}
		if originalWebhookSecret != "" {
			os.Setenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET", originalWebhookSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
		}
		if originalSignalURL != "" {
			os.Setenv("SIGNAL_RPC_URL", originalSignalURL)
		} else {
			os.Unsetenv("SIGNAL_RPC_URL")
		}
		if originalDBPath != "" {
			os.Setenv("DB_PATH", originalDBPath)
		} else {
			os.Unsetenv("DB_PATH")
		}
		if originalMediaDir != "" {
			os.Setenv("MEDIA_DIR", originalMediaDir)
		} else {
			os.Unsetenv("MEDIA_DIR")
		}
	}()

	config := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL:    "https://original-whatsapp.com",
			WebhookSecret: "original-secret",
		},
		Signal: models.SignalConfig{
			RPCURL: "https://original-signal.com",
		},
		Database: models.DatabaseConfig{
			Path: "/original/path/to/db.sqlite",
		},
		Media: models.MediaConfig{
			CacheDir: "/original/path/to/cache",
		},
	}

	// Set environment variables
	os.Setenv("WHATSAPP_API_URL", "https://env-whatsapp.com")
	os.Setenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET", "env-webhook-secret")
	os.Setenv("SIGNAL_RPC_URL", "https://env-signal.com")
	os.Setenv("DB_PATH", "/env/path/to/db.sqlite")
	os.Setenv("MEDIA_DIR", "/env/path/to/cache")

	applyEnvironmentOverrides(config)

	// Verify environment variables override config values
	assert.Equal(t, "https://env-whatsapp.com", config.WhatsApp.APIBaseURL)
	assert.Equal(t, "env-webhook-secret", config.WhatsApp.WebhookSecret)
	assert.Equal(t, "https://env-signal.com", config.Signal.RPCURL)
	assert.Equal(t, "/env/path/to/db.sqlite", config.Database.Path)
	assert.Equal(t, "/env/path/to/cache", config.Media.CacheDir)
}

func TestApplyEnvironmentOverrides_EmptyEnv(t *testing.T) {
	// Store original environment values
	originalWhatsAppURL := os.Getenv("WHATSAPP_API_URL")
	originalWebhookSecret := os.Getenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
	originalSignalURL := os.Getenv("SIGNAL_RPC_URL")
	originalDBPath := os.Getenv("DB_PATH")
	originalMediaDir := os.Getenv("MEDIA_DIR")

	defer func() {
		// Restore original environment
		if originalWhatsAppURL != "" {
			os.Setenv("WHATSAPP_API_URL", originalWhatsAppURL)
		} else {
			os.Unsetenv("WHATSAPP_API_URL")
		}
		if originalWebhookSecret != "" {
			os.Setenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET", originalWebhookSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
		}
		if originalSignalURL != "" {
			os.Setenv("SIGNAL_RPC_URL", originalSignalURL)
		} else {
			os.Unsetenv("SIGNAL_RPC_URL")
		}
		if originalDBPath != "" {
			os.Setenv("DB_PATH", originalDBPath)
		} else {
			os.Unsetenv("DB_PATH")
		}
		if originalMediaDir != "" {
			os.Setenv("MEDIA_DIR", originalMediaDir)
		} else {
			os.Unsetenv("MEDIA_DIR")
		}
	}()

	config := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL:    "https://original-whatsapp.com",
			WebhookSecret: "original-secret",
		},
		Signal: models.SignalConfig{
			RPCURL: "https://original-signal.com",
		},
		Database: models.DatabaseConfig{
			Path: "/original/path/to/db.sqlite",
		},
		Media: models.MediaConfig{
			CacheDir: "/original/path/to/cache",
		},
	}

	// Unset all environment variables
	os.Unsetenv("WHATSAPP_API_URL")
	os.Unsetenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET")
	os.Unsetenv("SIGNAL_RPC_URL")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("MEDIA_DIR")

	applyEnvironmentOverrides(config)

	// Verify config values remain unchanged when environment variables are not set
	assert.Equal(t, "https://original-whatsapp.com", config.WhatsApp.APIBaseURL)
	assert.Equal(t, "original-secret", config.WhatsApp.WebhookSecret)
	assert.Equal(t, "https://original-signal.com", config.Signal.RPCURL)
	assert.Equal(t, "/original/path/to/db.sqlite", config.Database.Path)
	assert.Equal(t, "/original/path/to/cache", config.Media.CacheDir)
}
