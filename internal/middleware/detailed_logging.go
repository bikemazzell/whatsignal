package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"whatsignal/internal/privacy"
	"whatsignal/internal/service"
	"whatsignal/internal/tracing"

	"github.com/sirupsen/logrus"
)

// DetailedLoggingConfig controls what gets logged
type DetailedLoggingConfig struct {
	LogRequestHeaders  bool     `json:"log_request_headers"`
	LogResponseHeaders bool     `json:"log_response_headers"`
	LogRequestBody     bool     `json:"log_request_body"`
	LogResponseBody    bool     `json:"log_response_body"`
	MaxBodySize        int      `json:"max_body_size"`     // Maximum bytes to log
	SensitiveHeaders   []string `json:"sensitive_headers"` // Headers to mask
	SkipEndpoints      []string `json:"skip_endpoints"`    // Endpoints to skip detailed logging
}

// DefaultDetailedLoggingConfig returns sensible defaults
func DefaultDetailedLoggingConfig() DetailedLoggingConfig {
	return DetailedLoggingConfig{
		LogRequestHeaders:  true,
		LogResponseHeaders: false,
		LogRequestBody:     false, // Off by default for security
		LogResponseBody:    false, // Off by default for performance
		MaxBodySize:        1024,  // 1KB
		SensitiveHeaders: []string{
			"authorization", "x-api-key", "x-webhook-hmac",
			"cookie", "set-cookie", "x-auth-token",
		},
		SkipEndpoints: []string{
			"/metrics", "/health", "/ping",
		},
	}
}

// DetailedLoggingMiddleware provides comprehensive request/response logging for debugging
func DetailedLoggingMiddleware(logger *logrus.Logger, config DetailedLoggingConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip detailed logging for certain endpoints
			for _, skip := range config.SkipEndpoints {
				if strings.Contains(r.URL.Path, skip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Get tracing info for correlation
			requestInfo := tracing.GetRequestInfo(r.Context())

			// Log request details
			logRequestDetails(logger, r, requestInfo, config)

			// Create response capture wrapper if needed
			var responseCapture *responseCaptureWrapper
			var wrappedWriter = w

			if config.LogResponseBody || config.LogResponseHeaders {
				responseCapture = &responseCaptureWrapper{
					ResponseWriter: w,
					body:           bytes.NewBuffer(nil),
					headers:        make(http.Header),
				}
				wrappedWriter = responseCapture
			}

			// Execute the request
			next.ServeHTTP(wrappedWriter, r)

			// Log response details
			if responseCapture != nil {
				logResponseDetails(logger, responseCapture, requestInfo, config)
			}
		})
	}
}

// logRequestDetails logs detailed request information
func logRequestDetails(logger *logrus.Logger, r *http.Request, requestInfo *tracing.RequestInfo, config DetailedLoggingConfig) {
	fields := logrus.Fields{
		service.LogFieldRequestID: requestInfo.RequestID,
		service.LogFieldTraceID:   requestInfo.TraceID,
		service.LogFieldMethod:    r.Method,
		service.LogFieldURL:       r.URL.String(), // Full URL including query params
		service.LogFieldRemoteIP:  GetClientIP(r),
		"content_length":          r.ContentLength,
		"protocol":                r.Proto,
	}

	// Add request headers if configured
	if config.LogRequestHeaders {
		headers := make(map[string]string)
		for name, values := range r.Header {
			// Mask sensitive headers
			if isSensitiveHeader(name, config.SensitiveHeaders) {
				headers[name] = "***MASKED***"
			} else {
				headers[name] = strings.Join(values, ", ")
			}
		}
		fields["request_headers"] = headers
	}

	// Add request body if configured and appropriate
	if config.LogRequestBody && shouldLogBody(r) {
		if r.ContentLength > 0 && r.ContentLength <= int64(config.MaxBodySize) {
			body, err := io.ReadAll(r.Body)
			if err == nil {
				// Restore body for the actual handler
				r.Body = io.NopCloser(bytes.NewReader(body))

				// Mask sensitive data in body before logging
				maskedBody := privacy.MaskSensitiveFields(map[string]interface{}{
					"body": string(body),
				})
				fields["request_body"] = maskedBody["body"]
			}
		}
	}

	logger.WithFields(fields).Debug("Detailed request logging")
}

// logResponseDetails logs detailed response information
func logResponseDetails(logger *logrus.Logger, capture *responseCaptureWrapper, requestInfo *tracing.RequestInfo, config DetailedLoggingConfig) {
	fields := logrus.Fields{
		service.LogFieldRequestID: requestInfo.RequestID,
		service.LogFieldTraceID:   requestInfo.TraceID,
		"status_code":             capture.statusCode,
		"response_size":           capture.body.Len(),
	}

	// Add response headers if configured
	if config.LogResponseHeaders {
		headers := make(map[string]string)
		for name, values := range capture.headers {
			// Mask sensitive headers
			if isSensitiveHeader(name, config.SensitiveHeaders) {
				headers[name] = "***MASKED***"
			} else {
				headers[name] = strings.Join(values, ", ")
			}
		}
		fields["response_headers"] = headers
	}

	// Add response body if configured
	if config.LogResponseBody && capture.body.Len() > 0 {
		bodySize := capture.body.Len()
		if bodySize <= config.MaxBodySize {
			// Mask sensitive data in response body before logging
			maskedBody := privacy.MaskSensitiveFields(map[string]interface{}{
				"body": capture.body.String(),
			})
			fields["response_body"] = maskedBody["body"]
		} else {
			fields["response_body"] = fmt.Sprintf("***TRUNCATED*** (size: %d bytes)", bodySize)
		}
	}

	logger.WithFields(fields).Debug("Detailed response logging")
}

// responseCaptureWrapper captures response data for logging
type responseCaptureWrapper struct {
	http.ResponseWriter
	body       *bytes.Buffer
	headers    http.Header
	statusCode int
}

func (rc *responseCaptureWrapper) Write(data []byte) (int, error) {
	// Write to both the actual response and our capture buffer
	n, err := rc.ResponseWriter.Write(data)
	if err == nil {
		rc.body.Write(data[:n])
	}
	return n, err
}

func (rc *responseCaptureWrapper) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	// Copy headers before they're sent
	for name, values := range rc.ResponseWriter.Header() {
		rc.headers[name] = values
	}
	rc.ResponseWriter.WriteHeader(statusCode)
}

func (rc *responseCaptureWrapper) Header() http.Header {
	return rc.ResponseWriter.Header()
}

// isSensitiveHeader checks if a header should be masked
func isSensitiveHeader(headerName string, sensitiveHeaders []string) bool {
	headerLower := strings.ToLower(headerName)
	for _, sensitive := range sensitiveHeaders {
		if strings.ToLower(sensitive) == headerLower {
			return true
		}
	}
	return false
}

// shouldLogBody determines if we should attempt to log the request body
func shouldLogBody(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")

	// Only log bodies for known text-based content types
	textTypes := []string{
		"application/json",
		"application/xml",
		"text/",
		"application/x-www-form-urlencoded",
	}

	for _, textType := range textTypes {
		if strings.Contains(contentType, textType) {
			return true
		}
	}

	return false
}
