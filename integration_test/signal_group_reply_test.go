package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"
)

// helper struct mirroring the webhook shape expected by handleSignalWebhook (with attachments and quote)
type signalWebhook struct {
	Account  string `json:"account"`
	Envelope struct {
		Source      string `json:"source"`
		SourceName  string `json:"sourceName"`
		Timestamp   int64  `json:"timestamp"`
		DataMessage struct {
			Timestamp   int64                               `json:"timestamp"`
			Message     string                              `json:"message"`
			Attachments []signaltypes.RestMessageAttachment `json:"attachments"`
			Quote       *signaltypes.RestMessageQuote       `json:"quote,omitempty"`
		} `json:"dataMessage"`
	} `json:"envelope"`
}

func TestSignalGroupTextReply_Quoted_UsesReplyTo(t *testing.T) {
	env := NewTestEnvironment(t, "sig_group_reply_text", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	// Arrange: mapping where Signal quoted ID maps to a WA group message ID
	mapping := models.MessageMapping{
		WhatsAppChatID:  "120363028123456789@g.us",
		WhatsAppMsgID:   "wamid.groupQuote1",
		SignalMsgID:     "1234567890000",
		SessionName:     "personal",
		DeliveryStatus:  models.DeliveryStatusDelivered,
		SignalTimestamp: time.Now().Add(-1 * time.Minute),
		ForwardedAt:     time.Now().Add(-1 * time.Minute),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := env.db.SaveMessageMapping(context.Background(), &mapping); err != nil {
		t.Fatalf("failed to save mapping: %v", err)
	}

	env.ResetMockAPICounters()

	// Build Signal webhook with group sender and quote referencing the mapping's SignalMsgID
	var payload signalWebhook
	payload.Account = "+1111111111"
	payload.Envelope.Source = "group.120363028123456789"
	payload.Envelope.SourceName = "Signal User"
	payload.Envelope.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Message = "Replying in thread"
	payload.Envelope.DataMessage.Quote = &signaltypes.RestMessageQuote{ID: 1234567890000, Author: "+1111111111", Text: "prev"}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		t.Fatalf("failed to POST webhook: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	time.Sleep(150 * time.Millisecond)

	if env.CountMockAPIRequests("whatsapp_send") != 1 {
		t.Errorf("expected 1 whatsapp send, got %d", env.CountMockAPIRequests("whatsapp_send"))
	}
	if env.CountMockAPIRequests("whatsapp_reply_to") != 1 {
		t.Errorf("expected reply_to to be set once, got %d", env.CountMockAPIRequests("whatsapp_reply_to"))
	}
}

func TestSignalGroupImageReply_Quoted_UsesReplyTo(t *testing.T) {
	env := NewTestEnvironment(t, "sig_group_reply_image", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	// Arrange mapping for quoted reply
	mapping := models.MessageMapping{
		WhatsAppChatID:  "120363028123456789@g.us",
		WhatsAppMsgID:   "wamid.groupImg1",
		SignalMsgID:     "1234567891111",
		SessionName:     "personal",
		DeliveryStatus:  models.DeliveryStatusDelivered,
		SignalTimestamp: time.Now().Add(-1 * time.Minute),
		ForwardedAt:     time.Now().Add(-1 * time.Minute),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := env.db.SaveMessageMapping(context.Background(), &mapping); err != nil {
		t.Fatalf("failed to save mapping: %v", err)
	}

	env.ResetMockAPICounters()

	// Create a sample image file in the media dir and reference it in attachments
	imgPath := filepath.Join(env.GetMediaDirectory(), "sample.png")
	if err := os.WriteFile(imgPath, env.GetMediaSamples().SmallImage(), 0644); err != nil {
		t.Fatalf("failed to write sample image: %v", err)
	}

	var payload signalWebhook
	payload.Account = "+1111111111"
	payload.Envelope.Source = "group.120363028123456789"
	payload.Envelope.SourceName = "Signal User"
	payload.Envelope.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Message = "image with reply"
	payload.Envelope.DataMessage.Attachments = []signaltypes.RestMessageAttachment{{
		ContentType: "image/png",
		Filename:    imgPath,
		ID:          "att1",
		Size:        1024,
	}}
	payload.Envelope.DataMessage.Quote = &signaltypes.RestMessageQuote{ID: 1234567891111, Author: "+1111111111", Text: "prev img"}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		t.Fatalf("failed to POST webhook: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	time.Sleep(200 * time.Millisecond)

	if env.CountMockAPIRequests("whatsapp_send_image") != 1 {
		t.Errorf("expected 1 whatsapp image send, got %d", env.CountMockAPIRequests("whatsapp_send_image"))
	}
	if env.CountMockAPIRequests("whatsapp_reply_to") != 1 {
		t.Errorf("expected reply_to once for image, got %d", env.CountMockAPIRequests("whatsapp_reply_to"))
	}
}

func TestSignalGroupNoQuote_FallbackNoReplyTo(t *testing.T) {
	env := NewTestEnvironment(t, "sig_group_no_quote", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	env.ResetMockAPICounters()

	var payload signalWebhook
	payload.Account = "+1111111111"
	payload.Envelope.Source = "group.120363028123456789"
	payload.Envelope.SourceName = "Signal User"
	payload.Envelope.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Message = "no quote fallback"

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		t.Fatalf("failed to POST webhook: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	time.Sleep(150 * time.Millisecond)

	if env.CountMockAPIRequests("whatsapp_send") != 1 {
		t.Errorf("expected 1 whatsapp send, got %d", env.CountMockAPIRequests("whatsapp_send"))
	}
	if env.CountMockAPIRequests("whatsapp_reply_to") != 0 {
		t.Errorf("expected reply_to to be absent, got %d", env.CountMockAPIRequests("whatsapp_reply_to"))
	}
}

func TestSignalGroupQuoted_MissingMapping_Returns500(t *testing.T) {
	env := NewTestEnvironment(t, "sig_group_missing_mapping", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	var payload signalWebhook
	payload.Account = "+1111111111"
	payload.Envelope.Source = "group.120363028123456789"
	payload.Envelope.SourceName = "Signal User"
	payload.Envelope.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Message = "quoted but no mapping"
	payload.Envelope.DataMessage.Quote = &signaltypes.RestMessageQuote{ID: 9876543210000, Author: "+1111111111", Text: "prev"}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		t.Fatalf("failed to POST webhook: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 for missing mapping, got %d", resp.StatusCode)
	}
}

func TestSignalGroupQuoted_NonGroupMapping_Returns500(t *testing.T) {
	env := NewTestEnvironment(t, "sig_group_wrong_mapping", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	// Create a mapping that points to a direct chat instead of a group
	mapping := models.MessageMapping{
		WhatsAppChatID:  "+1111111111@c.us",
		WhatsAppMsgID:   "wamid.direct1",
		SignalMsgID:     "7777777777",
		SessionName:     "personal",
		DeliveryStatus:  models.DeliveryStatusDelivered,
		SignalTimestamp: time.Now().Add(-1 * time.Minute),
		ForwardedAt:     time.Now().Add(-1 * time.Minute),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := env.db.SaveMessageMapping(context.Background(), &mapping); err != nil {
		t.Fatalf("failed to save mapping: %v", err)
	}

	var payload signalWebhook
	payload.Account = "+1111111111"
	payload.Envelope.Source = "group.120363028123456789"
	payload.Envelope.SourceName = "Signal User"
	payload.Envelope.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Message = "quoted to non-group"
	payload.Envelope.DataMessage.Quote = &signaltypes.RestMessageQuote{ID: 7777777777, Author: "+1111111111", Text: "prev"}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		t.Fatalf("failed to POST webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 for non-group mapping, got %d", resp.StatusCode)
	}
}
