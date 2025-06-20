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

// Client defines the interface for interacting with the Signal service.
// It includes context in all methods for better control over execution.
type Client interface {
	SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error)
	ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]types.SignalMessage, error)
	InitializeDevice(ctx context.Context) error
}

// SignalClient implements the Client interface for Signal REST API.
// Compatible with bbernhard/signal-cli-rest-api
type SignalClient struct {
	baseURL     string
	authToken   string
	client      *http.Client
	phoneNumber string
	deviceName  string
}

// NewClient creates a new Signal client for the REST API.
// If a custom httpClient is not provided (nil), a default one with a timeout is created.
func NewClient(baseURL, authToken, phoneNumber, deviceName string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second} // Default timeout
	}

	// Ensure baseURL doesn't end with slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &SignalClient{
		baseURL:     baseURL,
		authToken:   authToken,
		phoneNumber: phoneNumber,
		deviceName:  deviceName,
		client:      httpClient,
	}
}

// SendMessage sends a message to a recipient via Signal REST API.
// Compatible with bbernhard/signal-cli-rest-api /v2/send endpoint
func (c *SignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error) {
	// Prepare the REST API request payload
	payload := types.SendMessageRequest{
		Message:    message,
		Number:     c.phoneNumber,
		Recipients: []string{recipient},
	}

	// Add attachments if provided
	if len(attachments) > 0 {
		payload.Base64Attachments = make([]types.Attachment, len(attachments))
		for i, attachment := range attachments {
			// For now, assume attachments are file paths
			// In a full implementation, you'd read and base64 encode the files
			payload.Base64Attachments[i] = types.Attachment{
				Filename:    attachment,
				ContentType: "application/octet-stream", // Default content type
				Data:        "",                         // Would need to be base64 encoded file data
			}
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create the REST API request
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

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var result types.SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert REST response to our expected format
	// Signal CLI uses the timestamp as the internal message ID for quotes
	timestamp := result.Timestamp.Int64()
	response := &types.SendMessageResponse{
		Timestamp: timestamp,
		MessageID: fmt.Sprintf("%d", timestamp),
	}

	return response, nil
}

// ReceiveMessages polls for new messages from Signal REST API.
// Compatible with bbernhard/signal-cli-rest-api /v1/receive endpoint
func (c *SignalClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]types.SignalMessage, error) {
	// Build the REST API request URL with query parameters
	endpoint := fmt.Sprintf("%s/v1/receive/%s", c.baseURL, url.QueryEscape(c.phoneNumber))

	// Add timeout as query parameter if specified
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

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	
	// Parse the response - the REST API returns an array of messages directly
	var messages []types.RestMessage
	if err := json.Unmarshal(bodyBytes, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert REST messages to our expected format
	result := make([]types.SignalMessage, 0, len(messages))
	for _, msg := range messages {
		// Skip messages without dataMessage (receipts, sync messages, etc.)
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

		// Handle quoted messages if present
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

// Helper function to extract attachment file paths from REST API attachments
func extractAttachmentPaths(attachments []types.RestMessageAttachment) []string {
	if len(attachments) == 0 {
		return nil
	}

	paths := make([]string, len(attachments))
	for i, att := range attachments {
		paths[i] = att.ID // Use attachment ID as path
	}
	return paths
}

// InitializeDevice checks if the device is properly initialized with the Signal REST API.
// For bbernhard/signal-cli-rest-api, this checks the /v1/about endpoint to verify connectivity.
func (c *SignalClient) InitializeDevice(ctx context.Context) error {
	// Use the /v1/about endpoint to check if the service is accessible
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

	// Check if the service is responding properly
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device initialization failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the about response to verify the service is working
	var aboutResponse types.AboutResponse
	if err := json.NewDecoder(resp.Body).Decode(&aboutResponse); err != nil {
		return fmt.Errorf("failed to decode about response: %w", err)
	}

	// Verify that the service supports the required API versions
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
