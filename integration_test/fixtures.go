package integration_test

import (
	"fmt"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/whatsapp/types"
)

// TestFixtures provides predefined test data for consistent testing
type TestFixtures struct{}

// NewTestFixtures creates a new TestFixtures instance
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{}
}

// Contacts provides standard test contacts
func (f *TestFixtures) Contacts() map[string]models.Contact {
	return map[string]models.Contact{
		"alice": {
			ContactID:   "1234567890@c.us",
			PhoneNumber: "+1234567890",
			Name:        "Alice Johnson",
			PushName:    "Alice",
			ShortName:   "Alice",
			IsBlocked:   false,
			IsGroup:     false,
			IsMyContact: true,
			CachedAt:    time.Now().Add(-1 * time.Hour),
		},
		"bob": {
			ContactID:   "0987654321@c.us",
			PhoneNumber: "+0987654321",
			Name:        "Bob Smith",
			PushName:    "Bob",
			ShortName:   "Bob",
			IsBlocked:   false,
			IsGroup:     false,
			IsMyContact: true,
			CachedAt:    time.Now().Add(-30 * time.Minute),
		},
		"group": {
			ContactID:   "120363028123456789@g.us",
			PhoneNumber: "",
			Name:        "Test Group",
			PushName:    "",
			ShortName:   "Test Group",
			IsBlocked:   false,
			IsGroup:     true,
			IsMyContact: false,
			CachedAt:    time.Now().Add(-2 * time.Hour),
		},
		"blocked": {
			ContactID:   "5555555555@c.us",
			PhoneNumber: "+5555555555",
			Name:        "Blocked User",
			PushName:    "Blocked",
			ShortName:   "Blocked",
			IsBlocked:   true,
			IsGroup:     false,
			IsMyContact: false,
			CachedAt:    time.Now().Add(-24 * time.Hour),
		},
	}
}

// WhatsAppContacts provides WhatsApp API contact structures
func (f *TestFixtures) WhatsAppContacts() map[string]types.Contact {
	return map[string]types.Contact{
		"alice": {
			ID:          "1234567890@c.us",
			Number:      "+1234567890",
			Name:        "Alice Johnson",
			PushName:    "Alice",
			ShortName:   "Alice",
			IsMe:        false,
			IsGroup:     false,
			IsWAContact: true,
			IsMyContact: true,
			IsBlocked:   false,
		},
		"bob": {
			ID:          "0987654321@c.us",
			Number:      "+0987654321",
			Name:        "Bob Smith",
			PushName:    "Bob",
			ShortName:   "Bob",
			IsMe:        false,
			IsGroup:     false,
			IsWAContact: true,
			IsMyContact: true,
			IsBlocked:   false,
		},
		"group": {
			ID:          "120363028123456789@g.us",
			Number:      "",
			Name:        "Test Group",
			PushName:    "",
			ShortName:   "Test Group",
			IsMe:        false,
			IsGroup:     true,
			IsWAContact: false,
			IsMyContact: false,
			IsBlocked:   false,
		},
	}
}

