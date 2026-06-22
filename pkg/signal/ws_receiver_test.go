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
	logrustest "github.com/sirupsen/logrus/hooks/test"
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

	result, err := ReadMessage(ctx, conn, testLogger())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "+15550000001", result.Envelope.Source)
	require.NotNil(t, result.Envelope.DataMessage)
	assert.Equal(t, "Reply to you", result.Envelope.DataMessage.Message)
	require.NotNil(t, result.Envelope.DataMessage.Quote, "Quote must be preserved in WebSocket message")
	assert.Equal(t, int64(1774347098265), result.Envelope.DataMessage.Quote.ID)
}

func TestReadMessage_ReceiptMessagePreserved(t *testing.T) {
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

	result, err := ReadMessage(ctx, conn, testLogger())
	require.NoError(t, err)
	require.NotNil(t, result, "Receipt messages should be returned for downstream processing")
	assert.NotNil(t, result.Envelope.ReceiptMessage)
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

	_, err = ReadMessage(ctx, conn, testLogger())
	assert.Error(t, err, "Should return error on closed connection")
}

func TestReadMessage_ExceptionEnvelopeLogged(t *testing.T) {
	// signal-cli's json-rpc daemon emits this frame when it fails to process an
	// inbound message (e.g. the getServerGuid NPE of AsamK/signal-cli#2059 after
	// the 2026-06-10 Signal server change). The envelope is empty, so the frame
	// is non-actionable — but before it was logged, every such frame was silently
	// dropped, making a total receive outage invisible in whatsignal's own logs.
	rawFrame := `{"account":"+15550000002","exception":{"message":"getServerGuid(...) must not be null","type":"java.lang.NullPointerException"}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow() //nolint:errcheck
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(rawFrame))
		<-r.Context().Done()
	}))
	defer server.Close()

	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.InfoLevel)

	receiver := NewWSReceiver(server.URL, "+15550000002", logger)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := receiver.Connect(ctx)
	require.NoError(t, err)
	defer conn.CloseNow() //nolint:errcheck

	result, err := ReadMessage(ctx, conn, logger)
	require.NoError(t, err, "an exception frame must not be treated as a read error (would cause pointless reconnect churn)")
	assert.Nil(t, result, "exception frame carries no actionable envelope")

	var found *logrus.Entry
	for _, e := range hook.AllEntries() {
		if e.Data["exception_type"] != nil {
			entry := e
			found = entry
			break
		}
	}
	require.NotNil(t, found, "signal-cli exception frame must be logged so receive outages are visible")
	assert.Equal(t, logrus.ErrorLevel, found.Level, "a daemon-side receive failure is an error")
	assert.Equal(t, "java.lang.NullPointerException", found.Data["exception_type"])
	assert.Contains(t, found.Data["exception_message"], "getServerGuid")
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
