package tracing

import (
	"context"
	"testing"
	"time"
)

func BenchmarkGenerateRequestID(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GenerateRequestID()
	}
}

func BenchmarkGenerateTraceID(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GenerateTraceID()
	}
}

func BenchmarkGenerateSpanID(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GenerateSpanID()
	}
}

func BenchmarkWithFullTracing(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = WithFullTracing(ctx)
	}
}

func BenchmarkGetRequestInfo(b *testing.B) {
	ctx := WithFullTracing(context.Background())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GetRequestInfo(ctx)
	}
}

func BenchmarkDuration(b *testing.B) {
	ctx := WithFullTracing(context.Background())
	// Add a small delay to make the duration calculation meaningful
	time.Sleep(1 * time.Millisecond)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = Duration(ctx)
	}
}

func BenchmarkWithAndGetRequestID(b *testing.B) {
	ctx := context.Background()
	requestID := "test-request-id"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tracedCtx := WithRequestID(ctx, requestID)
		_ = GetRequestID(tracedCtx)
	}
}

func BenchmarkWithAndGetTraceID(b *testing.B) {
	ctx := context.Background()
	traceID := "test-trace-id"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tracedCtx := WithTraceID(ctx, traceID)
		_ = GetTraceID(tracedCtx)
	}
}

func BenchmarkWithAndGetSpanID(b *testing.B) {
	ctx := context.Background()
	spanID := "test-span-id"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tracedCtx := WithSpanID(ctx, spanID)
		_ = GetSpanID(tracedCtx)
	}
}

func BenchmarkWithAndGetStartTime(b *testing.B) {
	ctx := context.Background()
	startTime := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tracedCtx := WithStartTime(ctx, startTime)
		_ = GetStartTime(tracedCtx)
	}
}

func BenchmarkContextChaining(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		ctx = WithRequestID(ctx, GenerateRequestID())
		ctx = WithTraceID(ctx, GenerateTraceID())
		ctx = WithSpanID(ctx, GenerateSpanID())
		ctx = WithStartTime(ctx, time.Now())

		// Extract all values
		_ = GetRequestInfo(ctx)
	}
}

func BenchmarkMultipleFullTracing(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create multiple contexts with full tracing
		ctx1 := WithFullTracing(context.Background())
		ctx2 := WithFullTracing(ctx1)
		ctx3 := WithFullTracing(ctx2)

		// Get info from all contexts
		_ = GetRequestInfo(ctx1)
		_ = GetRequestInfo(ctx2)
		_ = GetRequestInfo(ctx3)
	}
}
