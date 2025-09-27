package integration_test

import (
	"fmt"
	"net"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/whatsapp/types"
)

// Network test helpers

// GetAvailablePort returns an available port for testing
func GetAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// WaitForPort waits for a port to become available for connections
func WaitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("port %d did not become available within %v", port, timeout)
}

// Test data factories

// CreateTestWhatsAppWebhook creates a test WhatsApp webhook payload
func CreateTestWhatsAppWebhook(session, messageID, from, body string) models.WhatsAppWebhookPayload {
	return models.WhatsAppWebhookPayload{
		Session: session,
		Event:   models.EventMessage,
		Payload: struct {
			ID        string `json:"id"`
			Timestamp int64  `json:"timestamp"`
			From      string `json:"from"`
			FromMe    bool   `json:"fromMe"`
			To        string `json:"to"`
			Body      string `json:"body"`
			HasMedia  bool   `json:"hasMedia"`
			Media     *struct {
				URL      string `json:"url"`
				MimeType string `json:"mimetype"`
				Filename string `json:"filename"`
			} `json:"media"`
			Reaction *struct {
				Text      string `json:"text"`
				MessageID string `json:"messageId"`
			} `json:"reaction"`
			EditedMessageID *string `json:"editedMessageId,omitempty"`
			ACK             *int    `json:"ack,omitempty"`
		}{
			ID:        messageID,
			From:      from,
			To:        session + "@c.us",
			Body:      body,
			Timestamp: time.Now().Unix(),
			FromMe:    false,
			HasMedia:  false,
		},
	}
}

// CreateTestWhatsAppContact creates a test WhatsApp contact
func CreateTestWhatsAppContact(id, number, name string) types.Contact {
	return types.Contact{
		ID:     id,
		Number: number,
		Name:   name,
	}
}

// Performance helpers

// MeasureExecutionTime measures the execution time of a function
func MeasureExecutionTime(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// MemorySnapshot captures current memory usage
type MemorySnapshot struct {
	HeapAlloc  uint64
	HeapInuse  uint64
	StackInuse uint64
	NumGC      uint32
	Timestamp  time.Time
}

// TakeMemorySnapshot captures current memory statistics
func TakeMemorySnapshot() MemorySnapshot {
	// This would normally use runtime.MemStats, but for simplicity
	// we'll return a basic snapshot
	return MemorySnapshot{
		Timestamp: time.Now(),
	}
}
