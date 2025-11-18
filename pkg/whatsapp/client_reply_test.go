package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_SendTextWithSessionReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sendText" {
			var payload map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "chat123@c.us", payload["chatId"])
			assert.Equal(t, "test-session", payload["session"])
			assert.Equal(t, "hello", payload["text"])
			assert.Equal(t, "true_chat123@c.us_ABCD", payload["reply_to"]) // WAHA reply threading

			_ = json.NewEncoder(w).Encode(types.WAHAMessageResponse{ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{FromMe: true, Remote: "chat123@c.us", ID: "m1", Serialized: "m1"}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, SessionName: "test-session", Timeout: 5 * time.Second})
	resp, err := client.SendTextWithSessionReply(context.Background(), "chat123@c.us", "hello", "true_chat123@c.us_ABCD", "test-session")
	require.NoError(t, err)
	assert.Equal(t, "m1", resp.MessageID)
}

func TestClient_SendImageWithSessionReply(t *testing.T) {
	// Create a temp image file
	f, err := os.CreateTemp("", "img-*.jpg")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()
	_, _ = f.Write([]byte("fake-jpg"))
	_ = f.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sendImage":
			var payload map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "g1@g.us", payload["chatId"])
			assert.Equal(t, "test-session", payload["session"])
			assert.Equal(t, "caption", payload["caption"])
			assert.Equal(t, "wa_msg_42", payload["reply_to"]) // reply threading
			file := payload["file"].(map[string]interface{})
			assert.Equal(t, "image/jpeg", file["mimetype"])
			assert.NotEmpty(t, file["data"]) // base64

			_ = json.NewEncoder(w).Encode(types.WAHAMessageResponse{ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{FromMe: true, Remote: "g1@g.us", ID: "m2", Serialized: "m2"}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, SessionName: "test-session", Timeout: 5 * time.Second})
	resp, err := client.SendImageWithSessionReply(context.Background(), "g1@g.us", f.Name(), "caption", "wa_msg_42", "test-session")
	require.NoError(t, err)
	assert.Equal(t, "m2", resp.MessageID)
}

func TestClient_SendVoiceAndDocumentWithSessionReply(t *testing.T) {
	// voice
	vf, err := os.CreateTemp("", "voice-*.ogg")
	require.NoError(t, err)
	defer func() { _ = os.Remove(vf.Name()) }()
	_, _ = vf.Write([]byte("OggS..."))
	_ = vf.Close()

	// document
	df, err := os.CreateTemp("", "doc-*.pdf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(df.Name()) }()
	_, _ = df.Write([]byte("%PDF-1.4"))
	_ = df.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/sendVoice", "/api/sendFile":
			var payload map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "test-session", payload["session"])
			assert.Equal(t, "reply123", payload["reply_to"]) // present for both
			_ = json.NewEncoder(w).Encode(types.WAHAMessageResponse{ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{FromMe: true, Remote: "chat@c.us", ID: "ok", Serialized: "ok"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, SessionName: "test-session", Timeout: 5 * time.Second})
	ctx := context.Background()

	vr, err := client.SendVoiceWithSessionReply(ctx, "chat@c.us", vf.Name(), "reply123", "test-session")
	require.NoError(t, err)
	assert.Equal(t, "ok", vr.MessageID)

	dr, err := client.SendDocumentWithSessionReply(ctx, "chat@c.us", df.Name(), "read", "reply123", "test-session")
	require.NoError(t, err)
	assert.Equal(t, "ok", dr.MessageID)
}

func TestClient_SendVideoWithSessionReply_VideoSupported(t *testing.T) {
	// create tmp video
	vf, err := os.CreateTemp("", "v-*.mp4")
	require.NoError(t, err)
	defer func() { _ = os.Remove(vf.Name()) }()
	_, _ = vf.Write([]byte("mp4"))
	_ = vf.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/server/version":
			_ = json.NewEncoder(w).Encode(types.ServerVersion{Version: "2024.2.3", Engine: "NOWEB", Tier: "PLUS", Browser: "/usr/bin/google-chrome-stable"})
		case "/api/sendVideo":
			var payload map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, "reply777", payload["reply_to"]) // reply threading present
			_ = json.NewEncoder(w).Encode(types.WAHAMessageResponse{ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{FromMe: true, Remote: "g@g.us", ID: "vid1", Serialized: "vid1"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{BaseURL: server.URL, SessionName: "test-session", Timeout: 5 * time.Second})
	resp, err := client.SendVideoWithSessionReply(context.Background(), "g@g.us", vf.Name(), "v", "reply777", "test-session")
	require.NoError(t, err)
	assert.Equal(t, "vid1", resp.MessageID)
}
