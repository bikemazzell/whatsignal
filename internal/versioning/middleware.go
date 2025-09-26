package versioning

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// VersionContext key for storing version information in request context
type contextKey string

const (
	VersionContextKey contextKey = "api_version"
	InfoContextKey    contextKey = "version_info"
)

// VersionHeaders contains standard version-related HTTP headers
const (
	// Request headers
	AcceptVersionHeader = "Accept-Version" // Client specifies desired version
	APIVersionHeader    = "X-API-Version"  // Alternative version specification

	// Response headers
	CurrentVersionHeader     = "X-Current-Version"     // Server's current version
	SupportedVersionsHeader  = "X-Supported-Versions"  // Range of supported versions
	DeprecationWarningHeader = "X-Deprecation-Warning" // Deprecation warnings
	VersionInfoHeader        = "X-Version-Info"        // Detailed version information
)

// VersionMiddleware handles API versioning for HTTP requests
type VersionMiddleware struct {
	logger *logrus.Logger
}

// NewVersionMiddleware creates a new version middleware
func NewVersionMiddleware(logger *logrus.Logger) *VersionMiddleware {
	return &VersionMiddleware{
		logger: logger,
	}
}

// VersionHandler is the middleware function
func (vm *VersionMiddleware) VersionHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract requested version from headers
		requestedVersion := vm.extractVersionFromRequest(r)

		// Check compatibility
		compatibility := CheckCompatibility(requestedVersion)

		// Set response headers
		vm.setVersionHeaders(w, compatibility)

		// Handle incompatible versions
		if !compatibility.Compatible {
			vm.handleIncompatibleVersion(w, r, compatibility)
			return
		}

		// Add version information to context
		ctx := vm.addVersionToContext(r.Context(), requestedVersion, compatibility)

		// Log version information
		vm.logVersionInfo(r, requestedVersion, compatibility)

		// Continue with request processing
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractVersionFromRequest extracts the API version from request headers
func (vm *VersionMiddleware) extractVersionFromRequest(r *http.Request) APIVersion {
	// Check Accept-Version header first (preferred)
	if versionStr := r.Header.Get(AcceptVersionHeader); versionStr != "" {
		if version, err := ParseVersion(versionStr); err == nil {
			return version
		}
		vm.logger.WithField("version_string", versionStr).Warn("Invalid version in Accept-Version header")
	}

	// Check X-API-Version header as fallback
	if versionStr := r.Header.Get(APIVersionHeader); versionStr != "" {
		if version, err := ParseVersion(versionStr); err == nil {
			return version
		}
		vm.logger.WithField("version_string", versionStr).Warn("Invalid version in X-API-Version header")
	}

	// Check URL path for version (e.g., /v1/endpoint, /api/v2/endpoint)
	if version := vm.extractVersionFromPath(r.URL.Path); version.Major > 0 {
		return version
	}

	// Default to current version if no version specified
	return CurrentVersion
}

// extractVersionFromPath extracts version from URL path
func (vm *VersionMiddleware) extractVersionFromPath(path string) APIVersion {
	// Common patterns: /v1/, /api/v1/, /v1.2/, etc.
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, "v") && len(part) > 1 {
			versionStr := strings.TrimPrefix(part, "v")

			// Handle simple version like "v1" -> "1.0.0"
			if !strings.Contains(versionStr, ".") {
				versionStr += ".0.0"
			} else if strings.Count(versionStr, ".") == 1 {
				versionStr += ".0"
			}

			if version, err := ParseVersion(versionStr); err == nil {
				return version
			}
		}
	}

	return APIVersion{} // Zero value indicates no version found
}

// setVersionHeaders sets appropriate response headers
func (vm *VersionMiddleware) setVersionHeaders(w http.ResponseWriter, compatibility VersionCompatibility) {
	// Always set current version
	w.Header().Set(CurrentVersionHeader, CurrentVersion.String())

	// Set supported version range
	w.Header().Set(SupportedVersionsHeader, GetVersionRange())

	// Add deprecation warnings if any
	if len(compatibility.DeprecatedFeatures) > 0 {
		warnings := make([]string, len(compatibility.DeprecatedFeatures))
		for i, feature := range compatibility.DeprecatedFeatures {
			warnings[i] = feature.Name
		}
		w.Header().Set(DeprecationWarningHeader, strings.Join(warnings, ", "))
	}

	// Add version info as JSON header (for debugging)
	if infoJSON, err := json.Marshal(DefaultVersionInfo()); err == nil {
		w.Header().Set(VersionInfoHeader, string(infoJSON))
	}
}

// handleIncompatibleVersion handles requests with incompatible versions
func (vm *VersionMiddleware) handleIncompatibleVersion(w http.ResponseWriter, r *http.Request, compatibility VersionCompatibility) {
	w.Header().Set("Content-Type", "application/json")

	// Set appropriate status code
	statusCode := http.StatusBadRequest
	if len(compatibility.Errors) > 0 {
		// Check if it's a "version too old" or "version too new" error
		for _, err := range compatibility.Errors {
			if strings.Contains(err, "no longer supported") {
				statusCode = http.StatusUpgradeRequired // 426
				break
			} else if strings.Contains(err, "not yet available") {
				statusCode = http.StatusNotImplemented // 501
				break
			}
		}
	}

	w.WriteHeader(statusCode)

	// Create error response
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "VERSION_INCOMPATIBLE",
			"message": "API version incompatible",
			"details": compatibility,
		},
		"version_info": DefaultVersionInfo(),
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		vm.logger.WithError(err).Error("Failed to encode version error response")
	}

	// Log the incompatible version request
	vm.logger.WithFields(logrus.Fields{
		"requested_version": compatibility.Requested.String(),
		"current_version":   compatibility.Current.String(),
		"compatible":        compatibility.Compatible,
		"errors":            compatibility.Errors,
		"remote_addr":       r.RemoteAddr,
		"user_agent":        r.UserAgent(),
		"path":              r.URL.Path,
	}).Warn("Incompatible API version requested")
}

