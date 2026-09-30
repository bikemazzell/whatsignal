package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"whatsignal/internal/models"
)

func TestWhatsAppReactionsPreserveReactorNames(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	server := NewServer(&models.Config{}, msgService, logger, nil, createTestChannelManager(), nil, nil)

	// Both people react to the same message. Each reaction must produce its own
	// named notification because Signal sees only the bridge account as author.
	for _, test := range []struct {
		payload string
		want    string
	}{
		{`{"from":"111111111111@lid","notifyName":"Tom","reaction":{"text":"👍","messageId":"wa-original"}}`, "Tom reacted with 👍"},
		{`{"from":"222222222222@lid","notifyName":"Jane","reaction":{"text":"❤️","messageId":"wa-original"}}`, "Jane reacted with ❤️"},
		{`{"from":"111111111111@lid","_data":{"notifyName":"Tom"},"reaction":{"text":"😮","messageId":"wa-original"}}`, "Tom reacted with 😮"},
		{`{"from":"222222222222@lid","_data":{"pushName":"Jane"},"reaction":{"text":"🔥","messageId":"wa-original"}}`, "Jane reacted with 🔥"},
		{`{"from":"111111111111@lid","notifyName":"Tom","reaction":{"text":"","messageId":"wa-original"}}`, "Tom removed reaction from message"},
	} {
		var payload models.WhatsAppWebhookPayload
		require.NoError(t, json.Unmarshal([]byte(`{"event":"message.reaction","session":"default","payload":`+test.payload+`}`), &payload))
		msgService.On("GetMessageMappingByWhatsAppID", mock.Anything, "wa-original").Return(&models.MessageMapping{
			WhatsAppMsgID: "wa-original",
			SignalMsgID:   "1790314875742",
			SessionName:   "default",
		}, nil).Once()
		msgService.On("SendSignalNotification", mock.Anything, "default", test.want).Return(nil).Once()
		// Allow the old path to return so the regression fails on its behavior.
		msgService.On("SendSignalReaction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		require.NoError(t, server.handleWhatsAppReaction(context.Background(), &payload))
	}

	msgService.AssertNotCalled(t, "SendSignalReaction", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	msgService.AssertExpectations(t)
}