// MessageMappings provides standard test message mappings
func (f *TestFixtures) MessageMappings() map[string]models.MessageMapping {
	now := time.Now()
	return map[string]models.MessageMapping{
		"text_message": {
			WhatsAppChatID:  "+1111111111@c.us", // Signal sender as WhatsApp chat
			WhatsAppMsgID:   "wamid.test123",
			SignalMsgID:     "signal-msg-123",
			SessionName:     "personal",
			DeliveryStatus:  models.DeliveryStatusDelivered,
			MediaType:       "",
			SignalTimestamp: now.Add(-10 * time.Minute),
			ForwardedAt:     now.Add(-9 * time.Minute),
			CreatedAt:       now.Add(-10 * time.Minute),
			UpdatedAt:       now.Add(-9 * time.Minute),
		},
		"image_message": {
			WhatsAppChatID:  "0987654321@c.us",
			WhatsAppMsgID:   "wamid.img456",
			SignalMsgID:     "signal-img-456",
			SessionName:     "business",
			DeliveryStatus:  models.DeliveryStatusPending,
			MediaType:       "image/jpeg",
			MediaPath:       stringPtr("/tmp/test-image.jpg"),
			SignalTimestamp: now.Add(-5 * time.Minute),
			ForwardedAt:     now.Add(-4 * time.Minute),
			CreatedAt:       now.Add(-5 * time.Minute),
			UpdatedAt:       now.Add(-4 * time.Minute),
		},
		"failed_message": {
			WhatsAppChatID:  "+1111111111@c.us", // Signal sender as WhatsApp chat
			WhatsAppMsgID:   "wamid.failed789",
			SignalMsgID:     "signal-failed-789",
			SessionName:     "personal",
			DeliveryStatus:  models.DeliveryStatusFailed,
			MediaType:       "",
			SignalTimestamp: now.Add(-1 * time.Hour),
			ForwardedAt:     now.Add(-59 * time.Minute),
			CreatedAt:       now.Add(-1 * time.Hour),
			UpdatedAt:       now.Add(-58 * time.Minute),
		},
		"signal_bidirectional": {
			WhatsAppChatID:  "+2222222222@c.us", // Different Signal sender for bidirectional test
			WhatsAppMsgID:   "wamid.bidir123",
			SignalMsgID:     "signal-bidir-123",
			SessionName:     "personal",
			DeliveryStatus:  models.DeliveryStatusDelivered,
			MediaType:       "",
			SignalTimestamp: now.Add(-30 * time.Minute),
			ForwardedAt:     now.Add(-29 * time.Minute),
			CreatedAt:       now.Add(-30 * time.Minute),
			UpdatedAt:       now.Add(-29 * time.Minute),
		},
	}
}

// WhatsAppWebhooks provides standard test webhook payloads
func (f *TestFixtures) WhatsAppWebhooks() map[string]models.WhatsAppWebhookPayload {
	return map[string]models.WhatsAppWebhookPayload{
		"text_message": {
			Session: "personal",
			Event:   models.EventMessage,
			Payload: struct {
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
				Reaction *struct {
					Text      string `json:"text"`
					MessageID string `json:"messageId"`
				} `json:"reaction"`
				EditedMessageID *string `json:"editedMessageId,omitempty"`
				ACK             *int    `json:"ack,omitempty"`
			}{
				ID:        "wamid.test123",
				Timestamp: time.Now().Unix(),
				From:      "+1111111111@c.us",
				FromMe:    false,
				To:        "personal@c.us",
				Body:      "Hello from Alice!",
				HasMedia:  false,
			},
		},
		"image_message": {
			Session: "business",
			Event:   models.EventMessage,
			Payload: struct {
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
				Reaction *struct {
					Text      string `json:"text"`
					MessageID string `json:"messageId"`
				} `json:"reaction"`
				EditedMessageID *string `json:"editedMessageId,omitempty"`
				ACK             *int    `json:"ack,omitempty"`
			}{
				ID:        "wamid.img456",
				Timestamp: time.Now().Unix(),
				From:      "0987654321@c.us",
				FromMe:    false,
				To:        "business@c.us",
				Body:      "",
				HasMedia:  true,
				Media: &struct {
					URL      string `json:"url"`
					MimeType string `json:"mimetype"`
					Filename string `json:"filename"`
				}{
					URL:      "https://example.com/media/image.jpg",
					MimeType: "image/jpeg",
					Filename: "image.jpg",
				},
			},
		},
		"status_update": {
			Session: "personal",
			Event:   models.EventMessageACK,
			Payload: struct {
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
				Reaction *struct {
					Text      string `json:"text"`
					MessageID string `json:"messageId"`
				} `json:"reaction"`
				EditedMessageID *string `json:"editedMessageId,omitempty"`
				ACK             *int    `json:"ack,omitempty"`
			}{
				ID:        "wamid.test123",
				Timestamp: time.Now().Unix(),
				From:      "+1111111111@c.us",
				FromMe:    true,
				To:        "personal@c.us",
				Body:      "",
				HasMedia:  false,
				ACK:       intPtr(2), // Delivered
			},
		},
		"reaction": {
			Session: "personal",
			Event:   models.EventMessage,
			Payload: struct {
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
				Reaction *struct {
					Text      string `json:"text"`
					MessageID string `json:"messageId"`
				} `json:"reaction"`
				EditedMessageID *string `json:"editedMessageId,omitempty"`
				ACK             *int    `json:"ack,omitempty"`
			}{
				ID:        "wamid.reaction789",
				Timestamp: time.Now().Unix(),
				From:      "+1111111111@c.us",
				FromMe:    false,
				To:        "personal@c.us",
				Body:      "",
				HasMedia:  false,
				Reaction: &struct {
					Text      string `json:"text"`
					MessageID string `json:"messageId"`
				}{
					Text:      "👍",
					MessageID: "wamid.test123",
				},
			},
		},
	}
}

