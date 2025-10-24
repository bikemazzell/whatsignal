package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"whatsignal/internal/metrics"
	"whatsignal/internal/models"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock ContactDatabaseService
type mockContactDatabaseService struct {
	mock.Mock
}

func (m *mockContactDatabaseService) SaveContact(ctx context.Context, contact *models.Contact) error {
	args := m.Called(ctx, contact)
	return args.Error(0)
}

func (m *mockContactDatabaseService) GetContact(ctx context.Context, contactID string) (*models.Contact, error) {
	args := m.Called(ctx, contactID)
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockContactDatabaseService) GetContactByPhone(ctx context.Context, phoneNumber string) (*models.Contact, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockContactDatabaseService) CleanupOldContacts(ctx context.Context, retentionDays int) error {
	args := m.Called(ctx, retentionDays)
	return args.Error(0)
}

// Mock WAClient
type mockWAClient struct {
	mock.Mock
}

func (m *mockWAClient) GetContact(ctx context.Context, contactID string) (*types.Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Contact), args.Error(1)
}

func (m *mockWAClient) GetAllContacts(ctx context.Context, limit, offset int) ([]types.Contact, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]types.Contact), args.Error(1)
}

func (m *mockWAClient) GetGroup(ctx context.Context, groupID string) (*types.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Group), args.Error(1)
}

func (m *mockWAClient) GetAllGroups(ctx context.Context, limit, offset int) ([]types.Group, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Group), args.Error(1)
}

// Implement other required methods for WAClient interface
func (m *mockWAClient) SendText(ctx context.Context, chatID, message string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, message)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendTextWithSession(ctx context.Context, chatID, message, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, message, sessionName)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, imagePath, caption)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendImageWithSession(ctx context.Context, chatID, imagePath, caption, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, imagePath, caption, sessionName)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, videoPath, caption)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVideoWithSession(ctx context.Context, chatID, videoPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, videoPath, caption, sessionName)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, docPath, caption)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendDocumentWithSession(ctx context.Context, chatID, docPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, docPath, caption, sessionName)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, filePath, caption)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, voicePath)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVoiceWithSession(ctx context.Context, chatID, voicePath, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, voicePath, sessionName)
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) CreateSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) StartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) StopSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	args := m.Called(ctx)
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *mockWAClient) GetSessionStatusByName(ctx context.Context, sessionName string) (*types.Session, error) {
	args := m.Called(ctx, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *mockWAClient) RestartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error {
	args := m.Called(ctx, maxWaitTime)
	return args.Error(0)
}

func (m *mockWAClient) SendReaction(ctx context.Context, chatID, messageID, reaction string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendReactionWithSession(ctx context.Context, chatID, messageID, reaction, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	args := m.Called(ctx, chatID, messageID)
	return args.Error(0)
}

func (m *mockWAClient) GetSessionName() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockWAClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) AckMessage(ctx context.Context, chatID, sessionName string) error {
	args := m.Called(ctx, chatID, sessionName)
	return args.Error(0)
}

func TestNewContactService(t *testing.T) {
	mockDB := &mockContactDatabaseService{}
	mockWA := &mockWAClient{}

	service := NewContactService(mockDB, mockWA)

	assert.NotNil(t, service)
	assert.Equal(t, mockDB, service.db)
	assert.Equal(t, mockWA, service.waClient)
	assert.Equal(t, 24, service.cacheValidHours)
}

func TestNewContactServiceWithConfig(t *testing.T) {
	mockDB := &mockContactDatabaseService{}
	mockWA := &mockWAClient{}

	tests := []struct {
		name               string
		cacheValidHours    int
		expectedCacheHours int
	}{
		{
			name:               "valid cache hours",
			cacheValidHours:    48,
			expectedCacheHours: 48,
		},
		{
			name:               "zero cache hours - fallback to default",
			cacheValidHours:    0,
			expectedCacheHours: 24,
		},
		{
			name:               "negative cache hours - fallback to default",
			cacheValidHours:    -5,
			expectedCacheHours: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewContactServiceWithConfig(mockDB, mockWA, tt.cacheValidHours)
			assert.NotNil(t, service)
			assert.Equal(t, tt.expectedCacheHours, service.cacheValidHours)
		})
	}
}

