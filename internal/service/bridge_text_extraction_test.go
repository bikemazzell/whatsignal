package service

import (
	"context"
	"errors"
	"testing"

	"whatsignal/internal/models"
	"whatsignal/pkg/signal/types"
	whtypes "whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)




func TestBridge_HandleSignalMessage_TextExtraction(t *testing.T) {
	tests := []struct {
		name                string
		quotedMessageID     string
		quotedText          string
		dbLookupError       error
		dbLookupResult      *models.MessageMapping
		expectedChatID      string
		expectTextExtraction bool
		expectError         bool
	}{
		{
			name:            "successful database lookup",
			quotedMessageID: "1234567890",
			quotedText:      "📱 1234567890123: original message",
			dbLookupError:   nil,
			dbLookupResult: &models.MessageMapping{
				WhatsAppChatID: "1234567890123@c.us",
			},
			expectedChatID:       "1234567890123@c.us",
			expectTextExtraction: false,
			expectError:          false,
		},
		{
			name:                 "database lookup fails, successful text extraction",
			quotedMessageID:      "9999999999",
			quotedText:           "📱 1234567890123: hello world",
			dbLookupError:        errors.New("not found"),
			dbLookupResult:       nil,
			expectedChatID:       "1234567890123@c.us",
			expectTextExtraction: true,
			expectError:          false,
		},
		{
			name:                 "text extraction with different phone number",
			quotedMessageID:      "8888888888",
			quotedText:           "📱 1234567890: test message",
			dbLookupError:        errors.New("not found"),
			dbLookupResult:       nil,
			expectedChatID:       "1234567890@c.us",
			expectTextExtraction: true,
			expectError:          false,
		},
		{
			name:                 "text extraction with complex message",
			quotedMessageID:      "7777777777",
			quotedText:           "📱 555123456: This is a longer message with punctuation!",
			dbLookupError:        errors.New("not found"),
			dbLookupResult:       nil,
			expectedChatID:       "555123456@c.us",
			expectTextExtraction: true,
			expectError:          false,
		},
		{
			name:                 "text extraction fails - no phone emoji",
			quotedMessageID:      "6666666666",
			quotedText:           "regular message without formatting",
			dbLookupError:        errors.New("not found"),
			dbLookupResult:       nil,
			expectedChatID:       "",
			expectTextExtraction: true,
			expectError:          true,
		},
		{
			name:                 "text extraction fails - no colon separator",
			quotedMessageID:      "5555555555",
			quotedText:           "📱 1234567890123 missing colon",
			dbLookupError:        errors.New("not found"),
			dbLookupResult:       nil,
			expectedChatID:       "",
			expectTextExtraction: true,
			expectError:          true,
		},
		{
			name:                 "text extraction fails - malformed phone",
			quotedMessageID:      "4444444444",
			quotedText:           "📱 : empty phone number",
			dbLookupError:        errors.New("not found"),
			dbLookupResult:       nil,
			expectedChatID:       "",
			expectTextExtraction: true,
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWA := &mockWhatsAppClient{}
			mockSig := &mockSignalClient{}
			mockDB := &mockDatabaseService{}
			mockMedia := &mockMediaHandler{}

			// Setup database mock
			if tt.expectTextExtraction && tt.dbLookupResult == nil {
				mockDB.On("GetMessageMappingBySignalID", mock.Anything, tt.quotedMessageID).Return((*models.MessageMapping)(nil), tt.dbLookupError)
			} else {
				mockDB.On("GetMessageMappingBySignalID", mock.Anything, tt.quotedMessageID).Return(tt.dbLookupResult, tt.dbLookupError)
			}

			// Setup WhatsApp client mock if we expect a successful call
			if !tt.expectError {
				mockWA.On("SendText", mock.Anything, tt.expectedChatID, "test reply").Return(&whtypes.SendMessageResponse{
					Status:    "sent",
					MessageID: "wa_msg_123",
				}, nil)

				mockDB.On("SaveMessageMapping", mock.Anything, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
					return mapping.WhatsAppChatID == tt.expectedChatID
				})).Return(nil)
			}

			// Create bridge without contact service for basic tests
			bridge := NewBridge(mockWA, mockSig, mockDB, mockMedia, models.RetryConfig{}, "+0987654321", nil)

			ctx := context.Background()
			signalMsg := &types.SignalMessage{
				MessageID: "signal_msg_456",
				Sender:    "+1234567890",
				Message:   "test reply",
				Timestamp: 1234567890,
				QuotedMessage: &struct {
					ID        string `json:"id"`
					Author    string `json:"author"`
					Text      string `json:"text"`
					Timestamp int64  `json:"timestamp"`
				}{
					ID:     tt.quotedMessageID,
					Author: "+0987654321",
					Text:   tt.quotedText,
				},
			}

			err := bridge.HandleSignalMessage(ctx, signalMsg)

			if tt.expectError {
				assert.Error(t, err)
				if tt.quotedMessageID != "" {
					assert.Contains(t, err.Error(), "no mapping found for quoted message")
				}
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
			if !tt.expectError {
				mockWA.AssertExpectations(t)
			}
		})
	}
}

