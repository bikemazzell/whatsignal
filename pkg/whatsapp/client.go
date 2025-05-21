package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Client interface {
	SendText(chatID, text string) (*SendMessageResponse, error)
	SendMedia(chatID, mediaPath, caption string) (*SendMessageResponse, error)
}

type WhatsAppClient struct {
	baseURL string
	client  *http.Client
}

type SendMessageResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func NewClient(baseURL string) Client {
	return &WhatsAppClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (c *WhatsAppClient) SendText(chatID, message string) (*SendMessageResponse, error) {
	payload := map[string]interface{}{
		"chatId": chatID,
		"text":   message,
	}

	return c.sendRequest("/api/sendText", payload)
}

func (c *WhatsAppClient) SendMedia(chatID, mediaPath, caption string) (*SendMessageResponse, error) {
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

	req, err := http.NewRequest("POST", c.baseURL+"/api/sendMedia", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}

func (c *WhatsAppClient) sendRequest(endpoint string, payload interface{}) (*SendMessageResponse, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, result.Error)
	}

	return &result, nil
}
