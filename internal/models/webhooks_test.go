package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhatsAppWebhookPayload_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		payload  WhatsAppWebhookPayload
		expected string
	}{
		{
			name: "complete text message",
			payload: WhatsAppWebhookPayload{
				Event: "message",
				Data: struct {
					ID        string `json:"id"`
					ChatID    string `json:"chatId"`
					Sender    string `json:"sender"`
					Type      string `json:"type"`
					Content   string `json:"content"`
					MediaPath string `json:"mediaPath,omitempty"`
				}{
					ID:      "msg123",
					ChatID:  "chat456",
					Sender:  "user789",
					Type:    "text",
					Content: "Hello, World!",
				},
			},
			expected: `{"event":"message","data":{"id":"msg123","chatId":"chat456","sender":"user789","type":"text","content":"Hello, World!"}}`,
		},
		{
			name: "media message with path",
			payload: WhatsAppWebhookPayload{
				Event: "message",
				Data: struct {
					ID        string `json:"id"`
					ChatID    string `json:"chatId"`
					Sender    string `json:"sender"`
					Type      string `json:"type"`
					Content   string `json:"content"`
					MediaPath string `json:"mediaPath,omitempty"`
				}{
					ID:        "msg124",
					ChatID:    "chat456",
					Sender:    "user789",
					Type:      "image",
					Content:   "Check this out!",
					MediaPath: "/path/to/image.jpg",
				},
			},
			expected: `{"event":"message","data":{"id":"msg124","chatId":"chat456","sender":"user789","type":"image","content":"Check this out!","mediaPath":"/path/to/image.jpg"}}`,
		},
		{
			name: "empty media path omitted",
			payload: WhatsAppWebhookPayload{
				Event: "message",
				Data: struct {
					ID        string `json:"id"`
					ChatID    string `json:"chatId"`
					Sender    string `json:"sender"`
					Type      string `json:"type"`
					Content   string `json:"content"`
					MediaPath string `json:"mediaPath,omitempty"`
				}{
					ID:      "msg125",
					ChatID:  "chat456",
					Sender:  "user789",
					Type:    "text",
					Content: "No media",
				},
			},
			expected: `{"event":"message","data":{"id":"msg125","chatId":"chat456","sender":"user789","type":"text","content":"No media"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(jsonData))

			// Test unmarshaling
			var unmarshaled WhatsAppWebhookPayload
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.payload, unmarshaled)
		})
	}
}

func TestWhatsAppWebhookPayload_JSONUnmarshaling(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected WhatsAppWebhookPayload
		wantErr  bool
	}{
		{
			name:     "valid complete payload",
			jsonData: `{"event":"message","data":{"id":"msg123","chatId":"chat456","sender":"user789","type":"text","content":"Hello!"}}`,
			expected: WhatsAppWebhookPayload{
				Event: "message",
				Data: struct {
					ID        string `json:"id"`
					ChatID    string `json:"chatId"`
					Sender    string `json:"sender"`
					Type      string `json:"type"`
					Content   string `json:"content"`
					MediaPath string `json:"mediaPath,omitempty"`
				}{
					ID:      "msg123",
					ChatID:  "chat456",
					Sender:  "user789",
					Type:    "text",
					Content: "Hello!",
				},
			},
		},
		{
			name:     "with media path",
			jsonData: `{"event":"message","data":{"id":"msg124","chatId":"chat456","sender":"user789","type":"image","content":"Photo","mediaPath":"/media/photo.jpg"}}`,
			expected: WhatsAppWebhookPayload{
				Event: "message",
				Data: struct {
					ID        string `json:"id"`
					ChatID    string `json:"chatId"`
					Sender    string `json:"sender"`
					Type      string `json:"type"`
					Content   string `json:"content"`
					MediaPath string `json:"mediaPath,omitempty"`
				}{
					ID:        "msg124",
					ChatID:    "chat456",
					Sender:    "user789",
					Type:      "image",
					Content:   "Photo",
					MediaPath: "/media/photo.jpg",
				},
			},
		},
		{
			name:     "invalid JSON",
			jsonData: `{"event":"message","data":{"invalid":}`,
			wantErr:  true,
		},
		{
			name:     "missing data field",
			jsonData: `{"event":"message"}`,
			expected: WhatsAppWebhookPayload{
				Event: "message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload WhatsAppWebhookPayload
			err := json.Unmarshal([]byte(tt.jsonData), &payload)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, payload)
		})
	}
}

func TestSignalWebhookPayload_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		payload  SignalWebhookPayload
		expected string
	}{
		{
			name: "basic text message",
			payload: SignalWebhookPayload{
				MessageID: "sig123",
				Sender:    "+1234567890",
				Message:   "Hello Signal!",
				Timestamp: 1234567890,
				Type:      "text",
				ThreadID:  "thread123",
				Recipient: "+0987654321",
			},
			expected: `{"messageId":"sig123","sender":"+1234567890","message":"Hello Signal!","timestamp":1234567890,"type":"text","threadId":"thread123","recipient":"+0987654321"}`,
		},
		{
			name: "message with attachments",
			payload: SignalWebhookPayload{
				MessageID:   "sig124",
				Sender:      "+1234567890",
				Message:     "Check this out!",
				Timestamp:   1234567890,
				Type:        "image",
				ThreadID:    "thread123",
				Recipient:   "+0987654321",
				Attachments: []string{"http://example.com/image.jpg", "http://example.com/doc.pdf"},
			},
			expected: `{"messageId":"sig124","sender":"+1234567890","message":"Check this out!","timestamp":1234567890,"type":"image","threadId":"thread123","recipient":"+0987654321","attachments":["http://example.com/image.jpg","http://example.com/doc.pdf"]}`,
		},
		{
			name: "message with media path",
			payload: SignalWebhookPayload{
				MessageID: "sig125",
				Sender:    "+1234567890",
				Message:   "Local media",
				Timestamp: 1234567890,
				Type:      "video",
				ThreadID:  "thread123",
				Recipient: "+0987654321",
				MediaPath: "/local/video.mp4",
			},
			expected: `{"messageId":"sig125","sender":"+1234567890","message":"Local media","timestamp":1234567890,"type":"video","threadId":"thread123","recipient":"+0987654321","mediaPath":"/local/video.mp4"}`,
		},
		{
			name: "omitted optional fields",
			payload: SignalWebhookPayload{
				MessageID: "sig126",
				Sender:    "+1234567890",
				Message:   "Minimal message",
				Timestamp: 1234567890,
				Type:      "text",
				ThreadID:  "thread123",
				Recipient: "+0987654321",
			},
			expected: `{"messageId":"sig126","sender":"+1234567890","message":"Minimal message","timestamp":1234567890,"type":"text","threadId":"thread123","recipient":"+0987654321"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(jsonData))

			// Test unmarshaling
			var unmarshaled SignalWebhookPayload
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.payload, unmarshaled)
		})
	}
}

