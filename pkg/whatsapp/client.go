package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/security"
	"whatsignal/pkg/whatsapp/types"
)

const (
	// TypingDurationPerChar is the typing duration per character in milliseconds
	TypingDurationPerChar = time.Duration(constants.TypingDurationPerCharMs) * time.Millisecond
	// MaxTypingDuration is the maximum typing duration
	MaxTypingDuration = time.Duration(constants.MaxTypingDurationSec) * time.Second
)

type WhatsAppClient struct {
	baseURL          string
	apiKey           string
	sessionName      string
	client           *http.Client
	sessionMgr       types.SessionManager
	supportsVideo    *bool  // Cached video support status
}

func NewClient(config types.ClientConfig) types.WAClient {
	client := &WhatsAppClient{
		baseURL:      config.BaseURL,
		apiKey:       config.APIKey,
		sessionName:  config.SessionName,
		client:       &http.Client{Timeout: config.Timeout},
		sessionMgr:   NewSessionManager(config.BaseURL, config.APIKey, config.Timeout),
	}
	return client
}

func (c *WhatsAppClient) GetSessionName() string {
	return c.sessionName
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
				_ = resp.Body.Close()
				
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
				_ = resp.Body.Close()
			}
		} else {
			_ = resp.Body.Close()
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
	return c.sendSeenWithSession(ctx, chatID, c.sessionName)
}

func (c *WhatsAppClient) sendSeenWithSession(ctx context.Context, chatID, sessionName string) error {
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": sessionName,
	}
	_, err := c.sendRequest(ctx, types.APIBase+types.EndpointSendSeen, payload)
	return err
}

func (c *WhatsAppClient) startTyping(ctx context.Context, chatID string) error {
	return c.startTypingWithSession(ctx, chatID, c.sessionName)
}

func (c *WhatsAppClient) startTypingWithSession(ctx context.Context, chatID, sessionName string) error {
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": sessionName,
	}
	_, err := c.sendRequest(ctx, types.APIBase+types.EndpointStartTyping, payload)
	return err
}

func (c *WhatsAppClient) stopTyping(ctx context.Context, chatID string) error {
	return c.stopTypingWithSession(ctx, chatID, c.sessionName)
}

func (c *WhatsAppClient) stopTypingWithSession(ctx context.Context, chatID, sessionName string) error {
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": sessionName,
	}
	_, err := c.sendRequest(ctx, types.APIBase+types.EndpointStopTyping, payload)
	return err
}

func (c *WhatsAppClient) SendText(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
	return c.SendTextWithSession(ctx, chatID, text, c.sessionName)
}

func (c *WhatsAppClient) SendTextWithSession(ctx context.Context, chatID, text, sessionName string) (*types.SendMessageResponse, error) {
	// Try to send seen status and typing indicators (optional)
	if err := c.sendSeenWithSession(ctx, chatID, sessionName); err != nil {
		// Continue if this fails - it's optional
	}

	if err := c.startTypingWithSession(ctx, chatID, sessionName); err != nil {
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
		// Best effort cleanup - ignore error as context is already cancelled
		_ = c.stopTypingWithSession(ctx, chatID, sessionName)
		return nil, ctx.Err()
	}

	if err := c.stopTypingWithSession(ctx, chatID, sessionName); err != nil {
		// Continue if this fails - it's optional
	}

	payload := map[string]interface{}{
		"chatId":  chatID,
		"text":    text,
		"session": sessionName,
	}

	return c.sendRequest(ctx, types.APIBase+types.EndpointSendText, payload)
}

func (c *WhatsAppClient) SendMedia(ctx context.Context, chatID, mediaPath, caption string, mediaType types.MediaType) (*types.SendMessageResponse, error) {
	return c.SendMediaWithSession(ctx, chatID, mediaPath, caption, mediaType, c.sessionName)
}

