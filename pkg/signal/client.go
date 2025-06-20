package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"whatsignal/pkg/signal/types"
)

type Client interface {
	SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error)
	ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]types.SignalMessage, error)
	InitializeDevice(ctx context.Context) error
}

type SignalClient struct {
	baseURL     string
	authToken   string
	client      *http.Client
	phoneNumber string
	deviceName  string
}

func NewClient(baseURL, authToken, phoneNumber, deviceName string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	return &SignalClient{
		baseURL:     baseURL,
		authToken:   authToken,
		phoneNumber: phoneNumber,
		deviceName:  deviceName,
		client:      httpClient,
	}
}

func (c *SignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error) {
	payload := types.SendMessageRequest{
		Message:    message,
		Number:     c.phoneNumber,
		Recipients: []string{recipient},
	}

	if len(attachments) > 0 {
		payload.Base64Attachments = make([]types.Attachment, len(attachments))
		for i, attachment := range attachments {
			payload.Base64Attachments[i] = types.Attachment{
				Filename:    attachment,
				ContentType: "application/octet-stream",
				Data:        "",
			}
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v2/send", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result types.SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	timestamp := result.Timestamp.Int64()
	response := &types.SendMessageResponse{
		Timestamp: timestamp,
		MessageID: fmt.Sprintf("%d", timestamp),
	}

	return response, nil
}

func (c *SignalClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]types.SignalMessage, error) {
	endpoint := fmt.Sprintf("%s/v1/receive/%s", c.baseURL, url.QueryEscape(c.phoneNumber))

	if timeoutSeconds > 0 {
		endpoint += fmt.Sprintf("?timeout=%d", timeoutSeconds)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	
	var messages []types.RestMessage
	if err := json.Unmarshal(bodyBytes, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]types.SignalMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Envelope.DataMessage == nil {
			continue
		}
		
		sigMsg := types.SignalMessage{
			Timestamp:   msg.Envelope.Timestamp,
			Sender:      msg.Envelope.Source,
			MessageID:   fmt.Sprintf("%d", msg.Envelope.Timestamp),
			Message:     msg.Envelope.DataMessage.Message,
			Attachments: extractAttachmentPaths(msg.Envelope.DataMessage.Attachments),
		}

		if msg.Envelope.DataMessage.Quote != nil {
			sigMsg.QuotedMessage = &struct {
				ID        string `json:"id"`
				Author    string `json:"author"`
				Text      string `json:"text"`
				Timestamp int64  `json:"timestamp"`
			}{
				ID:        fmt.Sprintf("%d", msg.Envelope.DataMessage.Quote.ID),
				Author:    msg.Envelope.DataMessage.Quote.Author,
				Text:      msg.Envelope.DataMessage.Quote.Text,
				Timestamp: msg.Envelope.DataMessage.Quote.ID,
			}
		}
		
		result = append(result, sigMsg)
	}

	return result, nil
}

func extractAttachmentPaths(attachments []types.RestMessageAttachment) []string {
	if len(attachments) == 0 {
		return nil
	}

	paths := make([]string, len(attachments))
	for i, att := range attachments {
		paths[i] = att.ID
	}
	return paths
}

func (c *SignalClient) InitializeDevice(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/v1/about", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create initialize device request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send initialize device request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device initialization failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var aboutResponse types.AboutResponse
	if err := json.NewDecoder(resp.Body).Decode(&aboutResponse); err != nil {
		return fmt.Errorf("failed to decode about response: %w", err)
	}

	hasV1 := false
	hasV2 := false
	for _, version := range aboutResponse.Versions {
		if version == "v1" {
			hasV1 = true
		}
		if version == "v2" {
			hasV2 = true
		}
	}

	if !hasV1 || !hasV2 {
		return fmt.Errorf("signal-cli-rest-api service does not support required API versions (v1, v2)")
	}

	return nil
}