func TestContactService_GetContactDisplayName(t *testing.T) {
	ctx := context.Background()

	t.Run("cached contact - fresh", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		// Recent cached contact
		cachedContact := &models.Contact{
			ContactID:   "+1234567890@c.us",
			PhoneNumber: "+1234567890",
			Name:        "John Doe",
			CachedAt:    time.Now().Add(-1 * time.Hour), // 1 hour ago
		}

		mockDB.On("GetContactByPhone", ctx, "+1234567890").Return(cachedContact, nil)

		result := service.GetContactDisplayName(ctx, "+1234567890")

		assert.Equal(t, "John Doe", result)
		mockDB.AssertExpectations(t)
		mockWA.AssertNotCalled(t, "GetContact")
	})

	t.Run("cached contact - stale, refresh from WhatsApp", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		// Stale cached contact
		staleContact := &models.Contact{
			ContactID:   "+1234567890@c.us",
			PhoneNumber: "+1234567890",
			Name:        "Old Name",
			CachedAt:    time.Now().Add(-48 * time.Hour), // 48 hours ago
		}

		// Fresh contact from WhatsApp
		waContact := &types.Contact{
			ID:     "+1234567890@c.us",
			Number: "+1234567890",
			Name:   "Updated Name",
		}

		mockDB.On("GetContactByPhone", ctx, "+1234567890").Return(staleContact, nil)
		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return(waContact, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		result := service.GetContactDisplayName(ctx, "+1234567890")

		assert.Equal(t, "Updated Name", result)
		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("no cached contact - fetch from WhatsApp", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		waContact := &types.Contact{
			ID:     "+1234567890@c.us",
			Number: "+1234567890",
			Name:   "Jane Doe",
		}

		mockDB.On("GetContactByPhone", ctx, "+1234567890").Return((*models.Contact)(nil), nil)
		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return(waContact, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		result := service.GetContactDisplayName(ctx, "+1234567890")

		assert.Equal(t, "Jane Doe", result)
		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("WhatsApp API error - fallback to cached", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		staleContact := &models.Contact{
			ContactID:   "+1234567890@c.us",
			PhoneNumber: "+1234567890",
			Name:        "Cached Name",
			CachedAt:    time.Now().Add(-48 * time.Hour),
		}

		mockDB.On("GetContactByPhone", ctx, "+1234567890").Return(staleContact, nil)
		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return((*types.Contact)(nil), errors.New("API error"))

		result := service.GetContactDisplayName(ctx, "+1234567890")

		assert.Equal(t, "Cached Name", result)
		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("WhatsApp API error - no cached contact - return phone number", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		mockDB.On("GetContactByPhone", ctx, "+1234567890").Return((*models.Contact)(nil), nil)
		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return((*types.Contact)(nil), errors.New("API error"))

		result := service.GetContactDisplayName(ctx, "+1234567890")

		assert.Equal(t, "+1234567890", result)
		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("contact not found in WhatsApp - return phone number", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		mockDB.On("GetContactByPhone", ctx, "+1234567890").Return((*models.Contact)(nil), nil)
		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return((*types.Contact)(nil), nil)

		result := service.GetContactDisplayName(ctx, "+1234567890")

		assert.Equal(t, "+1234567890", result)
		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("input already has @c.us suffix", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		waContact := &types.Contact{
			ID:     "+1234567890@c.us",
			Number: "+1234567890",
			Name:   "Test User",
		}

		mockDB.On("GetContactByPhone", ctx, "+1234567890@c.us").Return((*models.Contact)(nil), nil)
		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return(waContact, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		result := service.GetContactDisplayName(ctx, "+1234567890@c.us")

		assert.Equal(t, "Test User", result)
		mockWA.AssertExpectations(t)
	})
}

func TestContactService_RefreshContact(t *testing.T) {
	ctx := context.Background()

	t.Run("successful refresh", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		waContact := &types.Contact{
			ID:     "+1234567890@c.us",
			Number: "+1234567890",
			Name:   "Refreshed Name",
		}

		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return(waContact, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		err := service.RefreshContact(ctx, "+1234567890")

		assert.NoError(t, err)
		mockWA.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})

	t.Run("WhatsApp API error", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return((*types.Contact)(nil), errors.New("API error"))

		err := service.RefreshContact(ctx, "+1234567890")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch contact from WhatsApp API")
		mockWA.AssertExpectations(t)
	})

	t.Run("contact not found", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return((*types.Contact)(nil), nil)

		err := service.RefreshContact(ctx, "+1234567890")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "contact not found")
		mockWA.AssertExpectations(t)
	})

	t.Run("database save error", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		waContact := &types.Contact{
			ID:     "+1234567890@c.us",
			Number: "+1234567890",
			Name:   "Test Name",
		}

		mockWA.On("GetContact", ctx, "+1234567890@c.us").Return(waContact, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(errors.New("database error"))

		err := service.RefreshContact(ctx, "+1234567890")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		mockWA.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})
}

func TestContactService_SyncAllContacts(t *testing.T) {
	ctx := context.Background()

	t.Run("successful sync with multiple batches", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		// First batch - full
		batch1 := make([]types.Contact, 100)
		for i := 0; i < 100; i++ {
			batch1[i] = types.Contact{
				ID:     fmt.Sprintf("+123456789%d@c.us", i),
				Number: fmt.Sprintf("+123456789%d", i),
				Name:   fmt.Sprintf("Contact %d", i),
			}
		}

		// Second batch - partial
		batch2 := make([]types.Contact, 50)
		for i := 0; i < 50; i++ {
			batch2[i] = types.Contact{
				ID:     fmt.Sprintf("+987654321%d@c.us", i),
				Number: fmt.Sprintf("+987654321%d", i),
				Name:   fmt.Sprintf("Contact %d", i+100),
			}
		}

		mockWA.On("GetSessionName").Return("test-session")
		mockWA.On("GetAllContacts", ctx, 100, 0).Return(batch1, nil)
		mockWA.On("GetAllContacts", ctx, 100, 100).Return(batch2, nil)

		// Mock save for all contacts
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil).Times(150)

		err := service.SyncAllContacts(ctx)

		assert.NoError(t, err)
		mockWA.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})

	t.Run("API error", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		mockWA.On("GetSessionName").Return("test-session")
		mockWA.On("GetAllContacts", ctx, 100, 0).Return([]types.Contact(nil), errors.New("API error"))

		err := service.SyncAllContacts(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch contacts batch")
		mockWA.AssertExpectations(t)
	})

	t.Run("empty response", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		mockWA.On("GetSessionName").Return("test-session")
		mockWA.On("GetAllContacts", ctx, 100, 0).Return([]types.Contact{}, nil)

		err := service.SyncAllContacts(ctx)

		assert.NoError(t, err)
		mockWA.AssertExpectations(t)
	})

	t.Run("context cancellation", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		// Create a context that's already cancelled
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		batch1 := make([]types.Contact, 100)
		for i := 0; i < 100; i++ {
			batch1[i] = types.Contact{
				ID:     fmt.Sprintf("+123456789%d@c.us", i),
				Number: fmt.Sprintf("+123456789%d", i),
				Name:   fmt.Sprintf("Contact %d", i),
			}
		}

		mockWA.On("GetSessionName").Return("test-session")
		mockWA.On("GetAllContacts", cancelledCtx, 100, 0).Return(batch1, nil)
		mockDB.On("SaveContact", cancelledCtx, mock.AnythingOfType("*models.Contact")).Return(nil).Times(100)

		err := service.SyncAllContacts(cancelledCtx)

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("database error during save", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		batch1 := []types.Contact{
			{
				ID:     "+1234567890@c.us",
				Number: "+1234567890",
				Name:   "Test Contact",
			},
		}

		mockWA.On("GetSessionName").Return("test-session")
		mockWA.On("GetAllContacts", ctx, 100, 0).Return(batch1, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(errors.New("database error"))

		err := service.SyncAllContacts(ctx)

		// Should continue even with database errors and return no error
		assert.NoError(t, err)
		mockWA.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})
}

func TestContactService_SyncAllContacts_SessionLogging(t *testing.T) {
	ctx := context.Background()

	t.Run("logs include session name", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}

		// Set up mock to return a specific session name (called at the start of SyncAllContacts)
		mockWA.On("GetSessionName").Return("business-session")

		service := NewContactService(mockDB, mockWA)

		// Single batch of contacts
		batch := []types.Contact{
			{
				ID:     "+1234567890@c.us",
				Number: "+1234567890",
				Name:   "Test Contact",
			},
		}

		mockWA.On("GetAllContacts", ctx, 100, 0).Return(batch, nil)
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		err := service.SyncAllContacts(ctx)

		assert.NoError(t, err)
		mockWA.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})

	t.Run("handles save error with session name in logs", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}

		mockWA.On("GetSessionName").Return("personal-session")

		service := NewContactService(mockDB, mockWA)

		batch := []types.Contact{
			{
				ID:     "+1234567890@c.us",
				Number: "+1234567890",
				Name:   "Test Contact",
			},
		}

		mockWA.On("GetAllContacts", ctx, 100, 0).Return(batch, nil)

		// Mock database save error
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(errors.New("save error"))

		err := service.SyncAllContacts(ctx)

		// Should not return error even if individual contact save fails
		assert.NoError(t, err)
		mockWA.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})
}

