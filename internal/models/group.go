package models

import (
	"time"
	"whatsignal/pkg/whatsapp/types"
)

// Group represents a cached WhatsApp group in the database
type Group struct {
	ID               int       `json:"id"`
	GroupID          string    `json:"group_id"`          // WhatsApp group ID like "123456789@g.us"
	Subject          string    `json:"subject"`           // Group name/title
	Description      string    `json:"description"`       // Group description
	ParticipantCount int       `json:"participant_count"` // Number of participants
	SessionName      string    `json:"session_name"`      // WhatsApp session name
	CachedAt         time.Time `json:"cached_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// GetDisplayName returns the best available display name for the group
func (g *Group) GetDisplayName() string {
	if g.Subject != "" {
		return g.Subject
	}
	return g.GroupID
}

// FromWAGroup converts a WhatsApp types.Group to a models.Group
func (g *Group) FromWAGroup(waGroup *types.Group, sessionName string) {
	g.GroupID = waGroup.ID
	g.Subject = waGroup.Subject
	g.Description = waGroup.Description
	g.ParticipantCount = len(waGroup.Participants)
	g.SessionName = sessionName
}

// ToWAGroup converts a models.Group to a types.Group
func (g *Group) ToWAGroup() *types.Group {
	return &types.Group{
		ID:          g.GroupID,
		Subject:     g.Subject,
		Description: g.Description,
		// Note: Participants are not cached in the database, so this will be empty
		Participants: []types.GroupParticipant{},
	}
}
