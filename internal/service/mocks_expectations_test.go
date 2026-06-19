package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"whatsignal/pkg/whatsapp/types"
)

func TestMockWhatsAppClientMediaMethodsHonorExpectations(t *testing.T) {
	ctx := context.Background()
	expected := &types.SendMessageResponse{MessageID: "expected-message-id"}

	tests := []struct {
		name string
		call func(*mockWhatsAppClient) (*types.SendMessageResponse, error)
	}{
		{
			name: "image",
			call: func(client *mockWhatsAppClient) (*types.SendMessageResponse, error) {
				client.On("SendImageWithSession", ctx, "chat-1", "/tmp/image.jpg", "caption", "reply-1", "session-1").Return(expected, nil).Once()
				return client.SendImageWithSession(ctx, "chat-1", "/tmp/image.jpg", "caption", "reply-1", "session-1")
			},
		},
		{
			name: "video",
			call: func(client *mockWhatsAppClient) (*types.SendMessageResponse, error) {
				client.On("SendVideoWithSession", ctx, "chat-1", "/tmp/video.mp4", "caption", "reply-1", "session-1").Return(expected, nil).Once()
				return client.SendVideoWithSession(ctx, "chat-1", "/tmp/video.mp4", "caption", "reply-1", "session-1")
			},
		},
		{
			name: "document",
			call: func(client *mockWhatsAppClient) (*types.SendMessageResponse, error) {
				client.On("SendDocumentWithSession", ctx, "chat-1", "/tmp/file.pdf", "caption", "reply-1", "session-1").Return(expected, nil).Once()
				return client.SendDocumentWithSession(ctx, "chat-1", "/tmp/file.pdf", "caption", "reply-1", "session-1")
			},
		},
		{
			name: "voice",
			call: func(client *mockWhatsAppClient) (*types.SendMessageResponse, error) {
				client.On("SendVoiceWithSession", ctx, "chat-1", "/tmp/voice.ogg", "reply-1", "session-1").Return(expected, nil).Once()
				return client.SendVoiceWithSession(ctx, "chat-1", "/tmp/voice.ogg", "reply-1", "session-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockWhatsAppClient{}

			resp, err := tt.call(client)

			require.NoError(t, err)
			require.Same(t, expected, resp)
			client.AssertExpectations(t)
		})
	}
}
