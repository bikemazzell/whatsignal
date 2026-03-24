package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"whatsignal/pkg/signal/types"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"
)

// WSReceiver connects to signal-cli-rest-api's WebSocket endpoint for receiving
// messages in json-rpc mode. Each call to ReadMessage blocks until a message
// arrives or the connection is closed.
type WSReceiver struct {
	baseURL     string
	phoneNumber string
	logger      *logrus.Logger
}

// NewWSReceiver creates a new WebSocket receiver.
// baseURL is the signal-cli-rest-api base URL (http:// or https://).
func NewWSReceiver(baseURL, phoneNumber string, logger *logrus.Logger) *WSReceiver {
	return &WSReceiver{
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		phoneNumber: phoneNumber,
		logger:      logger,
	}
}

// Connect establishes a WebSocket connection to signal-cli-rest-api's receive endpoint.
// The caller owns the returned connection and must close it when done.
func (w *WSReceiver) Connect(ctx context.Context) (*websocket.Conn, error) {
	wsURL, err := httpToWS(w.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to convert base URL to WebSocket URL: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/receive/%s", wsURL, url.QueryEscape(w.phoneNumber))

	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	w.logger.WithField("endpoint", endpoint).Info("WebSocket connection established")
	return conn, nil
}

// ReadMessage reads and parses one message from the WebSocket connection.
// Returns the raw RestMessage for conversion by the caller.
// Returns nil RestMessage for non-actionable frames (pings, empty messages).
func ReadMessage(ctx context.Context, conn *websocket.Conn) (*types.RestMessage, error) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}

	var msg types.RestMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal WebSocket message: %w", err)
	}

	// Skip non-actionable messages (receipts, typing indicators)
	if msg.Envelope.DataMessage == nil && msg.Envelope.SyncMessage == nil {
		return nil, nil
	}

	return &msg, nil
}

// httpToWS converts an HTTP(S) URL to a WS(S) URL.
func httpToWS(httpURL string) (string, error) {
	u, err := url.Parse(httpURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// already a WebSocket URL
	default:
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	return u.String(), nil
}
