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

// TracingConfig contains OpenTelemetry configuration
type TracingConfig struct {
	ServiceName    string  `json:"service_name"`
	ServiceVersion string  `json:"service_version"`
	Environment    string  `json:"environment"`
	OTLPEndpoint   string  `json:"otlp_endpoint"`
	SampleRate     float64 `json:"sample_rate"`
	Enabled        bool    `json:"enabled"`
	UseStdout      bool    `json:"use_stdout"`
}

// DefaultTracingConfig returns sensible defaults
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		ServiceName:    "whatsignal",
		ServiceVersion: "dev",
		Environment:    "development",
		OTLPEndpoint:   "http://localhost:4318/v1/traces",
		SampleRate:     0.1, // 10% sampling
		Enabled:        false,
		UseStdout:      true,
	}
}

// TracingManager manages OpenTelemetry setup and lifecycle
type TracingManager struct {
	config         TracingConfig
	logger         *logrus.Logger
	tracerProvider *trace.TracerProvider
}

// NewTracingManager creates a new tracing manager
func NewTracingManager(config TracingConfig, logger *logrus.Logger) *TracingManager {
	return &TracingManager{
		config: config,
		logger: logger,
	}
}

// Initialize sets up OpenTelemetry tracing
func (tm *TracingManager) Initialize(ctx context.Context) error {
	if !tm.config.Enabled {
		tm.logger.Info("OpenTelemetry tracing is disabled")
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
		tm.logger.Info("Using stdout trace exporter")
	} else {
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(tm.config.OTLPEndpoint),
			otlptracehttp.WithInsecure(), // Use HTTP instead of HTTPS for local development
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
		}
		tm.logger.WithField("endpoint", tm.config.OTLPEndpoint).Info("Using OTLP HTTP trace exporter")
	}

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
		"sample_rate": tm.config.SampleRate,
	}).Info("OpenTelemetry tracing initialized")

	return nil
}

// Shutdown gracefully shuts down the tracing system
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm.tracerProvider == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := tm.tracerProvider.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}

	tm.logger.Info("OpenTelemetry tracing shutdown completed")
	return nil
}

// GetTracer returns a tracer instance
func (tm *TracingManager) GetTracer(name string) oteltrace.Tracer {
	return otel.Tracer(name)
}

// StartSpan starts a new span with the given name and context
func StartSpan(ctx context.Context, spanName string, attributes ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	tracer := otel.Tracer("whatsignal")
	spanCtx, span := tracer.Start(ctx, spanName)

	if len(attributes) > 0 {
		span.SetAttributes(attributes...)
	}

	return spanCtx, span
}

// StartSpanWithTracer starts a new span using a specific tracer
func StartSpanWithTracer(ctx context.Context, tracer oteltrace.Tracer, spanName string, attributes ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	spanCtx, span := tracer.Start(ctx, spanName)

	if len(attributes) > 0 {
		span.SetAttributes(attributes...)
	}

	return spanCtx, span
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.SetAttributes(attributes...)
	}
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.SetStatus(code, description)
	}
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, attributes ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil && span.IsRecording() {
		span.RecordError(err, oteltrace.WithAttributes(attributes...))
		span.SetStatus(codes.Error, err.Error())
	}
}

// GetTraceID returns the trace ID from the current context
func GetOtelTraceID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the current context
func GetOtelSpanID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// WithOtelTracing adds OpenTelemetry tracing to context while preserving legacy tracing
func WithOtelTracing(ctx context.Context, spanName string) (context.Context, oteltrace.Span) {
	// Start OpenTelemetry span
	spanCtx, span := StartSpan(ctx, spanName)

	// Add OpenTelemetry trace/span IDs to our legacy context
	otelTraceID := GetOtelTraceID(spanCtx)
	otelSpanID := GetOtelSpanID(spanCtx)

	if otelTraceID != "" {
		spanCtx = WithTraceID(spanCtx, otelTraceID)
	}
	if otelSpanID != "" {
		spanCtx = WithSpanID(spanCtx, otelSpanID)
	}

	return spanCtx, span
}
