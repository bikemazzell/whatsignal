// Package tracing provides distributed tracing capabilities for WhatSignal.
//
// It integrates OpenTelemetry for standards-based tracing while maintaining
// a legacy tracing system for backward compatibility. The package supports:
//
//   - OpenTelemetry-based distributed tracing with OTLP export
//   - Legacy trace/span ID generation for systems without OpenTelemetry
//   - Automatic fallback when OpenTelemetry is disabled
//   - Context-based trace propagation
//
// Basic usage:
//
//	// Initialize tracing manager
//	tm := tracing.NewTracingManager(config, logger)
//	if err := tm.Initialize(ctx); err != nil {
//		log.Fatal(err)
//	}
//	defer tm.Shutdown(context.Background())
//
//	// Create spans in request handlers
//	ctx, span := tracing.WithOtelTracing(r.Context(), "operation_name")
//	defer span.End()
//
// The package automatically generates legacy trace IDs when OpenTelemetry
// is disabled, ensuring all requests have unique trace identifiers.
package tracing

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	// invalidTraceID represents an all-zero trace ID (128 bits = 32 hex chars)
	// returned by OpenTelemetry when tracing is disabled or not initialized.
	invalidTraceID = "00000000000000000000000000000000"

	// invalidSpanID represents an all-zero span ID (64 bits = 16 hex chars)
	// returned by OpenTelemetry when tracing is disabled or not initialized.
	invalidSpanID = "0000000000000000"

	// defaultShutdownTimeout is the default timeout for graceful shutdown
	// of the tracer provider, allowing time for span export to complete.
	defaultShutdownTimeout = 5 * time.Second

	// defaultTracerName is the default name used for creating tracers
	// when no specific name is provided.
	defaultTracerName = "whatsignal"
)

// TracingConfig contains OpenTelemetry configuration.
type TracingConfig struct {
	ServiceName        string  `json:"service_name" mapstructure:"service_name"`
	ServiceVersion     string  `json:"service_version" mapstructure:"service_version"`
	Environment        string  `json:"environment" mapstructure:"environment"`
	OTLPEndpoint       string  `json:"otlp_endpoint" mapstructure:"otlp_endpoint"`
	SampleRate         float64 `json:"sample_rate" mapstructure:"sample_rate"`
	Enabled            bool    `json:"enabled" mapstructure:"enabled"`
	UseStdout          bool    `json:"use_stdout" mapstructure:"use_stdout"`
	ShutdownTimeoutSec int     `json:"shutdown_timeout_sec" mapstructure:"shutdown_timeout_sec"`
}

// DefaultTracingConfig returns sensible defaults for tracing configuration.
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		ServiceName:        "whatsignal",
		ServiceVersion:     "dev",
		Environment:        "development",
		OTLPEndpoint:       "http://localhost:4318/v1/traces",
		SampleRate:         0.1, // 10% sampling
		Enabled:            false,
		UseStdout:          true,
		ShutdownTimeoutSec: 5, // 5 seconds
	}
}

// Validate checks if the tracing configuration is valid.
// It returns an error if required fields are missing or values are out of range.
func (c *TracingConfig) Validate() error {
	if !c.Enabled {
		return nil // Skip validation if tracing is disabled
	}

	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required when tracing is enabled")
	}

	if c.SampleRate < 0 || c.SampleRate > 1 {
		return fmt.Errorf("sample_rate must be between 0 and 1, got %f", c.SampleRate)
	}

	if !c.UseStdout && c.OTLPEndpoint == "" {
		return fmt.Errorf("otlp_endpoint is required when use_stdout is false")
	}

	return nil
}

// TracingManager manages OpenTelemetry setup and lifecycle.
type TracingManager struct {
	config         TracingConfig
	logger         *logrus.Logger
	tracerProvider *trace.TracerProvider
}

// NewTracingManager creates a new tracing manager.
// If logger is nil, a default logger with WARN level is created.
func NewTracingManager(config TracingConfig, logger *logrus.Logger) *TracingManager {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}
	return &TracingManager{
		config: config,
		logger: logger,
	}
}

// Initialize sets up OpenTelemetry tracing with the configured exporter.
// It returns an error if tracing is enabled but initialization fails.
// When tracing is disabled, this is a no-op that returns nil.
//
// The context is used for resource creation and exporter initialization.
// If the context is cancelled, initialization is aborted.
func (tm *TracingManager) Initialize(ctx context.Context) error {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before initialization: %w", err)
	}

	// Validate configuration
	if err := tm.config.Validate(); err != nil {
		return fmt.Errorf("invalid tracing configuration: %w", err)
	}

	if !tm.config.Enabled {
		tm.logger.WithFields(logrus.Fields{
			"service": tm.config.ServiceName,
			"enabled": false,
		}).Info("OpenTelemetry tracing is disabled")
		return nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(tm.config.ServiceName),
			semconv.ServiceVersionKey.String(tm.config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(tm.config.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter trace.SpanExporter
	if tm.config.UseStdout {
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return fmt.Errorf("failed to create stdout exporter: %w", err)
		}
		tm.logger.WithFields(logrus.Fields{
			"service":  tm.config.ServiceName,
			"exporter": "stdout",
		}).Info("Using stdout trace exporter")
	} else {
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(tm.config.OTLPEndpoint),
			otlptracehttp.WithInsecure(), // Use HTTP instead of HTTPS for local development
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
		}
		tm.logger.WithFields(logrus.Fields{
			"service":  tm.config.ServiceName,
			"exporter": "otlp_http",
			"endpoint": tm.config.OTLPEndpoint,
		}).Info("Using OTLP HTTP trace exporter")
	}

	// Ensure exporter is cleaned up if initialization fails after this point
	var initErr error
	defer func() {
		if initErr != nil && exporter != nil {
			_ = exporter.Shutdown(context.Background())
		}
	}()

	// Create tracer provider
	tm.tracerProvider = trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(tm.config.SampleRate)),
	)

	// Set global provider and propagator
	otel.SetTracerProvider(tm.tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tm.logger.WithFields(logrus.Fields{
		"service":     tm.config.ServiceName,
		"environment": tm.config.Environment,
		"sample_rate": tm.config.SampleRate,
	}).Info("OpenTelemetry tracing initialized")

	return initErr
}

