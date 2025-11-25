package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"whatsignal/internal/httputil"
	"whatsignal/internal/metrics"
	"whatsignal/internal/privacy"
	"whatsignal/internal/service"
	"whatsignal/internal/tracing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ObservabilityMiddleware adds metrics collection and tracing to HTTP requests
func ObservabilityMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add tracing information to request context (legacy + OpenTelemetry)
			ctx, span := tracing.WithOtelTracing(r.Context(), "http_request")
			defer span.End()

			// Generate and add request ID for legacy tracing
			requestID := tracing.GenerateRequestID()
			ctx = tracing.WithRequestID(ctx, requestID)
			ctx = tracing.WithStartTime(ctx, time.Now())

			r = r.WithContext(ctx)

			// Add HTTP-specific OpenTelemetry attributes
			tracing.AddSpanAttributes(ctx,
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.route", r.URL.Path),
				attribute.String("user_agent.original", r.Header.Get("User-Agent")),
				attribute.String("client.address", httputil.GetClientIP(r)),
			)

			// Get tracing info for logging
			requestInfo := tracing.GetRequestInfo(ctx)

			// Create a response wrapper to capture status code and response size
			wrapper := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				responseSize:   0,
			}

			// Log request start with tracing fields
			logger.WithFields(logrus.Fields{
				service.LogFieldRequestID: requestInfo.RequestID,
				service.LogFieldTraceID:   requestInfo.TraceID,
				service.LogFieldMethod:    r.Method,
				service.LogFieldURL:       r.URL.Path,
				service.LogFieldRemoteIP:  httputil.GetClientIP(r),
				service.LogFieldUserAgent: r.Header.Get("User-Agent"),
				"content_length":          r.ContentLength,
			}).Info("HTTP request started")

			// Record request metrics
			metrics.IncrementCounter("http_requests_total", map[string]string{
				"method":   r.Method,
				"endpoint": r.URL.Path,
			}, "Total HTTP requests")

			// Track concurrent requests
			metrics.IncrementCounter("http_requests_active", nil, "Currently active HTTP requests")
			defer func() {
				metrics.AddToCounter("http_requests_active", -1, nil, "Currently active HTTP requests")
			}()

			// Process request
			next.ServeHTTP(wrapper, r)

			// Calculate request duration
			duration := tracing.Duration(ctx)

			// Add final OpenTelemetry attributes
			tracing.AddSpanAttributes(ctx,
				attribute.Int("http.response.status_code", wrapper.statusCode),
				attribute.Int64("http.response.size", wrapper.responseSize),
				attribute.Int64("http.request.duration_ms", duration.Milliseconds()),
			)

			// Set OpenTelemetry span status based on HTTP status
			if wrapper.statusCode >= 400 {
				tracing.SetSpanStatus(ctx, codes.Error, fmt.Sprintf("HTTP %d", wrapper.statusCode))
			} else {
				tracing.SetSpanStatus(ctx, codes.Ok, "")
			}

			// Record timing metrics
			metrics.RecordTimer("http_request_duration", duration, map[string]string{
				"method":      r.Method,
				"endpoint":    r.URL.Path,
				"status_code": strconv.Itoa(wrapper.statusCode),
			}, "HTTP request duration")

			// Record status code metrics
			metrics.IncrementCounter("http_responses_total", map[string]string{
				"method":      r.Method,
				"endpoint":    r.URL.Path,
				"status_code": strconv.Itoa(wrapper.statusCode),
			}, "HTTP responses by status code")

			// Record response size metrics
			if wrapper.responseSize > 0 {
				metrics.RecordTimer("http_response_size", time.Duration(wrapper.responseSize)*time.Nanosecond, map[string]string{
					"method":   r.Method,
					"endpoint": r.URL.Path,
				}, "HTTP response size in bytes")
			}

			// Determine log level based on status code
			logLevel := logrus.InfoLevel
			if wrapper.statusCode >= 400 && wrapper.statusCode < 500 {
				logLevel = logrus.WarnLevel
			} else if wrapper.statusCode >= 500 {
				logLevel = logrus.ErrorLevel
			}

			// Log request completion with metrics
			logger.WithFields(logrus.Fields{
				service.LogFieldRequestID:  requestInfo.RequestID,
				service.LogFieldTraceID:    requestInfo.TraceID,
				service.LogFieldMethod:     r.Method,
				service.LogFieldURL:        r.URL.Path,
				service.LogFieldStatusCode: wrapper.statusCode,
				service.LogFieldDuration:   duration.Milliseconds(),
				service.LogFieldRemoteIP:   httputil.GetClientIP(r),
				service.LogFieldSize:       wrapper.responseSize,
			}).Log(logLevel, "HTTP request completed")
		})
	}
}

