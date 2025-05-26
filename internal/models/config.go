package models

import "time"

// Config holds the application configuration
type Config struct {
	WhatsApp      WhatsAppConfig `mapstructure:"whatsapp"`
	Signal        SignalConfig   `mapstructure:"signal"`
	Database      DatabaseConfig `mapstructure:"database"`
	Media         MediaConfig    `mapstructure:"media"`
	Retry         RetryConfig    `mapstructure:"retry"`
	LogLevel      string         `mapstructure:"log_level"`
	RetentionDays int            `json:"retentionDays"`
}

// WhatsAppConfig holds WhatsApp related configurations
type WhatsAppConfig struct {
	APIBaseURL      string        `mapstructure:"api_base_url"`
	APIKey          string        `mapstructure:"api_key"`
	SessionName     string        `mapstructure:"session_name"`
	Timeout         time.Duration `mapstructure:"timeout_ms"`
	RetryCount      int           `mapstructure:"retry_count"`
	WebhookSecret   string        `mapstructure:"webhook_secret"`
	PollIntervalSec int           `json:"pollIntervalSec"`
}

// SignalConfig holds Signal related configurations
type SignalConfig struct {
	RPCURL        string `mapstructure:"rpc_url"`
	AuthToken     string `mapstructure:"auth_token"`
	PhoneNumber   string `mapstructure:"phone_number"`
	DeviceName    string `mapstructure:"device_name"`
	WebhookSecret string `mapstructure:"webhook_secret"`
}

// DatabaseConfig holds database related configurations
type DatabaseConfig struct {
	Path string `json:"path"`
}

// MediaConfig holds media related configurations
type MediaConfig struct {
	CacheDir string `json:"cacheDir"`
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