// Channels provides standard test channel configurations
func (f *TestFixtures) Channels() []models.Channel {
	return []models.Channel{
		{
			WhatsAppSessionName:          "personal",
			SignalDestinationPhoneNumber: "+1111111111",
		},
		{
			WhatsAppSessionName:          "business",
			SignalDestinationPhoneNumber: "+2222222222",
		},
		{
			WhatsAppSessionName:          "emergency",
			SignalDestinationPhoneNumber: "+3333333333",
		},
	}
}

// Configurations provides standard test configurations
func (f *TestFixtures) Configurations() map[string]*models.Config {
	return map[string]*models.Config{
		"minimal": {
			WhatsApp: models.WhatsAppConfig{
				APIBaseURL:            "http://localhost:3000",
				WebhookSecret:         "test-secret",
				ContactSyncOnStartup:  false,
				ContactCacheHours:     24,
				SessionHealthCheckSec: 300,
				SessionAutoRestart:    true,
			},
			Signal: models.SignalConfig{
				RPCURL:                  "http://localhost:8080",
				IntermediaryPhoneNumber: "+1234567890",
				DeviceName:              "test-device",
				HTTPTimeoutSec:          30,
			},
			Database: models.DatabaseConfig{
				Path:               ":memory:",
				MaxOpenConnections: 10,
				MaxIdleConnections: 5,
				ConnMaxLifetimeSec: 300,
				ConnMaxIdleTimeSec: 60,
			},
			Media: models.MediaConfig{
				CacheDir: "/tmp/whatsignal-test",
				MaxSizeMB: models.MediaSizeLimits{
					Image:    10,
					Video:    50,
					Document: 20,
					Voice:    5,
				},
				AllowedTypes: models.MediaAllowedTypes{
					Image:    []string{".jpg", ".jpeg", ".png", ".gif"},
					Video:    []string{".mp4", ".avi", ".mov"},
					Document: []string{".pdf", ".doc", ".docx"},
					Voice:    []string{".mp3", ".wav", ".ogg"},
				},
				DownloadTimeout: 60,
			},
			Server: models.ServerConfig{
				ReadTimeoutSec:          30,
				WriteTimeoutSec:         30,
				IdleTimeoutSec:          60,
				WebhookMaxSkewSec:       300,
				WebhookMaxBytes:         1048576,
				RateLimitPerMinute:      100,
				RateLimitCleanupMinutes: 60,
				CleanupIntervalHours:    24,
			},
			Channels: []models.Channel{
				{
					WhatsAppSessionName:          "test",
					SignalDestinationPhoneNumber: "+1111111111",
				},
			},
			RetentionDays: 7,
			LogLevel:      "info",
		},
		"multi_channel": {
			WhatsApp: models.WhatsAppConfig{
				APIBaseURL:            "http://localhost:3000",
				WebhookSecret:         "multi-secret",
				ContactSyncOnStartup:  true,
				ContactCacheHours:     12,
				SessionHealthCheckSec: 600,
				SessionAutoRestart:    false,
			},
			Signal: models.SignalConfig{
				RPCURL:                  "http://localhost:8080",
				IntermediaryPhoneNumber: "+9999999999",
				DeviceName:              "multi-device",
				HTTPTimeoutSec:          60,
			},
			Database: models.DatabaseConfig{
				Path:               ":memory:",
				MaxOpenConnections: 20,
				MaxIdleConnections: 10,
				ConnMaxLifetimeSec: 600,
				ConnMaxIdleTimeSec: 120,
			},
			Media: models.MediaConfig{
				CacheDir: "/tmp/whatsignal-multi",
				MaxSizeMB: models.MediaSizeLimits{
					Image:    20,
					Video:    100,
					Document: 50,
					Voice:    10,
				},
				AllowedTypes: models.MediaAllowedTypes{
					Image:    []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
					Video:    []string{".mp4", ".avi", ".mov", ".mkv"},
					Document: []string{".pdf", ".doc", ".docx", ".txt", ".xls"},
					Voice:    []string{".mp3", ".wav", ".ogg", ".aac"},
				},
				DownloadTimeout: 120,
			},
			Server: models.ServerConfig{
				ReadTimeoutSec:          60,
				WriteTimeoutSec:         60,
				IdleTimeoutSec:          120,
				WebhookMaxSkewSec:       600,
				WebhookMaxBytes:         2097152,
				RateLimitPerMinute:      200,
				RateLimitCleanupMinutes: 30,
				CleanupIntervalHours:    12,
			},
			Channels:      f.Channels(),
			RetentionDays: 30,
			LogLevel:      "debug",
		},
	}
}

