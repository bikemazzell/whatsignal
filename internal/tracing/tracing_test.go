package tracing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	// Should generate unique IDs
	if id1 == id2 {
		t.Fatal("Expected unique request IDs")
	}

	// Should have correct prefix
	if !strings.HasPrefix(id1, "req_") {
		t.Fatalf("Expected request ID to start with 'req_', got %s", id1)
	}

	// Should be reasonable length
	if len(id1) < 10 {
		t.Fatalf("Expected request ID to be at least 10 characters, got %d", len(id1))
	}
}

func TestGenerateTraceID(t *testing.T) {
	id1 := GenerateTraceID()
	id2 := GenerateTraceID()

	// Should generate unique IDs
	if id1 == id2 {
		t.Fatal("Expected unique trace IDs")
	}

	// Should be hex string (32 characters for 16 bytes)
	if len(id1) != 32 {
		t.Fatalf("Expected trace ID to be 32 characters, got %d", len(id1))
	}

	// Should contain only hex characters
	for _, char := range id1 {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			t.Fatalf("Expected trace ID to contain only hex characters, got %s", id1)
		}
	}
}

func TestGenerateSpanID(t *testing.T) {
	id1 := GenerateSpanID()
	id2 := GenerateSpanID()

	// Should generate unique IDs
	if id1 == id2 {
		t.Fatal("Expected unique span IDs")
	}

	// Should be hex string (16 characters for 8 bytes)
	if len(id1) != 16 {
		t.Fatalf("Expected span ID to be 16 characters, got %d", len(id1))
	}

	// Should contain only hex characters
	for _, char := range id1 {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			t.Fatalf("Expected span ID to contain only hex characters, got %s", id1)
		}
	}
}

func TestWithAndGetRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	// Test setting request ID
	ctx = WithRequestID(ctx, requestID)

	// Test getting request ID
	retrievedID := GetRequestID(ctx)
	if retrievedID != requestID {
		t.Fatalf("Expected request ID to be %s, got %s", requestID, retrievedID)
	}

	// Test empty context
	emptyCtx := context.Background()
	emptyID := GetRequestID(emptyCtx)
	if emptyID != "" {
		t.Fatalf("Expected empty request ID from empty context, got %s", emptyID)
	}
}

func TestWithAndGetTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "abc123def456"

	// Test setting trace ID
	ctx = WithTraceID(ctx, traceID)

	// Test getting trace ID
	retrievedID := GetTraceID(ctx)
	if retrievedID != traceID {
		t.Fatalf("Expected trace ID to be %s, got %s", traceID, retrievedID)
	}

	// Test empty context
	emptyCtx := context.Background()
	emptyID := GetTraceID(emptyCtx)
	if emptyID != "" {
		t.Fatalf("Expected empty trace ID from empty context, got %s", emptyID)
	}
}

func TestWithAndGetSpanID(t *testing.T) {
	ctx := context.Background()
	spanID := "span789xyz"

	// Test setting span ID
	ctx = WithSpanID(ctx, spanID)

	// Test getting span ID
	retrievedID := GetSpanID(ctx)
	if retrievedID != spanID {
		t.Fatalf("Expected span ID to be %s, got %s", spanID, retrievedID)
	}

	// Test empty context
	emptyCtx := context.Background()
	emptyID := GetSpanID(emptyCtx)
	if emptyID != "" {
		t.Fatalf("Expected empty span ID from empty context, got %s", emptyID)
	}
}

func TestWithAndGetStartTime(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now()

	// Test setting start time
	ctx = WithStartTime(ctx, startTime)

	// Test getting start time
	retrievedTime := GetStartTime(ctx)
	if !retrievedTime.Equal(startTime) {
		t.Fatalf("Expected start time to be %v, got %v", startTime, retrievedTime)
	}

	// Test empty context
	emptyCtx := context.Background()
	emptyTime := GetStartTime(emptyCtx)
	if !emptyTime.IsZero() {
		t.Fatalf("Expected zero time from empty context, got %v", emptyTime)
	}
}

