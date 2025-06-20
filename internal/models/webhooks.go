package models

type WhatsAppWebhookPayload struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Event     string `json:"event"`
	Session   string `json:"session"`
	Me        struct {
		ID       string `json:"id"`
		PushName string `json:"pushName"`
	} `json:"me"`
	Payload struct {
		ID        string `json:"id"`
		Timestamp int64  `json:"timestamp"`
		From      string `json:"from"`
		FromMe    bool   `json:"fromMe"`
		To        string `json:"to"`
		Body      string `json:"body"`
		HasMedia  bool   `json:"hasMedia"`
		Media     *struct {
			URL      string `json:"url"`
			MimeType string `json:"mimetype"`
			Filename string `json:"filename"`
		} `json:"media"`
	} `json:"payload"`
	Engine      string `json:"engine"`
	Environment struct {
		Version string `json:"version"`
		Engine  string `json:"engine"`
		Tier    string `json:"tier"`
		Browser string `json:"browser"`
	} `json:"environment"`
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