func (c *WhatsAppClient) SendMediaWithSession(ctx context.Context, chatID, mediaPath, caption string, mediaType types.MediaType, sessionName string) (*types.SendMessageResponse, error) {
	// Validate file path to prevent directory traversal
	if err := security.ValidateFilePath(mediaPath); err != nil {
		return nil, fmt.Errorf("invalid media path: %w", err)
	}

	// Check file size and warn for large files
	fileInfo, err := os.Stat(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	
	const maxRecommendedSize = 50 * 1024 * 1024 // 50MB
	if fileInfo.Size() > maxRecommendedSize {
		fmt.Printf("WARNING: Large file detected (%d MB). This may cause performance issues.\n", fileInfo.Size()/(1024*1024))
	}
	
	// Check video support and downgrade to document if not supported
	if mediaType == types.MediaTypeVideo && !c.checkVideoSupport(ctx) {
		fmt.Printf("Video support not available, sending as document instead\n")
		mediaType = types.MediaTypeFile
	}

	// Read and encode file as base64
	fileData, err := os.ReadFile(mediaPath) // #nosec G304 - Path validated by security.ValidateFilePath above
	if err != nil {
		return nil, fmt.Errorf("failed to read media file: %w", err)
	}

	// Encode file data as base64
	base64Data := base64.StdEncoding.EncodeToString(fileData)

	// Determine MIME type from file extension
	ext := strings.ToLower(filepath.Ext(mediaPath))
	mimeType, ok := constants.MimeTypes[ext]
	if !ok {
		mimeType = constants.DefaultMimeType
	}

	// Extract filename from the full path
	filename := filepath.Base(mediaPath)

	// Create JSON payload according to WAHA API documentation
	payload := map[string]interface{}{
		"chatId":  chatID,
		"session": sessionName,
		"file": map[string]interface{}{
			"mimetype": mimeType,
			"data":     base64Data,
			"filename": filename,
		},
	}

	if caption != "" {
		payload["caption"] = caption
	}

	// Add video-specific fields (from WAHA docs)
	if mediaType == types.MediaTypeVideo {
		payload["convert"] = false
		payload["asNote"] = false // false = regular video, true = video note (rounded)
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
	return c.sendRequest(ctx, endpoint, payload)
}

func (c *WhatsAppClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, imagePath, caption, types.MediaTypeImage)
}

func (c *WhatsAppClient) SendImageWithSession(ctx context.Context, chatID, imagePath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return c.SendMediaWithSession(ctx, chatID, imagePath, caption, types.MediaTypeImage, sessionName)
}

func (c *WhatsAppClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, filePath, caption, types.MediaTypeFile)
}

func (c *WhatsAppClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, voicePath, "", types.MediaTypeVoice)
}

func (c *WhatsAppClient) SendVoiceWithSession(ctx context.Context, chatID, voicePath, sessionName string) (*types.SendMessageResponse, error) {
	return c.SendMediaWithSession(ctx, chatID, voicePath, "", types.MediaTypeVoice, sessionName)
}

func (c *WhatsAppClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, videoPath, caption, types.MediaTypeVideo)
}

func (c *WhatsAppClient) SendVideoWithSession(ctx context.Context, chatID, videoPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return c.SendMediaWithSession(ctx, chatID, videoPath, caption, types.MediaTypeVideo, sessionName)
}

func (c *WhatsAppClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	return c.SendMedia(ctx, chatID, docPath, caption, types.MediaTypeFile)
}

func (c *WhatsAppClient) SendDocumentWithSession(ctx context.Context, chatID, docPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return c.SendMediaWithSession(ctx, chatID, docPath, caption, types.MediaTypeFile, sessionName)
}

func (c *WhatsAppClient) SendReaction(ctx context.Context, chatID, messageID, reaction string) (*types.SendMessageResponse, error) {
	return c.SendReactionWithSession(ctx, chatID, messageID, reaction, c.sessionName)
}

func (c *WhatsAppClient) SendReactionWithSession(ctx context.Context, chatID, messageID, reaction, sessionName string) (*types.SendMessageResponse, error) {
	payload := types.ReactionRequest{
		Session:   sessionName,
		MessageID: messageID,
		Reaction:  reaction,
	}

	endpoint := types.APIBase + types.EndpointReaction
	return c.sendReactionRequest(ctx, endpoint, payload)
}

