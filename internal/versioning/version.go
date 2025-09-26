package versioning

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// APIVersion represents a semantic version for the API
type APIVersion struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Patch      int    `json:"patch"`
	Prerelease string `json:"prerelease,omitempty"`
}

// String returns the version as a string (e.g., "1.2.3" or "1.2.3-beta")
func (v APIVersion) String() string {
	version := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		version += "-" + v.Prerelease
	}
	return version
}

// Compare compares this version with another version
// Returns: -1 if this < other, 0 if equal, 1 if this > other
func (v APIVersion) Compare(other APIVersion) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Handle prerelease comparison
	if v.Prerelease == "" && other.Prerelease == "" {
		return 0
	}
	if v.Prerelease == "" {
		return 1 // Release version is greater than prerelease
	}
	if other.Prerelease == "" {
		return -1 // Prerelease is less than release
	}

	// Both have prerelease, compare strings
	if v.Prerelease < other.Prerelease {
		return -1
	}
	if v.Prerelease > other.Prerelease {
		return 1
	}
	return 0
}

// IsCompatible checks if this version is compatible with the target version
// Compatible means same major version and this version >= target version
func (v APIVersion) IsCompatible(target APIVersion) bool {
	if v.Major != target.Major {
		return false
	}
	return v.Compare(target) >= 0
}

// SupportsFeature checks if this version supports a feature introduced in a specific version
func (v APIVersion) SupportsFeature(featureVersion APIVersion) bool {
	return v.Compare(featureVersion) >= 0
}

// Version constants
var (
	V1_0_0 = APIVersion{Major: 1, Minor: 0, Patch: 0}
	V1_1_0 = APIVersion{Major: 1, Minor: 1, Patch: 0}
	V1_2_0 = APIVersion{Major: 1, Minor: 2, Patch: 0}
	V2_0_0 = APIVersion{Major: 2, Minor: 0, Patch: 0}
)

// Current API version
var CurrentVersion = V1_2_0

// Minimum supported version for backwards compatibility
var MinimumSupportedVersion = V1_0_0

