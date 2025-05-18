package signal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	rpcURL string
	client *http.Client
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

func NewClient(rpcURL string) *Client {
	return &Client{
		rpcURL: rpcURL,
		client: &http.Client{},
	}
}

func (c *Client) SendMessage(recipient, message string, attachments []string) (*SendMessageResponse, error) {
	request := SendMessageRequest{
		Jsonrpc: "2.0",
		Method:  "send",
		ID:      1,
	}
	request.Params.Number = recipient
	request.Params.Message = message
	request.Params.Attachments = attachments

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.rpcURL, bytes.NewBuffer(jsonData))
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

func (c *Client) ReceiveMessages(timeoutSeconds int) ([]SignalMessage, error) {
	request := ReceiveMessageRequest{
		Jsonrpc: "2.0",
		Method:  "receive",
		ID:      1,
	}
	request.Params.Timeout = timeoutSeconds

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.rpcURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
