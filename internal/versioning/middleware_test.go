package versioning

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionMiddleware_ExtractVersionFromRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Silence logs during tests
	vm := NewVersionMiddleware(logger)

	tests := []struct {
		name            string
		setupRequest    func(*http.Request)
		expectedVersion APIVersion
	}{
		{
			name: "Accept-Version header",
			setupRequest: func(r *http.Request) {
				r.Header.Set(AcceptVersionHeader, "1.1.0")
			},
			expectedVersion: V1_1_0,
		},
		{
			name: "X-API-Version header",
			setupRequest: func(r *http.Request) {
				r.Header.Set(APIVersionHeader, "1.0.0")
			},
			expectedVersion: V1_0_0,
		},
		{
			name: "Accept-Version takes precedence",
			setupRequest: func(r *http.Request) {
				r.Header.Set(AcceptVersionHeader, "1.2.0")
				r.Header.Set(APIVersionHeader, "1.0.0")
			},
			expectedVersion: V1_2_0,
		},
		{
			name: "URL path version v1",
			setupRequest: func(r *http.Request) {
				r.URL.Path = "/v1/test"
			},
			expectedVersion: V1_0_0,
		},
		{
			name: "URL path version with API prefix",
			setupRequest: func(r *http.Request) {
				r.URL.Path = "/api/v1/test"
			},
			expectedVersion: V1_0_0,
		},
		{
			name: "URL path version v1.2",
			setupRequest: func(r *http.Request) {
				r.URL.Path = "/v1.2/test"
			},
			expectedVersion: V1_2_0,
		},
		{
			name: "no version specified",
			setupRequest: func(r *http.Request) {
				// No version headers or path
			},
			expectedVersion: CurrentVersion,
		},
		{
			name: "invalid version header",
			setupRequest: func(r *http.Request) {
				r.Header.Set(AcceptVersionHeader, "invalid")
			},
			expectedVersion: CurrentVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupRequest(req)

			version := vm.extractVersionFromRequest(req)
			assert.Equal(t, tt.expectedVersion, version)
		})
	}
}

func TestVersionMiddleware_VersionHandler_Compatible(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	vm := NewVersionMiddleware(logger)

	// Create a test handler that checks context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version, ok := GetVersionFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, V1_1_0, version)

		info, ok := GetVersionInfoFromContext(r.Context())
		assert.True(t, ok)
		assert.True(t, info.Compatible)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Create middleware chain
	handler := vm.VersionHandler(testHandler)

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(AcceptVersionHeader, "1.1.0")
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())

	// Check version headers
	assert.Equal(t, CurrentVersion.String(), w.Header().Get(CurrentVersionHeader))
	assert.Equal(t, GetVersionRange(), w.Header().Get(SupportedVersionsHeader))
}

func TestVersionMiddleware_VersionHandler_Incompatible_Old(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	vm := NewVersionMiddleware(logger)

	// Create test handler (should not be reached)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be reached for incompatible version")
	})

	handler := vm.VersionHandler(testHandler)

	// Create request with unsupported old version
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(AcceptVersionHeader, "0.9.0")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 426 Upgrade Required for unsupported old version
	assert.Equal(t, http.StatusUpgradeRequired, w.Code)

	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	errorInfo := response["error"].(map[string]interface{})
	assert.Equal(t, "VERSION_INCOMPATIBLE", errorInfo["code"])
}

func TestVersionMiddleware_VersionHandler_Incompatible_Future(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	vm := NewVersionMiddleware(logger)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be reached for incompatible version")
	})

	handler := vm.VersionHandler(testHandler)

	// Create request with future version
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(AcceptVersionHeader, "3.0.0")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 501 Not Implemented for future version
	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	errorInfo := response["error"].(map[string]interface{})
	assert.Equal(t, "VERSION_INCOMPATIBLE", errorInfo["code"])
}