// addVersionToContext adds version information to the request context
func (vm *VersionMiddleware) addVersionToContext(ctx context.Context, version APIVersion, compatibility VersionCompatibility) context.Context {
	ctx = context.WithValue(ctx, VersionContextKey, version)
	ctx = context.WithValue(ctx, InfoContextKey, compatibility)
	return ctx
}

// logVersionInfo logs version information for debugging
func (vm *VersionMiddleware) logVersionInfo(r *http.Request, version APIVersion, compatibility VersionCompatibility) {
	logFields := logrus.Fields{
		"api_version":     version.String(),
		"current_version": CurrentVersion.String(),
		"compatible":      compatibility.Compatible,
		"path":            r.URL.Path,
		"method":          r.Method,
	}

	if len(compatibility.Warnings) > 0 {
		logFields["warnings"] = compatibility.Warnings
	}

	if len(compatibility.DeprecatedFeatures) > 0 {
		deprecated := make([]string, len(compatibility.DeprecatedFeatures))
		for i, feature := range compatibility.DeprecatedFeatures {
			deprecated[i] = feature.Name
		}
		logFields["deprecated_features"] = deprecated
	}

	vm.logger.WithFields(logFields).Debug("API version information")
}

// Helper functions for extracting version from context

// GetVersionFromContext extracts the API version from request context
func GetVersionFromContext(ctx context.Context) (APIVersion, bool) {
	version, ok := ctx.Value(VersionContextKey).(APIVersion)
	return version, ok
}

// GetVersionInfoFromContext extracts version compatibility info from request context
func GetVersionInfoFromContext(ctx context.Context) (VersionCompatibility, bool) {
	info, ok := ctx.Value(InfoContextKey).(VersionCompatibility)
	return info, ok
}

// MustGetVersionFromContext extracts version from context, panicking if not found
func MustGetVersionFromContext(ctx context.Context) APIVersion {
	if version, ok := GetVersionFromContext(ctx); ok {
		return version
	}
	panic("API version not found in context")
}

// VersionAwareHandler wraps a handler with version-specific logic
type VersionAwareHandler struct {
	handlers map[string]http.HandlerFunc // version string -> handler
	fallback http.HandlerFunc            // fallback handler
}

// NewVersionAwareHandler creates a new version-aware handler
func NewVersionAwareHandler() *VersionAwareHandler {
	return &VersionAwareHandler{
		handlers: make(map[string]http.HandlerFunc),
	}
}

// AddVersionHandler adds a handler for a specific version
func (vh *VersionAwareHandler) AddVersionHandler(version APIVersion, handler http.HandlerFunc) {
	vh.handlers[version.String()] = handler
}

// SetFallbackHandler sets the fallback handler for unspecified versions
func (vh *VersionAwareHandler) SetFallbackHandler(handler http.HandlerFunc) {
	vh.fallback = handler
}

// ServeHTTP implements http.Handler interface
func (vh *VersionAwareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	version, ok := GetVersionFromContext(r.Context())
	if !ok {
		// No version in context, use fallback
		if vh.fallback != nil {
			vh.fallback(w, r)
		} else {
			http.Error(w, "Version information not available", http.StatusInternalServerError)
		}
		return
	}

	// Try exact version match first
	if handler, exists := vh.handlers[version.String()]; exists {
		handler(w, r)
		return
	}

	// Try to find compatible handler (same major version, highest minor)
	var bestMatch APIVersion
	var bestHandler http.HandlerFunc

	for versionStr, handler := range vh.handlers {
		if handlerVersion, err := ParseVersion(versionStr); err == nil {
			if handlerVersion.Major == version.Major &&
				handlerVersion.Compare(version) <= 0 &&
				handlerVersion.Compare(bestMatch) > 0 {
				bestMatch = handlerVersion
				bestHandler = handler
			}
		}
	}

	if bestHandler != nil {
		bestHandler(w, r)
		return
	}

	// Use fallback if no compatible handler found
	if vh.fallback != nil {
		vh.fallback(w, r)
	} else {
		http.Error(w, "No handler available for API version "+version.String(), http.StatusNotImplemented)
	}
}

// FeatureGate checks if a feature is available in the current request version
func FeatureGate(ctx context.Context, featureName string) bool {
	version, ok := GetVersionFromContext(ctx)
	if !ok {
		return false
	}

	feature, exists := GetFeature(featureName)
	if !exists {
		return false
	}

	// Check if feature is supported and not removed
	return version.SupportsFeature(feature.IntroducedIn) &&
		(feature.RemovedIn.Major == 0 || version.Compare(feature.RemovedIn) < 0)
}

// RequireFeature middleware that requires a specific feature to be available
func RequireFeature(featureName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !FeatureGate(r.Context(), featureName) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotImplemented)

				errorResponse := map[string]interface{}{
					"error": map[string]interface{}{
						"code":    "FEATURE_NOT_AVAILABLE",
						"message": "Feature not available in this API version",
						"feature": featureName,
					},
				}

				if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
					// Log error but continue since we're already in error handling
					// and can't change the response at this point
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
