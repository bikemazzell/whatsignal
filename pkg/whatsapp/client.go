package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/security"
	"whatsignal/pkg/whatsapp/types"
)

const (
	// TypingDurationPerChar is the typing duration per character in milliseconds
	TypingDurationPerChar = 50 * time.Millisecond
	// MaxTypingDuration is the maximum typing duration
	MaxTypingDuration = 3 * time.Second
)

type WhatsAppClient struct {
	baseURL     string
	apiKey      string
	sessionName string
	client      *http.Client
	sessionMgr  types.SessionManager
}

func NewClient(config types.ClientConfig) types.WAClient {
	client := &WhatsAppClient{
		baseURL:     config.BaseURL,
		apiKey:      config.APIKey,
		sessionName: config.SessionName,
		client:      &http.Client{Timeout: config.Timeout},
		sessionMgr:  NewSessionManager(config.BaseURL, config.APIKey, config.Timeout),
	}
	return client
}

func (c *WhatsAppClient) CreateSession(ctx context.Context) error {
	_, err := c.sessionMgr.Create(ctx, c.sessionName)
	return err
}

func (c *WhatsAppClient) StartSession(ctx context.Context) error {
	return c.sessionMgr.Start(ctx, c.sessionName)
}

func (c *WhatsAppClient) StopSession(ctx context.Context) error {
	return c.sessionMgr.Stop(ctx, c.sessionName)
}

func (c *WhatsAppClient) RestartSession(ctx context.Context) error {
	// Use WAHA's restart endpoint
	url := fmt.Sprintf("%s/api/sessions/%s/restart", c.baseURL, c.sessionName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create restart request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to restart session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errorResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			if errMsg, ok := errorResp["error"].(string); ok {
				return fmt.Errorf("restart failed with status %d: %s", resp.StatusCode, errMsg)
			}
		}
		return fmt.Errorf("restart failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *WhatsAppClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	// Get the real-time status from WAHA API instead of cached value
	url := fmt.Sprintf("%s/api/sessions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get sessions, status: %d", resp.StatusCode)
	}

	var sessions []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}

	// Find our session
	for _, session := range sessions {
		if name, ok := session["name"].(string); ok && name == c.sessionName {
			status := "unknown"
			if s, ok := session["status"].(string); ok {
				status = s
			}
			return &types.Session{
				Name:      c.sessionName,
				Status:    types.SessionStatus(status),
				UpdatedAt: time.Now(),
			}, nil
		}
	}

	return nil, fmt.Errorf("session %s not found", c.sessionName)
}

// WaitForSessionReady waits for the WhatsApp session to be in WORKING status
func (c *WhatsAppClient) WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error {
	deadline := time.Now().Add(maxWaitTime)
	backoff := time.Duration(constants.DefaultBackoffInitialMs) * time.Millisecond
	maxBackoff := time.Duration(constants.DefaultBackoffMaxSec) * time.Second

	for time.Now().Before(deadline) {
		// Get session status directly from WAHA API
		url := fmt.Sprintf("%s/api/sessions", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		if c.apiKey != "" {
			req.Header.Set("X-Api-Key", c.apiKey)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get sessions: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var sessions []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&sessions); err == nil {
				resp.Body.Close()
				
				// Find our session
				for _, session := range sessions {
					if name, ok := session["name"].(string); ok && name == c.sessionName {
						if status, ok := session["status"].(string); ok {
							if status == "WORKING" {
								return nil // Session is ready
							}
							// Log current status for debugging
							fmt.Printf("Session %s status: %s, waiting...\n", c.sessionName, status)
						}
						break
					}
				}
			} else {
				resp.Body.Close()
			}
		} else {
			resp.Body.Close()
		}

		// Wait with exponential backoff
		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("timeout waiting for session to be ready after %v", maxWaitTime)
}

func (c *WhatsAppClient) sendSeen(ctx context.Context, chatID string) error {
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": c.sessionName,
	}
	_, err := c.sendRequest(ctx, types.APIBase+types.EndpointSendSeen, payload)
	return err
}

func (c *WhatsAppClient) startTyping(ctx context.Context, chatID string) error {
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": c.sessionName,
	}
	_, err := c.sendRequest(ctx, types.APIBase+types.EndpointStartTyping, payload)
	return err
}

func (c *WhatsAppClient) stopTyping(ctx context.Context, chatID string) error {
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": c.sessionName,
	}
	_, err := c.sendRequest(ctx, types.APIBase+types.EndpointStopTyping, payload)
	return err
}

