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

func TestDefaultTracingConfig(t *testing.T) {
	config := DefaultTracingConfig()

	assert.Equal(t, "whatsignal", config.ServiceName)
	assert.Equal(t, "dev", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "http://localhost:4318/v1/traces", config.OTLPEndpoint)
	assert.Equal(t, 0.1, config.SampleRate)
	assert.False(t, config.Enabled)
	assert.True(t, config.UseStdout)
	assert.Equal(t, 5, config.ShutdownTimeoutSec)
}

func TestTracingConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      TracingConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config with stdout",
			config: TracingConfig{
				ServiceName: "test-service",
				SampleRate:  0.5,
				Enabled:     true,
				UseStdout:   true,
			},
			expectError: false,
		},
		{
			name: "Valid config with OTLP",
			config: TracingConfig{
				ServiceName:  "test-service",
				SampleRate:   1.0,
				Enabled:      true,
				UseStdout:    false,
				OTLPEndpoint: "http://localhost:4318/v1/traces",
			},
			expectError: false,
		},
		{
			name: "Disabled config - no validation",
			config: TracingConfig{
				Enabled: false,
				// Missing required fields, but should pass since disabled
			},
			expectError: false,
		},
		{
			name: "Missing service name",
			config: TracingConfig{
				SampleRate: 0.5,
				Enabled:    true,
				UseStdout:  true,
			},
			expectError: true,
			errorMsg:    "service_name is required",
		},
		{
			name: "Invalid sample rate - negative",
			config: TracingConfig{
				ServiceName: "test-service",
				SampleRate:  -0.1,
				Enabled:     true,
				UseStdout:   true,
			},
			expectError: true,
			errorMsg:    "sample_rate must be between 0 and 1",
		},
		{
			name: "Invalid sample rate - too high",
			config: TracingConfig{
				ServiceName: "test-service",
				SampleRate:  1.5,
				Enabled:     true,
				UseStdout:   true,
			},
			expectError: true,
			errorMsg:    "sample_rate must be between 0 and 1",
		},
		{
			name: "Missing OTLP endpoint when not using stdout",
			config: TracingConfig{
				ServiceName: "test-service",
				SampleRate:  0.5,
				Enabled:     true,
				UseStdout:   false,
			},
			expectError: true,
			errorMsg:    "otlp_endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewTracingManager_NilLogger(t *testing.T) {
	config := TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	// Should not panic with nil logger
	tm := NewTracingManager(config, nil)
	assert.NotNil(t, tm)
	assert.NotNil(t, tm.logger)

	// Should be able to initialize without panic
	ctx := context.Background()
	err := tm.Initialize(ctx)
	assert.NoError(t, err)
}

func TestTracingManager_DisabledTracing(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	config := TracingConfig{
		Enabled: false,
	}

	tm := NewTracingManager(config, logger)

	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)

	err = tm.Shutdown(ctx)
	require.NoError(t, err)
}

func TestTracingManager_EnabledWithStdout(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during test

	config := TracingConfig{
		ServiceName:        "test-service",
		ServiceVersion:     "1.0.0",
		Environment:        "test",
		SampleRate:         1.0, // 100% sampling for testing
		Enabled:            true,
		UseStdout:          true,
		ShutdownTimeoutSec: 3,
	}

	tm := NewTracingManager(config, logger)

	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)

	// Test that we can get a tracer
	tracer := tm.GetTracer("test-tracer")
	assert.NotNil(t, tracer)

	err = tm.Shutdown(ctx)
	require.NoError(t, err)
}

func TestTracingManager_IdempotentShutdown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName:        "test-service",
		ServiceVersion:     "1.0.0",
		Environment:        "test",
		SampleRate:         1.0,
		Enabled:            true,
		UseStdout:          true,
		ShutdownTimeoutSec: 2,
	}

	tm := NewTracingManager(config, logger)

	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)

	// First shutdown should succeed
	err = tm.Shutdown(ctx)
	require.NoError(t, err)

	// Second shutdown should also succeed (idempotent)
	err = tm.Shutdown(ctx)
	require.NoError(t, err)

	// Third shutdown should also succeed
	err = tm.Shutdown(ctx)
	require.NoError(t, err)
}

