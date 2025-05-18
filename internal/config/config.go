package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	WhatsApp struct {
		APIBaseURL      string `json:"apiBaseUrl"`
		WebhookSecret   string `json:"webhookSecret"`
		PollIntervalSec int    `json:"pollIntervalSec"`
	} `json:"whatsapp"`

	Signal struct {
		RPCURL    string `json:"rpcUrl"`
		AuthToken string `json:"authToken"`
	} `json:"signal"`

	Retry struct {
		InitialBackoffMs int `json:"initialBackoffMs"`
		MaxBackoffMs     int `json:"maxBackoffMs"`
		MaxAttempts      int `json:"maxAttempts"`
	} `json:"retry"`

	RetentionDays int `json:"retentionDays"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	config.applyEnvironmentOverrides()
	return &config, nil
}

func (c *Config) validate() error {
	if c.WhatsApp.APIBaseURL == "" {
		return ErrMissingWhatsAppURL
	}
	if c.Signal.RPCURL == "" {
		return ErrMissingSignalURL
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = 30
	}
	if c.WhatsApp.PollIntervalSec <= 0 {
		c.WhatsApp.PollIntervalSec = 30
	}
	return nil
}

func (c *Config) applyEnvironmentOverrides() {
	if url := os.Getenv("WHATSAPP_API_URL"); url != "" {
		c.WhatsApp.APIBaseURL = url
	}
	if secret := os.Getenv("WHATSAPP_WEBHOOK_SECRET"); secret != "" {
		c.WhatsApp.WebhookSecret = secret
	}
	if url := os.Getenv("SIGNAL_RPC_URL"); url != "" {
		c.Signal.RPCURL = url
	}
	if token := os.Getenv("SIGNAL_AUTH_TOKEN"); token != "" {
		c.Signal.AuthToken = token
	}
}

var (
	ErrMissingWhatsAppURL = ConfigError{"missing WhatsApp API URL"}
	ErrMissingSignalURL   = ConfigError{"missing Signal RPC URL"}
)

type ConfigError struct {
	msg string
}

func (e ConfigError) Error() string {
	return e.msg
}
