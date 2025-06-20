package models

import "time"

// Config holds the application configuration
type Config struct {
	WhatsApp      WhatsAppConfig `json:"whatsapp" mapstructure:"whatsapp"`
	Signal        SignalConfig   `json:"signal" mapstructure:"signal"`
	Database      DatabaseConfig `json:"database" mapstructure:"database"`
	Media         MediaConfig    `json:"media" mapstructure:"media"`
	Retry         RetryConfig    `json:"retry" mapstructure:"retry"`
	LogLevel      string         `json:"log_level" mapstructure:"log_level"`
	RetentionDays int            `json:"retentionDays"`
}

// WhatsAppConfig holds WhatsApp related configurations
type WhatsAppConfig struct {
	APIBaseURL           string        `json:"api_base_url" mapstructure:"api_base_url"`
	SessionName          string        `json:"session_name" mapstructure:"session_name"`
	Timeout              time.Duration `json:"timeout_ms" mapstructure:"timeout_ms"`
	RetryCount           int           `json:"retry_count" mapstructure:"retry_count"`
	WebhookSecret        string        `json:"webhook_secret" mapstructure:"webhook_secret"`
	PollIntervalSec      int           `json:"pollIntervalSec"`
	ContactSyncOnStartup bool          `json:"contactSyncOnStartup" mapstructure:"contactSyncOnStartup"`
	ContactCacheHours    int           `json:"contactCacheHours" mapstructure:"contactCacheHours"`
}

// SignalConfig holds Signal related configurations
type SignalConfig struct {
	RPCURL                   string `json:"rpc_url" mapstructure:"rpc_url"`
	AuthToken                string `json:"auth_token" mapstructure:"auth_token"`
	IntermediaryPhoneNumber  string `json:"intermediaryPhoneNumber" mapstructure:"intermediaryPhoneNumber"`   // Signal-CLI service number
	DestinationPhoneNumber   string `json:"destinationPhoneNumber" mapstructure:"destinationPhoneNumber"`    // Your Signal number to receive messages
	DeviceName               string `json:"device_name" mapstructure:"device_name"`
	WebhookSecret            string `json:"webhook_secret" mapstructure:"webhook_secret"`
	PollIntervalSec          int    `json:"pollIntervalSec" mapstructure:"pollIntervalSec"`
	PollTimeoutSec           int    `json:"pollTimeoutSec" mapstructure:"pollTimeoutSec"`
	PollingEnabled           bool   `json:"pollingEnabled" mapstructure:"pollingEnabled"`
}

// DatabaseConfig holds database related configurations
type DatabaseConfig struct {
	Path string `json:"path"`
}

// MediaConfig holds media related configurations
type MediaConfig struct {
	CacheDir     string            `json:"cache_dir"`
	MaxSizeMB    MediaSizeLimits   `json:"maxSizeMB"`
	AllowedTypes MediaAllowedTypes `json:"allowedTypes"`
}

// MediaSizeLimits defines size limits for different media types in MB
type MediaSizeLimits struct {
	Image    int `json:"image"`
	Video    int `json:"video"`
	Gif      int `json:"gif"`
	Document int `json:"document"`
	Voice    int `json:"voice"`
}

// MediaAllowedTypes defines allowed file extensions for different media types
type MediaAllowedTypes struct {
	Image    []string `json:"image"`
	Video    []string `json:"video"`
	Document []string `json:"document"`
	Voice    []string `json:"voice"`
}

// RetryConfig holds retry related configurations
type RetryConfig struct {
	InitialBackoffMs int `json:"initialBackoffMs"`
	MaxBackoffMs     int `json:"maxBackoffMs"`
	MaxAttempts      int `json:"maxAttempts"`
}

type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return e.Message
}
