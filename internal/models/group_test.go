package models

import (
	"testing"
	"time"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
)

func TestGroup_GetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		group    Group
		expected string
	}{
		{
			name: "with subject",
			group: Group{
				GroupID: "123456789@g.us",
				Subject: "Family Group",
			},
			expected: "Family Group",
		},
		{
			name: "without subject returns group ID",
			group: Group{
				GroupID: "123456789@g.us",
				Subject: "",
			},
			expected: "123456789@g.us",
		},
		{
			name: "empty subject returns group ID",
			group: Group{
				GroupID: "987654321@g.us",
				Subject: "",
			},
			expected: "987654321@g.us",
		},
		{
			name: "subject with special characters",
			group: Group{
				GroupID: "111222333@g.us",
				Subject: "👨‍👩‍👧‍👦 Family",
			},
			expected: "👨‍👩‍👧‍👦 Family",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.group.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroup_FromWAGroup(t *testing.T) {
	waGroup := &types.Group{
		ID:          "123456789@g.us",
		Subject:     "Test Group",
		Description: "A test group for testing",
		Participants: []types.GroupParticipant{
			{ID: "1111111111@c.us", Role: "admin", IsAdmin: true},
			{ID: "2222222222@c.us", Role: "member", IsAdmin: false},
			{ID: "3333333333@c.us", Role: "member", IsAdmin: false},
		},
	}

	group := &Group{}
	group.FromWAGroup(waGroup, "default")

	assert.Equal(t, "123456789@g.us", group.GroupID)
	assert.Equal(t, "Test Group", group.Subject)
	assert.Equal(t, "A test group for testing", group.Description)
	assert.Equal(t, 3, group.ParticipantCount)
	assert.Equal(t, "default", group.SessionName)
}

func TestGroup_FromWAGroup_EmptyParticipants(t *testing.T) {
	waGroup := &types.Group{
		ID:           "123456789@g.us",
		Subject:      "Empty Group",
		Description:  "",
		Participants: []types.GroupParticipant{},
	}

	group := &Group{}
	group.FromWAGroup(waGroup, "session1")

	assert.Equal(t, "123456789@g.us", group.GroupID)
	assert.Equal(t, "Empty Group", group.Subject)
	assert.Equal(t, "", group.Description)
	assert.Equal(t, 0, group.ParticipantCount)
	assert.Equal(t, "session1", group.SessionName)
}

func TestGroup_ToWAGroup(t *testing.T) {
	group := &Group{
		ID:               1,
		GroupID:          "123456789@g.us",
		Subject:          "Test Group",
		Description:      "A test group",
		ParticipantCount: 5,
		SessionName:      "default",
		CachedAt:         time.Now(),
		UpdatedAt:        time.Now(),
	}

	waGroup := group.ToWAGroup()

	assert.Equal(t, "123456789@g.us", waGroup.ID.String())
	assert.Equal(t, "Test Group", waGroup.Subject)
	assert.Equal(t, "A test group", waGroup.Description)
	assert.Empty(t, waGroup.Participants, "Participants should not be populated from database cache")
}

func TestGroup_ToWAGroup_EmptyFields(t *testing.T) {
	group := &Group{
		GroupID:          "987654321@g.us",
		Subject:          "",
		Description:      "",
		ParticipantCount: 0,
		SessionName:      "default",
	}

	waGroup := group.ToWAGroup()

	assert.Equal(t, "987654321@g.us", waGroup.ID.String())
	assert.Equal(t, "", waGroup.Subject)
	assert.Equal(t, "", waGroup.Description)
	assert.Empty(t, waGroup.Participants)
}

func TestGroup_RoundTrip(t *testing.T) {
	originalWAGroup := &types.Group{
		ID:          types.WAHAGroupID("123456789@g.us"),
		Subject:     "Test Group",
		Description: "Test Description",
		Participants: []types.GroupParticipant{
			{ID: "1111@c.us", Role: "admin", IsAdmin: true},
			{ID: "2222@c.us", Role: "member", IsAdmin: false},
		},
	}

	// Convert to models.Group
	modelGroup := &Group{}
	modelGroup.FromWAGroup(originalWAGroup, "default")

	// Convert back to types.Group
	convertedWAGroup := modelGroup.ToWAGroup()

	// Verify core fields match (participants won't match since they're not cached)
	assert.Equal(t, originalWAGroup.ID.String(), convertedWAGroup.ID.String())
	assert.Equal(t, originalWAGroup.Subject, convertedWAGroup.Subject)
	assert.Equal(t, originalWAGroup.Description, convertedWAGroup.Description)
}
