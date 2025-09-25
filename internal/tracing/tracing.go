package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ContextKey represents keys used for context values
type ContextKey string

const (
	// RequestIDKey is the context key for request IDs
	RequestIDKey ContextKey = "request_id"
	// TraceIDKey is the context key for trace IDs
	TraceIDKey ContextKey = "trace_id"
	// SpanIDKey is the context key for span IDs
	SpanIDKey ContextKey = "span_id"
	// StartTimeKey is the context key for request start time
	StartTimeKey ContextKey = "start_time"
)

// RequestInfo contains tracing information for a request
type RequestInfo struct {
	RequestID string    `json:"request_id"`
	TraceID   string    `json:"trace_id"`
	SpanID    string    `json:"span_id"`
	StartTime time.Time `json:"start_time"`
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto rand fails
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("req_%s", hex.EncodeToString(bytes))
}

// GenerateTraceID generates a unique trace ID
func GenerateTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto rand fails
		return fmt.Sprintf("trace_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// GenerateSpanID generates a unique span ID
func GenerateSpanID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto rand fails
		return fmt.Sprintf("span_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithSpanID adds a span ID to the context
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, SpanIDKey, spanID)
}

// WithStartTime adds a start time to the context
func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, StartTimeKey, startTime)
}

// GetRequestID extracts the request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetTraceID extracts the trace ID from context
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// GetSpanID extracts the span ID from context
func GetSpanID(ctx context.Context) string {
	if spanID, ok := ctx.Value(SpanIDKey).(string); ok {
		return spanID
	}
	return ""
}

// GetStartTime extracts the start time from context
func GetStartTime(ctx context.Context) time.Time {
	if startTime, ok := ctx.Value(StartTimeKey).(time.Time); ok {
		return startTime
	}
	return time.Time{}
}

// GetRequestInfo extracts all tracing information from context
func GetRequestInfo(ctx context.Context) *RequestInfo {
	return &RequestInfo{
		RequestID: GetRequestID(ctx),
		TraceID:   GetTraceID(ctx),
		SpanID:    GetSpanID(ctx),
		StartTime: GetStartTime(ctx),
	}
}

// WithFullTracing adds complete tracing information to context
func WithFullTracing(ctx context.Context) context.Context {
	requestID := GenerateRequestID()
	traceID := GenerateTraceID()
	spanID := GenerateSpanID()
	startTime := time.Now()

	ctx = WithRequestID(ctx, requestID)
	ctx = WithTraceID(ctx, traceID)
	ctx = WithSpanID(ctx, spanID)
	ctx = WithStartTime(ctx, startTime)

	return ctx
}

// Duration calculates the duration since the start time in context
func Duration(ctx context.Context) time.Duration {
	startTime := GetStartTime(ctx)
	if startTime.IsZero() {
		return 0
	}
	return time.Since(startTime)
}