// Shutdown gracefully shuts down the tracing system, flushing any pending spans.
// This method is idempotent and can be called multiple times safely.
//
// The context is used to control the shutdown timeout. If the context is cancelled
// or the configured timeout expires, shutdown may be incomplete.
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm.tracerProvider == nil {
		return nil
	}

	// Determine shutdown timeout
	timeout := time.Duration(tm.config.ShutdownTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := tm.tracerProvider.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}

	// Mark as shutdown to make this method idempotent
	tm.tracerProvider = nil

	tm.logger.WithFields(logrus.Fields{
		"service": tm.config.ServiceName,
	}).Info("OpenTelemetry tracing shutdown completed")
	return nil
}

// GetTracer returns a tracer instance with the given name.
func (tm *TracingManager) GetTracer(name string) oteltrace.Tracer {
	return otel.Tracer(name)
}

// StartSpan starts a new span with the given name and context.
// It uses the default tracer name and optionally sets attributes on the span.
func StartSpan(ctx context.Context, spanName string, attributes ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	tracer := otel.Tracer(defaultTracerName)
	spanCtx, span := tracer.Start(ctx, spanName)

	if len(attributes) > 0 {
		span.SetAttributes(attributes...)
	}

	return spanCtx, span
}

// StartSpanWithTracer starts a new span using a specific tracer.
// This allows using custom tracer names for different components.
func StartSpanWithTracer(ctx context.Context, tracer oteltrace.Tracer, spanName string, attributes ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	spanCtx, span := tracer.Start(ctx, spanName)

	if len(attributes) > 0 {
		span.SetAttributes(attributes...)
	}

	return spanCtx, span
}

// AddSpanAttributes adds attributes to the current span in the context.
// If no span is present or the span is not recording, this is a no-op.
func AddSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.SetAttributes(attributes...)
	}
}

// SetSpanStatus sets the status of the current span in the context.
// If no span is present or the span is not recording, this is a no-op.
func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.SetStatus(code, description)
	}
}

// RecordError records an error on the current span in the context.
// It also sets the span status to Error. If no span is present or
// the span is not recording, this is a no-op.
func RecordError(ctx context.Context, err error, attributes ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.RecordError(err, oteltrace.WithAttributes(attributes...))
		span.SetStatus(codes.Error, err.Error())
	}
}

// GetOtelTraceID returns the OpenTelemetry trace ID from the current context.
// It returns an empty string if no valid trace ID is present or if the trace ID
// is all zeros (which indicates OpenTelemetry is disabled or not initialized).
func GetOtelTraceID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}

	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return ""
	}

	traceID := spanCtx.TraceID().String()
	if traceID == invalidTraceID || traceID == "" {
		return ""
	}

	return traceID
}

// GetOtelSpanID returns the OpenTelemetry span ID from the current context.
// It returns an empty string if no valid span ID is present or if the span ID
// is all zeros (which indicates OpenTelemetry is disabled or not initialized).
func GetOtelSpanID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}

	spanCtx := span.SpanContext()
	if !spanCtx.IsValid() {
		return ""
	}

	spanID := spanCtx.SpanID().String()
	if spanID == invalidSpanID || spanID == "" {
		return ""
	}

	return spanID
}

// WithOtelTracing adds OpenTelemetry tracing to context while preserving legacy tracing.
// It creates a new span and ensures trace/span IDs are available in the context.
//
// If OpenTelemetry is disabled or not initialized, it automatically generates
// legacy trace/span IDs to ensure all requests have unique identifiers.
func WithOtelTracing(ctx context.Context, spanName string) (context.Context, oteltrace.Span) {
	// Start OpenTelemetry span
	spanCtx, span := StartSpan(ctx, spanName)

	// Try to get OpenTelemetry trace/span IDs
	otelTraceID := GetOtelTraceID(spanCtx)
	otelSpanID := GetOtelSpanID(spanCtx)

	// If OpenTelemetry tracing is disabled or not initialized,
	// generate our own legacy trace/span IDs
	if otelTraceID == "" {
		otelTraceID = GenerateTraceID()
	}
	if otelSpanID == "" {
		otelSpanID = GenerateSpanID()
	}

	// Add trace/span IDs to context
	spanCtx = WithTraceID(spanCtx, otelTraceID)
	spanCtx = WithSpanID(spanCtx, otelSpanID)

	return spanCtx, span
}