func TestTracingManager_CustomShutdownTimeout(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	tests := []struct {
		name         string
		timeoutSec   int
		expectedUsed time.Duration
		description  string
	}{
		{
			name:         "Custom timeout - 10 seconds",
			timeoutSec:   10,
			expectedUsed: 10 * time.Second,
			description:  "Should use configured timeout",
		},
		{
			name:         "Zero timeout - use default",
			timeoutSec:   0,
			expectedUsed: 5 * time.Second,
			description:  "Should fall back to default 5 seconds",
		},
		{
			name:         "Negative timeout - use default",
			timeoutSec:   -1,
			expectedUsed: 5 * time.Second,
			description:  "Should fall back to default 5 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := TracingConfig{
				ServiceName:        "test-service",
				ServiceVersion:     "1.0.0",
				Environment:        "test",
				SampleRate:         1.0,
				Enabled:            true,
				UseStdout:          true,
				ShutdownTimeoutSec: tt.timeoutSec,
			}

			tm := NewTracingManager(config, logger)

			ctx := context.Background()
			err := tm.Initialize(ctx)
			require.NoError(t, err)

			// Measure shutdown time
			start := time.Now()
			err = tm.Shutdown(ctx)
			elapsed := time.Since(start)

			require.NoError(t, err)
			// Shutdown should complete quickly (well under the timeout)
			assert.Less(t, elapsed, tt.expectedUsed, tt.description)
		})
	}
}

func TestTracingManager_InitializeWithCancelledContext(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := tm.Initialize(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestStartSpan(t *testing.T) {
	// Initialize tracing for testing
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	// Test starting a span
	spanCtx, span := StartSpan(ctx, "test-span")
	assert.NotNil(t, span)

	// Test that span context has trace info
	assert.True(t, span.SpanContext().IsValid())

	// Test adding attributes
	_, span2 := StartSpan(spanCtx, "test-span-with-attrs",
		attribute.String("test.key", "test.value"),
		attribute.Int("test.number", 42),
	)
	assert.NotNil(t, span2)

	span2.End()
	span.End()
}

func TestAddSpanAttributes(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	spanCtx, span := StartSpan(ctx, "test-span")

	// Test adding attributes to existing span
	AddSpanAttributes(spanCtx,
		attribute.String("added.key", "added.value"),
		attribute.Bool("added.bool", true),
	)

	span.End()
}

func TestSetSpanStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	spanCtx, span := StartSpan(ctx, "test-span")

	// Test setting span status
	SetSpanStatus(spanCtx, codes.Ok, "Operation completed successfully")

	span.End()
}

func TestRecordError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	spanCtx, span := StartSpan(ctx, "test-span")

	// Test recording an error
	testErr := assert.AnError
	RecordError(spanCtx, testErr, attribute.String("error.context", "test context"))

	span.End()
}

func TestGetOtelTraceAndSpanID(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	spanCtx, span := StartSpan(ctx, "test-span")

	// Test getting trace and span IDs
	traceID := GetOtelTraceID(spanCtx)
	spanID := GetOtelSpanID(spanCtx)

	assert.NotEmpty(t, traceID)
	assert.NotEmpty(t, spanID)
	assert.Len(t, traceID, 32) // Trace ID should be 32 hex characters
	assert.Len(t, spanID, 16)  // Span ID should be 16 hex characters

	span.End()
}

func TestWithOtelTracing(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	// Add legacy tracing to context
	ctx = WithRequestID(ctx, "test-request-id")
	ctx = WithStartTime(ctx, time.Now())

	// Test WithOtelTracing integrates both systems
	spanCtx, span := WithOtelTracing(ctx, "test-operation")

	// Legacy tracing should still work
	requestInfo := GetRequestInfo(spanCtx)
	assert.Equal(t, "test-request-id", requestInfo.RequestID)
	assert.NotEmpty(t, requestInfo.TraceID) // Should now have OpenTel trace ID
	assert.NotEmpty(t, requestInfo.SpanID)  // Should now have OpenTel span ID

	// OpenTelemetry should work
	otelTraceID := GetOtelTraceID(spanCtx)
	otelSpanID := GetOtelSpanID(spanCtx)
	assert.NotEmpty(t, otelTraceID)
	assert.NotEmpty(t, otelSpanID)

	// They should match
	assert.Equal(t, otelTraceID, requestInfo.TraceID)
	assert.Equal(t, otelSpanID, requestInfo.SpanID)

	span.End()
}

func TestTracingManager_ShutdownWithoutInit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	tm := NewTracingManager(TracingConfig{}, logger)

	// Should handle shutdown without initialization gracefully
	ctx := context.Background()
	err := tm.Shutdown(ctx)
	require.NoError(t, err)
}

func TestStartSpanWithTracer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	config := TracingConfig{
		ServiceName: "test-service",
		SampleRate:  1.0,
		Enabled:     true,
		UseStdout:   true,
	}

	tm := NewTracingManager(config, logger)
	ctx := context.Background()
	err := tm.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tm.Shutdown(ctx)
	}()

	tracer := tm.GetTracer("custom-tracer")
	_, span := StartSpanWithTracer(ctx, tracer, "custom-span",
		attribute.String("custom.attr", "value"),
	)

	assert.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())

	span.End()
}