func (c *WhatsAppClient) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	// Validate parameters
	if chatID == "" {
		return fmt.Errorf("chatID cannot be empty")
	}
	if messageID == "" {
		return fmt.Errorf("messageID cannot be empty")
	}
	
	// Build the URL according to WAHA API: DELETE /api/{session}/chats/{chatId}/messages/{messageId}
	url := fmt.Sprintf("%s/api/%s/chats/%s/messages/%s", c.baseURL, c.sessionName, chatID, messageID)
	
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		// Try to decode error response
		var errorResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			if errMsg, ok := errorResp["error"].(string); ok {
				return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, errMsg)
			}
		}
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *WhatsAppClient) sendReactionRequest(ctx context.Context, endpoint string, payload interface{}) (*types.SendMessageResponse, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	fullURL := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, bytes.NewBuffer(jsonData))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Try to decode error response
		var result types.SendMessageResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
		}
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle empty response (which is valid for reactions)
	if len(responseBody) == 0 {
		return &types.SendMessageResponse{
			Status: "sent",
		}, nil
	}

	// Try to decode JSON response
	var result types.SendMessageResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *WhatsAppClient) sendRequest(ctx context.Context, endpoint string, payload interface{}) (*types.SendMessageResponse, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Debug logging for troubleshooting (commented out due to large payloads)
	// fmt.Printf("DEBUG: Sending request to %s\n", c.baseURL+endpoint)
	// fmt.Printf("DEBUG: Payload: %s\n", string(jsonData))

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

	// Read response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Error details logged in structured error message below
		
		// Try to parse error response
		var errorResult types.WAHAErrorResponse
		if err := json.Unmarshal(bodyBytes, &errorResult); err == nil && errorResult.Error != "" {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, errorResult.Error)
		}
		
		// Fallback to raw response body if no structured error
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Check if response body is empty - this is acceptable for some WAHA responses
	if len(bodyBytes) == 0 {
		// Return a success response with empty message ID
		// WAHA sometimes returns 201 with empty body when message is sent successfully
		return &types.SendMessageResponse{
			Status: "sent",
			Error:  "",
		}, nil
	}
	
	// Parse the actual WAHA response format
	var wahaResult types.WAHAMessageResponse
	if err := json.Unmarshal(bodyBytes, &wahaResult); err != nil {
		// If JSON parsing fails but we got a success status code, treat as successful send
		// Log the error but don't fail the operation since WAHA confirmed the message was sent
		return &types.SendMessageResponse{
			Status: "sent",
			Error:  fmt.Sprintf("warning: could not parse response: %v", err),
		}, nil
	}

	// Extract message ID from the WAHA response
	var messageID string
	if wahaResult.ID != nil && wahaResult.ID.Serialized != "" {
		messageID = wahaResult.ID.Serialized
	} else if wahaResult.Data != nil && wahaResult.Data.ID != nil && wahaResult.Data.ID.Serialized != "" {
		messageID = wahaResult.Data.ID.Serialized
	}
	
	// Convert to our standard response format
	result := &types.SendMessageResponse{
		MessageID: messageID,
		Status:    "sent",
		Error:     "",
	}

	return result, nil
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

// getServerVersion retrieves the WAHA server version info
func (c *WhatsAppClient) getServerVersion(ctx context.Context) (*types.ServerVersion, error) {
	url := fmt.Sprintf("%s/api/server/version", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Api-Key", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get server version, status: %d", resp.StatusCode)
	}

	var version types.ServerVersion
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return nil, fmt.Errorf("failed to decode server version: %w", err)
	}

	return &version, nil
}

// checkVideoSupport checks if the WAHA server supports video sending
func (c *WhatsAppClient) checkVideoSupport(ctx context.Context) bool {
	// Return cached value if already checked
	if c.supportsVideo != nil {
		return *c.supportsVideo
	}

	// Default to false
	supportsVideo := false

	// Get server version
	version, err := c.getServerVersion(ctx)
	if err != nil {
		// Log error but don't fail - assume no video support
		fmt.Printf("Failed to check WAHA version for video support: %v\n", err)
		c.supportsVideo = &supportsVideo
		return supportsVideo
	}

	// Check if tier is PLUS and browser is chrome
	if version.Tier == "PLUS" && strings.Contains(version.Browser, "google-chrome") {
		supportsVideo = true
		fmt.Printf("WAHA Plus detected (tier: %s, browser: %s) - video sending enabled\n", version.Tier, version.Browser)
	} else {
		fmt.Printf("WAHA version does not support video (tier: %s, browser: %s) - videos will be sent as documents\n", version.Tier, version.Browser)
	}

	// Cache the result
	c.supportsVideo = &supportsVideo
	return supportsVideo
}