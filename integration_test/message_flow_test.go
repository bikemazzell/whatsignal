package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWhatsAppToSignalMessageFlow(t *testing.T) {
	env := NewTestEnvironment(t, "whatsapp_to_signal", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	tests := []struct {
		name          string
		scenario      string
		expectedAcks  int
		expectedSends int
	}{
		{
			name:          "basic_text_message",
			scenario:      "basic_text",
			expectedAcks:  1,
			expectedSends: 1,
		},
		{
			name:          "contact_sync_message",
			scenario:      "contact_sync",
			expectedAcks:  1,
			expectedSends: 1,
		},
		{
			name:          "group_message",
			scenario:      "group_message",
			expectedAcks:  1,
			expectedSends: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset counters for each sub-test
			env.ResetMockAPICounters()

			scenario := env.fixtures.Scenarios()[tt.scenario]

			webhook := scenario.WhatsAppWebhook
			webhook.Payload.ID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
			webhook.Payload.Timestamp = time.Now().Unix()

			webhookData, err := json.Marshal(webhook)
			if err != nil {
				t.Fatalf("Failed to marshal webhook: %v", err)
			}

			resp, err := http.Post(
				fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
				"application/json",
				strings.NewReader(string(webhookData)),
			)
			if err != nil {
				t.Fatalf("Failed to send webhook: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			time.Sleep(100 * time.Millisecond)

			acks := env.CountMockAPIRequests("ack")
			sends := env.CountMockAPIRequests("send")

			if acks != tt.expectedAcks {
				t.Errorf("Expected %d ACK requests, got %d", tt.expectedAcks, acks)
			}

			if sends != tt.expectedSends {
				t.Errorf("Expected %d send requests, got %d", tt.expectedSends, sends)
			}

			messageID := webhook.Payload.ID
			ctx := context.Background()
			mapping, err := env.db.GetMessageMapping(ctx, messageID)
			if err != nil {
				t.Errorf("Failed to get message mapping: %v", err)
			} else if mapping == nil {
				t.Error("Message mapping not found in database")
			}
		})
	}
}

func TestSignalToWhatsAppMessageFlow(t *testing.T) {
	env := NewTestEnvironment(t, "signal_to_whatsapp", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	tests := []struct {
		name          string
		scenario      string
		expectedSends int
	}{
		{
			name:          "basic_signal_text",
			scenario:      "signal_text",
			expectedSends: 1,
		},
		{
			name:          "signal_reply",
			scenario:      "signal_reply",
			expectedSends: 1,
		},
		{
			name:          "signal_group_reply",
			scenario:      "signal_group_reply",
			expectedSends: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset counters for each sub-test
			env.ResetMockAPICounters()

			scenario := env.fixtures.Scenarios()[tt.scenario]

			signalPayload := scenario.SignalWebhook
			signalPayload.Envelope.Timestamp = time.Now().UnixMilli()
			signalPayload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()

			webhookData, err := json.Marshal(signalPayload)
			if err != nil {
				t.Fatalf("Failed to marshal Signal webhook: %v", err)
			}

			resp, err := http.Post(
				fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
				"application/json",
				strings.NewReader(string(webhookData)),
			)
			if err != nil {
				t.Fatalf("Failed to send Signal webhook: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			time.Sleep(100 * time.Millisecond)

			sends := env.CountMockAPIRequests("whatsapp_send")

			if sends != tt.expectedSends {
				t.Errorf("Expected %d WhatsApp send requests, got %d", tt.expectedSends, sends)
			}
		})
	}
}

func TestBidirectionalMessageFlow(t *testing.T) {
	env := NewTestEnvironment(t, "bidirectional", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	whatsappWebhook := env.fixtures.Scenarios()["basic_text"].WhatsAppWebhook
	whatsappWebhook.Payload.ID = fmt.Sprintf("wa_msg_%d", time.Now().UnixNano())

	webhookData, err := json.Marshal(whatsappWebhook)
	if err != nil {
		t.Fatalf("Failed to marshal WhatsApp webhook: %v", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(webhookData)),
	)
	if err != nil {
		t.Fatalf("Failed to send WhatsApp webhook: %v", err)
	}
	resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	signalWebhook := env.fixtures.Scenarios()["signal_text"].SignalWebhook
	signalWebhook.Envelope.Timestamp = time.Now().UnixMilli()

	signalData, err := json.Marshal(signalWebhook)
	if err != nil {
		t.Fatalf("Failed to marshal Signal webhook: %v", err)
	}

	resp2, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(signalData)),
	)
	if err != nil {
		t.Fatalf("Failed to send Signal webhook: %v", err)
	}
	resp2.Body.Close()

	time.Sleep(100 * time.Millisecond)

	waAcks := env.CountMockAPIRequests("ack")
	signalSends := env.CountMockAPIRequests("send")
	whatsappSends := env.CountMockAPIRequests("whatsapp_send")

	if waAcks != 2 {
		t.Errorf("Expected 2 WhatsApp ACKs (1 from WA→Signal processing + 1 from Signal→WA sending), got %d", waAcks)
	}
	if signalSends != 1 {
		t.Errorf("Expected 1 Signal send, got %d", signalSends)
	}
	if whatsappSends != 1 {
		t.Errorf("Expected 1 WhatsApp send, got %d", whatsappSends)
	}

	// Verify individual message mappings were created
	ctx := context.Background()
	waMapping, err := env.db.GetMessageMapping(ctx, whatsappWebhook.Payload.ID)
	if err != nil || waMapping == nil {
		t.Errorf("WhatsApp message mapping not found: %v", err)
	}

	// Note: Signal webhook would need a different verification approach
	// since we don't track reverse mappings in the same way
}

func TestMessageFlowWithRetries(t *testing.T) {
	env := NewTestEnvironment(t, "retry_flow", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()
	env.SetMockAPIFailures("send", 2)

	webhook := env.fixtures.Scenarios()["basic_text"].WhatsAppWebhook
	webhook.Payload.ID = fmt.Sprintf("retry_msg_%d", time.Now().UnixNano())

	webhookData, err := json.Marshal(webhook)
	if err != nil {
		t.Fatalf("Failed to marshal webhook: %v", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(webhookData)),
	)
	if err != nil {
		t.Fatalf("Failed to send webhook: %v", err)
	}
	resp.Body.Close()

	time.Sleep(500 * time.Millisecond)

	sendAttempts := env.CountMockAPIRequests("send")
	if sendAttempts < 3 {
		t.Errorf("Expected at least 3 send attempts (original + retries), got %d", sendAttempts)
	}

	acks := env.CountMockAPIRequests("ack")
	if acks != 1 {
		t.Errorf("Expected 1 ACK despite retries, got %d", acks)
	}
}

func TestHighVolumeMessageFlow(t *testing.T) {
	env := NewTestEnvironment(t, "high_volume_flow", IsolationStrict)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	// Give the database and server time to fully initialize before high-volume load
	time.Sleep(500 * time.Millisecond)

	// Verify database is ready by checking for required tables
	if err := env.VerifyDatabaseConnection(); err != nil {
		t.Fatalf("Database not ready before high volume test: %v", err)
	}

	// Explicitly verify required tables exist
	ctx := context.Background()
	err := env.db.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("Database health check failed: %v", err)
	}

	const messageCount = 50
	const concurrentSenders = 5

	done := make(chan bool, concurrentSenders)
	startTime := time.Now()

	for i := 0; i < concurrentSenders; i++ {
		go func(senderID int) {
			defer func() { done <- true }()

			for j := 0; j < messageCount/concurrentSenders; j++ {
				webhook := env.fixtures.Scenarios()["basic_text"].WhatsAppWebhook
				webhook.Payload.ID = fmt.Sprintf("bulk_%d_%d_%d", senderID, j, time.Now().UnixNano())
				webhook.Payload.From = fmt.Sprintf("+155512345%02d", senderID*10+j)

				webhookData, err := json.Marshal(webhook)
				if err != nil {
					t.Errorf("Failed to marshal webhook: %v", err)
					return
				}

				resp, err := http.Post(
					fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
					"application/json",
					strings.NewReader(string(webhookData)),
				)
				if err != nil {
					t.Errorf("Failed to send webhook: %v", err)
					return
				}
				resp.Body.Close()

				if j%10 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	for i := 0; i < concurrentSenders; i++ {
		<-done
	}

	time.Sleep(1 * time.Second)
	totalTime := time.Since(startTime)

	acks := env.CountMockAPIRequests("ack")
	sends := env.CountMockAPIRequests("send")

	if acks != messageCount {
		t.Errorf("Expected %d ACKs, got %d", messageCount, acks)
	}
	t.Logf("SUCCESS: Got expected %d ACKs for %d messages", acks, messageCount)

	if sends != messageCount {
		t.Errorf("Expected %d sends, got %d", messageCount, sends)
	}

	avgTimePerMessage := totalTime / time.Duration(messageCount)
	if avgTimePerMessage > 100*time.Millisecond {
		t.Errorf("Average time per message too high: %v", avgTimePerMessage)
	}

	t.Logf("Processed %d messages in %v (avg: %v per message)", messageCount, totalTime, avgTimePerMessage)

	memory := env.GetMemoryUsage()
	if memory.HeapInuse > 50*1024*1024 {
		t.Errorf("Memory usage too high after bulk processing: %d bytes", memory.HeapInuse)
	}
}

func TestGroupMessageBidirectionalFlow(t *testing.T) {
	env := NewTestEnvironment(t, "group_bidirectional", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	t.Run("whatsapp_group_to_signal", func(t *testing.T) {
		env.ResetMockAPICounters()

		scenario := env.fixtures.Scenarios()["group_message"]
		webhook := scenario.WhatsAppWebhook
		webhook.Payload.ID = fmt.Sprintf("group_msg_%d", time.Now().UnixNano())
		webhook.Payload.Timestamp = time.Now().Unix()

		webhookData, err := json.Marshal(webhook)
		if err != nil {
			t.Fatalf("Failed to marshal group webhook: %v", err)
		}

		resp, err := http.Post(
			fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
			"application/json",
			strings.NewReader(string(webhookData)),
		)
		if err != nil {
			t.Fatalf("Failed to send webhook: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond)

		acks := env.CountMockAPIRequests("ack")
		sends := env.CountMockAPIRequests("send")

		if acks != 1 {
			t.Errorf("Expected 1 ACK for group message, got %d", acks)
		}
		if sends != 1 {
			t.Errorf("Expected 1 Signal send for group message, got %d", sends)
		}

		ctx := context.Background()
		mapping, err := env.db.GetMessageMapping(ctx, webhook.Payload.ID)
		if err != nil {
			t.Errorf("Failed to get message mapping: %v", err)
		}
		if mapping == nil {
			t.Error("Message mapping not found in database")
		} else {
			if mapping.WhatsAppChatID != "120363028123456789@g.us" {
				t.Errorf("Expected group chat ID 120363028123456789@g.us, got %s", mapping.WhatsAppChatID)
			}
		}
	})

	t.Run("signal_reply_to_group", func(t *testing.T) {
		env.ResetMockAPICounters()

		ctx := context.Background()
		groupMapping := env.fixtures.MessageMappings()["group_message"]
		if err := env.db.SaveMessageMapping(ctx, &groupMapping); err != nil {
			t.Fatalf("Failed to save group mapping: %v", err)
		}

		scenario := env.fixtures.Scenarios()["signal_group_reply"]
		signalPayload := scenario.SignalWebhook
		signalPayload.Envelope.Timestamp = time.Now().UnixMilli()
		signalPayload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()

		webhookData, err := json.Marshal(signalPayload)
		if err != nil {
			t.Fatalf("Failed to marshal Signal webhook: %v", err)
		}

		resp, err := http.Post(
			fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
			"application/json",
			strings.NewReader(string(webhookData)),
		)
		if err != nil {
			t.Fatalf("Failed to send Signal webhook: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for Signal reply to group, got %d", resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond)

		whatsappSends := env.CountMockAPIRequests("whatsapp_send")
		if whatsappSends != 1 {
			t.Errorf("Expected 1 WhatsApp send to group, got %d", whatsappSends)
		}

		t.Log("Successfully verified Signal reply is sent back to WhatsApp group")
	})
}
