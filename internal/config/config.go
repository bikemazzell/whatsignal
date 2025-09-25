package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"whatsignal/internal/constants"
	"whatsignal/internal/models"
	"whatsignal/internal/security"
	"whatsignal/internal/validation"
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

	if err := validateBounds(&config); err != nil {
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
	// Set default timeout if not provided
	if c.WhatsApp.Timeout == 0 {
		c.WhatsApp.Timeout = time.Duration(constants.DefaultWhatsAppTimeoutMs) * time.Millisecond
	}
	// Set default retry count if not provided
	if c.WhatsApp.RetryCount == 0 {
		c.WhatsApp.RetryCount = constants.DefaultWhatsAppRetryCount
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

// validateBounds performs bounds checking on configuration values
func validateBounds(c *models.Config) error {
	// Validate timeout values
	if err := validation.ValidateTimeout(c.WhatsApp.RetryCount, "WhatsApp retry count"); err != nil {
		return models.ConfigError{Message: err.Error()}
	}

	if c.WhatsApp.Timeout > 0 {
		// Convert nanoseconds to seconds for validation
		timeoutSec := int(c.WhatsApp.Timeout.Seconds())
		if timeoutSec < 1 {
			return models.ConfigError{Message: "WhatsApp timeout must be at least 1000ms (1 second)"}
		}
		if err := validation.ValidateTimeout(timeoutSec, "WhatsApp timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate Signal configuration
	if c.Signal.PollIntervalSec > 0 {
		if err := validation.ValidateTimeout(c.Signal.PollIntervalSec, "Signal poll interval"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Signal.PollTimeoutSec > 0 {
		if err := validation.ValidateTimeout(c.Signal.PollTimeoutSec, "Signal poll timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Signal.HTTPTimeoutSec > 0 {
		if err := validation.ValidateTimeout(c.Signal.HTTPTimeoutSec, "Signal HTTP timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate database configuration (only if values are set)
	if c.Database.MaxOpenConnections > 0 || c.Database.MaxIdleConnections > 0 {
		maxOpen := c.Database.MaxOpenConnections
		if maxOpen == 0 {
			maxOpen = constants.DefaultDBMaxOpenConnections
		}
		maxIdle := c.Database.MaxIdleConnections
		if maxIdle == 0 {
			maxIdle = constants.DefaultDBMaxIdleConnections
		}

		if err := validation.ValidateConnectionPool(maxOpen, maxIdle); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Database.ConnMaxLifetimeSec > 0 {
		if err := validation.ValidateTimeout(c.Database.ConnMaxLifetimeSec, "database connection max lifetime"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Database.ConnMaxIdleTimeSec > 0 {
		if err := validation.ValidateTimeout(c.Database.ConnMaxIdleTimeSec, "database connection max idle time"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate media configuration
	if err := validation.ValidateNumericRange(c.Media.MaxSizeMB.Image, "image max size", 1, 100); err != nil {
		return models.ConfigError{Message: err.Error()}
	}

	if err := validation.ValidateNumericRange(c.Media.MaxSizeMB.Video, "video max size", 1, 500); err != nil {
		return models.ConfigError{Message: err.Error()}
	}

	if err := validation.ValidateNumericRange(c.Media.MaxSizeMB.Document, "document max size", 1, 100); err != nil {
		return models.ConfigError{Message: err.Error()}
	}

	if err := validation.ValidateNumericRange(c.Media.MaxSizeMB.Voice, "voice max size", 1, 50); err != nil {
		return models.ConfigError{Message: err.Error()}
	}

	if c.Media.DownloadTimeout > 0 {
		if err := validation.ValidateTimeout(c.Media.DownloadTimeout, "media download timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate server configuration
	if c.Server.ReadTimeoutSec > 0 {
		if err := validation.ValidateTimeout(c.Server.ReadTimeoutSec, "server read timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.WriteTimeoutSec > 0 {
		if err := validation.ValidateTimeout(c.Server.WriteTimeoutSec, "server write timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.IdleTimeoutSec > 0 {
		if err := validation.ValidateTimeout(c.Server.IdleTimeoutSec, "server idle timeout"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.WebhookMaxSkewSec > 0 {
		if err := validation.ValidateTimeout(c.Server.WebhookMaxSkewSec, "webhook max skew"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.WebhookMaxBytes > 0 {
		if err := validation.ValidateNumericRange(c.Server.WebhookMaxBytes, "webhook max bytes", 1024, 50*1024*1024); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.RateLimitPerMinute > 0 {
		if err := validation.ValidateNumericRange(c.Server.RateLimitPerMinute, "rate limit per minute", 1, constants.MaxRateLimitPerMinute); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.RateLimitCleanupMinutes > 0 {
		if err := validation.ValidateNumericRange(c.Server.RateLimitCleanupMinutes, "rate limit cleanup minutes", 1, 60); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Server.CleanupIntervalHours > 0 {
		if err := validation.ValidateNumericRange(c.Server.CleanupIntervalHours, "cleanup interval hours", 1, 168); err != nil { // Max 1 week
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate retention days
	if c.RetentionDays > 0 {
		if err := validation.ValidateRetentionDays(c.RetentionDays); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate WhatsApp contact cache hours
	if c.WhatsApp.ContactCacheHours > 0 {
		if err := validation.ValidateNumericRange(c.WhatsApp.ContactCacheHours, "contact cache hours", 1, 168); err != nil { // Max 1 week
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate WhatsApp session health check interval
	if c.WhatsApp.SessionHealthCheckSec > 0 {
		if err := validation.ValidateTimeout(c.WhatsApp.SessionHealthCheckSec, "session health check interval"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate WhatsApp poll interval
	if c.WhatsApp.PollIntervalSec > 0 {
		if err := validation.ValidateTimeout(c.WhatsApp.PollIntervalSec, "WhatsApp poll interval"); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate retry configuration
	if c.Retry.InitialBackoffMs > 0 {
		if err := validation.ValidateNumericRange(c.Retry.InitialBackoffMs, "initial backoff milliseconds", 10, 10000); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	if c.Retry.MaxBackoffMs > 0 {
		if err := validation.ValidateNumericRange(c.Retry.MaxBackoffMs, "max backoff milliseconds", 100, 60000); err != nil { // Max 1 minute
			return models.ConfigError{Message: err.Error()}
		}

		// Ensure max backoff is greater than or equal to initial backoff
		if c.Retry.InitialBackoffMs > 0 && c.Retry.MaxBackoffMs < c.Retry.InitialBackoffMs {
			return models.ConfigError{Message: "max backoff must be greater than or equal to initial backoff"}
		}
	}

	if c.Retry.MaxAttempts > 0 {
		if err := validation.ValidateNumericRange(c.Retry.MaxAttempts, "max retry attempts", 1, 10); err != nil {
			return models.ConfigError{Message: err.Error()}
		}
	}

	// Validate channel configuration
	for i, channel := range c.Channels {
		if err := validation.ValidateSessionName(channel.WhatsAppSessionName); err != nil {
			return models.ConfigError{Message: fmt.Sprintf("channel %d WhatsApp session name: %s", i, err.Error())}
		}

		if err := validation.ValidatePhoneNumber(channel.SignalDestinationPhoneNumber); err != nil {
			return models.ConfigError{Message: fmt.Sprintf("channel %d Signal destination: %s", i, err.Error())}
		}
	}

	return nil
}
