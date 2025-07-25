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

func setupTestSessionManager(t *testing.T) (*sessionManager, *httptest.Server) {
	sessionStates := make(map[string]types.SessionStatus)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/sessions":
			sessionStates["test"] = types.SessionStatusInitialized
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case "GET /api/sessions/test":
			status, exists := sessionStates["test"]
			if !exists {
				status = types.SessionStatusInitialized
			}
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(types.Session{
				Name:      "test",
				Status:    status,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case "POST /api/sessions/test/start":
			sessionStates["test"] = types.SessionStatusStarting
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case "POST /api/sessions/test/stop":
			sessionStates["test"] = types.SessionStatusStopped
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case "DELETE /api/sessions/test":
			delete(sessionStates, "test")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	sm := NewSessionManager(server.URL, "test-api-key", 5*time.Second).(*sessionManager)
	return sm, server
}

func TestSessionManager_Create(t *testing.T) {
	sm, server := setupTestSessionManager(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful creation
	session, err := sm.Create(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, "test", session.Name)
	assert.Equal(t, types.SessionStatusInitialized, session.Status)

	// Test duplicate creation
	_, err = sm.Create(ctx, "test")
	assert.Error(t, err)
}

func TestSessionManager_Get(t *testing.T) {
	sm, server := setupTestSessionManager(t)
	defer server.Close()

	ctx := context.Background()

	// Create a session first
	_, err := sm.Create(ctx, "test")
	require.NoError(t, err)

	// Test successful get
	session, err := sm.Get(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, "test", session.Name)
	assert.Equal(t, types.SessionStatusInitialized, session.Status)

	// Test non-existent session
	_, err = sm.Get(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestSessionManager_Start(t *testing.T) {
	sm, server := setupTestSessionManager(t)
	defer server.Close()

	ctx := context.Background()

	// Create a session first
	_, err := sm.Create(ctx, "test")
	require.NoError(t, err)

	// Test successful start
	err = sm.Start(ctx, "test")
	require.NoError(t, err)

	session, err := sm.Get(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, types.SessionStatusStarting, session.Status)

	// Test non-existent session
	err = sm.Start(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestSessionManager_Stop(t *testing.T) {
	sm, server := setupTestSessionManager(t)
	defer server.Close()

	ctx := context.Background()

	// Create and start a session first
	_, err := sm.Create(ctx, "test")
	require.NoError(t, err)
	err = sm.Start(ctx, "test")
	require.NoError(t, err)

	// Test successful stop
	err = sm.Stop(ctx, "test")
	require.NoError(t, err)

	session, err := sm.Get(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, types.SessionStatusStopped, session.Status)

	// Test non-existent session
	err = sm.Stop(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestSessionManager_Delete(t *testing.T) {
	sm, server := setupTestSessionManager(t)
	defer server.Close()

	ctx := context.Background()

	// Create a session first
	_, err := sm.Create(ctx, "test")
	require.NoError(t, err)

	// Test successful delete
	err = sm.Delete(ctx, "test")
	require.NoError(t, err)

	// Verify session is deleted
	_, err = sm.Get(ctx, "test")
	assert.Error(t, err)

	// Test deleting non-existent session
	err = sm.Delete(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestSessionManager_ApiKeyHeader(t *testing.T) {
	apiKey := "test-api-key"
	var receivedApiKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedApiKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	sm := NewSessionManager(server.URL, apiKey, 5*time.Second)
	ctx := context.Background()

	_, err := sm.Create(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, apiKey, receivedApiKey)
}
