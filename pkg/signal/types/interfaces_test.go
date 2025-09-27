package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockClient is a mock implementation of the Client interface
type MockClient struct {
	mock.Mock
}

func (m *MockClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*SendMessageResponse, error) {
	args := m.Called(ctx, recipient, message, attachments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SendMessageResponse), args.Error(1)
}

func (m *MockClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]SignalMessage, error) {
	args := m.Called(ctx, timeoutSeconds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]SignalMessage), args.Error(1)
}

func (m *MockClient) InitializeDevice(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestClientInterface(t *testing.T) {
	// Test that MockClient implements Client interface
	var client Client = &MockClient{}
	assert.NotNil(t, client)
}

func TestMockClientSendMessage(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	// Test successful send
	expectedResponse := &SendMessageResponse{
		Timestamp: 1234567890,
		MessageID: "msg-123",
	}

	mockClient.On("SendMessage", ctx, "+1234567890", "Hello, World!", []string{}).Return(expectedResponse, nil)

	response, err := mockClient.SendMessage(ctx, "+1234567890", "Hello, World!", []string{})
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockClientSendMessageWithAttachments(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	expectedResponse := &SendMessageResponse{
		Timestamp: 1234567890,
		MessageID: "msg-456",
	}

	attachments := []string{"/path/to/image.jpg", "/path/to/document.pdf"}
	mockClient.On("SendMessage", ctx, "+1234567890", "Check this out!", attachments).Return(expectedResponse, nil)

	response, err := mockClient.SendMessage(ctx, "+1234567890", "Check this out!", attachments)
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)

	mockClient.AssertExpectations(t)
}

func TestMockClientSendMessageError(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	mockClient.On("SendMessage", ctx, "+1234567890", "Hello, World!", []string{}).Return(nil, assert.AnError)

	response, err := mockClient.SendMessage(ctx, "+1234567890", "Hello, World!", []string{})
	assert.Error(t, err)
	assert.Nil(t, response)

	mockClient.AssertExpectations(t)
}

func TestMockClientReceiveMessages(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	expectedMessages := []SignalMessage{
		{
			Timestamp:   1234567890,
			Sender:      "+1234567890",
			MessageID:   "msg-1",
			Message:     "First message",
			Attachments: []string{},
		},
		{
			Timestamp:   1234567891,
			Sender:      "+0987654321",
			MessageID:   "msg-2",
			Message:     "Second message",
			Attachments: []string{"/path/to/file.pdf"},
		},
	}

	mockClient.On("ReceiveMessages", ctx, 30).Return(expectedMessages, nil)

	messages, err := mockClient.ReceiveMessages(ctx, 30)
	assert.NoError(t, err)
	assert.Equal(t, expectedMessages, messages)
	assert.Len(t, messages, 2)

	mockClient.AssertExpectations(t)
}

func TestMockClientReceiveMessagesEmpty(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	mockClient.On("ReceiveMessages", ctx, 30).Return([]SignalMessage{}, nil)

	messages, err := mockClient.ReceiveMessages(ctx, 30)
	assert.NoError(t, err)
	assert.Empty(t, messages)

	mockClient.AssertExpectations(t)
}

func TestMockClientReceiveMessagesError(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	mockClient.On("ReceiveMessages", ctx, 30).Return(nil, assert.AnError)

	messages, err := mockClient.ReceiveMessages(ctx, 30)
	assert.Error(t, err)
	assert.Nil(t, messages)

	mockClient.AssertExpectations(t)
}

func TestMockClientInitializeDevice(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	// Test successful initialization
	mockClient.On("InitializeDevice", ctx).Return(nil)

	err := mockClient.InitializeDevice(ctx)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestMockClientInitializeDeviceError(t *testing.T) {
	mockClient := &MockClient{}
	ctx := context.Background()

	// Test initialization error
	mockClient.On("InitializeDevice", ctx).Return(assert.AnError)

	err := mockClient.InitializeDevice(ctx)
	assert.Error(t, err)

	mockClient.AssertExpectations(t)
}

func TestClientInterfaceMethods(t *testing.T) {
	// Test that the Client interface has the expected methods
	mockClient := &MockClient{}
	ctx := context.Background()

	// Test SendMessage method signature
	mockClient.On("SendMessage", ctx, "+1234567890", "test", []string{}).Return(&SendMessageResponse{
		Timestamp: 1234567890,
		MessageID: "test-msg",
	}, nil)
	_, err := mockClient.SendMessage(ctx, "+1234567890", "test", []string{})
	assert.NoError(t, err)

	// Test ReceiveMessages method signature
	mockClient.On("ReceiveMessages", ctx, 30).Return([]SignalMessage{}, nil)
	_, err = mockClient.ReceiveMessages(ctx, 30)
	assert.NoError(t, err)

	// Test InitializeDevice method signature
	mockClient.On("InitializeDevice", ctx).Return(nil)
	err = mockClient.InitializeDevice(ctx)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestClientInterfaceCompliance(t *testing.T) {
	// Verify that MockClient satisfies the Client interface at compile time
	var _ Client = (*MockClient)(nil)

	// Test interface method calls
	mockClient := &MockClient{}
	ctx := context.Background()

	// Setup expectations
	mockClient.On("SendMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&SendMessageResponse{
		Timestamp: 1234567890,
		MessageID: "test-msg",
	}, nil)
	mockClient.On("ReceiveMessages", mock.Anything, mock.Anything).Return([]SignalMessage{}, nil)
	mockClient.On("InitializeDevice", mock.Anything).Return(nil)

	// Call interface methods
	var client Client = mockClient
	_, err := client.SendMessage(ctx, "+1234567890", "test", []string{})
	assert.NoError(t, err)

	_, err = client.ReceiveMessages(ctx, 30)
	assert.NoError(t, err)

	err = client.InitializeDevice(ctx)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}
