package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGroupService_GetGroupName_CacheHit(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "123456789@g.us"
	sessionName := "default"

	// Setup cached group (recent)
	cachedGroup := &models.Group{
		GroupID:     groupID,
		Subject:     "Test Group",
		SessionName: sessionName,
		CachedAt:    time.Now().Add(-1 * time.Hour), // 1 hour old
	}

	mockDB.On("GetGroup", ctx, groupID, sessionName).Return(cachedGroup, nil)

	gs := NewGroupService(mockDB, mockWA)

	// Should return cached name without calling API
	name := gs.GetGroupName(ctx, groupID, sessionName)

	assert.Equal(t, "Test Group", name)
	mockDB.AssertExpectations(t)
	mockWA.AssertNotCalled(t, "GetGroup")
}

func TestGroupService_GetGroupName_CacheMiss(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "123456789@g.us"
	sessionName := "default"

	// No cached group
	mockDB.On("GetGroup", ctx, groupID, sessionName).Return(nil, nil)

	// API returns group
	waGroup := &types.Group{
		ID:          types.WAHAGroupID(groupID),
		Subject:     "Fresh Group",
		Description: "Description",
		Participants: []types.GroupParticipant{
			{ID: "111@c.us", Role: "admin", IsAdmin: true},
		},
	}
	mockWA.On("GetGroup", ctx, groupID).Return(waGroup, nil)

	// Should save to cache
	mockDB.On("SaveGroup", ctx, mock.MatchedBy(func(g *models.Group) bool {
		return g.GroupID == groupID && g.Subject == "Fresh Group" && g.SessionName == sessionName
	})).Return(nil)

	gs := NewGroupService(mockDB, mockWA)

	name := gs.GetGroupName(ctx, groupID, sessionName)

	assert.Equal(t, "Fresh Group", name)
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_GetGroupName_OldCache(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "123456789@g.us"
	sessionName := "default"

	// Setup old cached group (25 hours old, cache valid for 24 hours)
	oldGroup := &models.Group{
		GroupID:     groupID,
		Subject:     "Old Group Name",
		SessionName: sessionName,
		CachedAt:    time.Now().Add(-25 * time.Hour),
	}

	mockDB.On("GetGroup", ctx, groupID, sessionName).Return(oldGroup, nil)

	// API returns updated group
	waGroup := &types.Group{
		ID:      types.WAHAGroupID(groupID),
		Subject: "Updated Group Name",
	}
	mockWA.On("GetGroup", ctx, groupID).Return(waGroup, nil)

	// Should save updated version
	mockDB.On("SaveGroup", ctx, mock.MatchedBy(func(g *models.Group) bool {
		return g.Subject == "Updated Group Name"
	})).Return(nil)

	gs := NewGroupService(mockDB, mockWA)

	name := gs.GetGroupName(ctx, groupID, sessionName)

	assert.Equal(t, "Updated Group Name", name)
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_GetGroupName_APIFailure_UsesCache(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "123456789@g.us"
	sessionName := "default"

	// Old cached group
	oldGroup := &models.Group{
		GroupID:     groupID,
		Subject:     "Cached Group",
		SessionName: sessionName,
		CachedAt:    time.Now().Add(-30 * time.Hour),
	}

	mockDB.On("GetGroup", ctx, groupID, sessionName).Return(oldGroup, nil)

	// API fails
	mockWA.On("GetGroup", ctx, groupID).Return(nil, errors.New("API error"))

	gs := NewGroupService(mockDB, mockWA)

	// Should use cached version despite being old
	name := gs.GetGroupName(ctx, groupID, sessionName)

	assert.Equal(t, "Cached Group", name)
	assert.False(t, gs.degradedMode) // API error doesn't trigger degraded mode yet
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_GetGroupName_APIFailure_NoCache(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "123456789@g.us"
	sessionName := "default"

	// No cached group
	mockDB.On("GetGroup", ctx, groupID, sessionName).Return(nil, nil)

	// API fails
	mockWA.On("GetGroup", ctx, groupID).Return(nil, errors.New("API error"))

	gs := NewGroupService(mockDB, mockWA)

	// Should return group ID as fallback
	name := gs.GetGroupName(ctx, groupID, sessionName)

	assert.Equal(t, groupID, name)
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_GetGroupName_InvalidFormat(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	invalidID := "123456789@c.us" // Contact ID, not group
	sessionName := "default"

	gs := NewGroupService(mockDB, mockWA)

	// Should return the ID as-is without any lookups
	name := gs.GetGroupName(ctx, invalidID, sessionName)

	assert.Equal(t, invalidID, name)
	mockDB.AssertNotCalled(t, "GetGroup")
	mockWA.AssertNotCalled(t, "GetGroup")
}

func TestGroupService_GetGroupName_NotFound(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "nonexistent@g.us"
	sessionName := "default"

	mockDB.On("GetGroup", ctx, groupID, sessionName).Return(nil, nil)
	mockWA.On("GetGroup", ctx, groupID).Return(nil, nil) // Group not found

	gs := NewGroupService(mockDB, mockWA)

	name := gs.GetGroupName(ctx, groupID, sessionName)

	assert.Equal(t, groupID, name)
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_RefreshGroup(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "123456789@g.us"
	sessionName := "default"

	waGroup := &types.Group{
		ID:      types.WAHAGroupID(groupID),
		Subject: "Refreshed Group",
	}

	mockWA.On("GetGroup", ctx, groupID).Return(waGroup, nil)
	mockDB.On("SaveGroup", ctx, mock.MatchedBy(func(g *models.Group) bool {
		return g.GroupID == groupID && g.Subject == "Refreshed Group"
	})).Return(nil)

	gs := NewGroupService(mockDB, mockWA)

	err := gs.RefreshGroup(ctx, groupID, sessionName)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_RefreshGroup_InvalidFormat(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	invalidID := "123456789@c.us"
	sessionName := "default"

	gs := NewGroupService(mockDB, mockWA)

	err := gs.RefreshGroup(ctx, invalidID, sessionName)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid group ID format")
	mockDB.AssertNotCalled(t, "SaveGroup")
	mockWA.AssertNotCalled(t, "GetGroup")
}

func TestGroupService_RefreshGroup_NotFound(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	groupID := "nonexistent@g.us"
	sessionName := "default"

	mockWA.On("GetGroup", ctx, groupID).Return(nil, nil)

	gs := NewGroupService(mockDB, mockWA)

	err := gs.RefreshGroup(ctx, groupID, sessionName)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group not found")
	mockDB.AssertNotCalled(t, "SaveGroup")
	mockWA.AssertExpectations(t)
}

func TestGroupService_SyncAllGroups(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	sessionName := "default"

	// Create 100 groups for first batch (full batch)
	batch1 := make([]types.Group, 100)
	for i := 0; i < 100; i++ {
		batch1[i] = types.Group{
			ID:      types.WAHAGroupID(fmt.Sprintf("group%d@g.us", i)),
			Subject: fmt.Sprintf("Group %d", i),
		}
	}

	// Second batch with fewer groups (indicating end)
	batch2 := []types.Group{
		{ID: types.WAHAGroupID("group100@g.us"), Subject: "Group 100"},
		{ID: types.WAHAGroupID("group101@g.us"), Subject: "Group 101"},
	}

	mockWA.On("GetAllGroups", ctx, 100, 0).Return(batch1, nil)
	mockWA.On("GetAllGroups", ctx, 100, 100).Return(batch2, nil)

	// Expect SaveGroup for all groups (use Any matcher for simplicity)
	mockDB.On("SaveGroup", ctx, mock.AnythingOfType("*models.Group")).Return(nil).Times(102)

	gs := NewGroupService(mockDB, mockWA)

	err := gs.SyncAllGroups(ctx, sessionName)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	mockWA.AssertExpectations(t)
}

func TestGroupService_SyncAllGroups_APIError(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	sessionName := "default"

	mockWA.On("GetAllGroups", ctx, 100, 0).Return([]types.Group(nil), errors.New("API error"))

	gs := NewGroupService(mockDB, mockWA)

	err := gs.SyncAllGroups(ctx, sessionName)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch groups")
	mockDB.AssertNotCalled(t, "SaveGroup")
	mockWA.AssertExpectations(t)
}

func TestGroupService_CleanupOldGroups(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	ctx := context.Background()
	retentionDays := 14

	mockDB.On("CleanupOldGroups", ctx, retentionDays).Return(nil)

	gs := NewGroupService(mockDB, mockWA)

	err := gs.CleanupOldGroups(ctx, retentionDays)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestGroupService_NewGroupServiceWithConfig(t *testing.T) {
	mockDB := new(mockGroupDatabase)
	mockWA := new(mockWhatsAppClient)

	// Test with valid cache hours
	gs := NewGroupServiceWithConfig(mockDB, mockWA, 48)
	assert.Equal(t, 48, gs.cacheValidHours)

	// Test with invalid cache hours (should default to 24)
	gs = NewGroupServiceWithConfig(mockDB, mockWA, 0)
	assert.Equal(t, 24, gs.cacheValidHours)

	gs = NewGroupServiceWithConfig(mockDB, mockWA, -5)
	assert.Equal(t, 24, gs.cacheValidHours)
}

// Mock implementations

type mockGroupDatabase struct {
	mock.Mock
}

func (m *mockGroupDatabase) SaveGroup(ctx context.Context, group *models.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *mockGroupDatabase) GetGroup(ctx context.Context, groupID, sessionName string) (*models.Group, error) {
	args := m.Called(ctx, groupID, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *mockGroupDatabase) CleanupOldGroups(ctx context.Context, retentionDays int) error {
	args := m.Called(ctx, retentionDays)
	return args.Error(0)
}
