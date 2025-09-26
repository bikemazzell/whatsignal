package versioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIVersion_String(t *testing.T) {
	tests := []struct {
		name     string
		version  APIVersion
		expected string
	}{
		{
			name:     "basic version",
			version:  APIVersion{Major: 1, Minor: 2, Patch: 3},
			expected: "1.2.3",
		},
		{
			name:     "version with prerelease",
			version:  APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta"},
			expected: "1.2.3-beta",
		},
		{
			name:     "zero version",
			version:  APIVersion{Major: 0, Minor: 0, Patch: 0},
			expected: "0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.version.String())
		})
	}
}

func TestAPIVersion_Compare(t *testing.T) {
	tests := []struct {
		name     string
		v1       APIVersion
		v2       APIVersion
		expected int
	}{
		{
			name:     "equal versions",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 3},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 3},
			expected: 0,
		},
		{
			name:     "v1 greater major",
			v1:       APIVersion{Major: 2, Minor: 0, Patch: 0},
			v2:       APIVersion{Major: 1, Minor: 9, Patch: 9},
			expected: 1,
		},
		{
			name:     "v1 lesser major",
			v1:       APIVersion{Major: 1, Minor: 9, Patch: 9},
			v2:       APIVersion{Major: 2, Minor: 0, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 greater minor",
			v1:       APIVersion{Major: 1, Minor: 3, Patch: 0},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 9},
			expected: 1,
		},
		{
			name:     "v1 lesser minor",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 9},
			v2:       APIVersion{Major: 1, Minor: 3, Patch: 0},
			expected: -1,
		},
		{
			name:     "v1 greater patch",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 4},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 3},
			expected: 1,
		},
		{
			name:     "v1 lesser patch",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 3},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 4},
			expected: -1,
		},
		{
			name:     "release vs prerelease",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 3},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta"},
			expected: 1,
		},
		{
			name:     "prerelease vs release",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 3},
			expected: -1,
		},
		{
			name:     "prerelease comparison",
			v1:       APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta"},
			v2:       APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "alpha"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.v1.Compare(tt.v2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAPIVersion_IsCompatible(t *testing.T) {
	tests := []struct {
		name     string
		version  APIVersion
		target   APIVersion
		expected bool
	}{
		{
			name:     "same version",
			version:  APIVersion{Major: 1, Minor: 2, Patch: 3},
			target:   APIVersion{Major: 1, Minor: 2, Patch: 3},
			expected: true,
		},
		{
			name:     "newer minor version",
			version:  APIVersion{Major: 1, Minor: 3, Patch: 0},
			target:   APIVersion{Major: 1, Minor: 2, Patch: 0},
			expected: true,
		},
		{
			name:     "newer patch version",
			version:  APIVersion{Major: 1, Minor: 2, Patch: 4},
			target:   APIVersion{Major: 1, Minor: 2, Patch: 3},
			expected: true,
		},
		{
			name:     "older minor version",
			version:  APIVersion{Major: 1, Minor: 1, Patch: 0},
			target:   APIVersion{Major: 1, Minor: 2, Patch: 0},
			expected: false,
		},
		{
			name:     "different major version",
			version:  APIVersion{Major: 2, Minor: 0, Patch: 0},
			target:   APIVersion{Major: 1, Minor: 9, Patch: 9},
			expected: false,
		},
		{
			name:     "older major version",
			version:  APIVersion{Major: 1, Minor: 9, Patch: 9},
			target:   APIVersion{Major: 2, Minor: 0, Patch: 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.IsCompatible(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAPIVersion_SupportsFeature(t *testing.T) {
	tests := []struct {
		name           string
		version        APIVersion
		featureVersion APIVersion
		expected       bool
	}{
		{
			name:           "supports exact version",
			version:        APIVersion{Major: 1, Minor: 2, Patch: 0},
			featureVersion: APIVersion{Major: 1, Minor: 2, Patch: 0},
			expected:       true,
		},
		{
			name:           "supports newer version",
			version:        APIVersion{Major: 1, Minor: 3, Patch: 0},
			featureVersion: APIVersion{Major: 1, Minor: 2, Patch: 0},
			expected:       true,
		},
		{
			name:           "does not support older version",
			version:        APIVersion{Major: 1, Minor: 1, Patch: 0},
			featureVersion: APIVersion{Major: 1, Minor: 2, Patch: 0},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.SupportsFeature(tt.featureVersion)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionStr  string
		expected    APIVersion
		expectError bool
	}{
		{
			name:       "basic version",
			versionStr: "1.2.3",
			expected:   APIVersion{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:       "version with prerelease",
			versionStr: "1.2.3-beta",
			expected:   APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta"},
		},
		{
			name:       "version with complex prerelease",
			versionStr: "1.2.3-beta.1",
			expected:   APIVersion{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta.1"},
		},
		{
			name:       "zero version",
			versionStr: "0.0.0",
			expected:   APIVersion{Major: 0, Minor: 0, Patch: 0},
		},
		{
			name:        "invalid format - missing patch",
			versionStr:  "1.2",
			expectError: true,
		},
		{
			name:        "invalid format - non-numeric",
			versionStr:  "1.2.x",
			expectError: true,
		},
		{
			name:        "invalid format - empty",
			versionStr:  "",
			expectError: true,
		},
		{
			name:        "invalid format - missing minor",
			versionStr:  "1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseVersion(tt.versionStr)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetFeature(t *testing.T) {
	// Test existing feature
	feature, exists := GetFeature("webhook_authentication")
	assert.True(t, exists)
	assert.NotNil(t, feature)
	assert.Equal(t, "webhook_authentication", feature.Name)
	assert.Equal(t, V1_0_0, feature.IntroducedIn)

	// Test non-existing feature
	feature, exists = GetFeature("nonexistent_feature")
	assert.False(t, exists)
	assert.Nil(t, feature)
}

func TestGetSupportedFeatures(t *testing.T) {
	tests := []struct {
		name               string
		version            APIVersion
		expectedFeatures   []string
		unexpectedFeatures []string
	}{
		{
			name:               "version 1.0.0",
			version:            V1_0_0,
			expectedFeatures:   []string{"webhook_authentication", "message_bridging", "media_support"},
			unexpectedFeatures: []string{"rate_limiting", "feature_flags"},
		},
		{
			name:               "version 1.1.0",
			version:            V1_1_0,
			expectedFeatures:   []string{"webhook_authentication", "message_bridging", "rate_limiting", "structured_errors"},
			unexpectedFeatures: []string{"feature_flags", "api_versioning"},
		},
		{
			name:               "version 1.2.0",
			version:            V1_2_0,
			expectedFeatures:   []string{"webhook_authentication", "rate_limiting", "feature_flags", "api_versioning"},
			unexpectedFeatures: []string{}, // All current features should be supported
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := GetSupportedFeatures(tt.version)

			// Create a map for easier lookup
			featureMap := make(map[string]bool)
			for _, feature := range features {
				featureMap[feature.Name] = true
			}

			// Check expected features are present
			for _, expectedFeature := range tt.expectedFeatures {
				assert.True(t, featureMap[expectedFeature],
					"Feature %s should be supported in version %s", expectedFeature, tt.version.String())
			}

			// Check unexpected features are not present
			for _, unexpectedFeature := range tt.unexpectedFeatures {
				assert.False(t, featureMap[unexpectedFeature],
					"Feature %s should not be supported in version %s", unexpectedFeature, tt.version.String())
			}
		})
	}
}

func TestGetDeprecatedFeatures(t *testing.T) {
	// Test with a version that has deprecated features
	// Since we have a legacy_auth feature deprecated in 1.1.0
	features := GetDeprecatedFeatures(V1_1_0)

	// Check if legacy_auth is in deprecated features
	var legacyAuthFound bool
	for _, feature := range features {
		if feature.Name == "legacy_auth" {
			legacyAuthFound = true
			assert.Equal(t, V1_0_0, feature.IntroducedIn)
			assert.Equal(t, V1_1_0, feature.DeprecatedIn)
			assert.Equal(t, V2_0_0, feature.RemovedIn)
			break
		}
	}
	assert.True(t, legacyAuthFound, "legacy_auth should be deprecated in version 1.1.0")

	// Test with version 1.0.0 (no deprecated features yet)
	features = GetDeprecatedFeatures(V1_0_0)
	for _, feature := range features {
		assert.NotEqual(t, "legacy_auth", feature.Name, "legacy_auth should not be deprecated in version 1.0.0")
	}
}

func TestCheckCompatibility(t *testing.T) {
	tests := []struct {
		name             string
		requestedVersion APIVersion
		expectCompatible bool
		expectErrors     bool
		expectWarnings   bool
	}{
		{
			name:             "current version",
			requestedVersion: CurrentVersion,
			expectCompatible: true,
			expectErrors:     false,
			expectWarnings:   false,
		},
		{
			name:             "supported older version",
			requestedVersion: V1_0_0,
			expectCompatible: true,
			expectErrors:     false,
			expectWarnings:   true, // Should warn about using older version
		},
		{
			name:             "unsupported old version",
			requestedVersion: APIVersion{Major: 0, Minor: 9, Patch: 0},
			expectCompatible: false,
			expectErrors:     true,
			expectWarnings:   false,
		},
		{
			name:             "future version",
			requestedVersion: APIVersion{Major: 3, Minor: 0, Patch: 0},
			expectCompatible: false,
			expectErrors:     true,
			expectWarnings:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compat := CheckCompatibility(tt.requestedVersion)

			assert.Equal(t, tt.expectCompatible, compat.Compatible)
			assert.Equal(t, tt.requestedVersion, compat.Requested)
			assert.Equal(t, CurrentVersion, compat.Current)
			assert.Equal(t, MinimumSupportedVersion, compat.MinimumSupported)

			if tt.expectErrors {
				assert.NotEmpty(t, compat.Errors)
			} else {
				assert.Empty(t, compat.Errors)
			}

			if tt.expectWarnings {
				assert.NotEmpty(t, compat.Warnings)
			}

			// Should always have supported features if compatible
			if compat.Compatible {
				assert.NotEmpty(t, compat.SupportedFeatures)
			}
		})
	}
}

func TestIsVersionSupported(t *testing.T) {
	tests := []struct {
		name     string
		version  APIVersion
		expected bool
	}{
		{
			name:     "current version",
			version:  CurrentVersion,
			expected: true,
		},
		{
			name:     "minimum supported version",
			version:  MinimumSupportedVersion,
			expected: true,
		},
		{
			name:     "supported older version",
			version:  V1_1_0,
			expected: true,
		},
		{
			name:     "unsupported old version",
			version:  APIVersion{Major: 0, Minor: 9, Patch: 0},
			expected: false,
		},
		{
			name:     "future major version",
			version:  APIVersion{Major: 3, Minor: 0, Patch: 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVersionSupported(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVersionRange(t *testing.T) {
	versionRange := GetVersionRange()
	expectedRange := MinimumSupportedVersion.String() + " - " + CurrentVersion.String()
	assert.Equal(t, expectedRange, versionRange)
}

func TestVersionConstants(t *testing.T) {
	// Test that version constants are properly defined
	assert.Equal(t, 1, V1_0_0.Major)
	assert.Equal(t, 0, V1_0_0.Minor)
	assert.Equal(t, 0, V1_0_0.Patch)

	assert.Equal(t, 1, V1_1_0.Major)
	assert.Equal(t, 1, V1_1_0.Minor)
	assert.Equal(t, 0, V1_1_0.Patch)

	assert.Equal(t, 1, V1_2_0.Major)
	assert.Equal(t, 2, V1_2_0.Minor)
	assert.Equal(t, 0, V1_2_0.Patch)

	assert.Equal(t, 2, V2_0_0.Major)
	assert.Equal(t, 0, V2_0_0.Minor)
	assert.Equal(t, 0, V2_0_0.Patch)

	// Test that current version is reasonable
	assert.True(t, CurrentVersion.Major >= 1)
	assert.True(t, MinimumSupportedVersion.Compare(CurrentVersion) <= 0)
}

func TestDefaultVersionInfo(t *testing.T) {
	info := DefaultVersionInfo()

	assert.Equal(t, CurrentVersion, info.API)
	assert.Equal(t, CurrentVersion.String(), info.Build)
	assert.NotEmpty(t, info.GoVersion)
	assert.False(t, info.BuildTime.IsZero())
}
