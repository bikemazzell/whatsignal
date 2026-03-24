package signal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"whatsignal/pkg/signal/types"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.PanicLevel)
	return l
}

func TestWSReceiver_Connect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("Accept error: %v", err)
			return
		}
		defer conn.CloseNow() //nolint:errcheck //nolint:errcheck
		<-r.Context().Done()
	}))
	defer server.Close()

	receiver := NewWSReceiver(server.URL, "+1234567890", testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := receiver.Connect(ctx)
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.CloseNow()
}

func TestWSReceiver_Connect_InvalidURL(t *testing.T) {
	receiver := NewWSReceiver("http://127.0.0.1:1", "+1234567890", testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := receiver.Connect(ctx)
	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestReadMessage_DataMessageWithQuote(t *testing.T) {
	msg := types.RestMessage{
		Envelope: struct {
			Source         string                 `json:"source"`
			SourceNumber   string                 `json:"sourceNumber"`
			SourceUUID     string                 `json:"sourceUuid"`
			SourceName     string                 `json:"sourceName"`
			Timestamp      int64                  `json:"timestamp"`
			DataMessage    *types.RestDataMessage `json:"dataMessage,omitempty"`
			SyncMessage    *types.RestSyncMessage `json:"syncMessage,omitempty"`
			ReceiptMessage interface{}            `json:"receiptMessage,omitempty"`
			TypingMessage  interface{}            `json:"typingMessage,omitempty"`
		}{
			Source:    "+15550000001",
			Timestamp: 1774363197904,
			DataMessage: &types.RestDataMessage{
				Timestamp: 1774363197904,
				Message:   "Reply to you",
				Quote: &types.RestMessageQuote{
					ID:     1774347098265,
					Author: "+15550000002",
					Text:   "Original message",
				},
			},
		},
		Account: "+15550000002",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow() //nolint:errcheck

		data, _ := json.Marshal(msg)
		_ = conn.Write(r.Context(), websocket.MessageText, data)
		<-r.Context().Done()
	}))
	defer server.Close()

	receiver := NewWSReceiver(server.URL, "+15550000002", testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := receiver.Connect(ctx)
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck

	result, err := ReadMessage(ctx, conn)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "+15550000001", result.Envelope.Source)
	require.NotNil(t, result.Envelope.DataMessage)
	assert.Equal(t, "Reply to you", result.Envelope.DataMessage.Message)
	require.NotNil(t, result.Envelope.DataMessage.Quote, "Quote must be preserved in WebSocket message")
	assert.Equal(t, int64(1774347098265), result.Envelope.DataMessage.Quote.ID)
}

func TestReadMessage_ReceiptMessageIgnored(t *testing.T) {
	msg := types.RestMessage{
		Envelope: struct {
			Source         string                 `json:"source"`
			SourceNumber   string                 `json:"sourceNumber"`
			SourceUUID     string                 `json:"sourceUuid"`
			SourceName     string                 `json:"sourceName"`
			Timestamp      int64                  `json:"timestamp"`
			DataMessage    *types.RestDataMessage `json:"dataMessage,omitempty"`
			SyncMessage    *types.RestSyncMessage `json:"syncMessage,omitempty"`
			ReceiptMessage interface{}            `json:"receiptMessage,omitempty"`
			TypingMessage  interface{}            `json:"typingMessage,omitempty"`
		}{
			Source:         "+15550000001",
			Timestamp:      1774363197904,
			ReceiptMessage: map[string]interface{}{"type": "DELIVERY"},
		},
		Account: "+15550000002",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow() //nolint:errcheck

		data, _ := json.Marshal(msg)
		_ = conn.Write(r.Context(), websocket.MessageText, data)
		<-r.Context().Done()
	}))
	defer server.Close()

	receiver := NewWSReceiver(server.URL, "+15550000002", testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := receiver.Connect(ctx)
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck

	result, err := ReadMessage(ctx, conn)
	require.NoError(t, err)
	assert.Nil(t, result, "Receipt messages should be skipped")
}

func TestReadMessage_ConnectionClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	receiver := NewWSReceiver(server.URL, "+15550000002", testLogger())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := receiver.Connect(ctx)
	require.NoError(t, err)

	_, err = ReadMessage(ctx, conn)
	assert.Error(t, err, "Should return error on closed connection")
}

func TestHttpToWS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"http://localhost:8080", "ws://localhost:8080", false},
		{"https://signal.example.com", "wss://signal.example.com", false},
		{"ws://already-ws:8080", "ws://already-ws:8080", false},
		{"ftp://bad-scheme", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := httpToWS(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(result, tt.expected))
			}
		})
	}
}
