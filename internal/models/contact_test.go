package models

import (
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
)

func TestContact_GetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		contact  Contact
		expected string
	}{
		{
			name: "with name",
			contact: Contact{
				ID:          1,
				PhoneNumber: "+1234567890",
				Name:        "John Doe",
				PushName:    "JD",
				ShortName:   "Johnny",
			},
			expected: "John Doe",
		},
		{
			name: "without name but with push name",
			contact: Contact{
				ID:          2,
				PhoneNumber: "+1234567890",
				PushName:    "Jane Profile",
				ShortName:   "Jane Server",
			},
			expected: "Jane Profile",
		},
		{
			name: "only phone number",
			contact: Contact{
				ID:          4,
				PhoneNumber: "+1234567890",
			},
			expected: "+1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.contact.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromWAContact(t *testing.T) {
	waContact := &types.Contact{
		ID:          "123456@c.us",
		Number:      "123456",
		Name:        "Test User",
		ShortName:   "Test",
		PushName:    "TestPush",
		IsBlocked:   false,
		IsGroup:     false,
		IsMyContact: true,
	}

	contact := &Contact{}
	contact.FromWAContact(waContact)

	assert.Equal(t, "123456@c.us", contact.ContactID)
	assert.Equal(t, "123456", contact.PhoneNumber)
	assert.Equal(t, "Test User", contact.Name)
	assert.Equal(t, "TestPush", contact.PushName)
	assert.Equal(t, "Test", contact.ShortName)
	assert.False(t, contact.IsBlocked)
	assert.False(t, contact.IsGroup)
	assert.True(t, contact.IsMyContact)
}

func TestFromWAContact_GroupContact(t *testing.T) {
	waContact := &types.Contact{
		ID:      "987654321@g.us",
		Number:  "987654321",
		Name:    "Test Group",
		IsGroup: true,
	}

	contact := &Contact{}
	contact.FromWAContact(waContact)

	assert.Equal(t, "987654321@g.us", contact.ContactID)
	assert.Equal(t, "987654321", contact.PhoneNumber)
	assert.Equal(t, "Test Group", contact.Name)
	assert.True(t, contact.IsGroup)
}

func TestToWAContact(t *testing.T) {
	contact := Contact{
		ID:          5,
		ContactID:   "1234567890@c.us",
		PhoneNumber: "1234567890",
		Name:        "John Doe",
		PushName:    "JD",
		ShortName:   "Johnny",
		IsBlocked:   false,
		IsGroup:     false,
		IsMyContact: true,
		CachedAt:    time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now(),
	}

	waContact := contact.ToWAContact()

	assert.Equal(t, "1234567890@c.us", waContact.ID)
	assert.Equal(t, "1234567890", waContact.Number)
	assert.Equal(t, "John Doe", waContact.Name)
	assert.Equal(t, "JD", waContact.PushName)
	assert.Equal(t, "Johnny", waContact.ShortName)
	assert.False(t, waContact.IsBlocked)
	assert.False(t, waContact.IsGroup)
	assert.True(t, waContact.IsMyContact)
}

func TestToWAContact_GroupContact(t *testing.T) {
	contact := Contact{
		ID:          6,
		ContactID:   "123456789@g.us",
		PhoneNumber: "123456789",
		Name:        "Test Group",
		IsGroup:     true,
		IsMyContact: false,
		CachedAt:    time.Now(),
		UpdatedAt:   time.Now(),
	}

	waContact := contact.ToWAContact()

	assert.Equal(t, "123456789@g.us", waContact.ID)
	assert.Equal(t, "123456789", waContact.Number)
	assert.Equal(t, "Test Group", waContact.Name)
	assert.True(t, waContact.IsGroup)
	assert.False(t, waContact.IsMyContact)
}
