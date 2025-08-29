package config

import (
	"encoding/json"
	"fmt"
	"os"
	"whatsignal/internal/constants"
	"whatsignal/internal/models"
	"whatsignal/internal/security"
)

var (
	ErrMissingWhatsAppURL = models.ConfigError{Message: "missing WhatsApp API URL"}
	ErrMissingSignalURL   = models.ConfigError{Message: "missing Signal RPC URL"}
	ErrMissingDBPath      = models.ConfigError{Message: "missing database path"}
	ErrMissingMediaDir    = models.ConfigError{Message: "missing media cache directory"}
)

func LoadConfig(path string) (*models.Config, error) {
	// Validate config file path to prevent directory traversal
	if err := security.ValidateFilePath(path); err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	file, err := os.ReadFile(path) // #nosec G304 - Path validated by security.ValidateFilePath above
	if err != nil {
		return nil, err
	}

	var config models.Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	if err := validate(&config); err != nil {
		return nil, err
	}

	applyEnvironmentOverrides(&config)

	// Perform security validation after environment overrides
	if err := validateSecurity(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validate(c *models.Config) error {
	if c.WhatsApp.APIBaseURL == "" {
		return ErrMissingWhatsAppURL
	}
	if c.Signal.RPCURL == "" {
		return ErrMissingSignalURL
	}
	if c.Database.Path == "" {
		return ErrMissingDBPath
	}
	if c.Media.CacheDir == "" {
		return ErrMissingMediaDir
	}

	// Channels configuration is now required
	if len(c.Channels) == 0 {
		return models.ConfigError{Message: "channels array is required and must contain at least one channel"}
	}

	// Validate each channel
	sessionNames := make(map[string]bool)
	destinations := make(map[string]bool)
	for i, channel := range c.Channels {
		if channel.WhatsAppSessionName == "" {
			return models.ConfigError{Message: fmt.Sprintf("empty WhatsApp session name in channel %d", i)}
		}
		if channel.SignalDestinationPhoneNumber == "" {
			return models.ConfigError{Message: fmt.Sprintf("empty Signal destination in channel %d", i)}
		}

		// Check for duplicates
		if sessionNames[channel.WhatsAppSessionName] {
			return models.ConfigError{Message: fmt.Sprintf("duplicate WhatsApp session name: %s", channel.WhatsAppSessionName)}
		}
		if destinations[channel.SignalDestinationPhoneNumber] {
			return models.ConfigError{Message: fmt.Sprintf("duplicate Signal destination: %s", channel.SignalDestinationPhoneNumber)}
		}

		sessionNames[channel.WhatsAppSessionName] = true
		destinations[channel.SignalDestinationPhoneNumber] = true
	}

	// Set default media configuration if not provided
	if c.Media.MaxSizeMB.Image == 0 {
		c.Media.MaxSizeMB.Image = 5
	}
	if c.Media.MaxSizeMB.Video == 0 {
		c.Media.MaxSizeMB.Video = 100
	}
	if c.Media.MaxSizeMB.Document == 0 {
		c.Media.MaxSizeMB.Document = 100
	}
	if c.Media.MaxSizeMB.Voice == 0 {
		c.Media.MaxSizeMB.Voice = 16
	}

	// Set default allowed types if not provided
	if len(c.Media.AllowedTypes.Image) == 0 {
		c.Media.AllowedTypes.Image = constants.DefaultImageTypes
	}
	if len(c.Media.AllowedTypes.Video) == 0 {
		c.Media.AllowedTypes.Video = constants.DefaultVideoTypes
	}
	if len(c.Media.AllowedTypes.Document) == 0 {
		c.Media.AllowedTypes.Document = constants.DefaultDocumentTypes
	}
	if len(c.Media.AllowedTypes.Voice) == 0 {
		c.Media.AllowedTypes.Voice = constants.DefaultVoiceTypes
	}

	if c.RetentionDays <= 0 {
		c.RetentionDays = 30
	}
	if c.WhatsApp.PollIntervalSec <= 0 {
		c.WhatsApp.PollIntervalSec = constants.DefaultWhatsAppPollIntervalSec
	}
	// Default webhook skew if not provided
	if c.Server.WebhookMaxSkewSec <= 0 {
		c.Server.WebhookMaxSkewSec = constants.DefaultWebhookMaxSkewSec
	}
	return nil
}

func applyEnvironmentOverrides(c *models.Config) {
	if url := os.Getenv("WHATSAPP_API_URL"); url != "" {
		c.WhatsApp.APIBaseURL = url
	}

	// SECURITY: Webhook secrets should be set via environment variables
	if secret := os.Getenv("WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET"); secret != "" {
		c.WhatsApp.WebhookSecret = secret
	}

	if url := os.Getenv("SIGNAL_RPC_URL"); url != "" {
		c.Signal.RPCURL = url
	}
	// Signal CLI REST API typically doesn't require auth tokens
	// Remove the auth token override as it's not part of the standard API

	if path := os.Getenv("DB_PATH"); path != "" {
		c.Database.Path = path
	}
	if dir := os.Getenv("MEDIA_DIR"); dir != "" {
		c.Media.CacheDir = dir
	}
}

// validateSecurity performs security-specific validation
func validateSecurity(c *models.Config) error {
	// Check if we're in production mode
	isProduction := os.Getenv("WHATSIGNAL_ENV") == "production"

	if isProduction {
		// In production, webhook secrets are mandatory
		if c.WhatsApp.WebhookSecret == "" {
			return models.ConfigError{Message: "WhatsApp webhook secret is required in production (set WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET environment variable)"}
		}

		// Validate webhook secret strength
		if len(c.WhatsApp.WebhookSecret) < 32 {
			return models.ConfigError{Message: "WhatsApp webhook secret must be at least 32 characters long"}
		}

		// Signal CLI REST API typically doesn't require auth tokens in standard deployments
		// Remove auth token validation as it's not part of the standard Signal CLI REST API

		// Warn about debug logging in production
		if c.LogLevel == "debug" {
			return models.ConfigError{Message: "debug logging should not be used in production (security risk)"}
		}
	} else {
		// In development, warn if secrets are missing
		if c.WhatsApp.WebhookSecret == "" {
			fmt.Fprintf(os.Stderr, "WARNING: WhatsApp webhook secret not set. Set WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET environment variable for security.\n")
		}
		// Signal CLI REST API typically doesn't require auth tokens
		// Remove the warning as it's not applicable to standard Signal CLI REST API deployments
	}

	return nil
}
