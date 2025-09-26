package features

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFlagsConfig(t *testing.T) {
	config := DefaultFlagsConfig()

	assert.NotNil(t, config.Flags)
	assert.NotNil(t, config.Percentages)
	assert.False(t, config.EnableAll)
	assert.False(t, config.DisableAll)
	assert.Len(t, config.Flags, 0)
	assert.Len(t, config.Percentages, 0)
}

func TestFlagManager_LoadFromConfig(t *testing.T) {
	fm := NewFlagManager()
	fm.InitializeDefaults()

	config := FlagsConfig{
		Flags: map[string]bool{
			FlagAsyncProcessing: false, // Override default (true -> false)
			FlagNewContactSync:  true,  // Override default (false -> true)
			"custom_flag":       true,  // New flag
		},
		Percentages: map[string]int{
			FlagAsyncProcessing: 50,
			"custom_flag":       75,
		},
	}

	err := fm.LoadFromConfig(config)
	require.NoError(t, err)

	// Check overridden flags
	assert.False(t, fm.IsEnabled(FlagAsyncProcessing))
	assert.True(t, fm.IsEnabled(FlagNewContactSync))
	assert.True(t, fm.IsEnabled("custom_flag"))

	// Check percentages
	flag, err := fm.GetFlag(FlagAsyncProcessing)
	require.NoError(t, err)
	assert.Equal(t, 50, flag.Percentage)

	customFlag, err := fm.GetFlag("custom_flag")
	require.NoError(t, err)
	assert.Equal(t, 75, customFlag.Percentage)
	assert.Contains(t, customFlag.Tags, "config")
}

