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
	assert.Equal(t, "http://localhost:14268/api/traces", config.JaegerEndpoint)
	assert.Equal(t, 0.1, config.SampleRate)
	assert.False(t, config.Enabled)
	assert.True(t, config.UseStdout)
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
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		SampleRate:     1.0, // 100% sampling for testing
		Enabled:        true,
		UseStdout:      true,
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
