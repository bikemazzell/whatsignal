package config

import (
	"encoding/json"
	"fmt"
	"os"
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
	if c.Database.Path == "" {
		return ErrMissingDBPath
	}
	if c.Media.CacheDir == "" {
		return ErrMissingMediaDir
	}

	// Set default media configuration if not provided
	if c.Media.MaxSizeMB.Image == 0 {
		c.Media.MaxSizeMB.Image = 5
	}
	if c.Media.MaxSizeMB.Video == 0 {
		c.Media.MaxSizeMB.Video = 100
	}
	if c.Media.MaxSizeMB.Gif == 0 {
		c.Media.MaxSizeMB.Gif = 25
	}
	if c.Media.MaxSizeMB.Document == 0 {
		c.Media.MaxSizeMB.Document = 100
	}
	if c.Media.MaxSizeMB.Voice == 0 {
		c.Media.MaxSizeMB.Voice = 16
	}

	// Set default allowed types if not provided
	if len(c.Media.AllowedTypes.Image) == 0 {
		c.Media.AllowedTypes.Image = []string{"jpg", "jpeg", "png"}
	}
	if len(c.Media.AllowedTypes.Video) == 0 {
		c.Media.AllowedTypes.Video = []string{"mp4", "mov"}
	}
	if len(c.Media.AllowedTypes.Document) == 0 {
		c.Media.AllowedTypes.Document = []string{"pdf", "doc", "docx"}
	}
	if len(c.Media.AllowedTypes.Voice) == 0 {
		c.Media.AllowedTypes.Voice = []string{"ogg"}
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
	if path := os.Getenv("DB_PATH"); path != "" {
		c.Database.Path = path
	}
	if dir := os.Getenv("MEDIA_DIR"); dir != "" {
		c.Media.CacheDir = dir
	}
}
