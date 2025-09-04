package service

import (
	"context"
	"testing"
	"time"

	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestGetMessageThread_SuccessSingleMessage(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)
	ctx := context.Background()

	// Mapping exists
	db.On("GetMessageMapping", ctx, "thread-1").Return(&models.MessageMapping{
		WhatsAppMsgID:   "thread-1",
		WhatsAppChatID:  "chat-1",
		SignalTimestamp: time.Now(),
		DeliveryStatus:  models.DeliveryStatusDelivered,
	}, nil).Once()

	msgs, err := service.GetMessageThread(ctx, "thread-1")
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "thread-1", msgs[0].ID)
	assert.Equal(t, "chat-1", msgs[0].ChatID)
	assert.Equal(t, "whatsapp", msgs[0].Platform)
	assert.Equal(t, string(models.DeliveryStatusDelivered), msgs[0].DeliveryStatus)
}

func TestGetMessageThread_NotFoundNilMapping(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)
	ctx := context.Background()

	// DB returns nil mapping without error
	db.On("GetMessageMapping", ctx, "missing").Return(nil, nil).Once()

	msgs, err := service.GetMessageThread(ctx, "missing")
	assert.Error(t, err)
	assert.Nil(t, msgs)
}

func TestGetMessageThread_DBError(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)
	ctx := context.Background()

	db.On("GetMessageMapping", ctx, "boom").Return(nil, assert.AnError).Once()

	msgs, err := service.GetMessageThread(ctx, "boom")
	assert.Error(t, err)
	assert.Nil(t, msgs)
}