func (c *WhatsAppClient) SendText(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
	// Try to send seen status and typing indicators (optional)
	if err := c.sendSeen(ctx, chatID); err != nil {
		// Continue if this fails - it's optional
	}

	if err := c.startTyping(ctx, chatID); err != nil {
		// Continue if this fails - it's optional
	}

	typingDuration := time.Duration(len(text)) * TypingDurationPerChar
	if typingDuration > MaxTypingDuration {
		typingDuration = MaxTypingDuration
	}

	// Use context-aware sleep to avoid blocking indefinitely
	select {
	case <-time.After(typingDuration):
		// Normal completion
	case <-ctx.Done():
		// Context cancelled, stop typing and return
		c.stopTyping(ctx, chatID) // Best effort cleanup
		return nil, ctx.Err()
	}

	if err := c.stopTyping(ctx, chatID); err != nil {
		// Continue if this fails - it's optional
	}

	payload := map[string]interface{}{
		"chatId":  chatID,
		"text":    text,
		"session": c.sessionName,
	}

	return c.sendRequest(ctx, types.APIBase+types.EndpointSendText, payload)
}

func (c *WhatsAppClient) SendMedia(ctx context.Context, chatID, mediaPath, caption string, mediaType types.MediaType) (*types.SendMessageResponse, error) {
	// Validate file path to prevent directory traversal
	if err := security.ValidateFilePath(mediaPath); err != nil {
		return nil, fmt.Errorf("invalid media path: %w", err)
	}

	file, err := os.Open(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open media file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(mediaPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.WriteField("chatId", chatID); err != nil {
		return nil, fmt.Errorf("failed to write chatId field: %w", err)
	}
	if err := writer.WriteField("session", c.sessionName); err != nil {
		return nil, fmt.Errorf("failed to write session field: %w", err)
	}
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return nil, fmt.Errorf("failed to write caption field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	var apiActionPath string
	switch mediaType {
	case types.MediaTypeImage:
		apiActionPath = types.EndpointSendImage
	case types.MediaTypeFile:
		apiActionPath = types.EndpointSendFile
	case types.MediaTypeVoice:
		apiActionPath = types.EndpointSendVoice
	case types.MediaTypeVideo:
		apiActionPath = types.EndpointSendVideo
	default:
		return nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}

	endpoint := types.APIBase + apiActionPath
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result types.SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}

func (c *WhatsAppClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, imagePath, caption, types.MediaTypeImage)
}

func (c *WhatsAppClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, filePath, caption, types.MediaTypeFile)
}

func (c *WhatsAppClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, voicePath, "", types.MediaTypeVoice)
}

func (c *WhatsAppClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, videoPath, caption, types.MediaTypeVideo)
}

func (c *WhatsAppClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, docPath, caption, types.MediaTypeFile)
}

func (c *WhatsAppClient) sendRequest(ctx context.Context, endpoint string, payload interface{}) (*types.SendMessageResponse, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result types.SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}

// GetContact retrieves a specific contact by ID (phone number or chat ID)
func (c *WhatsAppClient) GetContact(ctx context.Context, contactID string) (*types.Contact, error) {
	// Build the URL with query parameters (session and contactId as query params)
	endpoint := fmt.Sprintf("%s%s", types.APIBase, types.EndpointContacts)
	url := fmt.Sprintf("%s%s?contactId=%s&session=%s", c.baseURL, endpoint, contactID, c.sessionName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Contact not found
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	var contact types.Contact
	if err := json.NewDecoder(resp.Body).Decode(&contact); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &contact, nil
}

// GetAllContacts retrieves all contacts with pagination
func (c *WhatsAppClient) GetAllContacts(ctx context.Context, limit, offset int) ([]types.Contact, error) {
	// Build the URL with query parameters (session as query param)
	endpoint := fmt.Sprintf("%s%s", types.APIBase, types.EndpointContactsAll)
	url := fmt.Sprintf("%s%s?session=%s&limit=%d&offset=%d&sortBy=name&sortOrder=asc", 
		c.baseURL, endpoint, c.sessionName, limit, offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try to decode error response
		var errorResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			if errMsg, ok := errorResp["error"].(string); ok {
				return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, errMsg)
			}
		}
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	var contacts []types.Contact
	if err := json.NewDecoder(resp.Body).Decode(&contacts); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return contacts, nil
}