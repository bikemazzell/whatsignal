package models

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

	RetentionDays int    `json:"retentionDays"`
	LogLevel      string `json:"logLevel"`
}

type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return e.Message
}
