package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestartSession_ErrorPaths(t *testing.T) {
	// Server that returns 500 with JSON error body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "restart not allowed"})
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, APIKey: "test-api-key", SessionName: "test-session", Timeout: 5 * time.Second}).(*WhatsAppClient)
	ctx := context.Background()
	err := client.RestartSession(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restart failed")

	// Network error
	badClient := NewClient(types.ClientConfig{BaseURL: "http://127.0.0.1:0", APIKey: "test-api-key", SessionName: "test-session", Timeout: 10 * time.Millisecond}).(*WhatsAppClient)
	err = badClient.RestartSession(ctx)
	assert.Error(t, err)
}

func TestSendTextWithSession_OptionalEndpointsErrors(t *testing.T) {
	// Server returns success for sendText, but 404 for optional endpoints
	var sendTextCalled, seenCalled, typingStartCalled, typingStopCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sessions":
			// Return session status for validation
			_ = json.NewEncoder(w).Encode([]types.Session{{Name: "sess", Status: "WORKING"}})
		case types.APIBase + types.EndpointSendText:
			sendTextCalled = true
			_ = json.NewEncoder(w).Encode(types.WAHAMessageResponse{ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{FromMe: true, Remote: "123@c.us", ID: "id1", Serialized: "id1"}})
		case types.APIBase + types.EndpointSendSeen:
			seenCalled = true
			w.WriteHeader(http.StatusNotFound)
		case types.APIBase + types.EndpointStartTyping:
			typingStartCalled = true
			w.WriteHeader(http.StatusNotFound)
		case types.APIBase + types.EndpointStopTyping:
			typingStopCalled = true
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, APIKey: "test-api-key", SessionName: "sess", Timeout: 5 * time.Second}).(*WhatsAppClient)
	ctx := context.Background()
	resp, err := client.SendTextWithSession(ctx, "123@c.us", "hello", "sess")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, sendTextCalled)
	assert.True(t, seenCalled)
	assert.True(t, typingStartCalled)
	assert.True(t, typingStopCalled)
}

func TestGetAllContacts_ErrorBodiesAndStatuses(t *testing.T) {
	// Server returns 429 with structured error body, then 503 with plain body
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "rate limited"})
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unavailable"))
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, APIKey: "test-api-key", SessionName: "sess", Timeout: 5 * time.Second}).(*WhatsAppClient)
	ctx := context.Background()
	contacts, err := client.GetAllContacts(ctx, 10, 0)
	assert.Error(t, err)
	assert.Nil(t, contacts)
	assert.Contains(t, err.Error(), "request failed")
}

func TestSendReactionRequest_Paths(t *testing.T) {
	// Cover success with empty body, success with JSON body, and error with JSON body
	type mode int
	const (
		modeEmpty mode = iota
		modeJSON
		modeErrJSON
	)
	current := modeEmpty

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch current {
		case modeEmpty:
			w.WriteHeader(http.StatusCreated)
			// no body
		case modeJSON:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(types.SendMessageResponse{MessageID: "rid", Status: "sent"})
		case modeErrJSON:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(types.SendMessageResponse{Error: "bad reaction"})
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, APIKey: "k", SessionName: "s", Timeout: 5 * time.Second}).(*WhatsAppClient)
	ctx := context.Background()

	// success with empty body
	current = modeEmpty
	resp, err := client.sendReactionRequest(ctx, types.APIBase+types.EndpointReaction, types.ReactionRequest{Session: "s", MessageID: "m", Reaction: "👍"})
	require.NoError(t, err)
	assert.Equal(t, "sent", resp.Status)

	// success with JSON body
	current = modeJSON
	resp, err = client.sendReactionRequest(ctx, types.APIBase+types.EndpointReaction, types.ReactionRequest{Session: "s", MessageID: "m", Reaction: "👍"})
	require.NoError(t, err)
	assert.Equal(t, "sent", resp.Status)
	assert.Equal(t, "rid", resp.MessageID)

	// error with JSON body
	current = modeErrJSON
	resp, err = client.sendReactionRequest(ctx, types.APIBase+types.EndpointReaction, types.ReactionRequest{Session: "s", MessageID: "m", Reaction: "👍"})
	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, err.Error(), "request failed")
}
