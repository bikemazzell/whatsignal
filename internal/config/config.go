package config

import (
	"encoding/json"
	"os"
	"whatsignal/internal/models"
)

var (
	ErrMissingWhatsAppURL = models.ConfigError{Message: "missing WhatsApp API URL"}
	ErrMissingSignalURL   = models.ConfigError{Message: "missing Signal RPC URL"}
)

func LoadConfig(path string) (*models.Config, error) {
	file, err := os.ReadFile(path)
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
	return &config, nil
}

func validate(c *models.Config) error {
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

func applyEnvironmentOverrides(c *models.Config) {
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