// SignalWebhookPayload represents a Signal webhook payload for testing
type SignalWebhookPayload struct {
	Envelope struct {
		Source      string `json:"source"`
		SourceName  string `json:"sourceName"`
		SourceUuid  string `json:"sourceUuid"`
		Timestamp   int64  `json:"timestamp"`
		DataMessage struct {
			Timestamp int64  `json:"timestamp"`
			Message   string `json:"message"`
			ExpiresIn int    `json:"expiresIn"`
			ViewOnce  bool   `json:"viewOnce"`
		} `json:"dataMessage"`
	} `json:"envelope"`
	Account string `json:"account"`
}

// TestScenarios provides predefined test scenarios combining multiple fixtures
type TestScenario struct {
	Name            string
	Description     string
	Config          *models.Config
	Contacts        []models.Contact
	Mappings        []models.MessageMapping
	WhatsAppWebhook models.WhatsAppWebhookPayload
	SignalWebhook   SignalWebhookPayload
	ExpectedFlow    string
}

// Scenarios provides comprehensive test scenarios
func (f *TestFixtures) Scenarios() map[string]TestScenario {
	contacts := f.Contacts()
	mappings := f.MessageMappings()
	webhooks := f.WhatsAppWebhooks()
	configs := f.Configurations()

	return map[string]TestScenario{
		"basic_text": {
			Name:            "Basic Text Message",
			Description:     "Simple WhatsApp to Signal text message flow",
			Config:          configs["minimal"],
			Contacts:        []models.Contact{contacts["alice"]},
			Mappings:        []models.MessageMapping{mappings["text_message"]},
			WhatsAppWebhook: webhooks["text_message"],
			ExpectedFlow:    "WhatsApp message → Database → Signal delivery",
		},
		"contact_sync": {
			Name:            "Contact Sync Message",
			Description:     "Message with contact synchronization",
			Config:          configs["minimal"],
			Contacts:        []models.Contact{contacts["alice"]},
			Mappings:        []models.MessageMapping{mappings["text_message"]},
			WhatsAppWebhook: webhooks["text_message"],
			ExpectedFlow:    "WhatsApp message → Contact sync → Signal delivery",
		},
		"group_message": {
			Name:            "Group Message",
			Description:     "Message in a WhatsApp group",
			Config:          configs["minimal"],
			Contacts:        []models.Contact{contacts["group"]},
			Mappings:        []models.MessageMapping{mappings["text_message"]},
			WhatsAppWebhook: webhooks["text_message"],
			ExpectedFlow:    "Group message → Processing → Signal delivery",
		},
		"signal_text": {
			Name:        "Signal Text Message",
			Description: "Signal to WhatsApp text message flow",
			Config:      configs["minimal"],
			Contacts:    []models.Contact{contacts["alice"]},
			Mappings:    []models.MessageMapping{mappings["signal_bidirectional"]},
			SignalWebhook: SignalWebhookPayload{
				Envelope: struct {
					Source      string `json:"source"`
					SourceName  string `json:"sourceName"`
					SourceUuid  string `json:"sourceUuid"`
					Timestamp   int64  `json:"timestamp"`
					DataMessage struct {
						Timestamp int64  `json:"timestamp"`
						Message   string `json:"message"`
						ExpiresIn int    `json:"expiresIn"`
						ViewOnce  bool   `json:"viewOnce"`
					} `json:"dataMessage"`
				}{
					Source:     "+2222222222",
					SourceName: "Test User",
					SourceUuid: "test-uuid-123",
					Timestamp:  time.Now().UnixMilli(),
					DataMessage: struct {
						Timestamp int64  `json:"timestamp"`
						Message   string `json:"message"`
						ExpiresIn int    `json:"expiresIn"`
						ViewOnce  bool   `json:"viewOnce"`
					}{
						Timestamp: time.Now().UnixMilli(),
						Message:   "Hello from Signal!",
						ExpiresIn: 0,
						ViewOnce:  false,
					},
				},
				Account: "+1111111111",
			},
			ExpectedFlow: "Signal message → Database → WhatsApp delivery",
		},
		"signal_reply": {
			Name:        "Signal Reply Message",
			Description: "Reply from Signal to WhatsApp",
			Config:      configs["minimal"],
			Contacts:    []models.Contact{contacts["alice"]},
			Mappings:    []models.MessageMapping{mappings["signal_bidirectional"]},
			SignalWebhook: SignalWebhookPayload{
				Envelope: struct {
					Source      string `json:"source"`
					SourceName  string `json:"sourceName"`
					SourceUuid  string `json:"sourceUuid"`
					Timestamp   int64  `json:"timestamp"`
					DataMessage struct {
						Timestamp int64  `json:"timestamp"`
						Message   string `json:"message"`
						ExpiresIn int    `json:"expiresIn"`
						ViewOnce  bool   `json:"viewOnce"`
					} `json:"dataMessage"`
				}{
					Source:     "+2222222222",
					SourceName: "Business User",
					SourceUuid: "test-uuid-456",
					Timestamp:  time.Now().UnixMilli(),
					DataMessage: struct {
						Timestamp int64  `json:"timestamp"`
						Message   string `json:"message"`
						ExpiresIn int    `json:"expiresIn"`
						ViewOnce  bool   `json:"viewOnce"`
					}{
						Timestamp: time.Now().UnixMilli(),
						Message:   "Thanks for the message!",
						ExpiresIn: 0,
						ViewOnce:  false,
					},
				},
				Account: "+1111111111",
			},
			ExpectedFlow: "Signal reply → Database → WhatsApp delivery",
		},
	}
}

