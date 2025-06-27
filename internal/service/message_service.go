package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/models"
	"whatsignal/pkg/signal"
	signaltypes "whatsignal/pkg/signal/types"

	"github.com/sirupsen/logrus"
)

type Database interface {
	SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error
	GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error)
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
	GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error)
	HasMessageHistoryBetween(ctx context.Context, sessionName, signalSender string) (bool, error)
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
	HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, content string, mediaPath string) error
	HandleSignalMessage(ctx context.Context, msg *models.Message) error
	ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error
	ProcessIncomingSignalMessageWithDestination(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage, destination string) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error
	PollSignalMessages(ctx context.Context) error
	SendSignalNotification(ctx context.Context, sessionName, message string) error
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
}

type messageService struct {
	logger         *logrus.Logger
	db             Database
	bridge         MessageBridge
	mediaCache     MediaCache
	signalClient   signal.Client
	signalConfig   models.SignalConfig
	channelManager *ChannelManager
	mu             sync.RWMutex
}

func NewMessageService(bridge MessageBridge, db Database, mediaCache MediaCache, signalClient signal.Client, signalConfig models.SignalConfig, channelManager *ChannelManager) MessageService {
	return &messageService{
		logger:         logrus.New(),
		bridge:         bridge,
		db:             db,
		mediaCache:     mediaCache,
		signalClient:   signalClient,
		signalConfig:   signalConfig,
		channelManager: channelManager,
		mu:             sync.RWMutex{},
	}
}

func generateUniqueID() string {
	bytes := make([]byte, constants.MessageIDRandomBytesLength)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("fallback_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (s *messageService) SendMessage(ctx context.Context, msg *models.Message) error {
	if msg.MediaURL != "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		msg.MediaPath = cachePath
	}

	if err := s.bridge.SendMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

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

	s.mu.Lock()
	err := s.db.SaveMessageMapping(ctx, mapping)
	s.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (s *messageService) ReceiveMessage(ctx context.Context, msg *models.Message) error {
	s.mu.RLock()
	existingMapping, err := s.db.GetMessageMapping(ctx, msg.ID)
	s.mu.RUnlock()

	if err == nil && existingMapping != nil {
		return nil
	}

	if msg.MediaURL != "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return err
		}
		msg.MediaPath = cachePath
	}

	err = s.bridge.SendMessage(ctx, msg)
	if err != nil {
		return err
	}

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
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	if mapping == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("thread not found: %s", threadID)
	}
	s.mu.RUnlock()

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

func (s *messageService) HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, content string, mediaPath string) error {
	s.mu.RLock()
	existingMapping, err := s.db.GetMessageMapping(ctx, msgID)
	s.mu.RUnlock()

	if err == nil && existingMapping != nil {
		s.logger.Debug("Message already processed, skipping")
		return nil
	}

	LogMessageProcessing(ctx, s.logger, "WhatsApp", chatID, msgID, sender, content)

	return s.bridge.HandleWhatsAppMessageWithSession(ctx, sessionName, chatID, msgID, sender, content, mediaPath)
}

func (s *messageService) HandleSignalMessage(ctx context.Context, msg *models.Message) error {
	if msg.Platform != "signal" {
		return fmt.Errorf("HandleSignalMessage called with non-Signal platform: %s", msg.Platform)
	}

	if strings.HasPrefix(msg.Sender, "group.") {
		return fmt.Errorf("group messages are not supported yet")
	}

	if msg.MediaURL != "" && msg.MediaPath == "" {
		cachePath, err := s.mediaCache.ProcessMedia(msg.MediaURL)
		if err != nil {
			return fmt.Errorf("failed to process media for HandleSignalMessage: %w", err)
		}
		msg.MediaPath = cachePath
	}

	if err := s.bridge.SendMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to send Signal message via bridge: %w", err)
	}
	return nil
}

func (s *messageService) ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error {
	// Legacy method - uses bridge's default logic
	return s.bridge.HandleSignalMessage(ctx, rawSignalMsg)
}

