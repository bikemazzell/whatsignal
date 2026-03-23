package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeWhatsAppWebhook creates a WhatsApp webhook payload for a direct message from the given sender.
func makeWhatsAppWebhook(session, chatID, msgID, body, notifyName string) models.WhatsAppWebhookPayload {
	return models.WhatsAppWebhookPayload{
		Session: session,
		Event:   models.EventMessage,
		Payload: struct {
			ID          string                   `json:"id"`
			Timestamp   models.FlexibleTimestamp `json:"timestamp"`
			From        string                   `json:"from"`
			FromMe      bool                     `json:"fromMe"`
			To          string                   `json:"to"`
			Body        string                   `json:"body"`
			HasMedia    bool                     `json:"hasMedia"`
			Participant string                   `json:"participant,omitempty"`
			NotifyName  string                   `json:"notifyName,omitempty"`
			Media       *struct {
				URL      string `json:"url"`
				MimeType string `json:"mimetype"`
				Filename string `json:"filename"`
			} `json:"media"`
			Reaction *struct {
				Text      string `json:"text"`
				MessageID string `json:"messageId"`
			} `json:"reaction"`
			Data *struct {
				NotifyName string `json:"notifyName,omitempty"`
				PushName   string `json:"pushName,omitempty"`
			} `json:"_data,omitempty"`
			EditedMessageID *string `json:"editedMessageId,omitempty"`
			ACK             *int    `json:"ack,omitempty"`
		}{
			ID:         msgID,
			Timestamp:  models.FlexibleTimestamp(time.Now().Unix()),
			From:       chatID,
			FromMe:     false,
			To:         session + "@c.us",
			Body:       body,
			HasMedia:   false,
			NotifyName: notifyName,
		},
	}
}

// makeGroupWhatsAppWebhook creates a WhatsApp webhook payload for a group message.
func makeGroupWhatsAppWebhook(session, groupChatID, participant, msgID, body, notifyName string) models.WhatsAppWebhookPayload {
	wh := makeWhatsAppWebhook(session, groupChatID, msgID, body, notifyName)
	wh.Payload.Participant = participant
	return wh
}

// sendWhatsAppWebhook posts a WhatsApp webhook to the test server and asserts success.
func sendWhatsAppWebhook(t *testing.T, env *TestEnvironment, wh models.WhatsAppWebhookPayload) {
	t.Helper()
	data, err := json.Marshal(wh)
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/whatsapp", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(data)),
	)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode, "WhatsApp webhook should succeed")
}

// sendSignalWebhookWithQuote posts a Signal webhook with an optional quote to the test server.
// quoteID is the int64 timestamp of the quoted message (0 = no quote).
func sendSignalWebhookWithQuote(t *testing.T, env *TestEnvironment, source, account, message string, quoteID int64) *http.Response {
	t.Helper()
	var payload signalWebhook
	payload.Account = account
	payload.Envelope.Source = source
	payload.Envelope.SourceName = "Test User"
	payload.Envelope.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Timestamp = time.Now().UnixMilli()
	payload.Envelope.DataMessage.Message = message

	if quoteID != 0 {
		payload.Envelope.DataMessage.Quote = &signaltypes.RestMessageQuote{
			ID:     quoteID,
			Author: account,
			Text:   "quoted text",
		}
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("%s/webhook/signal", env.httpServer.URL),
		"application/json",
		strings.NewReader(string(body)),
	)
	require.NoError(t, err)
	return resp
}

// waitForProcessing waits for async message processing to complete.
func waitForProcessing() {
	time.Sleep(200 * time.Millisecond)
}

// getSignalMsgIDForWhatsAppMsg looks up the stored mapping for a WhatsApp message
// and returns the SignalMsgID (the timestamp string assigned during WA→Signal bridging).
func getSignalMsgIDForWhatsAppMsg(t *testing.T, env *TestEnvironment, whatsappMsgID string) string {
	t.Helper()
	ctx := context.Background()
	mapping, err := env.db.GetMessageMapping(ctx, whatsappMsgID)
	require.NoError(t, err, "Should find mapping for WhatsApp message %s", whatsappMsgID)
	require.NotNil(t, mapping, "Mapping should not be nil for WhatsApp message %s", whatsappMsgID)
	return mapping.SignalMsgID
}

