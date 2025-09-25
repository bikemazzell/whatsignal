package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"whatsignal/internal/metrics"
	"whatsignal/internal/privacy"
	"whatsignal/internal/service"
	"whatsignal/internal/tracing"

	"github.com/sirupsen/logrus"
)

// ObservabilityMiddleware adds metrics collection and tracing to HTTP requests
func ObservabilityMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add tracing information to request context
			ctx := tracing.WithFullTracing(r.Context())
			r = r.WithContext(ctx)

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
				service.LogFieldRemoteIP:  GetClientIP(r),
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
				service.LogFieldRemoteIP:   GetClientIP(r),
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
				service.LogFieldRemoteIP:  GetClientIP(r),
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

// GetClientIP extracts the client IP address from the request
// This function would normally be imported from the main package, but we'll define it here for completeness
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (from load balancers/proxies)
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(xForwardedFor, ","); idx != -1 {
			return strings.TrimSpace(xForwardedFor[:idx])
		}
		return strings.TrimSpace(xForwardedFor)
	}

	// Check X-Real-IP header (from some proxies)
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// Fall back to RemoteAddr (direct connection)
	if remoteAddr := r.RemoteAddr; remoteAddr != "" {
		// RemoteAddr includes port, strip it
		if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
			return remoteAddr[:idx]
		}
		return remoteAddr
	}

	return "unknown"
}
