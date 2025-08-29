package service

import (
	"context"
	"time"
	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/mock"
)

// Mock WhatsApp client
type mockWhatsAppClient struct {
	mock.Mock
	sendTextResp     *types.SendMessageResponse
	sendTextErr      error
	sendImageResp    *types.SendMessageResponse
	sendImageErr     error
	sendVideoResp    *types.SendMessageResponse
	sendVideoErr     error
	sendVoiceResp    *types.SendMessageResponse
	sendVoiceErr     error
	sendDocumentResp *types.SendMessageResponse
	sendDocumentErr  error
}

func (m *mockWhatsAppClient) SendText(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
	return m.sendTextResp, m.sendTextErr
}

func (m *mockWhatsAppClient) SendTextWithSession(ctx context.Context, chatID, text, sessionName string) (*types.SendMessageResponse, error) {
	return m.sendTextResp, m.sendTextErr
}

func (m *mockWhatsAppClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) SendImageWithSession(ctx context.Context, chatID, imagePath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	return m.sendVideoResp, m.sendVideoErr
}

func (m *mockWhatsAppClient) SendVideoWithSession(ctx context.Context, chatID, videoPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return m.sendVideoResp, m.sendVideoErr
}

func (m *mockWhatsAppClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	return m.sendDocumentResp, m.sendDocumentErr
}

func (m *mockWhatsAppClient) SendDocumentWithSession(ctx context.Context, chatID, docPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return m.sendDocumentResp, m.sendDocumentErr
}

func (m *mockWhatsAppClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	return m.sendDocumentResp, m.sendDocumentErr
}

func (m *mockWhatsAppClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	return m.sendVoiceResp, m.sendVoiceErr
}

func (m *mockWhatsAppClient) SendVoiceWithSession(ctx context.Context, chatID, voicePath, sessionName string) (*types.SendMessageResponse, error) {
	return m.sendVoiceResp, m.sendVoiceErr
}

func (m *mockWhatsAppClient) SendReaction(ctx context.Context, chatID, messageID, reaction string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWhatsAppClient) SendReactionWithSession(ctx context.Context, chatID, messageID, reaction, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWhatsAppClient) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	args := m.Called(ctx, chatID, messageID)
	return args.Error(0)
}

func (m *mockWhatsAppClient) SendContact(ctx context.Context, chatID, contactID string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWhatsAppClient) SendLocation(ctx context.Context, chatID string, latitude, longitude float64, name, address string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, latitude, longitude, name, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWhatsAppClient) SendSticker(ctx context.Context, chatID, stickerPath string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, stickerPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWhatsAppClient) CreateSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) StartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) StopSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *mockWhatsAppClient) RestartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error {
	args := m.Called(ctx, maxWaitTime)
	return args.Error(0)
}


func (m *mockWhatsAppClient) SendSeen(ctx context.Context, chatID string) error {
	args := m.Called(ctx, chatID)
	return args.Error(0)
}

// Contact methods
func (m *mockWhatsAppClient) GetContact(ctx context.Context, contactID string) (*types.Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Contact), args.Error(1)
}

func (m *mockWhatsAppClient) GetAllContacts(ctx context.Context, limit, offset int) ([]types.Contact, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Contact), args.Error(1)
}

func (m *mockWhatsAppClient) GetSessionName() string {
	return "test-session"
}

// Mock Signal client
type mockSignalClient struct {
	mock.Mock
	sendMessageResp     *signaltypes.SendMessageResponse
	sendMessageErr      error
	initializeDeviceErr error
}

func (m *mockSignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*signaltypes.SendMessageResponse, error) {
	if m.sendMessageResp != nil || m.sendMessageErr != nil {
		return m.sendMessageResp, m.sendMessageErr
	}
	args := m.Called(ctx, recipient, message, attachments)
	if args.Get(0) == nil && args.Error(1) == nil {
		return nil, nil
	}
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*signaltypes.SendMessageResponse), args.Error(1)
}

func (m *mockSignalClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]signaltypes.SignalMessage, error) {
	args := m.Called(ctx, timeoutSeconds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]signaltypes.SignalMessage), args.Error(1)
}

