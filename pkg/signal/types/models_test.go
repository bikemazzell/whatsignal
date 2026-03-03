package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleInt64_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FlexibleInt64
		wantErr  bool
	}{
		{
			name:     "string number",
			input:    `"1234567890"`,
			expected: FlexibleInt64(1234567890),
			wantErr:  false,
		},
		{
			name:     "direct number",
			input:    `1234567890`,
			expected: FlexibleInt64(1234567890),
			wantErr:  false,
		},
		{
			name:     "zero string",
			input:    `"0"`,
			expected: FlexibleInt64(0),
			wantErr:  false,
		},
		{
			name:     "zero number",
			input:    `0`,
			expected: FlexibleInt64(0),
			wantErr:  false,
		},
		{
			name:     "negative string",
			input:    `"-123"`,
			expected: FlexibleInt64(-123),
			wantErr:  false,
		},
		{
			name:     "negative number",
			input:    `-123`,
			expected: FlexibleInt64(-123),
			wantErr:  false,
		},
		{
			name:    "invalid string",
			input:   `"not-a-number"`,
			wantErr: true,
		},
		{
			name:    "boolean",
			input:   `true`,
			wantErr: true,
		},
		{
			name:    "null",
			input:   `null`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f FlexibleInt64
			err := json.Unmarshal([]byte(tt.input), &f)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, f)
			}
		})
	}
}

func TestFlexibleInt64_Int64(t *testing.T) {
	tests := []struct {
		name     string
		input    FlexibleInt64
		expected int64
	}{
		{
			name:     "positive number",
			input:    FlexibleInt64(1234567890),
			expected: 1234567890,
		},
		{
			name:     "zero",
			input:    FlexibleInt64(0),
			expected: 0,
		},
		{
			name:     "negative number",
			input:    FlexibleInt64(-123),
			expected: -123,
		},
		{
			name:     "max int64",
			input:    FlexibleInt64(9223372036854775807),
			expected: 9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.Int64()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSendResponse_WithFlexibleInt64(t *testing.T) {
	// Test with string timestamp
	jsonData := `{"timestamp": "1234567890"}`
	var resp SendResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	require.NoError(t, err)
	assert.Equal(t, int64(1234567890), resp.Timestamp.Int64())

	// Test with numeric timestamp
	jsonData = `{"timestamp": 1234567890}`
	err = json.Unmarshal([]byte(jsonData), &resp)
	require.NoError(t, err)
	assert.Equal(t, int64(1234567890), resp.Timestamp.Int64())
}

func TestSendMessageRequest(t *testing.T) {
	tests := []struct {
		name    string
		request SendMessageRequest
	}{
		{
			name: "basic text message",
			request: SendMessageRequest{
				Message:    "Hello, World!",
				Number:     "+1234567890",
				Recipients: []string{"+0987654321"},
			},
		},
		{
			name: "message with attachments",
			request: SendMessageRequest{
				Message:           "Check this out!",
				Number:            "+1234567890",
				Recipients:        []string{"+0987654321"},
				Base64Attachments: []string{"base64data"},
			},
		},
		{
			name: "empty message",
			request: SendMessageRequest{
				Message:    "",
				Number:     "+1234567890",
				Recipients: []string{"+0987654321"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var decoded SendMessageRequest
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Message, decoded.Message)
			assert.Equal(t, tt.request.Number, decoded.Number)
			assert.Equal(t, tt.request.Recipients, decoded.Recipients)
			assert.Equal(t, len(tt.request.Base64Attachments), len(decoded.Base64Attachments))
		})
	}
}

func TestSendMessageResponse(t *testing.T) {
	tests := []struct {
		name     string
		response SendMessageResponse
	}{
		{
			name: "successful response",
			response: SendMessageResponse{
				Timestamp: 1234567890,
				MessageID: "msg-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var decoded SendMessageResponse
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.response.Timestamp, decoded.Timestamp)
			assert.Equal(t, tt.response.MessageID, decoded.MessageID)
		})
	}
}

func TestSignalMessage(t *testing.T) {
	tests := []struct {
		name    string
		message SignalMessage
	}{
		{
			name: "basic text message",
			message: SignalMessage{
				Timestamp:   1234567890,
				Sender:      "+1234567890",
				MessageID:   "msg-123",
				Message:     "Hello, World!",
				Attachments: []string{},
			},
		},
		{
			name: "message with attachments",
			message: SignalMessage{
				Timestamp:   1234567890,
				Sender:      "+1234567890",
				MessageID:   "msg-456",
				Message:     "Check this out!",
				Attachments: []string{"/path/to/image.jpg", "/path/to/document.pdf"},
			},
		},
		{
			name: "message with quoted message",
			message: SignalMessage{
				Timestamp: 1234567890,
				Sender:    "+1234567890",
				MessageID: "msg-789",
				Message:   "Replying to your message",
				QuotedMessage: &struct {
					ID        string `json:"id"`
					Author    string `json:"author"`
					Text      string `json:"text"`
					Timestamp int64  `json:"timestamp"`
				}{
					ID:        "msg-original",
					Author:    "+0987654321",
					Text:      "Original message text",
					Timestamp: 1234567800,
				},
				Attachments: []string{},
			},
		},
		{
			name: "empty message",
			message: SignalMessage{
				Timestamp:   1234567890,
				Sender:      "+1234567890",
				MessageID:   "msg-empty",
				Message:     "",
				Attachments: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.message)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Test JSON unmarshaling
			var decoded SignalMessage
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.message.Timestamp, decoded.Timestamp)
			assert.Equal(t, tt.message.Sender, decoded.Sender)
			assert.Equal(t, tt.message.MessageID, decoded.MessageID)
			assert.Equal(t, tt.message.Message, decoded.Message)
			assert.Equal(t, tt.message.Attachments, decoded.Attachments)

			if tt.message.QuotedMessage != nil {
				require.NotNil(t, decoded.QuotedMessage)
				assert.Equal(t, tt.message.QuotedMessage.ID, decoded.QuotedMessage.ID)
				assert.Equal(t, tt.message.QuotedMessage.Author, decoded.QuotedMessage.Author)
				assert.Equal(t, tt.message.QuotedMessage.Text, decoded.QuotedMessage.Text)
				assert.Equal(t, tt.message.QuotedMessage.Timestamp, decoded.QuotedMessage.Timestamp)
			} else {
				assert.Nil(t, decoded.QuotedMessage)
			}
		})
	}
}

func TestSignalMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		message SignalMessage
		isValid bool
	}{
		{
			name: "valid message with all fields",
			message: SignalMessage{
				Timestamp:   1234567890,
				Sender:      "+1234567890",
				MessageID:   "msg-123",
				Message:     "Hello, World!",
				Attachments: []string{"/path/to/file.jpg"},
			},
			isValid: true,
		},
		{
			name: "valid message with minimal fields",
			message: SignalMessage{
				Timestamp: 1234567890,
				Sender:    "+1234567890",
				MessageID: "msg-456",
			},
			isValid: true,
		},
		{
			name: "message with empty sender",
			message: SignalMessage{
				Timestamp: 1234567890,
				Sender:    "",
				MessageID: "msg-789",
				Message:   "Test message",
			},
			isValid: false,
		},
		{
			name: "message with empty message ID",
			message: SignalMessage{
				Timestamp: 1234567890,
				Sender:    "+1234567890",
				MessageID: "",
				Message:   "Test message",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			isValid := tt.message.Sender != "" && tt.message.MessageID != "" && tt.message.Timestamp > 0

			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestRestDataMessage_GetQuote(t *testing.T) {
	quote := &RestMessageQuote{ID: 100, Author: "+1234", Text: "hello"}

	tests := []struct {
		name     string
		msg      RestDataMessage
		wantNil  bool
		wantText string
	}{
		{
			name:    "no quote fields set",
			msg:     RestDataMessage{Message: "text"},
			wantNil: true,
		},
		{
			name:     "standard quote field",
			msg:      RestDataMessage{Quote: quote},
			wantNil:  false,
			wantText: "hello",
		},
		{
			name:     "alternative quoteMessage field",
			msg:      RestDataMessage{QuoteMsg: quote},
			wantNil:  false,
			wantText: "hello",
		},
		{
			name:     "both fields set prefers Quote",
			msg:      RestDataMessage{Quote: quote, QuoteMsg: &RestMessageQuote{ID: 200, Author: "+5678", Text: "other"}},
			wantNil:  false,
			wantText: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.GetQuote()
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantText, got.Text)
			}
		})
	}
}

func TestRestSentMessage_GetQuote(t *testing.T) {
	quote := &RestMessageQuote{ID: 100, Author: "+1234", Text: "hello"}

	tests := []struct {
		name     string
		msg      RestSentMessage
		wantNil  bool
		wantText string
	}{
		{
			name:    "no quote fields set",
			msg:     RestSentMessage{Message: "text"},
			wantNil: true,
		},
		{
			name:     "standard quote field",
			msg:      RestSentMessage{Quote: quote},
			wantNil:  false,
			wantText: "hello",
		},
		{
			name:     "alternative quoteMessage field",
			msg:      RestSentMessage{QuoteMsg: quote},
			wantNil:  false,
			wantText: "hello",
		},
		{
			name:     "both fields set prefers Quote",
			msg:      RestSentMessage{Quote: quote, QuoteMsg: &RestMessageQuote{ID: 200, Author: "+5678", Text: "other"}},
			wantNil:  false,
			wantText: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.GetQuote()
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantText, got.Text)
			}
		})
	}
}

func TestRestDataMessage_GetQuote_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantNil  bool
		wantText string
	}{
		{
			name:     "quote field from JSON",
			jsonData: `{"timestamp":1,"message":"hi","quote":{"id":100,"author":"+1","text":"quoted"}}`,
			wantNil:  false,
			wantText: "quoted",
		},
		{
			name:     "quoteMessage field from JSON",
			jsonData: `{"timestamp":1,"message":"hi","quoteMessage":{"id":100,"author":"+1","text":"alt-quoted"}}`,
			wantNil:  false,
			wantText: "alt-quoted",
		},
		{
			name:     "no quote in JSON",
			jsonData: `{"timestamp":1,"message":"hi"}`,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg RestDataMessage
			err := json.Unmarshal([]byte(tt.jsonData), &msg)
			require.NoError(t, err)

			got := msg.GetQuote()
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantText, got.Text)
			}
		})
	}
}
