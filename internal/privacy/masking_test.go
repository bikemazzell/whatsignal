package privacy

import (
	"testing"
)

func TestMaskPhoneNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Standard formats
		{"+1234567890", "+******7890"},
		{"+447712345678", "+********5678"},
		{"1234567890", "******7890"},
		{"447712345678", "********5678"},

		// Edge cases
		{"", ""},
		{"+123", "+***"},
		{"123", "***"},
		{"+1", "+*"},
		{"1", "*"},
		{"12", "**"},
		{"123", "***"},
		{"1234", "****"}, // Will be all masked since <= 4

		// Short numbers with +
		{"+12345", "+*2345"},
		{"+123456", "+**3456"},
	}

	for _, test := range tests {
		result := MaskPhoneNumber(test.input)
		if result != test.expected {
			t.Errorf("MaskPhoneNumber(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestMaskChatID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// WhatsApp format
		{"1234567890@c.us", "******7890@c.us"},
		{"447712345678@c.us", "********5678@c.us"},
		{"12345@g.us", "*2345@g.us"},
		{"1234567890@s.whatsapp.net", "******7890@s.whatsapp.net"},

		// Edge cases
		{"", ""},
		{"123@c.us", "***@c.us"},
		{"12@c.us", "**@c.us"},
		{"1@c.us", "*@c.us"},
		{"@c.us", "@c.us"},

		// Non-WhatsApp formats
		{"some-other-id", "*********r-id"},
		{"1234", "****"},
		{"123", "***"},
		{"12", "**"},
		{"1", "*"},

		// Multiple @ signs
		{"123@test@domain", "***@test@domain"},
	}

	for _, test := range tests {
		result := MaskChatID(test.input)
		if result != test.expected {
			t.Errorf("MaskChatID(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestMaskMessageID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// WhatsApp format
		{"true_1234567890@c.us_A1B2C3D4E5F6G7H8", "true_******7890@c.us_************G7H8"},
		{"false_447712345678@c.us_messageId123", "false_********5678@c.us_********d123"},
		{"true_12345@g.us_msg", "true_*2345@g.us_***"},

		// Edge cases
		{"", ""},
		{"true_123@c.us_m", "true_***@c.us_*"},
		{"simple_message_id", "simple_***sage_**"},
		{"shortmsg", "********"},                   // Less than 8 chars, all masked
		{"verylongmessageid", "*********essageid"}, // Last 8 chars

		// Malformed WhatsApp format
		{"true_", "*****"},
		{"_message", "********"},
		{"true", "****"},
	}

	for _, test := range tests {
		result := MaskMessageID(test.input)
		if result != test.expected {
			t.Errorf("MaskMessageID(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestMaskUserID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user123456", "******3456"},
		{"u12345", "**2345"},
		{"user", "****"},
		{"usr", "***"},
		{"us", "**"},
		{"u", "*"},
		{"", ""},
	}

	for _, test := range tests {
		result := MaskUserID(test.input)
		if result != test.expected {
			t.Errorf("MaskUserID(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestMaskContactID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Phone number formats (should use phone masking)
		{"+1234567890", "+******7890"},
		{"1234567890", "******7890"},   // 10+ digits, numeric
		{"12345678901", "*******8901"}, // 11+ digits, numeric

		// Non-phone formats
		{"user123", "***r123"},
		{"contact_abc", "*******_abc"},
		{"short", "*hort"},
		{"", ""},

		// Mixed formats (not all numeric, so generic masking)
		{"123abc7890", "******7890"},
		{"user123456", "******3456"},
	}

	for _, test := range tests {
		result := MaskContactID(test.input)
		if result != test.expected {
			t.Errorf("MaskContactID(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestMaskSessionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Hyphenated formats
		{"primary-session-user123", "primary-*******-****123"},
		{"main-prod", "main-*rod"},
		{"a-b-c-d", "a-*-*-*"},
		{"single-part", "single-*art"},

		// Simple formats
		{"session123", "*******123"},
		{"primary", "****ary"},
		{"ses", "***"},
		{"se", "**"},
		{"s", "*"},
		{"", ""},

		// Edge cases
		{"a-", "a-"},
		{"-b", "-*"},
		{"a-b", "a-*"},
	}

	for _, test := range tests {
		result := MaskSessionName(test.input)
		if result != test.expected {
			t.Errorf("MaskSessionName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestMaskString(t *testing.T) {
	tests := []struct {
		input    string
		keepLast int
		expected string
	}{
		{"hello world", 5, "******world"},
		{"test", 2, "**st"},
		{"test", 4, "****"},
		{"test", 5, "****"}, // keepLast > length
		{"", 3, ""},
		{"a", 1, "*"},
		{"ab", 1, "*b"},
	}

	for _, test := range tests {
		result := maskString(test.input, test.keepLast)
		if result != test.expected {
			t.Errorf("maskString(%q, %d) = %q, expected %q",
				test.input, test.keepLast, result, test.expected)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1234567890", true},
		{"0", true},
		{"123", true},
		{"", false}, // Empty string is not numeric
		{"123a", false},
		{"12.34", false}, // Decimal point not allowed
		{"+123", false},  // Plus sign not allowed
		{"-123", false},  // Minus sign not allowed
		{"abc", false},
		{"12 34", false}, // Space not allowed
	}

	for _, test := range tests {
		result := isNumeric(test.input)
		if result != test.expected {
			t.Errorf("isNumeric(%q) = %t, expected %t", test.input, result, test.expected)
		}
	}
}

func TestMaskSensitiveFields(t *testing.T) {
	input := map[string]interface{}{
		"phone":       "+1234567890",
		"chat_id":     "1234567890@c.us",
		"message_id":  "true_1234567890@c.us_ABC123XYZ",
		"user_id":     "user123456",
		"session":     "primary-session-user123",
		"other_field": "not_masked",
		"count":       42,
	}

	result := MaskSensitiveFields(input)

	expected := map[string]interface{}{
		"phone":       "+******7890",
		"chat_id":     "******7890@c.us",
		"message_id":  "true_******7890@c.us_*****3XYZ",
		"user_id":     "******3456",
		"session":     "primary-*******-****123",
		"other_field": "not_masked",
		"count":       42,
	}

	for key, expectedVal := range expected {
		if result[key] != expectedVal {
			t.Errorf("MaskSensitiveFields()[%q] = %v, expected %v",
				key, result[key], expectedVal)
		}
	}

	// Test nil input
	nilResult := MaskSensitiveFields(nil)
	if nilResult != nil {
		t.Error("MaskSensitiveFields(nil) should return nil")
	}
}

func TestMaskSensitiveFields_AllFieldTypes(t *testing.T) {
	// Test all supported field name variations
	input := map[string]interface{}{
		"phone_number": "+1234567890",
		"from":         "+0987654321",
		"to":           "+1122334455",
		"chatId":       "chat123@c.us",
		"chat":         "chat456@c.us",
		"messageId":    "msg789",
		"msg_id":       "msg123",
		"userId":       "user789",
		"contactId":    "+5566778899",
		"sessionName":  "test-session-123",
	}

	result := MaskSensitiveFields(input)

	// Verify that all fields were processed
	for key, value := range result {
		strVal, ok := value.(string)
		if !ok {
			continue // Skip non-string values
		}

		switch key {
		case "phone_number", "from", "to":
			if !contains(strVal, "*") {
				t.Errorf("Expected phone field %q to be masked, got %q", key, strVal)
			}
		case "chatId", "chat":
			if !contains(strVal, "*") {
				t.Errorf("Expected chat field %q to be masked, got %q", key, strVal)
			}
		case "messageId", "msg_id":
			if !contains(strVal, "*") {
				t.Errorf("Expected message field %q to be masked, got %q", key, strVal)
			}
		case "userId", "contactId":
			if !contains(strVal, "*") {
				t.Errorf("Expected user/contact field %q to be masked, got %q", key, strVal)
			}
		case "sessionName":
			if !contains(strVal, "*") {
				t.Errorf("Expected session field %q to be masked, got %q", key, strVal)
			}
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
