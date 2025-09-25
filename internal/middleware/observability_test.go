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

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "X-Forwarded-For single IP",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 172.16.0.1"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "203.0.113.1"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			headers:    map[string]string{"X-Forwarded-For": "192.168.1.100", "X-Real-IP": "203.0.113.1"},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.50:8080",
			expectedIP: "192.168.1.50",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.50",
			expectedIP: "192.168.1.50",
		},
		{
			name:       "Empty RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "",
			expectedIP: "unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = test.remoteAddr

			for key, value := range test.headers {
				req.Header.Set(key, value)
			}

			result := GetClientIP(req)
			if result != test.expectedIP {
				t.Errorf("Expected IP %q, got %q", test.expectedIP, result)
			}
		})
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