// WebhookObservabilityMiddleware adds specific observability for webhook endpoints
func WebhookObservabilityMiddleware(logger *logrus.Logger, webhookType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Start OpenTelemetry span for webhook
			ctx, span := tracing.WithOtelTracing(r.Context(), "webhook_request")
			defer span.End()
			r = r.WithContext(ctx)

			// Add webhook-specific OpenTelemetry attributes
			tracing.AddSpanAttributes(ctx,
				attribute.String("webhook.type", webhookType),
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("client.address", httputil.GetClientIP(r)),
				attribute.String("http.request.header.content-type", r.Header.Get("Content-Type")),
				attribute.Int64("http.request.content_length", r.ContentLength),
			)

			// Increment webhook-specific metrics
			metrics.IncrementCounter("webhook_requests_total", map[string]string{
				"type": webhookType,
			}, "Total webhook requests by type")

			// Get tracing info from context
			requestInfo := tracing.GetRequestInfo(r.Context())

			// Create response wrapper
			wrapper := &responseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				responseSize:   0,
			}

			// Log webhook start with privacy-aware fields
			logFields := privacy.MaskSensitiveFields(map[string]interface{}{
				service.LogFieldRequestID: requestInfo.RequestID,
				service.LogFieldTraceID:   requestInfo.TraceID,
				service.LogFieldService:   "webhook",
				service.LogFieldComponent: webhookType,
				service.LogFieldRemoteIP:  httputil.GetClientIP(r),
				"content_type":            r.Header.Get("Content-Type"),
				"content_length":          r.ContentLength,
			})

			logrusFields := make(logrus.Fields)
			for k, v := range logFields {
				logrusFields[k] = v
			}

			logger.WithFields(logrusFields).Info("Webhook request started")

			// Process the webhook
			next.ServeHTTP(wrapper, r)

			// Calculate processing time
			processingTime := time.Since(startTime)

			// Add final OpenTelemetry attributes for webhook
			tracing.AddSpanAttributes(ctx,
				attribute.Int("http.response.status_code", wrapper.statusCode),
				attribute.Int64("http.response.size", wrapper.responseSize),
				attribute.Int64("webhook.processing_duration_ms", processingTime.Milliseconds()),
			)

			// Set OpenTelemetry span status for webhook
			if wrapper.statusCode >= 400 {
				tracing.SetSpanStatus(ctx, codes.Error, fmt.Sprintf("Webhook failed with HTTP %d", wrapper.statusCode))
			} else {
				tracing.SetSpanStatus(ctx, codes.Ok, "Webhook processed successfully")
			}

			// Record webhook timing
			metrics.RecordTimer("webhook_processing_duration", processingTime, map[string]string{
				"type":        webhookType,
				"status_code": strconv.Itoa(wrapper.statusCode),
			}, "Webhook processing duration")

			// Record webhook status metrics
			if wrapper.statusCode >= 400 {
				metrics.IncrementCounter("webhook_errors_total", map[string]string{
					"type":        webhookType,
					"status_code": strconv.Itoa(wrapper.statusCode),
				}, "Webhook processing errors")
			} else {
				metrics.IncrementCounter("webhook_success_total", map[string]string{
					"type": webhookType,
				}, "Successful webhook processing")
			}

			// Log webhook completion
			logLevel := logrus.InfoLevel
			if wrapper.statusCode >= 400 {
				logLevel = logrus.ErrorLevel
			}

			completionFields := privacy.MaskSensitiveFields(map[string]interface{}{
				service.LogFieldRequestID:  requestInfo.RequestID,
				service.LogFieldTraceID:    requestInfo.TraceID,
				service.LogFieldService:    "webhook",
				service.LogFieldComponent:  webhookType,
				service.LogFieldStatusCode: wrapper.statusCode,
				service.LogFieldDuration:   processingTime.Milliseconds(),
				service.LogFieldSize:       wrapper.responseSize,
			})

			completionLogrusFields := make(logrus.Fields)
			for k, v := range completionFields {
				completionLogrusFields[k] = v
			}

			logger.WithFields(completionLogrusFields).Log(logLevel, "Webhook request completed")
		})
	}
}

// responseWrapper captures response metrics
type responseWrapper struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

func (rw *responseWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWrapper) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.responseSize += int64(n)
	return n, err
}