func TestSignalWebhookPayload_JSONUnmarshaling(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		expected SignalWebhookPayload
		wantErr  bool
	}{
		{
			name:     "valid minimal payload",
			jsonData: `{"messageId":"sig123","sender":"+1234567890","message":"Hello","timestamp":1234567890,"type":"text","threadId":"thread123","recipient":"+0987654321"}`,
			expected: SignalWebhookPayload{
				MessageID: "sig123",
				Sender:    "+1234567890",
				Message:   "Hello",
				Timestamp: 1234567890,
				Type:      "text",
				ThreadID:  "thread123",
				Recipient: "+0987654321",
			},
		},
		{
			name:     "with all optional fields",
			jsonData: `{"messageId":"sig124","sender":"+1234567890","message":"Complete","timestamp":1234567890,"type":"image","threadId":"thread123","recipient":"+0987654321","mediaPath":"/path/image.jpg","attachments":["http://example.com/file.pdf"]}`,
			expected: SignalWebhookPayload{
				MessageID:   "sig124",
				Sender:      "+1234567890",
				Message:     "Complete",
				Timestamp:   1234567890,
				Type:        "image",
				ThreadID:    "thread123",
				Recipient:   "+0987654321",
				MediaPath:   "/path/image.jpg",
				Attachments: []string{"http://example.com/file.pdf"},
			},
		},
		{
			name:     "empty attachments array",
			jsonData: `{"messageId":"sig125","sender":"+1234567890","message":"No attachments","timestamp":1234567890,"type":"text","threadId":"thread123","recipient":"+0987654321","attachments":[]}`,
			expected: SignalWebhookPayload{
				MessageID:   "sig125",
				Sender:      "+1234567890",
				Message:     "No attachments",
				Timestamp:   1234567890,
				Type:        "text",
				ThreadID:    "thread123",
				Recipient:   "+0987654321",
				Attachments: []string{},
			},
		},
		{
			name:     "invalid JSON",
			jsonData: `{"messageId":"sig126","invalid":}`,
			wantErr:  true,
		},
		{
			name:     "missing required fields handled gracefully",
			jsonData: `{"messageId":"sig127"}`,
			expected: SignalWebhookPayload{
				MessageID: "sig127",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload SignalWebhookPayload
			err := json.Unmarshal([]byte(tt.jsonData), &payload)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, payload)
		})
	}
}

func TestSignalWebhookPayload_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		validate func(t *testing.T, payload SignalWebhookPayload)
	}{
		{
			name:     "null attachments",
			jsonData: `{"messageId":"sig128","sender":"+1234567890","message":"Test","timestamp":1234567890,"type":"text","threadId":"thread123","recipient":"+0987654321","attachments":null}`,
			validate: func(t *testing.T, payload SignalWebhookPayload) {
				assert.Nil(t, payload.Attachments)
			},
		},
		{
			name:     "zero timestamp",
			jsonData: `{"messageId":"sig129","sender":"+1234567890","message":"Test","timestamp":0,"type":"text","threadId":"thread123","recipient":"+0987654321"}`,
			validate: func(t *testing.T, payload SignalWebhookPayload) {
				assert.Equal(t, int64(0), payload.Timestamp)
			},
		},
		{
			name:     "empty strings",
			jsonData: `{"messageId":"","sender":"","message":"","timestamp":1234567890,"type":"","threadId":"","recipient":""}`,
			validate: func(t *testing.T, payload SignalWebhookPayload) {
				assert.Equal(t, "", payload.MessageID)
				assert.Equal(t, "", payload.Sender)
				assert.Equal(t, "", payload.Message)
				assert.Equal(t, "", payload.Type)
				assert.Equal(t, "", payload.ThreadID)
				assert.Equal(t, "", payload.Recipient)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload SignalWebhookPayload
			err := json.Unmarshal([]byte(tt.jsonData), &payload)
			require.NoError(t, err)
			tt.validate(t, payload)
		})
	}
}
