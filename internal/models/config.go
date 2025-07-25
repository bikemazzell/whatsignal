package models

import "time"

// Config holds the application configuration
type Config struct {
	WhatsApp      WhatsAppConfig `json:"whatsapp" mapstructure:"whatsapp"`
	Signal        SignalConfig   `json:"signal" mapstructure:"signal"`
	Database      DatabaseConfig `json:"database" mapstructure:"database"`
	Media         MediaConfig    `json:"media" mapstructure:"media"`
	Retry         RetryConfig    `json:"retry" mapstructure:"retry"`
	Server        ServerConfig   `json:"server" mapstructure:"server"`
	LogLevel      string         `json:"log_level" mapstructure:"log_level"`
	RetentionDays int            `json:"retentionDays"`
	Channels      []Channel      `json:"channels" mapstructure:"channels"` // Multi-channel support
}

// WhatsAppConfig holds WhatsApp related configurations
type WhatsAppConfig struct {
	APIBaseURL               string        `json:"api_base_url" mapstructure:"api_base_url"`
	Timeout                  time.Duration `json:"timeout_ms" mapstructure:"timeout_ms"`
	RetryCount               int           `json:"retry_count" mapstructure:"retry_count"`
	WebhookSecret            string        `json:"webhook_secret" mapstructure:"webhook_secret"`
	PollIntervalSec          int           `json:"pollIntervalSec"`
	ContactSyncOnStartup     bool          `json:"contactSyncOnStartup" mapstructure:"contactSyncOnStartup"`
	ContactCacheHours        int           `json:"contactCacheHours" mapstructure:"contactCacheHours"`
	SessionHealthCheckSec    int           `json:"sessionHealthCheckSec" mapstructure:"sessionHealthCheckSec"`
	SessionAutoRestart       bool          `json:"sessionAutoRestart" mapstructure:"sessionAutoRestart"`
}

// SignalConfig holds Signal related configurations
type SignalConfig struct {
	RPCURL                   string `json:"rpc_url" mapstructure:"rpc_url"`
	IntermediaryPhoneNumber  string `json:"intermediaryPhoneNumber" mapstructure:"intermediaryPhoneNumber"`   // Signal-CLI service number
	DeviceName               string `json:"device_name" mapstructure:"device_name"`
	PollIntervalSec          int    `json:"pollIntervalSec" mapstructure:"pollIntervalSec"`
	PollTimeoutSec           int    `json:"pollTimeoutSec" mapstructure:"pollTimeoutSec"`
	PollingEnabled           bool   `json:"pollingEnabled" mapstructure:"pollingEnabled"`
	AttachmentsDir           string `json:"attachmentsDir" mapstructure:"attachmentsDir"`
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

// ServerConfig holds server related configurations
type ServerConfig struct {
	ReadTimeoutSec  int `json:"readTimeoutSec" mapstructure:"readTimeoutSec"`
	WriteTimeoutSec int `json:"writeTimeoutSec" mapstructure:"writeTimeoutSec"`
	IdleTimeoutSec  int `json:"idleTimeoutSec" mapstructure:"idleTimeoutSec"`
}

// Channel represents a WhatsApp-Signal channel pairing
type Channel struct {
	WhatsAppSessionName           string `json:"whatsappSessionName" mapstructure:"whatsappSessionName"`
	SignalDestinationPhoneNumber  string `json:"signalDestinationPhoneNumber" mapstructure:"signalDestinationPhoneNumber"`
}

type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return e.Message
}
