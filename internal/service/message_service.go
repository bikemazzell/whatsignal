package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"whatsignal/internal/models"

	"github.com/sirupsen/logrus"
)

type Bridge interface {
	SendMessage(ctx context.Context, msg *models.Message) error
}

type Database interface {
	SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error
	GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error)
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status string) error
}

type MediaCache interface {
	ProcessMedia(path string) (string, error)
	CleanupOldFiles(maxAge int64) error
}

type MessageService interface {
	SendMessage(ctx context.Context, msg *models.Message) error
	ReceiveMessage(ctx context.Context, msg *models.Message) error
	GetMessageByID(ctx context.Context, id string) (*models.Message, error)
	GetMessageThread(ctx context.Context, threadID string) ([]*models.Message, error)
	MarkMessageDelivered(ctx context.Context, id string) error
	DeleteMessage(ctx context.Context, id string) error
	HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error
	HandleSignalMessage(ctx context.Context, msg *models.Message) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error
}

type messageService struct {
	logger     *logrus.Logger
	db         Database
	bridge     Bridge
	mediaCache MediaCache
	mu         sync.RWMutex
}

func NewMessageService(bridge Bridge, db Database, mediaCache MediaCache) MessageService {
	return &messageService{
		logger:     logrus.New(),
		bridge:     bridge,
		db:         db,
		mediaCache: mediaCache,
		mu:         sync.RWMutex{},
	}
}

func (s *messageService) SendMessage(ctx context.Context, msg *models.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Process media if present
	if msg.MediaURL != "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		msg.MediaPath = cachePath
	}

	// Forward message through bridge
	if err := s.bridge.SendMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Save message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  msg.ChatID,
		WhatsAppMsgID:   msg.ID,
		SignalMsgID:     msg.ID, // TODO: Get from bridge response
		SignalTimestamp: msg.Timestamp,
		ForwardedAt:     msg.Timestamp,
		DeliveryStatus:  models.DeliveryStatusSent,
	}
	if msg.MediaPath != "" {
		mapping.MediaPath = &msg.MediaPath
	}

	if err := s.db.SaveMessageMapping(ctx, mapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (s *messageService) ReceiveMessage(ctx context.Context, msg *models.Message) error {
	// Check if message already processed
	existingMapping, err := s.db.GetMessageMappingByWhatsAppID(ctx, msg.ID)
	if err == nil && existingMapping != nil {
		return nil // Already processed
	}

	// Process media if present
	if msg.MediaURL != "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return err
		}
		msg.MediaPath = cachePath
	}

	// Forward message through bridge
	err = s.bridge.SendMessage(ctx, msg)
	if err != nil {
		return err
	}

	// Save message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  msg.ChatID,
		WhatsAppMsgID:   msg.ID,
		SignalMsgID:     msg.ID, // TODO: Get from bridge response
		SignalTimestamp: msg.Timestamp,
		ForwardedAt:     msg.Timestamp,
		DeliveryStatus:  "received",
	}

	if msg.MediaPath != "" {
		mapping.MediaPath = &msg.MediaPath
	}

	return s.db.SaveMessageMapping(ctx, mapping)
}

func (s *messageService) GetMessageByID(ctx context.Context, id string) (*models.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mapping, err := s.db.GetMessageMapping(ctx, id)
	if err != nil {
		return nil, err
	}
	if mapping == nil {
		return nil, fmt.Errorf("message not found: %s", id)
	}

	msg := &models.Message{
		ID:             mapping.WhatsAppMsgID,
		ChatID:         mapping.WhatsAppChatID,
		Type:           models.TextMessage,
		Platform:       "whatsapp",
		Timestamp:      mapping.SignalTimestamp,
		DeliveryStatus: string(mapping.DeliveryStatus),
	}

	if mapping.MediaPath != nil {
		msg.Type = models.ImageMessage
		msg.MediaPath = *mapping.MediaPath
	}

	return msg, nil
}

func (s *messageService) GetMessageThread(ctx context.Context, threadID string) ([]*models.Message, error) {
	mapping, err := s.db.GetMessageMapping(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message thread: %w", err)
	}
	if mapping == nil {
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}

	// For now, just return a single message
	msg := &models.Message{
		ID:             mapping.WhatsAppMsgID,
		ChatID:         mapping.WhatsAppChatID,
		Type:           models.TextMessage,
		Platform:       "whatsapp",
		DeliveryStatus: string(mapping.DeliveryStatus),
	}

	return []*models.Message{msg}, nil
}

func (s *messageService) MarkMessageDelivered(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.UpdateDeliveryStatus(ctx, id, "delivered")
}

func (s *messageService) DeleteMessage(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: Implement message deletion
	return nil
}

func (s *messageService) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if message already processed
	existingMapping, err := s.db.GetMessageMappingByWhatsAppID(ctx, msgID)
	if err == nil && existingMapping != nil {
		return nil // Already processed
	}

	msg := &models.Message{
		ID:        msgID,
		ChatID:    chatID,
		Content:   content,
		Platform:  "whatsapp",
		Type:      models.TextMessage,
		Timestamp: time.Now(),
		Sender:    sender,
	}

	if mediaPath != "" {
		msg.Type = models.ImageMessage
		msg.MediaURL = mediaPath
	}

	return s.ReceiveMessage(ctx, msg)
}

func (s *messageService) HandleSignalMessage(ctx context.Context, msg *models.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Handle group messages
	if strings.HasPrefix(msg.Sender, "group.") {
		return fmt.Errorf("group messages are not supported yet")
	}

	// Process media if present
	if msg.MediaURL != "" {
		msg.Type = models.ImageMessage
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		msg.MediaPath = cachePath
	}

	// Forward message through bridge
	if err := s.bridge.SendMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Save message mapping
	mapping := &models.MessageMapping{
		SignalMsgID:     msg.ID,
		SignalTimestamp: msg.Timestamp,
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	if msg.MediaPath != "" {
		mapping.MediaPath = &msg.MediaPath
	}

	if err := s.db.SaveMessageMapping(ctx, mapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (s *messageService) UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.UpdateDeliveryStatus(ctx, msgID, status)
}