func TestContactService_CleanupOldContacts(t *testing.T) {
	t.Run("successful cleanup", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)
		ctx := context.Background()

		mockDB.On("CleanupOldContacts", ctx, 30).Return(nil)

		err := service.CleanupOldContacts(ctx, 30)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)
		ctx := context.Background()

		mockDB.On("CleanupOldContacts", ctx, 30).Return(errors.New("database error"))

		err := service.CleanupOldContacts(ctx, 30)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		mockDB.AssertExpectations(t)
	})
}

// TestContactService_CacheMetrics tests that cache hit/miss metrics are recorded correctly
func TestContactService_CacheMetrics(t *testing.T) {
	t.Run("cache hit metric", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		ctx := context.Background()
		phoneNumber := "+1234567890"

		// Create a fresh contact that should be in cache
		cachedContact := &models.Contact{
			ID:          1,
			ContactID:   phoneNumber + "@c.us",
			PhoneNumber: phoneNumber,
			Name:        "Test Contact",
			CachedAt:    time.Now(), // Fresh cache entry
		}

		// Mock database to return cached contact
		mockDB.On("GetContactByPhone", ctx, phoneNumber).Return(cachedContact, nil)

		// Get initial metrics count
		initialMetrics := metrics.GetAllMetrics()
		initialHits := float64(0)
		if hitMetric, exists := initialMetrics.Counters["contact_cache_hits_total"]; exists {
			initialHits = hitMetric.Value
		}

		// Call GetContactDisplayName which should hit cache
		displayName := service.GetContactDisplayName(ctx, phoneNumber)

		// Verify cache hit
		assert.Equal(t, "Test Contact", displayName)

		// Verify cache hit metric was incremented
		finalMetrics := metrics.GetAllMetrics()
		if hitMetric, exists := finalMetrics.Counters["contact_cache_hits_total"]; exists {
			assert.Equal(t, initialHits+1, hitMetric.Value, "Cache hit metric should be incremented")
		} else {
			t.Fatal("Expected contact_cache_hits_total metric to exist")
		}

		mockDB.AssertExpectations(t)
	})

	t.Run("cache miss metric", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		ctx := context.Background()
		phoneNumber := "+1234567891"
		contactID := phoneNumber + "@c.us"

		// Mock database to return no cached contact (cache miss)
		mockDB.On("GetContactByPhone", ctx, phoneNumber).Return((*models.Contact)(nil), errors.New("not found"))

		// Mock WhatsApp API to return contact
		waContact := &types.Contact{
			ID:   contactID,
			Name: "New Contact",
		}
		mockWA.On("GetContact", ctx, contactID).Return(waContact, nil)

		// Mock database save for the new contact
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		// Get initial metrics count
		initialMetrics := metrics.GetAllMetrics()
		initialMisses := float64(0)
		if missMetric, exists := initialMetrics.Counters["contact_cache_misses_total"]; exists {
			initialMisses = missMetric.Value
		}

		// Call GetContactDisplayName which should miss cache
		displayName := service.GetContactDisplayName(ctx, phoneNumber)

		// Verify result
		assert.Equal(t, "New Contact", displayName)

		// Verify cache miss metric was incremented
		finalMetrics := metrics.GetAllMetrics()
		if missMetric, exists := finalMetrics.Counters["contact_cache_misses_total"]; exists {
			assert.Equal(t, initialMisses+1, missMetric.Value, "Cache miss metric should be incremented")
		} else {
			t.Fatal("Expected contact_cache_misses_total metric to exist")
		}

		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("cache refresh metric", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		ctx := context.Background()
		phoneNumber := "+1234567892"
		contactID := phoneNumber + "@c.us"

		// Create an old cached contact that needs refresh
		oldContact := &models.Contact{
			ID:          2,
			ContactID:   contactID,
			PhoneNumber: phoneNumber,
			Name:        "Old Contact",
			CachedAt:    time.Now().Add(-25 * time.Hour), // Older than 24 hours
		}

		// Mock database to return old cached contact
		mockDB.On("GetContactByPhone", ctx, phoneNumber).Return(oldContact, nil)

		// Mock WhatsApp API to return updated contact
		waContact := &types.Contact{
			ID:   contactID,
			Name: "Updated Contact",
		}
		mockWA.On("GetContact", ctx, contactID).Return(waContact, nil)

		// Mock database save for the refreshed contact
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		// Get initial metrics count
		initialMetrics := metrics.GetAllMetrics()
		initialRefreshes := float64(0)
		if refreshMetric, exists := initialMetrics.Counters["contact_cache_refreshes_total"]; exists {
			initialRefreshes = refreshMetric.Value
		}

		// Call GetContactDisplayName which should refresh cache
		displayName := service.GetContactDisplayName(ctx, phoneNumber)

		// Verify result
		assert.Equal(t, "Updated Contact", displayName)

		// Verify cache refresh metric was incremented
		finalMetrics := metrics.GetAllMetrics()
		if refreshMetric, exists := finalMetrics.Counters["contact_cache_refreshes_total"]; exists {
			assert.Equal(t, initialRefreshes+1, refreshMetric.Value, "Cache refresh metric should be incremented")
		} else {
			t.Fatal("Expected contact_cache_refreshes_total metric to exist")
		}

		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})

	t.Run("cache miss metric with API error", func(t *testing.T) {
		mockDB := &mockContactDatabaseService{}
		mockWA := &mockWAClient{}
		service := NewContactService(mockDB, mockWA)

		ctx := context.Background()
		phoneNumber := "+1234567893"
		contactID := phoneNumber + "@c.us"

		// Mock database to return no cached contact (cache miss)
		mockDB.On("GetContactByPhone", ctx, phoneNumber).Return((*models.Contact)(nil), errors.New("not found"))

		// Mock WhatsApp API to return error
		mockWA.On("GetContact", ctx, contactID).Return((*types.Contact)(nil), errors.New("API error"))

		// Get initial metrics count
		initialMetrics := metrics.GetAllMetrics()
		initialMisses := float64(0)
		if missMetric, exists := initialMetrics.Counters["contact_cache_misses_total"]; exists {
			initialMisses = missMetric.Value
		}

		// Call GetContactDisplayName which should miss cache and fail API call
		displayName := service.GetContactDisplayName(ctx, phoneNumber)

		// Should fallback to phone number
		assert.Equal(t, phoneNumber, displayName)

		// Verify cache miss metric was still incremented even though API failed
		finalMetrics := metrics.GetAllMetrics()
		if missMetric, exists := finalMetrics.Counters["contact_cache_misses_total"]; exists {
			assert.Equal(t, initialMisses+1, missMetric.Value, "Cache miss metric should be incremented even on API error")
		} else {
			t.Fatal("Expected contact_cache_misses_total metric to exist")
		}

		mockDB.AssertExpectations(t)
		mockWA.AssertExpectations(t)
	})
}
