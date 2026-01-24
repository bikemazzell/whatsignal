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
	HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, senderDisplayName, content string, mediaPath string) error
	HandleSignalMessage(ctx context.Context, msg *models.Message) error
	ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error
	ProcessIncomingSignalMessageWithDestination(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage, destination string) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error
	PollSignalMessages(ctx context.Context) error
	SendSignalNotification(ctx context.Context, sessionName, message string) error
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
}

// chatLockManager provides per-chat locking to ensure message ordering within a chat
type chatLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newChatLockManager() *chatLockManager {
	return &chatLockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

// getLock returns a mutex for the given chat ID, creating one if it doesn't exist
func (m *chatLockManager) getLock(chatID string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locks[chatID] == nil {
		m.locks[chatID] = &sync.Mutex{}
	}
	return m.locks[chatID]
}

// cleanup removes locks that are no longer held (should be called periodically)
func (m *chatLockManager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// In a more sophisticated implementation, we could track usage counts
	// and remove unused locks. For now, we just reset if the map gets too large.
	if len(m.locks) > constants.MaxChatLocks {
		m.locks = make(map[string]*sync.Mutex)
	}
}

type messageService struct {
	logger             *logrus.Logger
	db                 Database
	bridge             MessageBridge
	mediaCache         MediaCache
	signalClient       signal.Client
	signalConfig       models.SignalConfig
	channelManager     *ChannelManager
	mu                 sync.RWMutex
	chatLockManager    *chatLockManager
	inProgressMessages sync.Map // tracks message IDs currently being processed
}

func NewMessageService(bridge MessageBridge, db Database, mediaCache MediaCache, signalClient signal.Client, signalConfig models.SignalConfig, channelManager *ChannelManager) MessageService {
	return &messageService{
		logger:          logrus.New(),
		bridge:          bridge,
		db:              db,
		mediaCache:      mediaCache,
		signalClient:    signalClient,
		signalConfig:    signalConfig,
		channelManager:  channelManager,
		mu:              sync.RWMutex{},
		chatLockManager: newChatLockManager(),
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

func (s *messageService) HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, senderDisplayName, content string, mediaPath string) error {
	// Check if message is already being processed (in-flight deduplication)
	if _, alreadyProcessing := s.inProgressMessages.LoadOrStore(msgID, true); alreadyProcessing {
		s.logger.Debug("Message already being processed, skipping duplicate webhook")
		return nil
	}
	// Ensure we clean up the in-progress marker when done
	defer s.inProgressMessages.Delete(msgID)

	// Check if message was already processed (persisted deduplication)
	s.mu.RLock()
	existingMapping, err := s.db.GetMessageMapping(ctx, msgID)
	s.mu.RUnlock()

	if err == nil && existingMapping != nil {
		s.logger.Debug("Message already processed, skipping")
		return nil
	}

	LogMessageProcessing(ctx, s.logger, "WhatsApp", chatID, msgID, sender, content)

	return s.bridge.HandleWhatsAppMessageWithSession(ctx, sessionName, chatID, msgID, sender, senderDisplayName, content, mediaPath)
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
	// Check if message is already being processed (in-flight deduplication)
	if _, alreadyProcessing := s.inProgressMessages.LoadOrStore(rawSignalMsg.MessageID, true); alreadyProcessing {
		s.logger.WithField("messageID", rawSignalMsg.MessageID).Debug("Signal message already being processed, skipping duplicate")
		return nil
	}
	// Ensure we clean up the in-progress marker when done
	defer s.inProgressMessages.Delete(rawSignalMsg.MessageID)

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

	if len(messages) == 0 {
		return nil
	}

	// Determine number of workers
	numWorkers := s.signalConfig.PollWorkers
	if numWorkers <= 0 {
		numWorkers = constants.DefaultSignalPollWorkers
	}

	// Use worker pool for parallel processing
	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

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
					"sender":    SanitizePhoneNumber(msg.Sender),
					"messageID": SanitizeMessageID(msg.MessageID),
				}).Warn("Could not determine destination for Signal sender, skipping message")
				continue
			}
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore slot
		go func(m signaltypes.SignalMessage, dest string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore slot

			// Acquire per-chat lock to ensure message ordering within a chat
			// Key combines sender and destination to handle multi-channel routing
			chatKey := m.Sender + ":" + dest
			chatLock := s.chatLockManager.getLock(chatKey)
			chatLock.Lock()
			defer chatLock.Unlock()

			if err := s.ProcessIncomingSignalMessageWithDestination(ctx, &m, dest); err != nil {
				if IsVerboseLogging(ctx) {
					s.logger.WithError(err).WithField("messageID", m.MessageID).Error("Failed to process Signal message from polling")
				} else {
					s.logger.WithError(err).Error("Failed to process Signal message from polling")
				}
			}
		}(msg, destination)
	}

	wg.Wait()

	// Opportunistic cleanup of per-chat locks
	s.chatLockManager.cleanup()

	return nil
}

func (s *messageService) determineDestinationForSender(ctx context.Context, sender string, availableDestinations []string) string {
	for _, destination := range availableDestinations {
		if sender == destination {
			session, err := s.channelManager.GetWhatsAppSession(destination)
			if err != nil {
				s.logger.WithError(err).WithField("destination", SanitizePhoneNumber(destination)).Warn("Failed to get WhatsApp session for destination")
				continue
			}

			s.logger.WithFields(logrus.Fields{
				"sender":      SanitizePhoneNumber(sender),
				"destination": SanitizePhoneNumber(destination),
				"session":     session,
			}).Debug("Signal sender matches configured destination, routing to corresponding session")

			return destination
		}
	}

	sessions := s.channelManager.GetAllWhatsAppSessions()

	for _, session := range sessions {
		// Check if we have any message history between this session and the sender
		hasHistory, err := s.db.HasMessageHistoryBetween(ctx, session, sender)
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"session": session,
				"sender":  SanitizePhoneNumber(sender),
			}).Warn("Failed to check message history")
			continue
		}

		if hasHistory {
			destination, err := s.channelManager.GetSignalDestination(session)
			if err != nil {
				s.logger.WithError(err).WithField("session", session).Warn("Failed to get Signal destination for session")
				continue
			}

			// Verify this destination is in our available list
			for _, availableDest := range availableDestinations {
				if destination == availableDest {
					s.logger.WithFields(logrus.Fields{
						"sender":      SanitizePhoneNumber(sender),
						"destination": SanitizePhoneNumber(destination),
						"session":     session,
					}).Debug("Found matching destination based on message history")
					return destination
				}
			}
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