// ParseVersion parses a version string into an APIVersion
func ParseVersion(versionStr string) (APIVersion, error) {
	// Regular expression to match semantic version
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?$`)
	matches := re.FindStringSubmatch(versionStr)

	if len(matches) < 4 {
		return APIVersion{}, fmt.Errorf("invalid version format: %s", versionStr)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return APIVersion{}, fmt.Errorf("invalid major version: %s", matches[1])
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return APIVersion{}, fmt.Errorf("invalid minor version: %s", matches[2])
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return APIVersion{}, fmt.Errorf("invalid patch version: %s", matches[3])
	}

	var prerelease string
	if len(matches) > 4 && matches[4] != "" {
		prerelease = matches[4]
	}

	return APIVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
	}, nil
}

// VersionInfo contains comprehensive version information
type VersionInfo struct {
	API       APIVersion `json:"api_version"`
	Build     string     `json:"build_version"`
	Commit    string     `json:"git_commit,omitempty"`
	BuildTime time.Time  `json:"build_time"`
	GoVersion string     `json:"go_version"`
}

// DefaultVersionInfo returns default version information
func DefaultVersionInfo() VersionInfo {
	return VersionInfo{
		API:       CurrentVersion,
		Build:     CurrentVersion.String(),
		BuildTime: time.Now(),
		GoVersion: "go1.24.6", // Will be replaced by build-time injection
	}
}

// Feature version mapping for backwards compatibility
type FeatureVersion struct {
	Name         string     `json:"name"`
	IntroducedIn APIVersion `json:"introduced_in"`
	DeprecatedIn APIVersion `json:"deprecated_in,omitempty"`
	RemovedIn    APIVersion `json:"removed_in,omitempty"`
	ReplacedBy   string     `json:"replaced_by,omitempty"`
	Description  string     `json:"description"`
}

// API feature registry
var APIFeatures = []FeatureVersion{
	// Core features
	{
		Name:         "webhook_authentication",
		IntroducedIn: V1_0_0,
		Description:  "HMAC-based webhook authentication",
	},
	{
		Name:         "message_bridging",
		IntroducedIn: V1_0_0,
		Description:  "Basic message bridging between WhatsApp and Signal",
	},
	{
		Name:         "media_support",
		IntroducedIn: V1_0_0,
		Description:  "Support for images, videos, and documents",
	},

	// Version 1.1 features
	{
		Name:         "rate_limiting",
		IntroducedIn: V1_1_0,
		Description:  "API rate limiting with configurable limits",
	},
	{
		Name:         "structured_errors",
		IntroducedIn: V1_1_0,
		Description:  "Structured error responses with error codes",
	},
	{
		Name:         "metrics_endpoint",
		IntroducedIn: V1_1_0,
		Description:  "Metrics and observability endpoint",
	},

	// Version 1.2 features
	{
		Name:         "feature_flags",
		IntroducedIn: V1_2_0,
		Description:  "Feature flag management and runtime toggling",
	},
	{
		Name:         "api_versioning",
		IntroducedIn: V1_2_0,
		Description:  "API versioning and backwards compatibility",
	},
	{
		Name:         "batch_operations",
		IntroducedIn: V1_2_0,
		Description:  "Batch processing for multiple operations",
	},

	// Deprecated features (examples for future use)
	{
		Name:         "legacy_auth",
		IntroducedIn: V1_0_0,
		DeprecatedIn: V1_1_0,
		RemovedIn:    V2_0_0,
		ReplacedBy:   "webhook_authentication",
		Description:  "Legacy authentication method",
	},
}

// GetFeature returns feature information by name
func GetFeature(name string) (*FeatureVersion, bool) {
	for _, feature := range APIFeatures {
		if feature.Name == name {
			return &feature, true
		}
	}
	return nil, false
}

// GetSupportedFeatures returns all features supported by a given API version
func GetSupportedFeatures(version APIVersion) []FeatureVersion {
	var supported []FeatureVersion

	for _, feature := range APIFeatures {
		// Check if feature was introduced in or before this version
		if version.SupportsFeature(feature.IntroducedIn) {
			// Check if feature is not yet removed
			if feature.RemovedIn.Major == 0 || version.Compare(feature.RemovedIn) < 0 {
				supported = append(supported, feature)
			}
		}
	}

	return supported
}

// GetDeprecatedFeatures returns features that are deprecated in the given version
func GetDeprecatedFeatures(version APIVersion) []FeatureVersion {
	var deprecated []FeatureVersion

	for _, feature := range APIFeatures {
		if feature.DeprecatedIn.Major > 0 &&
			version.SupportsFeature(feature.DeprecatedIn) &&
			(feature.RemovedIn.Major == 0 || version.Compare(feature.RemovedIn) < 0) {
			deprecated = append(deprecated, feature)
		}
	}

	return deprecated
}

// VersionCompatibility contains compatibility information
type VersionCompatibility struct {
	Requested          APIVersion       `json:"requested_version"`
	Current            APIVersion       `json:"current_version"`
	MinimumSupported   APIVersion       `json:"minimum_supported"`
	Compatible         bool             `json:"compatible"`
	SupportedFeatures  []FeatureVersion `json:"supported_features"`
	DeprecatedFeatures []FeatureVersion `json:"deprecated_features,omitempty"`
	Warnings           []string         `json:"warnings,omitempty"`
	Errors             []string         `json:"errors,omitempty"`
}

// CheckCompatibility checks version compatibility and returns detailed information
func CheckCompatibility(requestedVersion APIVersion) VersionCompatibility {
	compat := VersionCompatibility{
		Requested:        requestedVersion,
		Current:          CurrentVersion,
		MinimumSupported: MinimumSupportedVersion,
		Compatible:       false,
	}

	// Check if version is supported
	if requestedVersion.Compare(MinimumSupportedVersion) < 0 {
		compat.Errors = append(compat.Errors,
			fmt.Sprintf("Version %s is no longer supported. Minimum supported version is %s",
				requestedVersion.String(), MinimumSupportedVersion.String()))
		return compat
	}

	// Check if version is too new
	if requestedVersion.Major > CurrentVersion.Major {
		compat.Errors = append(compat.Errors,
			fmt.Sprintf("Version %s is not yet available. Current version is %s",
				requestedVersion.String(), CurrentVersion.String()))
		return compat
	}

	// Version is compatible
	compat.Compatible = true

	// Get supported and deprecated features
	compat.SupportedFeatures = GetSupportedFeatures(requestedVersion)
	compat.DeprecatedFeatures = GetDeprecatedFeatures(requestedVersion)

	// Add warnings for deprecated features
	for _, feature := range compat.DeprecatedFeatures {
		warning := fmt.Sprintf("Feature '%s' is deprecated", feature.Name)
		if feature.RemovedIn.Major > 0 {
			warning += fmt.Sprintf(" and will be removed in version %s", feature.RemovedIn.String())
		}
		if feature.ReplacedBy != "" {
			warning += fmt.Sprintf(". Use '%s' instead", feature.ReplacedBy)
		}
		compat.Warnings = append(compat.Warnings, warning)
	}

	// Add warning if using older version
	if requestedVersion.Compare(CurrentVersion) < 0 {
		compat.Warnings = append(compat.Warnings,
			fmt.Sprintf("You are using version %s. Consider upgrading to %s for latest features",
				requestedVersion.String(), CurrentVersion.String()))
	}

	return compat
}

// IsVersionSupported checks if a version is supported
func IsVersionSupported(version APIVersion) bool {
	return version.Compare(MinimumSupportedVersion) >= 0 &&
		version.Major <= CurrentVersion.Major
}

// GetVersionRange returns the supported version range as a string
func GetVersionRange() string {
	return fmt.Sprintf("%s - %s", MinimumSupportedVersion.String(), CurrentVersion.String())
}
