package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWAClient is a mock implementation of the WAClient interface
type MockWAClient struct {
	mock.Mock
}

func (m *MockWAClient) SendText(ctx context.Context, chatID, message string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, message)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, imagePath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, videoPath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, docPath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, filePath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) SendVoice(ctx context.Context, chatID, voicePath string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, voicePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) CreateSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWAClient) SendReaction(ctx context.Context, chatID, messageID, reaction string) (*SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockWAClient) StartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWAClient) StopSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWAClient) GetSessionStatus(ctx context.Context) (*Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Session), args.Error(1)
}

func (m *MockWAClient) GetContact(ctx context.Context, contactID string) (*Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Contact), args.Error(1)
}

func (m *MockWAClient) GetAllContacts(ctx context.Context, limit, offset int) ([]Contact, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Contact), args.Error(1)
}

func (m *MockWAClient) RestartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWAClient) WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error {
	args := m.Called(ctx, maxWaitTime)
	return args.Error(0)
}

// MockSessionManager is a mock implementation of the SessionManager interface
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) Create(ctx context.Context, name string) (*Session, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Session), args.Error(1)
}

func (m *MockSessionManager) Get(ctx context.Context, name string) (*Session, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Session), args.Error(1)
}

func (m *MockSessionManager) Start(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockSessionManager) Stop(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockSessionManager) Delete(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

// MockWebhookHandler is a mock implementation of the WebhookHandler interface
type MockWebhookHandler struct {
	mock.Mock
}

func (m *MockWebhookHandler) Handle(ctx context.Context, event *WebhookEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockWebhookHandler) RegisterEventHandler(eventType string, handler func(context.Context, []byte) error) {
	m.Called(eventType, handler)
}

func TestWAClientInterface(t *testing.T) {
	// Test that MockWAClient implements WAClient interface
	var client WAClient = &MockWAClient{}
	assert.NotNil(t, client)
}

func TestSessionManagerInterface(t *testing.T) {
	// Test that MockSessionManager implements SessionManager interface
	var manager SessionManager = &MockSessionManager{}
	assert.NotNil(t, manager)
}

func TestWebhookHandlerInterface(t *testing.T) {
	// Test that MockWebhookHandler implements WebhookHandler interface
	var handler WebhookHandler = &MockWebhookHandler{}
	assert.NotNil(t, handler)
}

func TestMockWAClientSendText(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		MessageID: "msg-123",
		Status:    "sent",
	}

	mockClient.On("SendText", ctx, "chat123", "Hello, World!").Return(expectedResponse, nil)

	response, err := mockClient.SendText(ctx, "chat123", "Hello, World!")
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockWAClientSendImage(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		MessageID: "msg-456",
		Status:    "sent",
	}

	mockClient.On("SendImage", ctx, "chat123", "/path/to/image.jpg", "Check this out!").Return(expectedResponse, nil)

	response, err := mockClient.SendImage(ctx, "chat123", "/path/to/image.jpg", "Check this out!")
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockWAClientSendVideo(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		MessageID: "msg-789",
		Status:    "sent",
	}

	mockClient.On("SendVideo", ctx, "chat123", "/path/to/video.mp4", "Watch this!").Return(expectedResponse, nil)

	response, err := mockClient.SendVideo(ctx, "chat123", "/path/to/video.mp4", "Watch this!")
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockWAClientSendDocument(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		MessageID: "msg-101",
		Status:    "sent",
	}

	mockClient.On("SendDocument", ctx, "chat123", "/path/to/document.pdf", "Important document").Return(expectedResponse, nil)

	response, err := mockClient.SendDocument(ctx, "chat123", "/path/to/document.pdf", "Important document")
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockWAClientSendFile(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		MessageID: "msg-102",
		Status:    "sent",
	}

	mockClient.On("SendFile", ctx, "chat123", "/path/to/file.zip", "Archive file").Return(expectedResponse, nil)

	response, err := mockClient.SendFile(ctx, "chat123", "/path/to/file.zip", "Archive file")
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockWAClientSendVoice(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		MessageID: "msg-103",
		Status:    "sent",
	}

	mockClient.On("SendVoice", ctx, "chat123", "/path/to/voice.mp3").Return(expectedResponse, nil)

	response, err := mockClient.SendVoice(ctx, "chat123", "/path/to/voice.mp3")
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockWAClientSessionOperations(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	// Test CreateSession
	mockClient.On("CreateSession", ctx).Return(nil)
	err := mockClient.CreateSession(ctx)
	assert.NoError(t, err)

	// Test StartSession
	mockClient.On("StartSession", ctx).Return(nil)
	err = mockClient.StartSession(ctx)
	assert.NoError(t, err)

	// Test StopSession
	mockClient.On("StopSession", ctx).Return(nil)
	err = mockClient.StopSession(ctx)
	assert.NoError(t, err)

	// Test GetSessionStatus
	expectedSession := &Session{
		Name:   "test-session",
		Status: SessionStatusRunning,
	}
	mockClient.On("GetSessionStatus", ctx).Return(expectedSession, nil)
	session, err := mockClient.GetSessionStatus(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expectedSession, session)

	mockClient.AssertExpectations(t)
}

func TestMockSessionManagerOperations(t *testing.T) {
	mockManager := &MockSessionManager{}
	ctx := context.Background()

	expectedSession := &Session{
		Name:   "test-session",
		Status: SessionStatusInitialized,
	}

	// Test Create
	mockManager.On("Create", ctx, "test-session").Return(expectedSession, nil)
	session, err := mockManager.Create(ctx, "test-session")
	assert.NoError(t, err)
	assert.Equal(t, expectedSession, session)

	// Test Get
	mockManager.On("Get", ctx, "test-session").Return(expectedSession, nil)
	session, err = mockManager.Get(ctx, "test-session")
	assert.NoError(t, err)
	assert.Equal(t, expectedSession, session)

	// Test Start
	mockManager.On("Start", ctx, "test-session").Return(nil)
	err = mockManager.Start(ctx, "test-session")
	assert.NoError(t, err)

	// Test Stop
	mockManager.On("Stop", ctx, "test-session").Return(nil)
	err = mockManager.Stop(ctx, "test-session")
	assert.NoError(t, err)

	// Test Delete
	mockManager.On("Delete", ctx, "test-session").Return(nil)
	err = mockManager.Delete(ctx, "test-session")
	assert.NoError(t, err)

	mockManager.AssertExpectations(t)
}

func TestMockWebhookHandlerOperations(t *testing.T) {
	mockHandler := &MockWebhookHandler{}
	ctx := context.Background()

	// Test Handle
	event := &WebhookEvent{
		Event:   "message.any",
		Payload: []byte(`{"test": "data"}`),
	}
	mockHandler.On("Handle", ctx, event).Return(nil)
	err := mockHandler.Handle(ctx, event)
	assert.NoError(t, err)

	// Test RegisterEventHandler
	handler := func(ctx context.Context, payload []byte) error {
		return nil
	}
	mockHandler.On("RegisterEventHandler", "message.any", mock.AnythingOfType("func(context.Context, []uint8) error")).Return()
	mockHandler.RegisterEventHandler("message.any", handler)

	mockHandler.AssertExpectations(t)
}

func TestInterfaceCompliance(t *testing.T) {
	// Verify that all mock types satisfy their respective interfaces at compile time
	var _ WAClient = (*MockWAClient)(nil)
	var _ SessionManager = (*MockSessionManager)(nil)
	var _ WebhookHandler = (*MockWebhookHandler)(nil)
}

func TestWAClientErrorHandling(t *testing.T) {
	mockClient := &MockWAClient{}
	ctx := context.Background()

	// Test error responses
	mockClient.On("SendText", ctx, "chat123", "Hello").Return(nil, assert.AnError)
	response, err := mockClient.SendText(ctx, "chat123", "Hello")
	assert.Error(t, err)
	assert.Nil(t, response)

	mockClient.On("CreateSession", ctx).Return(assert.AnError)
	err = mockClient.CreateSession(ctx)
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
}

func TestSessionManagerErrorHandling(t *testing.T) {
	mockManager := &MockSessionManager{}
	ctx := context.Background()

	// Test error responses
	mockManager.On("Create", ctx, "test-session").Return(nil, assert.AnError)
	session, err := mockManager.Create(ctx, "test-session")
	assert.Error(t, err)
	assert.Nil(t, session)

	mockManager.On("Start", ctx, "test-session").Return(assert.AnError)
	err = mockManager.Start(ctx, "test-session")
	assert.Error(t, err)

	mockManager.AssertExpectations(t)
}

func TestWebhookHandlerErrorHandling(t *testing.T) {
	mockHandler := &MockWebhookHandler{}
	ctx := context.Background()

	event := &WebhookEvent{
		Event:   "message.any",
		Payload: []byte(`{"test": "data"}`),
	}

	mockHandler.On("Handle", ctx, event).Return(assert.AnError)
	err := mockHandler.Handle(ctx, event)
	assert.Error(t, err)

	mockHandler.AssertExpectations(t)
}
