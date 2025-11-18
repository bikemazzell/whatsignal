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

// TestMultipleGroupsScenarios tests scenarios with multiple WhatsApp groups
func TestMultipleGroupsScenarios(t *testing.T) {
	env := NewTestEnvironment(t, "multiple_groups", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	t.Run("same_user_in_multiple_groups", func(t *testing.T) {
		env.ResetMockAPICounters()

		// Send message from Test Group
		groupMsg1 := env.fixtures.WhatsAppWebhooks()["group_message"]
		groupMsg1.Payload.ID = fmt.Sprintf("multi_group1_%d", time.Now().UnixNano())
		groupMsg1.Payload.Timestamp = time.Now().Unix()

		webhookData1, err := json.Marshal(groupMsg1)
		if err != nil {
			t.Fatalf("Failed to marshal webhook 1: %v", err)
		}

		resp1, err := http.Post(
			fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
			"application/json",
			strings.NewReader(string(webhookData1)),
		)
		if err != nil {
			t.Fatalf("Failed to send webhook 1: %v", err)
		}
		_ = resp1.Body.Close()

		time.Sleep(100 * time.Millisecond)

		// Send message from Family Group
		familyMsg := env.fixtures.WhatsAppWebhooks()["group_family_message"]
		familyMsg.Payload.ID = fmt.Sprintf("multi_family_%d", time.Now().UnixNano())
		familyMsg.Payload.Timestamp = time.Now().Unix()

		webhookData2, err := json.Marshal(familyMsg)
		if err != nil {
			t.Fatalf("Failed to marshal webhook 2: %v", err)
		}

		resp2, err := http.Post(
			fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
			"application/json",
			strings.NewReader(string(webhookData2)),
		)
		if err != nil {
			t.Fatalf("Failed to send webhook 2: %v", err)
		}
		_ = resp2.Body.Close()

		time.Sleep(100 * time.Millisecond)

		// Send message from Work Group
		workMsg := env.fixtures.WhatsAppWebhooks()["group_work_message"]
		workMsg.Payload.ID = fmt.Sprintf("multi_work_%d", time.Now().UnixNano())
		workMsg.Payload.Timestamp = time.Now().Unix()

		webhookData3, err := json.Marshal(workMsg)
		if err != nil {
			t.Fatalf("Failed to marshal webhook 3: %v", err)
		}

		resp3, err := http.Post(
			fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
			"application/json",
			strings.NewReader(string(webhookData3)),
		)
		if err != nil {
			t.Fatalf("Failed to send webhook 3: %v", err)
		}
		_ = resp3.Body.Close()

		time.Sleep(100 * time.Millisecond)

		// Verify all messages were processed
		acks := env.CountMockAPIRequests("ack")
		sends := env.CountMockAPIRequests("send")

		if acks != 3 {
			t.Errorf("Expected 3 ACKs (one per group message), got %d", acks)
		}
		if sends != 3 {
			t.Errorf("Expected 3 Signal sends (one per group message), got %d", sends)
		}

		// Verify message mappings for each group
		ctx := context.Background()

		mapping1, err := env.db.GetMessageMapping(ctx, groupMsg1.Payload.ID)
		if err != nil || mapping1 == nil {
			t.Errorf("Failed to get mapping for Test Group message: %v", err)
		} else if mapping1.WhatsAppChatID != "120363028123456789@g.us" {
			t.Errorf("Test Group: expected chat ID 120363028123456789@g.us, got %s", mapping1.WhatsAppChatID)
		}

		mapping2, err := env.db.GetMessageMapping(ctx, familyMsg.Payload.ID)
		if err != nil || mapping2 == nil {
			t.Errorf("Failed to get mapping for Family Group message: %v", err)
		} else if mapping2.WhatsAppChatID != "120363028987654321@g.us" {
			t.Errorf("Family Group: expected chat ID 120363028987654321@g.us, got %s", mapping2.WhatsAppChatID)
		}

		mapping3, err := env.db.GetMessageMapping(ctx, workMsg.Payload.ID)
		if err != nil || mapping3 == nil {
			t.Errorf("Failed to get mapping for Work Group message: %v", err)
		} else if mapping3.WhatsAppChatID != "120363029999888777@g.us" {
			t.Errorf("Work Group: expected chat ID 120363029999888777@g.us, got %s", mapping3.WhatsAppChatID)
		}

		t.Log("Successfully verified messages from 3 different groups with correct chat IDs")
	})

	t.Run("reply_to_correct_group", func(t *testing.T) {
		env.ResetMockAPICounters()
		ctx := context.Background()

		// Save mappings for all three groups
		testGroupMapping := env.fixtures.MessageMappings()["group_message"]
		familyGroupMapping := env.fixtures.MessageMappings()["group_family_message"]
		workGroupMapping := env.fixtures.MessageMappings()["group_work_message"]

		if err := env.db.SaveMessageMapping(ctx, &testGroupMapping); err != nil {
			t.Fatalf("Failed to save test group mapping: %v", err)
		}
		if err := env.db.SaveMessageMapping(ctx, &familyGroupMapping); err != nil {
			t.Fatalf("Failed to save family group mapping: %v", err)
		}
		if err := env.db.SaveMessageMapping(ctx, &workGroupMapping); err != nil {
			t.Fatalf("Failed to save work group mapping: %v", err)
		}

		// Reply from Signal (should go to most recent group - Work Group)
		signalPayload := SignalWebhookPayload{
			Envelope: struct {
				Source      string `json:"source"`
				SourceName  string `json:"sourceName"`
				SourceUuid  string `json:"sourceUuid"`
				Timestamp   int64  `json:"timestamp"`
				DataMessage struct {
					Timestamp int64  `json:"timestamp"`
					Message   string `json:"message"`
					ExpiresIn int    `json:"expiresIn"`
					ViewOnce  bool   `json:"viewOnce"`
				} `json:"dataMessage"`
			}{
				Source:     "+1111111111",
				SourceName: "Signal User",
				SourceUuid: "test-uuid-multi",
				Timestamp:  time.Now().UnixMilli(),
				DataMessage: struct {
					Timestamp int64  `json:"timestamp"`
					Message   string `json:"message"`
					ExpiresIn int    `json:"expiresIn"`
					ViewOnce  bool   `json:"viewOnce"`
				}{
					Timestamp: time.Now().UnixMilli(),
					Message:   "Reply to last group",
					ExpiresIn: 0,
					ViewOnce:  false,
				},
			},
			Account: "+1111111111",
		}

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
		_ = resp.Body.Close()

		time.Sleep(2 * time.Second)

		whatsappSends := env.CountMockAPIRequests("whatsapp_send")
		if whatsappSends != 1 {
			t.Errorf("Expected 1 WhatsApp send, got %d", whatsappSends)
		}

		t.Log("Successfully verified Signal reply routes to the most recent group")
	})
}

// TestGroupSpecificFeatures tests group-specific features like mentions and quoted messages
func TestGroupSpecificFeatures(t *testing.T) {
	env := NewTestEnvironment(t, "group_features", IsolationProcess)
	defer env.Cleanup()

	env.StartMessageFlowServer()

	t.Run("group_message_with_mention", func(t *testing.T) {
		env.ResetMockAPICounters()

		// Send a group message with @mention
		workMsg := env.fixtures.WhatsAppWebhooks()["group_work_message"]
		workMsg.Payload.ID = fmt.Sprintf("mention_msg_%d", time.Now().UnixNano())
		workMsg.Payload.Timestamp = time.Now().Unix()
		workMsg.Payload.Body = "@Alice can you review the PR?"

		webhookData, err := json.Marshal(workMsg)
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
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond)

		acks := env.CountMockAPIRequests("ack")
		sends := env.CountMockAPIRequests("send")

		if acks != 1 {
			t.Errorf("Expected 1 ACK for mention message, got %d", acks)
		}
		if sends != 1 {
			t.Errorf("Expected 1 Signal send for mention message, got %d", sends)
		}

		// Verify the message was stored with correct group ID
		ctx := context.Background()
		mapping, err := env.db.GetMessageMapping(ctx, workMsg.Payload.ID)
		if err != nil {
			t.Errorf("Failed to get message mapping: %v", err)
		}
		if mapping == nil {
			t.Error("Message mapping not found")
		} else {
			if mapping.WhatsAppChatID != "120363029999888777@g.us" {
				t.Errorf("Expected Work Group chat ID, got %s", mapping.WhatsAppChatID)
			}
		}

		t.Log("Successfully processed group message with @mention")
	})

	t.Run("group_message_quoted_reply", func(t *testing.T) {
		env.ResetMockAPICounters()

		// Send a quoted message in a group
		quotedMsg := env.fixtures.WhatsAppWebhooks()["group_message_quoted"]
		quotedMsg.Payload.ID = fmt.Sprintf("quoted_msg_%d", time.Now().UnixNano())
		quotedMsg.Payload.Timestamp = time.Now().Unix()

		webhookData, err := json.Marshal(quotedMsg)
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
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond)

		acks := env.CountMockAPIRequests("ack")
		sends := env.CountMockAPIRequests("send")

		if acks != 1 {
			t.Errorf("Expected 1 ACK for quoted message, got %d", acks)
		}
		if sends != 1 {
			t.Errorf("Expected 1 Signal send for quoted message, got %d", sends)
		}

		// Verify the quoted message was stored with correct group ID
		ctx := context.Background()
		mapping, err := env.db.GetMessageMapping(ctx, quotedMsg.Payload.ID)
		if err != nil {
			t.Errorf("Failed to get message mapping: %v", err)
		}
		if mapping == nil {
			t.Error("Message mapping not found")
		} else {
			if mapping.WhatsAppChatID != "120363028123456789@g.us" {
				t.Errorf("Expected Test Group chat ID, got %s", mapping.WhatsAppChatID)
			}
		}

		t.Log("Successfully processed quoted message in group")
	})

	t.Run("multiple_messages_same_group", func(t *testing.T) {
		env.ResetMockAPICounters()

		messageCount := 5

		// Send multiple messages to the same group
		for i := 0; i < messageCount; i++ {
			webhook := env.fixtures.WhatsAppWebhooks()["group_message"]
			webhook.Payload.ID = fmt.Sprintf("multi_msg_%d_%d", i, time.Now().UnixNano())
			webhook.Payload.Timestamp = time.Now().Unix()
			webhook.Payload.Body = fmt.Sprintf("Message %d in the group", i+1)

			webhookData, err := json.Marshal(webhook)
			if err != nil {
				t.Fatalf("Failed to marshal webhook %d: %v", i, err)
			}

			resp, err := http.Post(
				fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
				"application/json",
				strings.NewReader(string(webhookData)),
			)
			if err != nil {
				t.Fatalf("Failed to send webhook %d: %v", i, err)
			}
			_ = resp.Body.Close()

			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(200 * time.Millisecond)

		acks := env.CountMockAPIRequests("ack")
		sends := env.CountMockAPIRequests("send")

		if acks != messageCount {
			t.Errorf("Expected %d ACKs for multiple messages, got %d", messageCount, acks)
		}
		if sends != messageCount {
			t.Errorf("Expected %d Signal sends for multiple messages, got %d", messageCount, sends)
		}

		t.Logf("Successfully processed %d messages from the same group", messageCount)
	})
}
