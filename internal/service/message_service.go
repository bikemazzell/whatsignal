package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"

	"github.com/sirupsen/logrus"
)

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
	ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error
}

type messageService struct {
	logger     *logrus.Logger
	db         Database
	bridge     MessageBridge
	mediaCache MediaCache
	mu         sync.RWMutex
}

func NewMessageService(bridge MessageBridge, db Database, mediaCache MediaCache) MessageService {
	return &messageService{
		logger:     logrus.New(),
		bridge:     bridge,
		db:         db,
		mediaCache: mediaCache,
		mu:         sync.RWMutex{},
	}
}

// generateUniqueID generates a unique ID for message mapping
func generateUniqueID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (s *messageService) SendMessage(ctx context.Context, msg *models.Message) error {
	// Process media outside of mutex
	if msg.MediaURL != "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		msg.MediaPath = cachePath
	}

	// Send message outside of mutex
	if err := s.bridge.SendMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Generate unique IDs for mapping
	signalMsgID := generateUniqueID()

	mapping := &models.MessageMapping{
		WhatsAppChatID:  msg.ChatID,
		WhatsAppMsgID:   msg.ID,
		SignalMsgID:     signalMsgID,
		SignalTimestamp: msg.Timestamp,
		ForwardedAt:     msg.Timestamp,
		DeliveryStatus:  models.DeliveryStatusSent,
	}
	if msg.MediaPath != "" {
		mapping.MediaPath = &msg.MediaPath
	}

	// Only protect database operations with mutex
	s.mu.Lock()
	err := s.db.SaveMessageMapping(ctx, mapping)
	s.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (s *messageService) ReceiveMessage(ctx context.Context, msg *models.Message) error {
	// Check for existing mapping with read lock
	s.mu.RLock()
	existingMapping, err := s.db.GetMessageMappingByWhatsAppID(ctx, msg.ID)
	s.mu.RUnlock()

	if err == nil && existingMapping != nil {
		return nil
	}

	// Process media outside of mutex
	if msg.MediaURL != "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return err
		}
		msg.MediaPath = cachePath
	}

	// Send message outside of mutex
	err = s.bridge.SendMessage(ctx, msg)
	if err != nil {
		return err
	}

	// Generate unique Signal message ID
	signalMsgID := generateUniqueID()

	mapping := &models.MessageMapping{
		WhatsAppChatID:  msg.ChatID,
		WhatsAppMsgID:   msg.ID,
		SignalMsgID:     signalMsgID,
		SignalTimestamp: msg.Timestamp,
		ForwardedAt:     msg.Timestamp,
		DeliveryStatus:  "received",
	}

	if msg.MediaPath != "" {
		mapping.MediaPath = &msg.MediaPath
	}

	// Only protect database write with mutex
	s.mu.Lock()
	err = s.db.SaveMessageMapping(ctx, mapping)
	s.mu.Unlock()

	return err
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
	s.mu.RLock()
	mapping, err := s.db.GetMessageMapping(ctx, threadID)
	s.mu.RUnlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get message thread: %w", err)
	}
	if mapping == nil {
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}

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

	return nil
}

func (s *messageService) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	// Check for existing mapping with read lock
	s.mu.RLock()
	existingMapping, err := s.db.GetMessageMappingByWhatsAppID(ctx, msgID)
	s.mu.RUnlock()

	if err == nil && existingMapping != nil {
		return nil
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

	// ReceiveMessage handles its own locking
	return s.ReceiveMessage(ctx, msg)
}

func (s *messageService) HandleSignalMessage(ctx context.Context, msg *models.Message) error {
	if msg.Platform != "signal" {
		return fmt.Errorf("HandleSignalMessage called with non-Signal platform: %s", msg.Platform)
	}

	if strings.HasPrefix(msg.Sender, "group.") {
		return fmt.Errorf("group messages are not supported yet")
	}

	// Process media outside of mutex
	if msg.MediaURL != "" && msg.MediaPath == "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return fmt.Errorf("failed to process media for HandleSignalMessage: %w", err)
		}
		msg.MediaPath = cachePath
	}

	// Send message outside of mutex
	if err := s.bridge.SendMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to send Signal message via bridge: %w", err)
	}
	return nil
}

func (s *messageService) ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error {
	s.logger.WithFields(logrus.Fields{
		"signalMsgID": rawSignalMsg.MessageID,
		"sender":      rawSignalMsg.Sender,
	}).Info("Processing incoming Signal message in service layer (ProcessIncomingSignalMessage)")

	// Bridge operations don't need mutex protection
	return s.bridge.HandleSignalMessage(ctx, rawSignalMsg)
}

func (s *messageService) UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.UpdateDeliveryStatus(ctx, msgID, status)
}
