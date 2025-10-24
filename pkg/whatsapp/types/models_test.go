package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				ID:       "123@c.us",
				Number:   "+1234567890",
				Name:     "John Doe",
				PushName: "JD",
			},
			expected: "John Doe",
		},
		{
			name: "without name but with push name",
			contact: Contact{
				ID:       "123@c.us",
				Number:   "+1234567890",
				PushName: "Jane Profile",
			},
			expected: "Jane Profile",
		},
		{
			name: "only number",
			contact: Contact{
				ID:     "123@c.us",
				Number: "+1234567890",
			},
			expected: "+1234567890",
		},
		{
			name: "empty contact",
			contact: Contact{
				ID: "123@c.us",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.contact.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSessionStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   SessionStatus
		expected string
	}{
		{
			name:     "initialized status",
			status:   SessionStatusInitialized,
			expected: "initialized",
		},
		{
			name:     "starting status",
			status:   SessionStatusStarting,
			expected: "starting",
		},
		{
			name:     "running status",
			status:   SessionStatusRunning,
			expected: "running",
		},
		{
			name:     "stopped status",
			status:   SessionStatusStopped,
			expected: "stopped",
		},
		{
			name:     "error status",
			status:   SessionStatusError,
			expected: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestSession_Marshal(t *testing.T) {
	now := time.Now()
	session := Session{
		Name:      "test-session",
		Status:    SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
		Error:     "",
	}

	data, err := json.Marshal(session)
	require.NoError(t, err)

	var unmarshaled Session
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, session.Name, unmarshaled.Name)
	assert.Equal(t, session.Status, unmarshaled.Status)
	assert.Equal(t, session.Error, unmarshaled.Error)
	// Time comparison with tolerance
	assert.WithinDuration(t, session.CreatedAt, unmarshaled.CreatedAt, time.Second)
	assert.WithinDuration(t, session.UpdatedAt, unmarshaled.UpdatedAt, time.Second)
}

func TestSession_WithError(t *testing.T) {
	session := Session{
		Name:   "test-session",
		Status: SessionStatusError,
		Error:  "Connection failed",
	}

	data, err := json.Marshal(session)
	require.NoError(t, err)

	var unmarshaled Session
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, session.Name, unmarshaled.Name)
	assert.Equal(t, session.Status, unmarshaled.Status)
	assert.Equal(t, session.Error, unmarshaled.Error)
}

func TestWebhookEvent_Marshal(t *testing.T) {
	payload := map[string]interface{}{
		"id":      "msg123",
		"content": "Hello, World!",
		"sender":  "+1234567890",
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	event := WebhookEvent{
		Event:   "message",
		Payload: payloadBytes,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled WebhookEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.Event, unmarshaled.Event)

	// Verify payload can be unmarshaled back to original structure
	var decodedPayload map[string]interface{}
	err = json.Unmarshal(unmarshaled.Payload, &decodedPayload)
	require.NoError(t, err)
	assert.Equal(t, "msg123", decodedPayload["id"])
	assert.Equal(t, "Hello, World!", decodedPayload["content"])
	assert.Equal(t, "+1234567890", decodedPayload["sender"])
}

func TestWebhookEvent_EmptyPayload(t *testing.T) {
	event := WebhookEvent{
		Event:   "status",
		Payload: json.RawMessage(`{}`),
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled WebhookEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.Event, unmarshaled.Event)
	assert.Equal(t, `{}`, string(unmarshaled.Payload))
}

func TestMessagePayload_Marshal(t *testing.T) {
	now := time.Now()
	payload := MessagePayload{
		ID:        "msg123",
		ChatID:    "chat456",
		Sender:    "+1234567890",
		Timestamp: now,
		Type:      "text",
		Content:   "Hello, World!",
		MediaURL:  "",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var unmarshaled MessagePayload
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, payload.ID, unmarshaled.ID)
	assert.Equal(t, payload.ChatID, unmarshaled.ChatID)
	assert.Equal(t, payload.Sender, unmarshaled.Sender)
	assert.Equal(t, payload.Type, unmarshaled.Type)
	assert.Equal(t, payload.Content, unmarshaled.Content)
	assert.Equal(t, payload.MediaURL, unmarshaled.MediaURL)
	assert.WithinDuration(t, payload.Timestamp, unmarshaled.Timestamp, time.Second)
}

func TestSendMessageResponse_Marshal(t *testing.T) {
	resp := SendMessageResponse{
		MessageID: "msg789",
		Status:    "sent",
		Error:     "",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled SendMessageResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.MessageID, unmarshaled.MessageID)
	assert.Equal(t, resp.Status, unmarshaled.Status)
	assert.Equal(t, resp.Error, unmarshaled.Error)
}

func TestSendMessageResponse_WithError(t *testing.T) {
	resp := SendMessageResponse{
		MessageID: "",
		Status:    "failed",
		Error:     "Network timeout",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled SendMessageResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, resp.MessageID, unmarshaled.MessageID)
	assert.Equal(t, resp.Status, unmarshaled.Status)
	assert.Equal(t, resp.Error, unmarshaled.Error)
}

func TestContact_Marshal(t *testing.T) {
	contact := Contact{
		ID:          "123456@c.us",
		Number:      "+1234567890",
		Name:        "John Doe",
		PushName:    "JD",
		ShortName:   "John",
		IsMe:        false,
		IsGroup:     false,
		IsWAContact: true,
		IsMyContact: true,
		IsBlocked:   false,
	}

	data, err := json.Marshal(contact)
	require.NoError(t, err)

	var unmarshaled Contact
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, contact.ID, unmarshaled.ID)
	assert.Equal(t, contact.Number, unmarshaled.Number)
	assert.Equal(t, contact.Name, unmarshaled.Name)
	assert.Equal(t, contact.PushName, unmarshaled.PushName)
	assert.Equal(t, contact.ShortName, unmarshaled.ShortName)
	assert.Equal(t, contact.IsMe, unmarshaled.IsMe)
	assert.Equal(t, contact.IsGroup, unmarshaled.IsGroup)
	assert.Equal(t, contact.IsWAContact, unmarshaled.IsWAContact)
	assert.Equal(t, contact.IsMyContact, unmarshaled.IsMyContact)
	assert.Equal(t, contact.IsBlocked, unmarshaled.IsBlocked)
}

func TestContactsResponse_Marshal(t *testing.T) {
	contacts := []Contact{
		{
			ID:     "123@c.us",
			Number: "+1234567890",
			Name:   "John Doe",
		},
		{
			ID:     "456@c.us",
			Number: "+0987654321",
			Name:   "Jane Smith",
		},
	}

	resp := ContactsResponse{
		Contacts: contacts,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled ContactsResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Len(t, unmarshaled.Contacts, 2)
	assert.Equal(t, contacts[0].ID, unmarshaled.Contacts[0].ID)
	assert.Equal(t, contacts[0].Name, unmarshaled.Contacts[0].Name)
	assert.Equal(t, contacts[1].ID, unmarshaled.Contacts[1].ID)
	assert.Equal(t, contacts[1].Name, unmarshaled.Contacts[1].Name)
}

func TestContactsResponse_SingleContact(t *testing.T) {
	contact := &Contact{
		ID:     "123@c.us",
		Number: "+1234567890",
		Name:   "John Doe",
	}

	resp := ContactsResponse{
		Contact: contact,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled ContactsResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	require.NotNil(t, unmarshaled.Contact)
	assert.Equal(t, contact.ID, unmarshaled.Contact.ID)
	assert.Equal(t, contact.Number, unmarshaled.Contact.Number)
	assert.Equal(t, contact.Name, unmarshaled.Contact.Name)
}

func TestMessagePayload_IsGroupMessage(t *testing.T) {
	tests := []struct {
		name     string
		payload  MessagePayload
		expected bool
	}{
		{
			name: "group message with @g.us suffix",
			payload: MessagePayload{
				ChatID: "123456789@g.us",
			},
			expected: true,
		},
		{
			name: "direct message with @c.us suffix",
			payload: MessagePayload{
				ChatID: "1234567890@c.us",
			},
			expected: false,
		},
		{
			name: "empty chatID",
			payload: MessagePayload{
				ChatID: "",
			},
			expected: false,
		},
		{
			name: "chatID without suffix",
			payload: MessagePayload{
				ChatID: "1234567890",
			},
			expected: false,
		},
		{
			name: "chatID with partial @g.us",
			payload: MessagePayload{
				ChatID: "1234567890@g.u",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.payload.IsGroupMessage()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessagePayload_ExtractGroupID(t *testing.T) {
	tests := []struct {
		name     string
		payload  MessagePayload
		expected string
	}{
		{
			name: "group message returns group ID",
			payload: MessagePayload{
				ChatID: "123456789@g.us",
			},
			expected: "123456789@g.us",
		},
		{
			name: "direct message returns empty string",
			payload: MessagePayload{
				ChatID: "1234567890@c.us",
			},
			expected: "",
		},
		{
			name: "empty chatID returns empty string",
			payload: MessagePayload{
				ChatID: "",
			},
			expected: "",
		},
		{
			name: "invalid format returns empty string",
			payload: MessagePayload{
				ChatID: "invalid-format",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.payload.ExtractGroupID()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroup_GetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		group    Group
		expected string
	}{
		{
			name: "with subject",
			group: Group{
				ID:      "123456789@g.us",
				Subject: "Family Group",
			},
			expected: "Family Group",
		},
		{
			name: "without subject returns ID",
			group: Group{
				ID:      "123456789@g.us",
				Subject: "",
			},
			expected: "123456789@g.us",
		},
		{
			name: "empty subject returns ID",
			group: Group{
				ID:      "987654321@g.us",
				Subject: "",
			},
			expected: "987654321@g.us",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.group.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroup_Marshal(t *testing.T) {
	group := Group{
		ID:          "123456789@g.us",
		Subject:     "Test Group",
		Description: "A test group",
		Participants: []GroupParticipant{
			{
				ID:      "1111111111@c.us",
				Role:    "admin",
				IsAdmin: true,
			},
			{
				ID:      "2222222222@c.us",
				Role:    "member",
				IsAdmin: false,
			},
		},
		InviteLink: "https://chat.whatsapp.com/invite123",
		CreatedAt:  1234567890,
	}

	data, err := json.Marshal(group)
	require.NoError(t, err)

	var unmarshaled Group
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, group.ID, unmarshaled.ID)
	assert.Equal(t, group.Subject, unmarshaled.Subject)
	assert.Equal(t, group.Description, unmarshaled.Description)
	assert.Equal(t, group.InviteLink, unmarshaled.InviteLink)
	assert.Equal(t, group.CreatedAt, unmarshaled.CreatedAt)
	assert.Len(t, unmarshaled.Participants, 2)
	assert.Equal(t, group.Participants[0].ID, unmarshaled.Participants[0].ID)
	assert.Equal(t, group.Participants[0].Role, unmarshaled.Participants[0].Role)
	assert.Equal(t, group.Participants[0].IsAdmin, unmarshaled.Participants[0].IsAdmin)
}

func TestGroupParticipant_Marshal(t *testing.T) {
	participant := GroupParticipant{
		ID:      "1234567890@c.us",
		Role:    "admin",
		IsAdmin: true,
	}

	data, err := json.Marshal(participant)
	require.NoError(t, err)

	var unmarshaled GroupParticipant
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, participant.ID, unmarshaled.ID)
	assert.Equal(t, participant.Role, unmarshaled.Role)
	assert.Equal(t, participant.IsAdmin, unmarshaled.IsAdmin)
}
