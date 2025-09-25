package privacy

import (
	"strings"
)

// MaskPhoneNumber masks a phone number showing only the last 4 digits
// Example: "+1234567890" -> "+******7890"
func MaskPhoneNumber(phone string) string {
	if phone == "" {
		return ""
	}

	// Handle + prefix numbers specially
	if strings.HasPrefix(phone, "+") {
		if len(phone) == 1 { // Just "+"
			return phone
		}
		if len(phone) <= 5 { // "+1234" or shorter
			return "+" + strings.Repeat("*", len(phone)-1)
		}
		return "+" + strings.Repeat("*", len(phone)-5) + phone[len(phone)-4:]
	}

	// For numbers without + prefix
	if len(phone) <= 4 {
		return strings.Repeat("*", len(phone))
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}

// MaskChatID masks a chat ID to show structure but hide sensitive parts
// Example: "1234567890@c.us" -> "****7890@c.us"
func MaskChatID(chatID string) string {
	if chatID == "" {
		return ""
	}

	// Handle WhatsApp chat ID format (number@c.us or number@g.us)
	if strings.Contains(chatID, "@") {
		parts := strings.Split(chatID, "@")
		if len(parts) >= 2 {
			numberPart := parts[0]
			domainPart := "@" + strings.Join(parts[1:], "@")

			if len(numberPart) <= 4 {
				return strings.Repeat("*", len(numberPart)) + domainPart
			}
			return strings.Repeat("*", len(numberPart)-4) + numberPart[len(numberPart)-4:] + domainPart
		}
	}

	// For other formats, mask most of it
	if len(chatID) <= 4 {
		return strings.Repeat("*", len(chatID))
	}
	return strings.Repeat("*", len(chatID)-4) + chatID[len(chatID)-4:]
}

// MaskMessageID masks a message ID while preserving some structure for debugging
// Example: "true_1234567890@c.us_A1B2C3D4E5F6G7H8" -> "true_****7890@c.us_****G7H8"
func MaskMessageID(messageID string) string {
	if messageID == "" {
		return ""
	}

	// Handle WhatsApp message ID format: "true_chatId@domain_messageId"
	if strings.Contains(messageID, "_") {
		parts := strings.Split(messageID, "_")
		if len(parts) >= 3 {
			prefix := parts[0]                          // "true" or "false"
			chatPart := parts[1]                        // phone@c.us
			messagePart := strings.Join(parts[2:], "_") // actual message ID

			maskedChat := MaskChatID(chatPart)
			maskedMessage := maskString(messagePart, 4)

			return prefix + "_" + maskedChat + "_" + maskedMessage
		}
	}

	// For other formats, show last 8 characters
	return maskString(messageID, 8)
}

// MaskUserID masks a user identifier
// Example: "user123456" -> "****3456"
func MaskUserID(userID string) string {
	if userID == "" {
		return ""
	}
	return maskString(userID, 4)
}

// MaskContactID masks a contact identifier similar to phone numbers
func MaskContactID(contactID string) string {
	if contactID == "" {
		return ""
	}

	// If it looks like a phone number, use phone masking
	if strings.HasPrefix(contactID, "+") || (len(contactID) >= 10 && isNumeric(contactID)) {
		return MaskPhoneNumber(contactID)
	}

	// Otherwise use generic masking
	return maskString(contactID, 4)
}

// MaskSessionName masks a session name while keeping some readability for debugging
// Example: "primary-session-user123" -> "primary-****-***123"
func MaskSessionName(sessionName string) string {
	if sessionName == "" {
		return ""
	}

	// If it contains hyphens, mask middle parts
	if strings.Contains(sessionName, "-") {
		parts := strings.Split(sessionName, "-")
		if len(parts) >= 2 {
			// Keep first part, mask middle parts, show end of last part
			result := parts[0]
			for i := 1; i < len(parts)-1; i++ {
				result += "-" + strings.Repeat("*", len(parts[i]))
			}
			if len(parts) > 1 {
				lastPart := parts[len(parts)-1]
				result += "-" + maskString(lastPart, 3)
			}
			return result
		}
	}

	// For simple session names, show last 3 characters
	return maskString(sessionName, 3)
}

// maskString masks a string showing only the last n characters
func maskString(s string, keepLast int) string {
	if s == "" {
		return ""
	}

	if len(s) <= keepLast {
		return strings.Repeat("*", len(s))
	}

	return strings.Repeat("*", len(s)-keepLast) + s[len(s)-keepLast:]
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// MaskSensitiveFields applies appropriate masking to common logging fields
func MaskSensitiveFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}

	masked := make(map[string]interface{})
	for k, v := range fields {
		switch k {
		case "phone", "phone_number", "from", "to":
			if s, ok := v.(string); ok {
				masked[k] = MaskPhoneNumber(s)
			} else {
				masked[k] = v
			}
		case "chat_id", "chatId", "chat":
			if s, ok := v.(string); ok {
				masked[k] = MaskChatID(s)
			} else {
				masked[k] = v
			}
		case "message_id", "messageId", "msg_id":
			if s, ok := v.(string); ok {
				masked[k] = MaskMessageID(s)
			} else {
				masked[k] = v
			}
		case "user_id", "userId", "contact_id", "contactId":
			if s, ok := v.(string); ok {
				masked[k] = MaskContactID(s)
			} else {
				masked[k] = v
			}
		case "session", "session_name", "sessionName":
			if s, ok := v.(string); ok {
				masked[k] = MaskSessionName(s)
			} else {
				masked[k] = v
			}
		default:
			masked[k] = v
		}
	}

	return masked
}
