package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"whatsignal/pkg/whatsapp/types"
)

type sessionManager struct {
	baseURL  string
	apiKey   string
	client   *http.Client
	sessions map[string]*types.Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(baseURL, apiKey string, timeout time.Duration) types.SessionManager {
	return &sessionManager{
		baseURL:  baseURL,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: timeout},
		sessions: make(map[string]*types.Session),
	}
}

func (sm *sessionManager) Create(ctx context.Context, name string) (*types.Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[name]; exists {
		return nil, fmt.Errorf("session %s already exists", name)
	}

	session := &types.Session{
		Name:      name,
		Status:    types.SessionStatusInitialized,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/sessions", sm.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if sm.apiKey != "" {
		req.Header.Set("X-Api-Key", sm.apiKey)
	}

	resp, err := sm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create session, status: %d", resp.StatusCode)
	}

	sm.sessions[name] = session
	return session, nil
}

func (sm *sessionManager) Get(ctx context.Context, name string) (*types.Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[name]
	if !exists {
		return nil, fmt.Errorf("session %s not found", name)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/sessions/%s", sm.baseURL, name), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if sm.apiKey != "" {
		req.Header.Set("X-Api-Key", sm.apiKey)
	}

	resp, err := sm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session, status: %d", resp.StatusCode)
	}

	var serverSession types.Session
	if err := json.NewDecoder(resp.Body).Decode(&serverSession); err != nil {
		return nil, fmt.Errorf("failed to decode session response: %w", err)
	}

	session.Status = serverSession.Status
	session.UpdatedAt = serverSession.UpdatedAt

	return session, nil
}

func (sm *sessionManager) Start(ctx context.Context, name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[name]
	if !exists {
		return fmt.Errorf("session %s not found", name)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/sessions/%s/start", sm.baseURL, name), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if sm.apiKey != "" {
		req.Header.Set("X-Api-Key", sm.apiKey)
	}

	resp, err := sm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to start session, status: %d", resp.StatusCode)
	}

	session.Status = types.SessionStatusStarting
	session.UpdatedAt = time.Now()
	return nil
}

func (sm *sessionManager) Stop(ctx context.Context, name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[name]
	if !exists {
		return fmt.Errorf("session %s not found", name)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/sessions/%s/stop", sm.baseURL, name), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if sm.apiKey != "" {
		req.Header.Set("X-Api-Key", sm.apiKey)
	}

	resp, err := sm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to stop session, status: %d", resp.StatusCode)
	}

	session.Status = types.SessionStatusStopped
	session.UpdatedAt = time.Now()
	return nil
}

func (sm *sessionManager) Delete(ctx context.Context, name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[name]; !exists {
		return fmt.Errorf("session %s not found", name)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/api/sessions/%s", sm.baseURL, name), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if sm.apiKey != "" {
		req.Header.Set("X-Api-Key", sm.apiKey)
	}

	resp, err := sm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete session, status: %d", resp.StatusCode)
	}

	delete(sm.sessions, name)
	return nil
}