func TestFlagManager_LoadFromConfig_GlobalOverrides(t *testing.T) {
	tests := []struct {
		name      string
		config    FlagsConfig
		expectAll func(*testing.T, *FlagManager)
	}{
		{
			name: "enable all overrides individual settings",
			config: FlagsConfig{
				Flags: map[string]bool{
					FlagAsyncProcessing: false,
				},
				EnableAll: true,
			},
			expectAll: func(t *testing.T, fm *FlagManager) {
				flags := fm.ListFlags()
				for _, flag := range flags {
					assert.True(t, flag.Enabled, "Flag %s should be enabled", flag.Name)
				}
			},
		},
		{
			name: "disable all overrides individual settings",
			config: FlagsConfig{
				Flags: map[string]bool{
					FlagAsyncProcessing: true,
				},
				DisableAll: true,
			},
			expectAll: func(t *testing.T, fm *FlagManager) {
				flags := fm.ListFlags()
				for _, flag := range flags {
					assert.False(t, flag.Enabled, "Flag %s should be disabled", flag.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFlagManager()
			fm.InitializeDefaults()

			err := fm.LoadFromConfig(tt.config)
			require.NoError(t, err)

			tt.expectAll(t, fm)
		})
	}
}

func TestFlagManager_LoadFromEnvironment(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, env := range []string{
		"WHATSIGNAL_FEATURE_ASYNC_PROCESSING",
		"WHATSIGNAL_FEATURE_NEW_CONTACT_SYNC",
		"WHATSIGNAL_FEATURE_ASYNC_PROCESSING_PERCENTAGE",
		"WHATSIGNAL_FEATURE_CUSTOM_FLAG",
		"WHATSIGNAL_FEATURES_ENABLE_ALL",
		"WHATSIGNAL_FEATURES_DISABLE_ALL",
	} {
		originalEnv[env] = os.Getenv(env)
	}

	// Clean up environment after test
	defer func() {
		for env, value := range originalEnv {
			if value == "" {
				os.Unsetenv(env)
			} else {
				os.Setenv(env, value)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("WHATSIGNAL_FEATURE_ASYNC_PROCESSING", "false")
	os.Setenv("WHATSIGNAL_FEATURE_NEW_CONTACT_SYNC", "true")
	os.Setenv("WHATSIGNAL_FEATURE_ASYNC_PROCESSING_PERCENTAGE", "25")
	os.Setenv("WHATSIGNAL_FEATURE_CUSTOM_FLAG", "true")

	fm := NewFlagManager()
	fm.InitializeDefaults()

	// Load from environment
	fm.LoadFromEnvironment()

	// Check environment overrides
	assert.False(t, fm.IsEnabled(FlagAsyncProcessing))
	assert.True(t, fm.IsEnabled(FlagNewContactSync))
	assert.True(t, fm.IsEnabled("custom_flag"))

	// Check percentage
	flag, err := fm.GetFlag(FlagAsyncProcessing)
	require.NoError(t, err)
	assert.Equal(t, 25, flag.Percentage)

	// Check custom flag was created
	customFlag, err := fm.GetFlag("custom_flag")
	require.NoError(t, err)
	assert.Contains(t, customFlag.Tags, "env")
}

func TestFlagManager_LoadFromEnvironment_GlobalOverrides(t *testing.T) {
	tests := []struct {
		name      string
		envVar    string
		envValue  string
		expectAll func(*testing.T, *FlagManager)
	}{
		{
			name:     "enable all from environment",
			envVar:   "WHATSIGNAL_FEATURES_ENABLE_ALL",
			envValue: "true",
			expectAll: func(t *testing.T, fm *FlagManager) {
				flags := fm.ListFlags()
				for _, flag := range flags {
					assert.True(t, flag.Enabled, "Flag %s should be enabled", flag.Name)
				}
			},
		},
		{
			name:     "disable all from environment",
			envVar:   "WHATSIGNAL_FEATURES_DISABLE_ALL",
			envValue: "true",
			expectAll: func(t *testing.T, fm *FlagManager) {
				flags := fm.ListFlags()
				for _, flag := range flags {
					assert.False(t, flag.Enabled, "Flag %s should be disabled", flag.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			original := os.Getenv(tt.envVar)
			defer func() {
				if original == "" {
					os.Unsetenv(tt.envVar)
				} else {
					os.Setenv(tt.envVar, original)
				}
			}()

			os.Setenv(tt.envVar, tt.envValue)

			fm := NewFlagManager()
			fm.InitializeDefaults()
			fm.LoadFromEnvironment()

			tt.expectAll(t, fm)
		})
	}
}

func TestFlagManager_ToConfig(t *testing.T) {
	fm := NewFlagManager()

	// Create some flags
	err := fm.CreateFlag("flag1", "Flag 1", true, []string{"test"})
	require.NoError(t, err)

	err = fm.CreateFlag("flag2", "Flag 2", false, []string{"test"})
	require.NoError(t, err)

	err = fm.SetPercentage("flag1", 75)
	require.NoError(t, err)

	// Export to config
	config := fm.ToConfig()

	assert.True(t, config.Flags["flag1"])
	assert.False(t, config.Flags["flag2"])
	assert.Equal(t, 75, config.Percentages["flag1"])

	// flag2 should not be in percentages (it's 100%)
	_, exists := config.Percentages["flag2"]
	assert.False(t, exists)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      FlagsConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: FlagsConfig{
				Flags:       map[string]bool{"flag1": true},
				Percentages: map[string]int{"flag1": 50},
			},
			expectError: false,
		},
		{
			name: "invalid percentage - negative",
			config: FlagsConfig{
				Percentages: map[string]int{"flag1": -1},
			},
			expectError: true,
			errorMsg:    "invalid percentage for flag flag1: -1",
		},
		{
			name: "invalid percentage - over 100",
			config: FlagsConfig{
				Percentages: map[string]int{"flag1": 101},
			},
			expectError: true,
			errorMsg:    "invalid percentage for flag flag1: 101",
		},
		{
			name: "conflicting global settings",
			config: FlagsConfig{
				EnableAll:  true,
				DisableAll: true,
			},
			expectError: true,
			errorMsg:    "cannot set both enable_all and disable_all to true",
		},
		{
			name: "valid global setting - enable all",
			config: FlagsConfig{
				EnableAll: true,
			},
			expectError: false,
		},
		{
			name: "valid global setting - disable all",
			config: FlagsConfig{
				DisableAll: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetEnvironmentOverrides(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	testEnvVars := []string{
		"WHATSIGNAL_FEATURE_TEST_FLAG",
		"WHATSIGNAL_FEATURE_ANOTHER_FLAG",
		"WHATSIGNAL_FEATURES_ENABLE_ALL",
		"WHATSIGNAL_FEATURES_DISABLE_ALL",
	}

	for _, env := range testEnvVars {
		originalEnv[env] = os.Getenv(env)
	}

	// Clean up after test
	defer func() {
		for env, value := range originalEnv {
			if value == "" {
				os.Unsetenv(env)
			} else {
				os.Setenv(env, value)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("WHATSIGNAL_FEATURE_TEST_FLAG", "true")
	os.Setenv("WHATSIGNAL_FEATURE_ANOTHER_FLAG", "false")
	os.Setenv("WHATSIGNAL_FEATURES_ENABLE_ALL", "true")
	os.Setenv("SOME_OTHER_VAR", "value") // Should be ignored

	overrides := GetEnvironmentOverrides()

	assert.Contains(t, overrides, "WHATSIGNAL_FEATURE_TEST_FLAG")
	assert.Equal(t, "true", overrides["WHATSIGNAL_FEATURE_TEST_FLAG"])

	assert.Contains(t, overrides, "WHATSIGNAL_FEATURE_ANOTHER_FLAG")
	assert.Equal(t, "false", overrides["WHATSIGNAL_FEATURE_ANOTHER_FLAG"])

	assert.Contains(t, overrides, "WHATSIGNAL_FEATURES_ENABLE_ALL")
	assert.Equal(t, "true", overrides["WHATSIGNAL_FEATURES_ENABLE_ALL"])

	// Should not contain non-feature variables
	assert.NotContains(t, overrides, "SOME_OTHER_VAR")

	// Should not contain DISABLE_ALL since it's not set
	assert.NotContains(t, overrides, "WHATSIGNAL_FEATURES_DISABLE_ALL")
}

func TestEnvironmentParsing_EdgeCases(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	testEnvVars := []string{
		"WHATSIGNAL_FEATURE_INVALID_BOOL",
		"WHATSIGNAL_FEATURE_VALID_FLAG_PERCENTAGE",
		"WHATSIGNAL_FEATURE_INVALID_PERCENTAGE",
	}

	for _, env := range testEnvVars {
		originalEnv[env] = os.Getenv(env)
	}

	defer func() {
		for env, value := range originalEnv {
			if value == "" {
				os.Unsetenv(env)
			} else {
				os.Setenv(env, value)
			}
		}
	}()

	// Set invalid values
	os.Setenv("WHATSIGNAL_FEATURE_INVALID_BOOL", "not-a-boolean")
	os.Setenv("WHATSIGNAL_FEATURE_VALID_FLAG_PERCENTAGE", "50")
	os.Setenv("WHATSIGNAL_FEATURE_INVALID_PERCENTAGE", "not-a-number")

	fm := NewFlagManager()
	_ = fm.CreateFlag("valid_flag", "Valid flag", false, nil)

	// Should not crash on invalid values
	fm.LoadFromEnvironment()

	// Invalid boolean should be ignored
	assert.False(t, fm.IsEnabled("invalid_bool"))

	// Valid percentage should be applied
	flag, err := fm.GetFlag("valid_flag")
	if err == nil {
		// If environment var was applied, check percentage
		assert.Equal(t, 50, flag.Percentage)
	}
}
