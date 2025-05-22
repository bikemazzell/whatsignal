package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client defines the interface for interacting with the Signal service.
// It includes context in all methods for better control over execution.
type Client interface {
	SendMessage(ctx context.Context, recipient, message string, attachments []string) (*SendMessageResponse, error)
	ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]SignalMessage, error)
	InitializeDevice(ctx context.Context) error
}

// SignalClient implements the Client interface for Signal.
// It now includes a configurable http.Client.
type SignalClient struct {
	rpcURL      string
	authToken   string
	client      *http.Client
	phoneNumber string
	deviceName  string
}

type SendMessageRequest struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Number      string   `json:"number"`
		Message     string   `json:"message"`
		Attachments []string `json:"attachments,omitempty"`
	} `json:"params"`
	ID int `json:"id"`
}

type SendMessageResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Timestamp int64  `json:"timestamp"`
		MessageID string `json:"messageId"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

// NewClient creates a new Signal client.
// If a custom httpClient is not provided (nil), a default one with a timeout is created.
func NewClient(rpcURL, authToken, phoneNumber, deviceName string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second} // Default timeout
	}
	return &SignalClient{
		rpcURL:      rpcURL,
		authToken:   authToken,
		phoneNumber: phoneNumber,
		deviceName:  deviceName,
		client:      httpClient,
	}
}

// SendMessage sends a message to a recipient via Signal JSON-RPC.
// It now accepts a context.
func (c *SignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*SendMessageResponse, error) {
	request := SendMessageRequest{
		Jsonrpc: "2.0",
		Method:  "send",
		ID:      1, // Consider making ID dynamic if concurrent client use is expected
	}
	request.Params.Number = recipient
	request.Params.Message = message
	request.Params.Attachments = attachments

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewBuffer(jsonData)) // Used NewRequestWithContext
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

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return &result, fmt.Errorf("signal-cli error: %s (code: %d)", result.Error.Message, result.Error.Code)
	}

	return &result, nil
}

type ReceiveMessageRequest struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Timeout int `json:"timeout"`
	} `json:"params"`
	ID int `json:"id"`
}

type SignalMessage struct {
	Timestamp     int64    `json:"timestamp"`
	Sender        string   `json:"sender"`
	MessageID     string   `json:"messageId"`
	Message       string   `json:"message"`
	Attachments   []string `json:"attachments"`
	QuotedMessage *struct {
		ID        string `json:"id"`
		Author    string `json:"author"`
		Text      string `json:"text"`
		Timestamp int64  `json:"timestamp"`
	} `json:"quotedMessage,omitempty"`
}

type ReceiveMessageResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  []SignalMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

// ReceiveMessages polls for new messages from Signal JSON-RPC.
// It now accepts a context.
func (c *SignalClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]SignalMessage, error) {
	request := ReceiveMessageRequest{
		Jsonrpc: "2.0",
		Method:  "receive",
		ID:      1, // Consider making ID dynamic
	}
	request.Params.Timeout = timeoutSeconds

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewBuffer(jsonData)) // Used NewRequestWithContext
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

	var result ReceiveMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("signal-cli error: %s (code: %d)", result.Error.Message, result.Error.Code)
	}

	return result.Result, nil
}

// InitializeDevice attempts to initialize the device with the Signal JSON-RPC service.
// This might be for linking or ensuring the account specified by phoneNumber and deviceName is active.
// True number registration and verification must typically be done directly with signal-cli commands first.
// It now accepts a context.
func (c *SignalClient) InitializeDevice(ctx context.Context) error {
	request := struct {
		Jsonrpc string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			// Parameters for this method depend on the specific signal-cli JSON-RPC version and setup.
			// For linking, it might expect parameters like "uri" from `signal-cli link`.
			// For checking an existing account, parameters might differ.
			// The current params are based on the previous Register method.
			PhoneNumber string `json:"phone,omitempty"`      // Using omitempty as usage is unclear
			DeviceName  string `json:"deviceName,omitempty"` // Using omitempty
		} `json:"params"`
		ID int `json:"id"`
	}{
		Jsonrpc: "2.0",
		Method:  "register", // This method name "register" via JSON-RPC might not perform full registration.
		// It could be for checking an existing account or a specific linking step.
		// Signal-CLI docs suggest full registration is not typically done via JSON-RPC directly.
		ID: 1, // Consider making ID dynamic
	}
	// Only set params if they are provided; behavior depends on signal-cli
	if c.phoneNumber != "" {
		request.Params.PhoneNumber = c.phoneNumber
	}
	if c.deviceName != "" {
		request.Params.DeviceName = c.deviceName
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal initialize device request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewBuffer(jsonData)) // Used NewRequestWithContext
	if err != nil {
		return fmt.Errorf("failed to create initialize device request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send initialize device request: %w", err)
	}
	defer resp.Body.Close()

	// For a method like this, the response might not be a standard SendMessageResponse or ReceiveMessageResponse.
	// It could be a generic success/failure, or specific registration/linking status.
	// For now, just checking status code. A more robust implementation would parse the specific response.
	if resp.StatusCode != http.StatusOK {
		// Attempt to read error body for more details
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device initialization failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