func TestGetRequestInfo(t *testing.T) {
	ctx := context.Background()
	requestID := "req-123"
	traceID := "trace-456"
	spanID := "span-789"
	startTime := time.Now()

	// Add all tracing info to context
	ctx = WithRequestID(ctx, requestID)
	ctx = WithTraceID(ctx, traceID)
	ctx = WithSpanID(ctx, spanID)
	ctx = WithStartTime(ctx, startTime)

	// Get request info
	info := GetRequestInfo(ctx)

	if info.RequestID != requestID {
		t.Fatalf("Expected request ID to be %s, got %s", requestID, info.RequestID)
	}
	if info.TraceID != traceID {
		t.Fatalf("Expected trace ID to be %s, got %s", traceID, info.TraceID)
	}
	if info.SpanID != spanID {
		t.Fatalf("Expected span ID to be %s, got %s", spanID, info.SpanID)
	}
	if !info.StartTime.Equal(startTime) {
		t.Fatalf("Expected start time to be %v, got %v", startTime, info.StartTime)
	}
}

func TestWithFullTracing(t *testing.T) {
	ctx := context.Background()

	// Add full tracing to context
	tracedCtx := WithFullTracing(ctx)

	// Get all tracing info
	info := GetRequestInfo(tracedCtx)

	// Verify all fields are populated
	if info.RequestID == "" {
		t.Fatal("Expected non-empty request ID")
	}
	if info.TraceID == "" {
		t.Fatal("Expected non-empty trace ID")
	}
	if info.SpanID == "" {
		t.Fatal("Expected non-empty span ID")
	}
	if info.StartTime.IsZero() {
		t.Fatal("Expected non-zero start time")
	}

	// Verify proper formats
	if !strings.HasPrefix(info.RequestID, "req_") {
		t.Fatalf("Expected request ID to start with 'req_', got %s", info.RequestID)
	}

	if len(info.TraceID) != 32 {
		t.Fatalf("Expected trace ID to be 32 characters, got %d", len(info.TraceID))
	}

	if len(info.SpanID) != 16 {
		t.Fatalf("Expected span ID to be 16 characters, got %d", len(info.SpanID))
	}

	// Start time should be recent
	if time.Since(info.StartTime) > time.Second {
		t.Fatal("Expected start time to be recent")
	}
}

func TestDuration(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now()

	// Test with start time in context
	ctx = WithStartTime(ctx, startTime)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	duration := Duration(ctx)

	// Duration should be positive and reasonable
	if duration <= 0 {
		t.Fatal("Expected positive duration")
	}

	if duration < 10*time.Millisecond {
		t.Fatal("Expected duration to be at least 10ms")
	}

	if duration > time.Second {
		t.Fatal("Expected duration to be less than 1 second")
	}

	// Test with empty context
	emptyCtx := context.Background()
	emptyDuration := Duration(emptyCtx)
	if emptyDuration != 0 {
		t.Fatalf("Expected zero duration from empty context, got %v", emptyDuration)
	}
}

func TestContextChaining(t *testing.T) {
	ctx := context.Background()

	// Test chaining multiple context operations
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithSpanID(ctx, "span-1")
	ctx = WithStartTime(ctx, time.Now())

	// Verify all values are preserved
	if GetRequestID(ctx) != "req-1" {
		t.Fatal("Request ID not preserved in chain")
	}
	if GetTraceID(ctx) != "trace-1" {
		t.Fatal("Trace ID not preserved in chain")
	}
	if GetSpanID(ctx) != "span-1" {
		t.Fatal("Span ID not preserved in chain")
	}
	if GetStartTime(ctx).IsZero() {
		t.Fatal("Start time not preserved in chain")
	}
}

func TestMultipleFullTracing(t *testing.T) {
	// Test that multiple calls generate different IDs
	ctx1 := WithFullTracing(context.Background())
	ctx2 := WithFullTracing(context.Background())

	info1 := GetRequestInfo(ctx1)
	info2 := GetRequestInfo(ctx2)

	// All IDs should be different
	if info1.RequestID == info2.RequestID {
		t.Fatal("Expected different request IDs")
	}
	if info1.TraceID == info2.TraceID {
		t.Fatal("Expected different trace IDs")
	}
	if info1.SpanID == info2.SpanID {
		t.Fatal("Expected different span IDs")
	}
	// Start times might be the same if very fast execution
}
