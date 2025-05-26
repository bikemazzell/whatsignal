package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendMessageRequest(t *testing.T) {
	tests := []struct {
		name    string
		request SendMessageRequest
	}{
		{
			name: "basic text message",
			request: SendMessageRequest{
				Jsonrpc: "2.0",
				Method:  "send",
				Params: struct {
					Number      string   `json:"number"`
					Message     string   `json:"message"`
					Attachments []string `json:"attachments,omitempty"`
				}{
					Number:  "+1234567890",
					Message: "Hello, World!",
				},
				ID: 1,
			},
		},
		{
			name: "message with attachments",
			request: SendMessageRequest{
				Jsonrpc: "2.0",
				Method:  "send",
				Params: struct {
					Number      string   `json:"number"`
					Message     string   `json:"message"`
					Attachments []string `json:"attachments,omitempty"`
				}{
					Number:      "+1234567890",
					Message:     "Check this out!",
					Attachments: []string{"/path/to/image.jpg", "/path/to/document.pdf"},
				},
				ID: 2,
			},
		},
		{
			name: "empty message",
			request: SendMessageRequest{
				Jsonrpc: "2.0",
				Method:  "send",
				Params: struct {
					Number      string   `json:"number"`
					Message     string   `json:"message"`
					Attachments []string `json:"attachments,omitempty"`
				}{
					Number:  "+1234567890",
					Message: "",
				},
				ID: 3,
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

			assert.Equal(t, tt.request.Jsonrpc, decoded.Jsonrpc)
			assert.Equal(t, tt.request.Method, decoded.Method)
			assert.Equal(t, tt.request.Params.Number, decoded.Params.Number)
			assert.Equal(t, tt.request.Params.Message, decoded.Params.Message)
			assert.Equal(t, tt.request.Params.Attachments, decoded.Params.Attachments)
			assert.Equal(t, tt.request.ID, decoded.ID)
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
				Jsonrpc: "2.0",
				Result: struct {
					Timestamp int64  `json:"timestamp"`
					MessageID string `json:"messageId"`
				}{
					Timestamp: 1234567890,
					MessageID: "msg-123",
				},
				ID: 1,
			},
		},
		{
			name: "error response",
			response: SendMessageResponse{
				Jsonrpc: "2.0",
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -1,
					Message: "Failed to send message",
				},
				ID: 2,
			},
		},
		{
			name: "response with both result and error",
			response: SendMessageResponse{
				Jsonrpc: "2.0",
				Result: struct {
					Timestamp int64  `json:"timestamp"`
					MessageID string `json:"messageId"`
				}{
					Timestamp: 1234567890,
					MessageID: "msg-456",
				},
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    0,
					Message: "Warning: partial success",
				},
				ID: 3,
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

			assert.Equal(t, tt.response.Jsonrpc, decoded.Jsonrpc)
			assert.Equal(t, tt.response.Result.Timestamp, decoded.Result.Timestamp)
			assert.Equal(t, tt.response.Result.MessageID, decoded.Result.MessageID)
			assert.Equal(t, tt.response.ID, decoded.ID)

			if tt.response.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.response.Error.Code, decoded.Error.Code)
				assert.Equal(t, tt.response.Error.Message, decoded.Error.Message)
			} else {
				assert.Nil(t, decoded.Error)
			}
		})
	}
}

func TestReceiveMessageRequest(t *testing.T) {
	tests := []struct {
		name    string
		request ReceiveMessageRequest
	}{
		{
			name: "basic receive request",
			request: ReceiveMessageRequest{
				Jsonrpc: "2.0",
				Method:  "receive",
				Params: struct {
					Timeout int `json:"timeout"`
				}{
					Timeout: 30,
				},
				ID: 1,
			},
		},
		{
			name: "receive request with zero timeout",
			request: ReceiveMessageRequest{
				Jsonrpc: "2.0",
				Method:  "receive",
				Params: struct {
					Timeout int `json:"timeout"`
				}{
					Timeout: 0,
				},
				ID: 2,
			},
		},
		{
			name: "receive request with long timeout",
			request: ReceiveMessageRequest{
				Jsonrpc: "2.0",
				Method:  "receive",
				Params: struct {
					Timeout int `json:"timeout"`
				}{
					Timeout: 3600,
				},
				ID: 3,
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
			var decoded ReceiveMessageRequest
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Jsonrpc, decoded.Jsonrpc)
			assert.Equal(t, tt.request.Method, decoded.Method)
			assert.Equal(t, tt.request.Params.Timeout, decoded.Params.Timeout)
			assert.Equal(t, tt.request.ID, decoded.ID)
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

func TestReceiveMessageResponse(t *testing.T) {
	tests := []struct {
		name     string
		response ReceiveMessageResponse
	}{
		{
			name: "successful response with messages",
			response: ReceiveMessageResponse{
				Jsonrpc: "2.0",
				Result: []SignalMessage{
					{
						Timestamp:   1234567890,
						Sender:      "+1234567890",
						MessageID:   "msg-1",
						Message:     "First message",
						Attachments: []string{},
					},
					{
						Timestamp:   1234567891,
						Sender:      "+0987654321",
						MessageID:   "msg-2",
						Message:     "Second message",
						Attachments: []string{"/path/to/file.pdf"},
					},
				},
				ID: 1,
			},
		},
		{
			name: "empty response",
			response: ReceiveMessageResponse{
				Jsonrpc: "2.0",
				Result:  []SignalMessage{},
				ID:      2,
			},
		},
		{
			name: "error response",
			response: ReceiveMessageResponse{
				Jsonrpc: "2.0",
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -1,
					Message: "Failed to receive messages",
				},
				ID: 3,
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
			var decoded ReceiveMessageResponse
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.response.Jsonrpc, decoded.Jsonrpc)
			assert.Equal(t, len(tt.response.Result), len(decoded.Result))
			assert.Equal(t, tt.response.ID, decoded.ID)

			for i, msg := range tt.response.Result {
				assert.Equal(t, msg.Timestamp, decoded.Result[i].Timestamp)
				assert.Equal(t, msg.Sender, decoded.Result[i].Sender)
				assert.Equal(t, msg.MessageID, decoded.Result[i].MessageID)
				assert.Equal(t, msg.Message, decoded.Result[i].Message)
				assert.Equal(t, msg.Attachments, decoded.Result[i].Attachments)
			}

			if tt.response.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.response.Error.Code, decoded.Error.Code)
				assert.Equal(t, tt.response.Error.Message, decoded.Error.Message)
			} else {
				assert.Nil(t, decoded.Error)
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