// Helper functions for pointer types
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// MediaSamples provides test media content
type MediaSamples struct{}

// NewMediaSamples creates a new MediaSamples instance
func NewMediaSamples() *MediaSamples {
	return &MediaSamples{}
}

// SmallImage returns a minimal valid PNG image
func (m *MediaSamples) SmallImage() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}

// TestJSON returns a small valid JSON document
func (m *MediaSamples) TestJSON() []byte {
	return []byte(`{"test": "document", "timestamp": "2023-01-01T00:00:00Z"}`)
}

// TestText returns sample text content
func (m *MediaSamples) TestText() []byte {
	return []byte("This is a test document for integration testing.")
}

// RandomTestData generates random test data for load testing
func (f *TestFixtures) RandomTestData(count int) []models.Contact {
	contacts := make([]models.Contact, count)
	for i := 0; i < count; i++ {
		contacts[i] = models.Contact{
			ContactID:   fmt.Sprintf("test%d@c.us", i),
			PhoneNumber: fmt.Sprintf("+1%09d", i),
			Name:        fmt.Sprintf("Test User %d", i),
			PushName:    fmt.Sprintf("User%d", i),
			ShortName:   fmt.Sprintf("U%d", i),
			IsBlocked:   i%10 == 0, // 10% blocked
			IsGroup:     i%5 == 0,  // 20% groups
			IsMyContact: i%3 != 0,  // 66% my contacts
			CachedAt:    time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}
	return contacts
}
