package models

type WhatsAppWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		ID        string `json:"id"`
		ChatID    string `json:"chatId"`
		Sender    string `json:"sender"`
		Type      string `json:"type"`
		Content   string `json:"content"`
		MediaPath string `json:"mediaPath,omitempty"`
	} `json:"data"`
}

type SignalWebhookPayload struct {
	MessageID   string   `json:"messageId"`
	Sender      string   `json:"sender"`
	Message     string   `json:"message"`
	Timestamp   int64    `json:"timestamp"`
	Type        string   `json:"type"`
	ThreadID    string   `json:"threadId"`
	Recipient   string   `json:"recipient"`
	MediaPath   string   `json:"mediaPath,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
}
