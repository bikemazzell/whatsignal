package tracing

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithOtelTracing_DisabledTracing verifies that trace IDs are generated
// even when OpenTelemetry tracing is disabled
func TestWithOtelTracing_DisabledTracing(t *testing.T) {
	// Don't initialize OpenTelemetry - this simulates the default state
	// when tracing is disabled in config
	ctx := context.Background()

	// Call WithOtelTracing which should generate legacy trace IDs
	tracedCtx, span := WithOtelTracing(ctx, "test-operation")
	defer span.End()

	// Extract trace ID from context
	traceID := GetTraceID(tracedCtx)
	spanID := GetSpanID(tracedCtx)

	// Verify trace ID is not empty and not all zeros
	assert.NotEmpty(t, traceID, "Trace ID should not be empty")
	assert.NotEqual(t, "00000000000000000000000000000000", traceID, "Trace ID should not be all zeros")
	assert.NotEqual(t, "", traceID, "Trace ID should be generated")

	// Verify span ID is not empty and not all zeros
	assert.NotEmpty(t, spanID, "Span ID should not be empty")
	assert.NotEqual(t, "0000000000000000", spanID, "Span ID should not be all zeros")
	assert.NotEqual(t, "", spanID, "Span ID should be generated")

	// Verify trace ID has correct format (32 hex characters)
	assert.Len(t, traceID, 32, "Trace ID should be 32 characters (16 bytes in hex)")

	// Verify span ID has correct format (16 hex characters)
	assert.Len(t, spanID, 16, "Span ID should be 16 characters (8 bytes in hex)")

	t.Logf("Generated trace ID: %s", traceID)
	t.Logf("Generated span ID: %s", spanID)
}

// TestWithOtelTracing_EnabledTracing verifies that OpenTelemetry trace IDs
// are used when tracing is enabled
func TestWithOtelTracing_EnabledTracing(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	// Initialize OpenTelemetry tracing
	config := TracingConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		SampleRate:     1.0, // 100% sampling
		Enabled:        true,
		UseStdout:      true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	// Call WithOtelTracing which should use OpenTelemetry trace IDs
	tracedCtx, span := WithOtelTracing(ctx, "test-operation")
	defer span.End()

	// Extract trace ID from context
	traceID := GetTraceID(tracedCtx)
	spanID := GetSpanID(tracedCtx)

	// Verify trace ID is not empty and not all zeros
	assert.NotEmpty(t, traceID, "Trace ID should not be empty")
	assert.NotEqual(t, "00000000000000000000000000000000", traceID, "Trace ID should not be all zeros")

	// Verify span ID is not empty and not all zeros
	assert.NotEmpty(t, spanID, "Span ID should not be empty")
	assert.NotEqual(t, "0000000000000000", spanID, "Span ID should not be all zeros")

	// Verify trace ID has correct format (32 hex characters)
	assert.Len(t, traceID, 32, "Trace ID should be 32 characters (16 bytes in hex)")

	// Verify span ID has correct format (16 hex characters)
	assert.Len(t, spanID, 16, "Span ID should be 16 characters (8 bytes in hex)")

	t.Logf("OpenTelemetry trace ID: %s", traceID)
	t.Logf("OpenTelemetry span ID: %s", spanID)
}

// TestGetOtelTraceID_InvalidSpan verifies that GetOtelTraceID returns empty
// string for invalid spans (all zeros)
func TestGetOtelTraceID_InvalidSpan(t *testing.T) {
	// Don't initialize OpenTelemetry - spans will be no-op with invalid trace IDs
	ctx := context.Background()

	// Start a span without initializing OpenTelemetry
	spanCtx, span := StartSpan(ctx, "test-span")
	defer span.End()

	// GetOtelTraceID should return empty string for invalid trace ID
	traceID := GetOtelTraceID(spanCtx)
	assert.Empty(t, traceID, "GetOtelTraceID should return empty string for invalid trace ID")
}

// TestGetOtelSpanID_InvalidSpan verifies that GetOtelSpanID returns empty
// string for invalid spans (all zeros)
func TestGetOtelSpanID_InvalidSpan(t *testing.T) {
	// Don't initialize OpenTelemetry - spans will be no-op with invalid span IDs
	ctx := context.Background()

	// Start a span without initializing OpenTelemetry
	spanCtx, span := StartSpan(ctx, "test-span")
	defer span.End()

	// GetOtelSpanID should return empty string for invalid span ID
	spanID := GetOtelSpanID(spanCtx)
	assert.Empty(t, spanID, "GetOtelSpanID should return empty string for invalid span ID")
}

// TestMultipleWithOtelTracing_UniqueIDs verifies that each call to
// WithOtelTracing generates unique trace and span IDs
func TestMultipleWithOtelTracing_UniqueIDs(t *testing.T) {
	ctx := context.Background()

	// Generate multiple trace contexts
	const numTraces = 10
	traceIDs := make(map[string]bool)
	spanIDs := make(map[string]bool)

	for i := 0; i < numTraces; i++ {
		tracedCtx, span := WithOtelTracing(ctx, "test-operation")

		traceID := GetTraceID(tracedCtx)
		spanID := GetSpanID(tracedCtx)

		// Verify IDs are not empty or all zeros
		assert.NotEmpty(t, traceID)
		assert.NotEqual(t, "00000000000000000000000000000000", traceID)
		assert.NotEmpty(t, spanID)
		assert.NotEqual(t, "0000000000000000", spanID)

		// Check for uniqueness
		assert.False(t, traceIDs[traceID], "Trace ID %s should be unique", traceID)
		assert.False(t, spanIDs[spanID], "Span ID %s should be unique", spanID)

		traceIDs[traceID] = true
		spanIDs[spanID] = true

		span.End()
	}

	assert.Len(t, traceIDs, numTraces, "Should have %d unique trace IDs", numTraces)
	assert.Len(t, spanIDs, numTraces, "Should have %d unique span IDs", numTraces)
}

// TestRequestInfo_WithDisabledTracing verifies that GetRequestInfo returns
// valid trace IDs even when OpenTelemetry is disabled
func TestRequestInfo_WithDisabledTracing(t *testing.T) {
	ctx := context.Background()

	// Add tracing to context
	tracedCtx, span := WithOtelTracing(ctx, "test-operation")
	defer span.End()

	// Add request ID
	requestID := GenerateRequestID()
	tracedCtx = WithRequestID(tracedCtx, requestID)

	// Get request info
	info := GetRequestInfo(tracedCtx)

	// Verify all fields are populated
	assert.NotEmpty(t, info.RequestID, "Request ID should not be empty")
	assert.NotEmpty(t, info.TraceID, "Trace ID should not be empty")
	assert.NotEmpty(t, info.SpanID, "Span ID should not be empty")

	// Verify trace ID is not all zeros
	assert.NotEqual(t, "00000000000000000000000000000000", info.TraceID, "Trace ID should not be all zeros")
	assert.NotEqual(t, "0000000000000000", info.SpanID, "Span ID should not be all zeros")

	t.Logf("Request Info - RequestID: %s, TraceID: %s, SpanID: %s",
		info.RequestID, info.TraceID, info.SpanID)
}