func TestBridge_TextExtractionLogic(t *testing.T) {
	// Test the specific text extraction logic in isolation
	tests := []struct {
		name           string
		quotedText     string
		expectedPhone  string
		expectedChatID string
		shouldExtract  bool
	}{
		{
			name:           "standard format",
			quotedText:     "📱 1234567890123: hello",
			expectedPhone:  "1234567890123",
			expectedChatID: "1234567890123@c.us",
			shouldExtract:  true,
		},
		{
			name:           "with country code",
			quotedText:     "📱 +1234567890: message",
			expectedPhone:  "+1234567890",
			expectedChatID: "+1234567890@c.us",
			shouldExtract:  true,
		},
		{
			name:           "international format",
			quotedText:     "📱 441234567890: test",
			expectedPhone:  "441234567890",
			expectedChatID: "441234567890@c.us",
			shouldExtract:  true,
		},
		{
			name:          "missing phone emoji",
			quotedText:    "441234567890: test",
			shouldExtract: false,
		},
		{
			name:          "missing colon",
			quotedText:    "📱 441234567890 test",
			shouldExtract: false,
		},
		{
			name:          "empty phone",
			quotedText:    "📱 : test",
			shouldExtract: false,
		},
		{
			name:          "multiple colons",
			quotedText:    "📱 441234567890: message: with: colons",
			expectedPhone: "441234567890",
			expectedChatID: "441234567890@c.us",
			shouldExtract: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the extraction logic by checking if it would work in the bridge
			mockWA := &mockWhatsAppClient{}
			mockSig := &mockSignalClient{}
			mockDB := &mockDatabaseService{}
			mockMedia := &mockMediaHandler{}

			// Always return error for database lookup to force text extraction
			mockDB.On("GetMessageMappingBySignalID", mock.Anything, "test_msg_id").Return((*models.MessageMapping)(nil), errors.New("not found"))

			if tt.shouldExtract {
				mockWA.On("SendText", mock.Anything, tt.expectedChatID, "reply").Return(&whtypes.SendMessageResponse{
					Status:    "sent",
					MessageID: "wa_msg_123",
				}, nil)

				mockDB.On("SaveMessageMapping", mock.Anything, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
					return mapping.WhatsAppChatID == tt.expectedChatID
				})).Return(nil)
			}

			// Create bridge without contact service for basic tests
			bridge := NewBridge(mockWA, mockSig, mockDB, mockMedia, models.RetryConfig{}, "+0987654321", nil)

			ctx := context.Background()
			signalMsg := &types.SignalMessage{
				MessageID: "signal_reply",
				Sender:    "+1111111111",
				Message:   "reply",
				Timestamp: 1234567890,
				QuotedMessage: &struct {
					ID        string `json:"id"`
					Author    string `json:"author"`
					Text      string `json:"text"`
					Timestamp int64  `json:"timestamp"`
				}{
					ID:     "test_msg_id",
					Author: "+0987654321",
					Text:   tt.quotedText,
				},
			}

			err := bridge.HandleSignalMessage(ctx, signalMsg)

			if tt.shouldExtract {
				assert.NoError(t, err)
				mockWA.AssertExpectations(t)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no mapping found")
			}

			mockDB.AssertExpectations(t)
		})
	}
}