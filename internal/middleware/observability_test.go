package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"whatsignal/internal/metrics"
	"whatsignal/internal/tracing"

	"github.com/sirupsen/logrus"
)

func TestObservabilityMiddleware(t *testing.T) {
	// Create a test logger
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify tracing information is available
		requestInfo := tracing.GetRequestInfo(r.Context())
		if requestInfo.RequestID == "" {
			t.Error("Expected request ID to be set in context")
		}
		if requestInfo.TraceID == "" {
			t.Error("Expected trace ID to be set in context")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Wrap with observability middleware
	middleware := ObservabilityMiddleware(logger)
	wrappedHandler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.100:12345"

	w := httptest.NewRecorder()

	// Clear metrics before test
	metrics.GetRegistry().GetAllMetrics() // Reset state

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify metrics were recorded
	allMetrics := metrics.GetAllMetrics()
	counters := allMetrics.Counters
	timers := allMetrics.Timers

	// Check for HTTP request metrics
	found := false
	for key := range counters {
		if strings.Contains(key, "http_requests_total") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected http_requests_total metric to be recorded")
	}

	// Check for timing metrics
	found = false
	for key := range timers {
		if strings.Contains(key, "http_request_duration") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected http_request_duration metric to be recorded")
	}

	// Verify logging occurred
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "HTTP request started") {
		t.Error("Expected 'HTTP request started' log message")
	}
	if !strings.Contains(logOutput, "HTTP request completed") {
		t.Error("Expected 'HTTP request completed' log message")
	}
	if !strings.Contains(logOutput, "request_id") {
		t.Error("Expected 'request_id' field in logs")
	}
	if !strings.Contains(logOutput, "trace_id") {
		t.Error("Expected 'trace_id' field in logs")
	}
}

func TestObservabilityMiddleware_ErrorStatus(t *testing.T) {
	// Create test logger
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create test handler that returns error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	// Wrap with observability middleware
	middleware := ObservabilityMiddleware(logger)
	wrappedHandler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/error", nil)
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify error status
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Verify error-level logging
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, `"level":"error"`) {
		t.Error("Expected error level log for 500 status")
	}
}

func TestWebhookObservabilityMiddleware(t *testing.T) {
	// Create test logger
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("webhook processed"))
	})

	// Wrap with webhook observability middleware
	middleware := WebhookObservabilityMiddleware(logger, "whatsapp")
	wrappedHandler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader("test webhook data"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	// Add tracing context
	ctx := tracing.WithFullTracing(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Clear metrics before test
	metrics.GetRegistry().GetAllMetrics() // Reset state

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify webhook-specific metrics
	allMetrics := metrics.GetAllMetrics()
	counters := allMetrics.Counters
	timers := allMetrics.Timers

	// Check for webhook request metrics
	found := false
	for key := range counters {
		if strings.Contains(key, "webhook_requests_total") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected webhook_requests_total metric to be recorded")
	}

	// Check for webhook success metrics
	found = false
	for key := range counters {
		if strings.Contains(key, "webhook_success_total") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected webhook_success_total metric to be recorded")
	}

	// Check for webhook timing metrics
	found = false
	for key := range timers {
		if strings.Contains(key, "webhook_processing_duration") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected webhook_processing_duration metric to be recorded")
	}

	// Verify webhook-specific logging
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "Webhook request started") {
		t.Error("Expected 'Webhook request started' log message")
	}
	if !strings.Contains(logOutput, "Webhook request completed") {
		t.Error("Expected 'Webhook request completed' log message")
	}
	if !strings.Contains(logOutput, `"component":"whatsapp"`) {
		t.Error("Expected webhook component to be logged")
	}
}

func TestWebhookObservabilityMiddleware_Error(t *testing.T) {
	// Create test logger
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create test handler that returns error
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	})

	// Wrap with webhook observability middleware
	middleware := WebhookObservabilityMiddleware(logger, "signal")
	wrappedHandler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/webhook/signal", nil)
	ctx := tracing.WithFullTracing(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify error status
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify webhook error metrics
	allMetrics := metrics.GetAllMetrics()
	counters := allMetrics.Counters

	// Check for webhook error metrics
	found := false
	for key := range counters {
		if strings.Contains(key, "webhook_errors_total") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected webhook_errors_total metric to be recorded")
	}

	// Verify error-level logging
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, `"level":"error"`) {
		t.Error("Expected error level log for webhook error")
	}
}

func TestResponseWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	wrapper := &responseWrapper{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		responseSize:   0,
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusCreated)
	if wrapper.statusCode != http.StatusCreated {
		t.Errorf("Expected status code 201, got %d", wrapper.statusCode)
	}

	// Test Write
	data := []byte("test response data")
	n, err := wrapper.Write(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	if wrapper.responseSize != int64(len(data)) {
		t.Errorf("Expected response size %d, got %d", len(data), wrapper.responseSize)
	}

	// Test multiple writes
	data2 := []byte(" more data")
	_, err = wrapper.Write(data2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectedSize := int64(len(data) + len(data2))
	if wrapper.responseSize != expectedSize {
		t.Errorf("Expected response size %d, got %d", expectedSize, wrapper.responseSize)
	}
}

func TestMiddleware_MetricsAccumulation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{}) // Suppress log output

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := ObservabilityMiddleware(logger)
	wrappedHandler := middleware(testHandler)

	// Clear metrics
	metrics.GetRegistry().GetAllMetrics()

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
	}

	// Verify metrics accumulation
	allMetrics := metrics.GetAllMetrics()
	counters := allMetrics.Counters

	// Find the total requests counter
	var totalRequests float64
	for key, counter := range counters {
		if strings.Contains(key, "http_requests_total") {
			totalRequests += counter.Value
		}
	}

	if totalRequests < 5 {
		t.Errorf("Expected at least 5 total requests, got %f", totalRequests)
	}
}

func TestMiddleware_ConcurrentRequests(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(&bytes.Buffer{}) // Suppress log output

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := ObservabilityMiddleware(logger)
	wrappedHandler := middleware(testHandler)

	// Clear metrics
	metrics.GetRegistry().GetAllMetrics()

	// Make concurrent requests
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify that requests were tracked
	allMetrics := metrics.GetAllMetrics()
	counters := allMetrics.Counters

	var totalRequests float64
	for key, counter := range counters {
		if strings.Contains(key, "http_requests_total") {
			totalRequests += counter.Value
		}
	}

	if totalRequests < 3 {
		t.Errorf("Expected at least 3 total requests, got %f", totalRequests)
	}
}

