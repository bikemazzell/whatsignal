package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func TestGenerateIDFunctions(t *testing.T) {
	// Test that ID generation functions work correctly
	t.Run("GenerateRequestID", func(t *testing.T) {
		id := GenerateRequestID()
		assert.NotEmpty(t, id)
		assert.True(t, len(id) > 4) // Should have req_ prefix plus content
		assert.Contains(t, id, "req_")

		// Generate multiple IDs to ensure uniqueness
		id2 := GenerateRequestID()
		assert.NotEqual(t, id, id2)
	})

	t.Run("GenerateTraceID", func(t *testing.T) {
		id := GenerateTraceID()
		assert.NotEmpty(t, id)
		assert.Equal(t, 32, len(id)) // Should be 32 hex characters

		// Generate multiple IDs to ensure uniqueness
		id2 := GenerateTraceID()
		assert.NotEqual(t, id, id2)
	})

	t.Run("GenerateSpanID", func(t *testing.T) {
		id := GenerateSpanID()
		assert.NotEmpty(t, id)
		assert.Equal(t, 16, len(id)) // Should be 16 hex characters

		// Generate multiple IDs to ensure uniqueness
		id2 := GenerateSpanID()
		assert.NotEqual(t, id, id2)
	})
}

func TestTracingManager_InitializeWithErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	tests := []struct {
		name   string
		config TracingConfig
	}{
		{
			name: "Initialize with stdout exporter",
			config: TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				OTLPEndpoint:   "", // Use stdout when no endpoint
				SampleRate:     1.0,
				Enabled:        true,
				UseStdout:      true,
			},
		},
		{
			name: "Initialize with OTLP exporter",
			config: TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				OTLPEndpoint:   "http://localhost:4318/v1/traces",
				SampleRate:     1.0,
				Enabled:        true,
				UseStdout:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewTracingManager(tt.config, logger)
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			err := manager.Initialize(ctx)
			// These should initialize successfully or fail gracefully
			if tt.config.UseStdout {
				assert.NoError(t, err)
			} else {
				// OTLP may fail if endpoint unreachable, that's ok
				_ = err
			}

			// Shutdown should always work regardless of init status
			err = manager.Shutdown(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestTracingManager_ShutdownErrors(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	manager := NewTracingManager(TracingConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "",
		SampleRate:     1.0,
		Enabled:        true,
		UseStdout:      true,
	}, logger)

	// Initialize
	ctx := context.Background()
	err := manager.Initialize(ctx)
	require.NoError(t, err)

	// Shutdown with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = manager.Shutdown(cancelledCtx)
	// Should handle cancelled context gracefully
	assert.Error(t, err)

	// Try to shutdown again (should be idempotent)
	err = manager.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestGetOtelTraceID_NotSampled(t *testing.T) {
	ctx := context.Background()
	// Context without trace should return empty or zero value
	traceID := GetOtelTraceID(ctx)
	// OpenTelemetry may return either empty string or zero trace ID
	assert.True(t, traceID == "" || traceID == "00000000000000000000000000000000")
}

func TestGetOtelSpanID_NotSampled(t *testing.T) {
	ctx := context.Background()
	// Context without span should return empty or zero value
	spanID := GetOtelSpanID(ctx)
	// OpenTelemetry may return either empty string or zero span ID
	assert.True(t, spanID == "" || spanID == "0000000000000000")
}

func TestTracingManager_DisabledTracingEdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	manager := NewTracingManager(TracingConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "localhost:4317",
		SampleRate:     1.0,
		Enabled:        false, // Disabled
	}, logger)

	// Initialize should be a no-op when disabled
	ctx := context.Background()
	err := manager.Initialize(ctx)
	assert.NoError(t, err)

	// GetTracer should work
	tracer := manager.GetTracer("test")
	assert.NotNil(t, tracer)

	// Shutdown should be a no-op when disabled
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestSpanHelpers_EdgeCases(t *testing.T) {
	// Test span operations without active span context
	ctx := context.Background()

	// AddSpanAttributes should not panic with no span
	AddSpanAttributes(ctx, attribute.String("test", "value"))

	// SetSpanStatus should not panic with no span
	SetSpanStatus(ctx, codes.Ok, "test")

	// RecordError should not panic with no span
	RecordError(ctx, assert.AnError)
}

func TestContextHelpers_EdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test adding full tracing without prior context
	tracedCtx := WithFullTracing(ctx)

	// Verify all values are set
	requestInfo := GetRequestInfo(tracedCtx)
	assert.NotEmpty(t, requestInfo.RequestID)
	assert.NotEmpty(t, requestInfo.TraceID)
	assert.NotEmpty(t, requestInfo.SpanID)
	assert.False(t, requestInfo.StartTime.IsZero())

	// Test Duration calculation
	duration := Duration(tracedCtx)
	assert.True(t, duration >= 0)

	// Test Duration with empty context
	emptyDuration := Duration(ctx)
	assert.Equal(t, time.Duration(0), emptyDuration)
}