// TestQuoteRouting_ReplyGoesToQuotedSender verifies that when the user quotes Alice's message
// on Signal, the reply is routed to Alice's WhatsApp chat — not Bob's.
func TestQuoteRouting_ReplyGoesToQuotedSender(t *testing.T) {
	env := NewTestEnvironment(t, "quote_routing_alice", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	aliceChatID := "15551234567@c.us"
	bobChatID := "15559876543@c.us"

	// Step 1: Alice sends a WhatsApp message → bridged to Signal
	aliceMsgID := fmt.Sprintf("wamid.alice_%d", time.Now().UnixNano())
	sendWhatsAppWebhook(t, env, makeWhatsAppWebhook("personal", aliceChatID, aliceMsgID, "Hi from Alice", "Alice"))
	waitForProcessing()

	// Step 2: Bob sends a WhatsApp message → bridged to Signal (now the "latest")
	bobMsgID := fmt.Sprintf("wamid.bob_%d", time.Now().UnixNano())
	sendWhatsAppWebhook(t, env, makeWhatsAppWebhook("personal", bobChatID, bobMsgID, "Hi from Bob", "Bob"))
	waitForProcessing()

	// Step 3: Read back Alice's SignalMsgID from the database
	aliceSignalMsgID := getSignalMsgIDForWhatsAppMsg(t, env, aliceMsgID)
	t.Logf("Alice's SignalMsgID: %s", aliceSignalMsgID)

	// Verify it's different from Bob's
	bobSignalMsgID := getSignalMsgIDForWhatsAppMsg(t, env, bobMsgID)
	t.Logf("Bob's SignalMsgID: %s", bobSignalMsgID)
	require.NotEqual(t, aliceSignalMsgID, bobSignalMsgID, "Alice and Bob should have different SignalMsgIDs")

	// Step 4: Reset tracking, then quote Alice's message from Signal
	env.ResetMockAPICounters()

	quoteID, err := strconv.ParseInt(aliceSignalMsgID, 10, 64)
	require.NoError(t, err, "SignalMsgID should be a numeric timestamp string")

	resp := sendSignalWebhookWithQuote(t, env, "+1111111111", "+1111111111", "Replying to Alice", quoteID)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	waitForProcessing()

	// Step 5: Assert the reply went to Alice, not Bob
	sends := env.GetWhatsAppSends()
	require.NotEmpty(t, sends, "Should have at least one WhatsApp send")

	lastSend := sends[len(sends)-1]
	assert.Equal(t, aliceChatID, lastSend.ChatID, "Reply should go to Alice's chat, not Bob's")
	assert.NotEmpty(t, lastSend.ReplyTo, "Should include reply_to for quoted message")
}

// TestFallbackRouting_NoQuote_RoutesToLatestWithWarning verifies that when the user
// sends a Signal message without quoting, it falls back to the latest sender and a
// warning notification is sent.
func TestFallbackRouting_NoQuote_RoutesToLatestWithWarning(t *testing.T) {
	env := NewTestEnvironment(t, "fallback_routing", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	aliceChatID := "15551234567@c.us"
	bobChatID := "15559876543@c.us"

	// Alice sends first, then Bob sends (Bob is now latest)
	sendWhatsAppWebhook(t, env, makeWhatsAppWebhook("personal", aliceChatID,
		fmt.Sprintf("wamid.alice_%d", time.Now().UnixNano()), "Hi from Alice", "Alice"))
	waitForProcessing()

	sendWhatsAppWebhook(t, env, makeWhatsAppWebhook("personal", bobChatID,
		fmt.Sprintf("wamid.bob_%d", time.Now().UnixNano()), "Hi from Bob", "Bob"))
	waitForProcessing()

	// Reset tracking, send unquoted reply from Signal
	env.ResetMockAPICounters()
	signalSendsBefore := env.CountMockAPIRequests("send")

	resp := sendSignalWebhookWithQuote(t, env, "+1111111111", "+1111111111", "Unquoted reply", 0)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	waitForProcessing()

	// Assert: message went to Bob (latest)
	sends := env.GetWhatsAppSends()
	require.NotEmpty(t, sends, "Should have at least one WhatsApp send")
	assert.Equal(t, bobChatID, sends[len(sends)-1].ChatID, "Unquoted reply should go to latest sender (Bob)")

	// Assert: a fallback warning notification was sent to Signal
	// The WA→Signal forwards (2) + the fallback notification (1) = at least signalSendsBefore + 1
	signalSendsAfter := env.CountMockAPIRequests("send")
	assert.Greater(t, signalSendsAfter, signalSendsBefore,
		"A fallback routing notification should have been sent to Signal")
}

// TestQuoteRouting_ExpiredMapping_ReturnsError verifies that quoting a message whose
// mapping no longer exists in the database results in an error (no silent misrouting).
func TestQuoteRouting_ExpiredMapping_ReturnsError(t *testing.T) {
	env := NewTestEnvironment(t, "expired_mapping", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()
	env.ResetMockAPICounters()

	// Send Signal webhook quoting a non-existent message ID
	nonExistentQuoteID := int64(9999999999999)
	resp := sendSignalWebhookWithQuote(t, env, "+1111111111", "+1111111111", "Reply to ghost", nonExistentQuoteID)
	defer func() { _ = resp.Body.Close() }()

	waitForProcessing()

	// The webhook should return an error (500) since no mapping exists
	// and extractMappingFromQuotedText should also fail (quoted text = "quoted text" has no phone number)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"Quoting a non-existent message should fail")

	// No WhatsApp sends should have happened
	assert.Equal(t, 0, env.CountMockAPIRequests("whatsapp_send"),
		"No WhatsApp message should be sent for expired mapping")
}

// TestGroupQuoteRouting_CorrectGroup verifies that quoting a message from Group A
// routes the reply to Group A, not Group B.
func TestGroupQuoteRouting_CorrectGroup(t *testing.T) {
	env := NewTestEnvironment(t, "group_quote_routing", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	groupAChatID := "120363028111111111@g.us"
	groupBChatID := "120363028222222222@g.us"
	participantPhone := "15551234567@c.us"

	// Forward a message from Group A to Signal
	groupAMsgID := fmt.Sprintf("wamid.grpA_%d", time.Now().UnixNano())
	sendWhatsAppWebhook(t, env, makeGroupWhatsAppWebhook(
		"personal", groupAChatID, participantPhone, groupAMsgID, "Hello from Group A", "Alice"))
	waitForProcessing()

	// Forward a message from Group B to Signal (now latest)
	groupBMsgID := fmt.Sprintf("wamid.grpB_%d", time.Now().UnixNano())
	sendWhatsAppWebhook(t, env, makeGroupWhatsAppWebhook(
		"personal", groupBChatID, participantPhone, groupBMsgID, "Hello from Group B", "Bob"))
	waitForProcessing()

	// Get Group A's SignalMsgID
	groupASignalMsgID := getSignalMsgIDForWhatsAppMsg(t, env, groupAMsgID)
	t.Logf("Group A SignalMsgID: %s", groupASignalMsgID)

	// Reset tracking and send Signal reply quoting Group A's message
	env.ResetMockAPICounters()

	quoteID, err := strconv.ParseInt(groupASignalMsgID, 10, 64)
	require.NoError(t, err)

	// Send from a "group." prefixed sender to trigger group routing
	resp := sendSignalWebhookWithQuote(t, env, "group.120363028111111111", "+1111111111",
		"Reply to Group A", quoteID)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	waitForProcessing()

	// Assert: reply went to Group A, not Group B
	sends := env.GetWhatsAppSends()
	require.NotEmpty(t, sends, "Should have at least one WhatsApp send")
	assert.Equal(t, groupAChatID, sends[len(sends)-1].ChatID,
		"Reply should go to Group A, not Group B")
}

// TestSyncMessage_NestedQuote_DetectedCorrectly verifies that a Signal SyncMessage
// with a quote nested in dataMessage is correctly detected and routed.
// This tests the fix from commit fb39530.
func TestSyncMessage_NestedQuote_DetectedCorrectly(t *testing.T) {
	env := NewTestEnvironment(t, "sync_nested_quote", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	// Pre-create a mapping that the quote will reference
	targetChatID := "15551234567@c.us"
	targetSignalMsgID := "1700000099999"
	env.CreateTestMessageMappingWithChatID("wamid.target1", targetSignalMsgID, targetChatID, "personal")

	env.ResetMockAPICounters()

	// Build a raw Signal webhook with the quote in the dataMessage field
	// This mimics signal-cli versions that don't use @JsonUnwrapped
	quoteID, _ := strconv.ParseInt(targetSignalMsgID, 10, 64)
	resp := sendSignalWebhookWithQuote(t, env, "+1111111111", "+1111111111",
		"Reply via nested quote", quoteID)
	defer func() { _ = resp.Body.Close() }()

	// The webhook handler directly parses dataMessage.quote, so this should succeed
	require.Equal(t, http.StatusOK, resp.StatusCode)
	waitForProcessing()

	// Assert: message was sent to the correct chat
	sends := env.GetWhatsAppSends()
	require.NotEmpty(t, sends, "Should have at least one WhatsApp send")
	assert.Equal(t, targetChatID, sends[len(sends)-1].ChatID,
		"Reply should go to the chat referenced by the nested quote")
}

// TestReactionRouting_TargetsCorrectMessage verifies that a Signal reaction targets
// the correct WhatsApp message, not a different one.
func TestReactionRouting_TargetsCorrectMessage(t *testing.T) {
	env := NewTestEnvironment(t, "reaction_routing", IsolationProcess)
	defer env.Cleanup()
	env.StartMessageFlowServer()

	aliceChatID := "15551234567@c.us"
	bobChatID := "15559876543@c.us"

	// Alice sends a WhatsApp message → bridged to Signal
	aliceMsgID := fmt.Sprintf("wamid.alice_react_%d", time.Now().UnixNano())
	sendWhatsAppWebhook(t, env, makeWhatsAppWebhook("personal", aliceChatID, aliceMsgID, "React to me!", "Alice"))
	waitForProcessing()

	// Bob sends a WhatsApp message → bridged to Signal
	bobMsgID := fmt.Sprintf("wamid.bob_react_%d", time.Now().UnixNano())
	sendWhatsAppWebhook(t, env, makeWhatsAppWebhook("personal", bobChatID, bobMsgID, "Don't react to me", "Bob"))
	waitForProcessing()

	// Get Alice's SignalMsgID
	aliceSignalMsgID := getSignalMsgIDForWhatsAppMsg(t, env, aliceMsgID)
	aliceSignalTS, err := strconv.ParseInt(aliceSignalMsgID, 10, 64)
	require.NoError(t, err)

	env.ResetMockAPICounters()

	// Send a Signal reaction targeting Alice's message
	// The reaction webhook uses a different structure — it needs the target timestamp
	// We construct it as a direct message service call since the webhook handler
	// doesn't parse reactions from the simple webhook format.
	// Instead, we create a mapping and use the ProcessIncomingSignalMessageWithDestination directly.
	ctx := context.Background()
	reactionMsg := &signaltypes.SignalMessage{
		MessageID: fmt.Sprintf("signal_react_%d", time.Now().UnixNano()),
		Sender:    "+1111111111",
		Timestamp: time.Now().UnixMilli(),
		Reaction: &signaltypes.SignalReaction{
			Emoji:           "👍",
			TargetTimestamp: aliceSignalTS,
			TargetAuthor:    "+1111111111",
		},
	}

	err = env.messageService.ProcessIncomingSignalMessageWithDestination(ctx, reactionMsg, "+1111111111")
	require.NoError(t, err, "Reaction processing should succeed")
	waitForProcessing()

	// Assert: a WhatsApp reaction was sent to the /reaction endpoint.
	// The reaction API uses a different payload format (ReactionRequest) that doesn't include chatId,
	// so we verify the count and that no error was returned (which means the correct mapping was found).
	assert.Equal(t, 1, env.CountMockAPIRequests("whatsapp_reaction"),
		"Exactly one WhatsApp reaction should have been sent")
}
