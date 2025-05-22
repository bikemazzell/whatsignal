package models

import "time"

type Config struct {
	WhatsApp struct {
		APIBaseURL      string        `json:"apiBaseUrl"`
		APIKey          string        `json:"apiKey"`
		SessionName     string        `json:"sessionName"`
		Timeout         time.Duration `json:"timeout"`
		RetryCount      int           `json:"retryCount"`
		WebhookSecret   string        `json:"webhookSecret"`
		PollIntervalSec int           `json:"pollIntervalSec"`
	} `json:"whatsapp"`

	Signal struct {
		RPCURL      string `json:"rpcUrl"`
		AuthToken   string `json:"authToken"`
		PhoneNumber string `json:"phoneNumber"`
		DeviceName  string `json:"deviceName"`
	} `json:"signal"`

	Retry struct {
		InitialBackoffMs int `json:"initialBackoffMs"`
		MaxBackoffMs     int `json:"maxBackoffMs"`
		MaxAttempts      int `json:"maxAttempts"`
	} `json:"retry"`

	RetentionDays int    `json:"retentionDays"`
	LogLevel      string `json:"logLevel"`
}

type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return e.Message
}