func (m *mockSignalClient) InitializeDevice(ctx context.Context) error {
	if m.initializeDeviceErr != nil {
		return m.initializeDeviceErr
	}
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockSignalClient) DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	args := m.Called(ctx, attachmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockSignalClient) ListAttachments(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// Mock media handler
type mockMediaHandler struct {
	mock.Mock
}

func (h *mockMediaHandler) ProcessMedia(sourcePath string) (string, error) {
	args := h.Called(sourcePath)
	return args.String(0), args.Error(1)
}

func (h *mockMediaHandler) CleanupOldFiles(maxAgeSeconds int64) error {
	args := h.Called(maxAgeSeconds)
	return args.Error(0)
}

// Mock channel manager
type mockChannelManager struct {
	mock.Mock
}

func (m *mockChannelManager) GetSignalDestination(sessionName string) (string, error) {
	args := m.Called(sessionName)
	return args.String(0), args.Error(1)
}

func (m *mockChannelManager) GetWhatsAppSession(destination string) (string, error) {
	args := m.Called(destination)
	return args.String(0), args.Error(1)
}


func (m *mockChannelManager) GetAllWhatsAppSessions() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *mockChannelManager) IsValidSession(sessionName string) bool {
	args := m.Called(sessionName)
	return args.Bool(0)
}

func (m *mockChannelManager) IsValidDestination(destination string) bool {
	args := m.Called(destination)
	return args.Bool(0)
}

func (m *mockChannelManager) GetChannelCount() int {
	args := m.Called()
	return args.Int(0)
}

// Mock database service
type mockDatabaseService struct {
	mock.Mock
}

func (m *mockDatabaseService) SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}

func (m *mockDatabaseService) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, whatsappID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabaseService) GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabaseService) GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, signalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabaseService) GetLatestMessageMappingByWhatsAppChatID(ctx context.Context, whatsappChatID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, whatsappChatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabaseService) GetLatestMessageMapping(ctx context.Context) (*models.MessageMapping, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabaseService) UpdateDeliveryStatus(ctx context.Context, id string, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockDatabaseService) CleanupOldRecords(retentionDays int) error {
	args := m.Called(retentionDays)
	return args.Error(0)
}

// Contact methods for mocking ContactService dependency
func (m *mockDatabaseService) SaveContact(ctx context.Context, contact *models.Contact) error {
	args := m.Called(ctx, contact)
	return args.Error(0)
}

func (m *mockDatabaseService) GetContact(ctx context.Context, contactID string) (*models.Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockDatabaseService) GetContactByPhone(ctx context.Context, phoneNumber string) (*models.Contact, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockDatabaseService) CleanupOldContacts(retentionDays int) error {
	args := m.Called(retentionDays)
	return args.Error(0)
}

func (m *mockDatabaseService) GetLatestMessageMappingBySession(ctx context.Context, sessionName string) (*models.MessageMapping, error) {
	args := m.Called(ctx, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabaseService) HasMessageHistoryBetween(ctx context.Context, sessionName, signalSender string) (bool, error) {
	args := m.Called(ctx, sessionName, signalSender)
	return args.Bool(0), args.Error(1)
}

// Mock contact service
type mockContactService struct {
	mock.Mock
}

func (m *mockContactService) GetContactDisplayName(ctx context.Context, phoneNumber string) string {
	args := m.Called(ctx, phoneNumber)
	return args.String(0)
}

func (m *mockContactService) SyncContacts(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockContactService) GetContact(ctx context.Context, contactID string) (*models.Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockContactService) GetAllContacts(ctx context.Context, limit, offset int) ([]models.Contact, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Contact), args.Error(1)
}

func (m *mockContactService) CleanupOldContacts(retentionDays int) error {
	args := m.Called(retentionDays)
	return args.Error(0)
}

func (m *mockContactService) RefreshContact(ctx context.Context, contactID string) error {
	args := m.Called(ctx, contactID)
	return args.Error(0)
}

func (m *mockContactService) SyncAllContacts(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