func TestVersionMiddleware_ExtractVersionFromPath(t *testing.T) {
	logger := logrus.New()
	vm := NewVersionMiddleware(logger)

	tests := []struct {
		name     string
		path     string
		expected APIVersion
	}{
		{
			name:     "simple v1",
			path:     "/v1/test",
			expected: V1_0_0,
		},
		{
			name:     "api prefix v1",
			path:     "/api/v1/test",
			expected: V1_0_0,
		},
		{
			name:     "v1.2 format",
			path:     "/v1.2/test",
			expected: V1_2_0,
		},
		{
			name:     "v1.2.3 format",
			path:     "/v1.2.3/test",
			expected: APIVersion{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:     "no version in path",
			path:     "/test/endpoint",
			expected: APIVersion{}, // Zero value
		},
		{
			name:     "invalid version",
			path:     "/vX/test",
			expected: APIVersion{}, // Zero value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vm.extractVersionFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVersionFromContext(t *testing.T) {
	// Test with version in context
	ctx := context.WithValue(context.Background(), VersionContextKey, V1_1_0)
	version, ok := GetVersionFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, V1_1_0, version)

	// Test with no version in context
	ctx = context.Background()
	version, ok = GetVersionFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, APIVersion{}, version)
}

func TestGetVersionInfoFromContext(t *testing.T) {
	compat := VersionCompatibility{
		Requested:  V1_1_0,
		Current:    CurrentVersion,
		Compatible: true,
	}

	// Test with info in context
	ctx := context.WithValue(context.Background(), InfoContextKey, compat)
	info, ok := GetVersionInfoFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, compat, info)

	// Test with no info in context
	ctx = context.Background()
	info, ok = GetVersionInfoFromContext(ctx)
	assert.False(t, ok)
	assert.Equal(t, VersionCompatibility{}, info)
}

func TestVersionAwareHandler(t *testing.T) {
	vh := NewVersionAwareHandler()

	// Add handlers for different versions
	v1Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("v1 handler"))
	})
	v2Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("v2 handler"))
	})
	fallbackHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fallback handler"))
	})

	vh.AddVersionHandler(V1_0_0, v1Handler)
	vh.AddVersionHandler(V1_1_0, v1Handler) // Use same handler for 1.x
	vh.AddVersionHandler(V2_0_0, v2Handler)
	vh.SetFallbackHandler(fallbackHandler)

	tests := []struct {
		name           string
		contextVersion APIVersion
		expectedBody   string
	}{
		{
			name:           "exact version match v1.0.0",
			contextVersion: V1_0_0,
			expectedBody:   "v1 handler",
		},
		{
			name:           "exact version match v1.1.0",
			contextVersion: V1_1_0,
			expectedBody:   "v1 handler",
		},
		{
			name:           "exact version match v2.0.0",
			contextVersion: V2_0_0,
			expectedBody:   "v2 handler",
		},
		{
			name:           "compatible version (v1.0.5 -> v1.0.0)",
			contextVersion: APIVersion{Major: 1, Minor: 0, Patch: 5},
			expectedBody:   "v1 handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			ctx := context.WithValue(req.Context(), VersionContextKey, tt.contextVersion)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			vh.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestVersionAwareHandler_NoContext(t *testing.T) {
	vh := NewVersionAwareHandler()
	fallbackHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fallback"))
	})
	vh.SetFallbackHandler(fallbackHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	vh.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "fallback", w.Body.String())
}

func TestVersionAwareHandler_NoHandler(t *testing.T) {
	vh := NewVersionAwareHandler()
	// No fallback handler set

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), VersionContextKey, V1_0_0)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	vh.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestFeatureGate(t *testing.T) {
	tests := []struct {
		name     string
		version  APIVersion
		feature  string
		expected bool
	}{
		{
			name:     "supported feature",
			version:  V1_1_0,
			feature:  "rate_limiting",
			expected: true,
		},
		{
			name:     "unsupported feature (too old)",
			version:  V1_0_0,
			feature:  "rate_limiting",
			expected: false,
		},
		{
			name:     "nonexistent feature",
			version:  V1_2_0,
			feature:  "nonexistent_feature",
			expected: false,
		},
		{
			name:     "no version in context",
			version:  APIVersion{}, // Will not be added to context
			feature:  "rate_limiting",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.version.Major > 0 {
				ctx = context.WithValue(ctx, VersionContextKey, tt.version)
			}

			result := FeatureGate(ctx, tt.feature)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequireFeature(t *testing.T) {
	middleware := RequireFeature("rate_limiting")

	// Handler that should be reached if feature is available
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("feature available"))
	})

	handler := middleware(testHandler)

	tests := []struct {
		name           string
		version        APIVersion
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "feature available",
			version:        V1_1_0,
			expectedStatus: http.StatusOK,
			expectedBody:   "feature available",
		},
		{
			name:           "feature not available",
			version:        V1_0_0,
			expectedStatus: http.StatusNotImplemented,
			expectedBody:   "", // Will be JSON error response
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			ctx := context.WithValue(req.Context(), VersionContextKey, tt.version)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			} else {
				// Check that it's a JSON error response
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				errorInfo := response["error"].(map[string]interface{})
				assert.Equal(t, "FEATURE_NOT_AVAILABLE", errorInfo["code"])
				assert.Equal(t, "rate_limiting", errorInfo["feature"])
			}
		})
	}
}

func TestNewVersionMiddleware(t *testing.T) {
	logger := logrus.New()
	vm := NewVersionMiddleware(logger)

	assert.NotNil(t, vm)
	assert.Equal(t, logger, vm.logger)
}