func TestDetailedLoggingMiddleware_DefaultConfig(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.DebugLevel)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "success"}`))
	})

	config := DefaultDetailedLoggingConfig()
	middleware := DetailedLoggingMiddleware(logger, config)
	wrappedHandler := middleware(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"data": "test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("User-Agent", "test-client")

	ctx := tracing.WithFullTracing(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	logOutput := logBuffer.String()

	// Should log request details
	if !strings.Contains(logOutput, "Detailed request logging") {
		t.Error("Expected detailed request logging message")
	}

	// Should mask authorization header
	if !strings.Contains(logOutput, "***MASKED***") {
		t.Error("Expected authorization header to be masked")
	}

	// Should include request headers (default config)
	if !strings.Contains(logOutput, "request_headers") {
		t.Error("Expected request headers in log")
	}

	// Should NOT include request body (default config)
	if strings.Contains(logOutput, "request_body") {
		t.Error("Should not log request body with default config")
	}
}

func TestDetailedLoggingMiddleware_FullLogging(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.DebugLevel)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 123, "status": "created"}`))
	})

	config := DetailedLoggingConfig{
		LogRequestHeaders:  true,
		LogResponseHeaders: true,
		LogRequestBody:     true,
		LogResponseBody:    true,
		MaxBodySize:        1024,
		SensitiveHeaders:   []string{"authorization", "x-api-key"},
		SkipEndpoints:      []string{},
	}

	middleware := DetailedLoggingMiddleware(logger, config)
	wrappedHandler := middleware(testHandler)

	requestBody := `{"name": "test", "password": "secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret-key")

	ctx := tracing.WithFullTracing(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	logOutput := logBuffer.String()

	// Should log both request and response details
	if !strings.Contains(logOutput, "Detailed request logging") {
		t.Error("Expected detailed request logging message")
	}
	if !strings.Contains(logOutput, "Detailed response logging") {
		t.Error("Expected detailed response logging message")
	}

	// Should mask sensitive headers
	if !strings.Contains(logOutput, "***MASKED***") {
		t.Error("Expected X-API-Key header to be masked")
	}

	// Should include request body (config enabled)
	if !strings.Contains(logOutput, "request_body") {
		t.Error("Expected request body in log")
	}

	// Should include response body (config enabled)
	if !strings.Contains(logOutput, "response_body") {
		t.Error("Expected response body in log")
	}

	// Should include response headers (config enabled)
	if !strings.Contains(logOutput, "response_headers") {
		t.Error("Expected response headers in log")
	}

	// Should include status code
	if !strings.Contains(logOutput, "status_code") || !strings.Contains(logOutput, "201") {
		t.Error("Expected status code 201 in log")
	}
}

func TestDetailedLoggingMiddleware_SkipEndpoints(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.DebugLevel)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	config := DefaultDetailedLoggingConfig()
	middleware := DetailedLoggingMiddleware(logger, config)
	wrappedHandler := middleware(testHandler)

	// Test endpoints that should be skipped
	skipPaths := []string{"/metrics", "/health", "/ping"}

	for _, path := range skipPaths {
		logBuffer.Reset()

		req := httptest.NewRequest(http.MethodGet, path, nil)
		ctx := tracing.WithFullTracing(req.Context())
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for %s, got %d", path, w.Code)
		}

		logOutput := logBuffer.String()
		if strings.Contains(logOutput, "Detailed request logging") {
			t.Errorf("Should not log detailed info for skipped endpoint: %s", path)
		}
	}
}

func TestDetailedLoggingMiddleware_LargeBody(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.DebugLevel)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a large response
		largeResponse := strings.Repeat("x", 2048)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeResponse))
	})

	config := DetailedLoggingConfig{
		LogRequestHeaders:  true,
		LogResponseHeaders: false,
		LogRequestBody:     true,
		LogResponseBody:    true,
		MaxBodySize:        1024, // Smaller than response
		SensitiveHeaders:   []string{},
		SkipEndpoints:      []string{},
	}

	middleware := DetailedLoggingMiddleware(logger, config)
	wrappedHandler := middleware(testHandler)

	// Small request body (should be logged)
	smallBody := strings.Repeat("a", 500)
	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(smallBody))
	req.Header.Set("Content-Type", "application/json")

	ctx := tracing.WithFullTracing(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	logOutput := logBuffer.String()

	// Should include small request body
	if !strings.Contains(logOutput, "request_body") {
		t.Error("Expected small request body to be logged")
	}

	// Should truncate large response body
	if !strings.Contains(logOutput, "***TRUNCATED***") {
		t.Error("Expected large response body to be truncated")
	}
}

func TestDetailedLoggingMiddleware_NonTextBody(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.DebugLevel)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	config := DetailedLoggingConfig{
		LogRequestHeaders:  false,
		LogResponseHeaders: false,
		LogRequestBody:     true,
		LogResponseBody:    false,
		MaxBodySize:        1024,
		SensitiveHeaders:   []string{},
		SkipEndpoints:      []string{},
	}

	middleware := DetailedLoggingMiddleware(logger, config)
	wrappedHandler := middleware(testHandler)

	// Binary content type (should not log body)
	req := httptest.NewRequest(http.MethodPost, "/api/upload", strings.NewReader("binary data"))
	req.Header.Set("Content-Type", "application/octet-stream")

	ctx := tracing.WithFullTracing(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	logOutput := logBuffer.String()

	// Should NOT include request body for binary content
	if strings.Contains(logOutput, "request_body") {
		t.Error("Should not log request body for binary content type")
	}
}

func TestResponseCaptureWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	wrapper := &responseCaptureWrapper{
		ResponseWriter: w,
		body:           bytes.NewBuffer(nil),
		headers:        make(http.Header),
		statusCode:     0,
	}

	// Test Header method
	wrapper.Header().Set("X-Test", "value")
	if wrapper.Header().Get("X-Test") != "value" {
		t.Error("Header method should delegate to underlying ResponseWriter")
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusAccepted)
	if wrapper.statusCode != http.StatusAccepted {
		t.Errorf("Expected status code 202, got %d", wrapper.statusCode)
	}

	// Test Write
	testData := []byte("test response data")
	n, err := wrapper.Write(testData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}
	if wrapper.body.String() != string(testData) {
		t.Errorf("Expected captured body %q, got %q", string(testData), wrapper.body.String())
	}

	// Test multiple writes
	moreData := []byte(" more")
	_, _ = wrapper.Write(moreData)
	expectedBody := string(testData) + string(moreData)
	if wrapper.body.String() != expectedBody {
		t.Errorf("Expected captured body %q, got %q", expectedBody, wrapper.body.String())
	}
}

func TestIsSensitiveHeader(t *testing.T) {
	sensitiveHeaders := []string{"authorization", "x-api-key", "cookie"}

	tests := []struct {
		header   string
		expected bool
	}{
		{"Authorization", true},
		{"AUTHORIZATION", true},
		{"authorization", true},
		{"X-API-Key", true},
		{"x-api-key", true},
		{"Cookie", true},
		{"Content-Type", false},
		{"User-Agent", false},
		{"X-Custom-Header", false},
	}

	for _, test := range tests {
		result := isSensitiveHeader(test.header, sensitiveHeaders)
		if result != test.expected {
			t.Errorf("isSensitiveHeader(%q) = %v, expected %v", test.header, result, test.expected)
		}
	}
}

func TestShouldLogBody(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/json", true},
		{"application/xml", true},
		{"text/plain", true},
		{"text/html", true},
		{"application/x-www-form-urlencoded", true},
		{"application/octet-stream", false},
		{"image/jpeg", false},
		{"video/mp4", false},
		{"", false},
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		if test.contentType != "" {
			req.Header.Set("Content-Type", test.contentType)
		}

		result := shouldLogBody(req)
		if result != test.expected {
			t.Errorf("shouldLogBody for content-type %q = %v, expected %v", test.contentType, result, test.expected)
		}
	}
}

func TestDetailedLoggingConfig_Defaults(t *testing.T) {
	config := DefaultDetailedLoggingConfig()

	if !config.LogRequestHeaders {
		t.Error("Expected LogRequestHeaders to be true by default")
	}
	if config.LogResponseHeaders {
		t.Error("Expected LogResponseHeaders to be false by default")
	}
	if config.LogRequestBody {
		t.Error("Expected LogRequestBody to be false by default")
	}
	if config.LogResponseBody {
		t.Error("Expected LogResponseBody to be false by default")
	}
	if config.MaxBodySize != 1024 {
		t.Errorf("Expected MaxBodySize to be 1024, got %d", config.MaxBodySize)
	}

	expectedSensitive := []string{"authorization", "x-api-key", "x-webhook-hmac", "cookie", "set-cookie", "x-auth-token"}
	if len(config.SensitiveHeaders) != len(expectedSensitive) {
		t.Errorf("Expected %d sensitive headers, got %d", len(expectedSensitive), len(config.SensitiveHeaders))
	}

	expectedSkip := []string{"/metrics", "/health", "/ping"}
	if len(config.SkipEndpoints) != len(expectedSkip) {
		t.Errorf("Expected %d skip endpoints, got %d", len(expectedSkip), len(config.SkipEndpoints))
	}
}

// TestObservabilityMiddleware_TraceIDNotAllZeros verifies that trace IDs
// are generated correctly and not all zeros when OpenTelemetry is disabled
func TestObservabilityMiddleware_TraceIDNotAllZeros(t *testing.T) {
	// Create a test logger with JSON formatter for easier parsing
	var logBuffer bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify tracing information is available and valid
		requestInfo := tracing.GetRequestInfo(r.Context())

		if requestInfo.RequestID == "" {
			t.Error("Expected request ID to be set in context")
		}
		if requestInfo.TraceID == "" {
			t.Error("Expected trace ID to be set in context")
		}

		// Verify trace ID is not all zeros
		if requestInfo.TraceID == "00000000000000000000000000000000" {
			t.Error("Trace ID should not be all zeros")
		}

		// Verify span ID is not all zeros
		if requestInfo.SpanID == "0000000000000000" {
			t.Error("Span ID should not be all zeros")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	// Wrap with observability middleware (OpenTelemetry is not initialized)
	middleware := ObservabilityMiddleware(logger)
	wrappedHandler := middleware(testHandler)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify logging occurred with valid trace IDs
	logOutput := logBuffer.String()

	// Check that trace_id field exists in logs
	if !strings.Contains(logOutput, "trace_id") {
		t.Error("Expected 'trace_id' field in logs")
	}

	// Check that trace_id is not all zeros
	if strings.Contains(logOutput, `"trace_id":"00000000000000000000000000000000"`) {
		t.Error("Trace ID in logs should not be all zeros")
	}

	// Log output for debugging
	t.Logf("Log output:\n%s", logOutput)
}
