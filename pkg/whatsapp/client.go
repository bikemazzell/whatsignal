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

	"whatsignal/pkg/whatsapp/types"
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

func (c *WhatsAppClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	return c.sessionMgr.Get(ctx, c.sessionName)
}

func (c *WhatsAppClient) sendSeen(ctx context.Context, chatID string) error {
	payload := map[string]interface{}{
		"chatId": chatID,
	}
	_, err := c.sendRequest(ctx, fmt.Sprintf(types.APIBase+types.EndpointSendSeen, c.sessionName), payload)
	return err
}

func (c *WhatsAppClient) startTyping(ctx context.Context, chatID string) error {
	payload := map[string]interface{}{
		"chatId": chatID,
	}
	_, err := c.sendRequest(ctx, fmt.Sprintf(types.APIBase+types.EndpointStartTyping, c.sessionName), payload)
	return err
}

func (c *WhatsAppClient) stopTyping(ctx context.Context, chatID string) error {
	payload := map[string]interface{}{
		"chatId": chatID,
	}
	_, err := c.sendRequest(ctx, fmt.Sprintf(types.APIBase+types.EndpointStopTyping, c.sessionName), payload)
	return err
}

func (c *WhatsAppClient) SendText(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
	// 1. Send seen
	if err := c.sendSeen(ctx, chatID); err != nil {
		return nil, fmt.Errorf("failed to send seen: %w", err)
	}

	// 2. Start typing
	if err := c.startTyping(ctx, chatID); err != nil {
		return nil, fmt.Errorf("failed to start typing: %w", err)
	}

	// 3. Wait based on message length (simulating typing)
	typingDuration := time.Duration(len(text)) * 50 * time.Millisecond // 50ms per character
	if typingDuration > 3*time.Second {
		typingDuration = 3 * time.Second // Cap at 3 seconds
	}
	time.Sleep(typingDuration)

	// 4. Stop typing
	if err := c.stopTyping(ctx, chatID); err != nil {
		return nil, fmt.Errorf("failed to stop typing: %w", err)
	}

	// 5. Send the message
	payload := map[string]interface{}{
		"chatId": chatID,
		"text":   text,
	}

	return c.sendRequest(ctx, fmt.Sprintf(types.APIBase+types.EndpointSendText, c.sessionName), payload)
}

func (c *WhatsAppClient) SendMedia(ctx context.Context, chatID, mediaPath, caption string, mediaType types.MediaType) (*types.SendMessageResponse, error) {
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

	writer.WriteField("chatId", chatID)
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	var apiActionPath string
	switch mediaType {
	case types.MediaTypeImage:
		apiActionPath = types.EndpointSendImage
	case types.MediaTypeFile: // Used by SendFile and SendDocument
		apiActionPath = types.EndpointSendFile
	case types.MediaTypeVoice:
		apiActionPath = types.EndpointSendVoice
	case types.MediaTypeVideo:
		apiActionPath = types.EndpointSendVideo
	default:
		return nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}

	endpoint := fmt.Sprintf(types.APIBase+apiActionPath, c.sessionName)
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

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}

// Convenience methods for different media types
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

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}
