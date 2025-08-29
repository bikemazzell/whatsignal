package models

import (
	"time"
	"whatsignal/pkg/whatsapp/types"
)

// Contact represents a cached WhatsApp contact in the database
type Contact struct {
	ID          int       `json:"id"`
	ContactID   string    `json:"contact_id"`   // WhatsApp ID like "1234567890@c.us"
	PhoneNumber string    `json:"phone_number"` // Just the phone number "1234567890"
	Name        string    `json:"name"`         // Contact book name (highest priority)
	PushName    string    `json:"push_name"`    // User's display name (fallback)
	ShortName   string    `json:"short_name"`   // Shortened name
	IsBlocked   bool      `json:"is_blocked"`
	IsGroup     bool      `json:"is_group"`
	IsMyContact bool      `json:"is_my_contact"`
	CachedAt    time.Time `json:"cached_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetDisplayName returns the best available display name for the contact
func (c *Contact) GetDisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	if c.PushName != "" {
		return c.PushName
	}
	return c.PhoneNumber
}

// FromWAContact converts a WhatsApp types.Contact to a models.Contact
func (c *Contact) FromWAContact(waContact *types.Contact) {
	c.ContactID = waContact.ID
	c.PhoneNumber = waContact.Number
	c.Name = waContact.Name
	c.PushName = waContact.PushName
	c.ShortName = waContact.ShortName
	c.IsBlocked = waContact.IsBlocked
	c.IsGroup = waContact.IsGroup
	c.IsMyContact = waContact.IsMyContact
}

// ToWAContact converts a models.Contact to a types.Contact
func (c *Contact) ToWAContact() *types.Contact {
	return &types.Contact{
		ID:          c.ContactID,
		Number:      c.PhoneNumber,
		Name:        c.Name,
		PushName:    c.PushName,
		ShortName:   c.ShortName,
		IsBlocked:   c.IsBlocked,
		IsGroup:     c.IsGroup,
		IsMyContact: c.IsMyContact,
	}
}