func (s *messageService) ProcessIncomingSignalMessageWithDestination(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage, destination string) error {
	LogMessageProcessing(ctx, s.logger, "Signal", "", rawSignalMsg.MessageID, rawSignalMsg.Sender, rawSignalMsg.Message)


	return s.bridge.HandleSignalMessageWithDestination(ctx, rawSignalMsg, destination)
}

func (s *messageService) UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.UpdateDeliveryStatus(ctx, msgID, status)
}

func (s *messageService) PollSignalMessages(ctx context.Context) error {
	
	pollTimeout := s.signalConfig.PollTimeoutSec
	if pollTimeout <= 0 {
		pollTimeout = s.signalConfig.PollIntervalSec
	}
	messages, err := s.signalClient.ReceiveMessages(ctx, pollTimeout)
	if err != nil {
		return fmt.Errorf("failed to poll Signal messages: %w", err)
	}

	LogSignalPolling(ctx, s.logger, len(messages))

	for _, msg := range messages {
		// For polled messages, we need to determine the correct Signal destination
		destinations := s.channelManager.GetAllSignalDestinations()
		if len(destinations) == 0 {
			s.logger.Error("No Signal destinations configured")
			continue
		}
		
		var destination string
		if len(destinations) == 1 {
			// If there's only one channel, use its destination
			destination = destinations[0]
		} else {
			// For multiple channels, determine destination based on message history
			// Check which session has previously communicated with this Signal sender
			destination = s.determineDestinationForSender(ctx, msg.Sender, destinations)
			if destination == "" {
				s.logger.WithFields(logrus.Fields{
					"sender": SanitizePhoneNumber(msg.Sender),
					"messageID": SanitizeMessageID(msg.MessageID),
				}).Warn("Could not determine destination for Signal sender, skipping message")
				continue
			}
		}
		
		if err := s.ProcessIncomingSignalMessageWithDestination(ctx, &msg, destination); err != nil {
			if IsVerboseLogging(ctx) {
				s.logger.WithError(err).WithField("messageID", msg.MessageID).Error("Failed to process Signal message from polling")
			} else {
				s.logger.WithError(err).Error("Failed to process Signal message from polling")
			}
		}
	}

	return nil
}

// determineDestinationForSender finds which Signal destination should handle a message from a given sender
// by checking message history to see which session has previously communicated with this sender
func (s *messageService) determineDestinationForSender(ctx context.Context, sender string, availableDestinations []string) string {
	// Check each destination to see if we have message history with this sender
	for _, destination := range availableDestinations {
		// Get the session for this destination
		session, err := s.channelManager.GetWhatsAppSession(destination)
		if err != nil {
			s.logger.WithError(err).WithField("destination", destination).Warn("Failed to get session for destination")
			continue
		}
		
		// Check if we have any message history between this session and the sender
		// We look for any message mapping where the Signal sender matches and the session matches
		hasHistory, err := s.db.HasMessageHistoryBetween(ctx, session, sender)
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"session": session,
				"sender": SanitizePhoneNumber(sender),
			}).Warn("Failed to check message history")
			continue
		}
		
		if hasHistory {
			s.logger.WithFields(logrus.Fields{
				"sender": SanitizePhoneNumber(sender),
				"destination": SanitizePhoneNumber(destination),
				"session": session,
			}).Debug("Found matching destination based on message history")
			return destination
		}
	}
	
	s.logger.WithField("sender", SanitizePhoneNumber(sender)).Debug("No message history found for sender")
	return ""
}

func (s *messageService) SendSignalNotification(ctx context.Context, sessionName, message string) error {
	// Use the bridge to send a Signal notification for the given session
	// This will handle session-to-destination mapping automatically
	return s.bridge.SendSignalNotificationForSession(ctx, sessionName, message)
}

func (s *messageService) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.GetMessageMappingByWhatsAppID(ctx, whatsappID)
}
