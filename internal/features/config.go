package features

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// FlagsConfig represents feature flags configuration
type FlagsConfig struct {
	// Map of flag name to enabled state
	Flags map[string]bool `json:"flags" mapstructure:"flags"`

	// Map of flag name to percentage rollout
	Percentages map[string]int `json:"percentages" mapstructure:"percentages"`

	// Environment-based overrides
	EnableAll  bool `json:"enable_all" mapstructure:"enable_all"`
	DisableAll bool `json:"disable_all" mapstructure:"disable_all"`
}

// DefaultFlagsConfig returns default configuration
func DefaultFlagsConfig() FlagsConfig {
	return FlagsConfig{
		Flags:       make(map[string]bool),
		Percentages: make(map[string]int),
		EnableAll:   false,
		DisableAll:  false,
	}
}

// LoadFromConfig applies configuration to the flag manager
func (fm *FlagManager) LoadFromConfig(config FlagsConfig) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// First apply explicit flag settings
	for flagName, enabled := range config.Flags {
		if flag, exists := fm.flags[flagName]; exists {
			flag.Enabled = enabled
			flag.UpdatedAt = time.Now()
		} else {
			// Create flag if it doesn't exist
			now := time.Now()
			fm.flags[flagName] = &Flag{
				Name:        flagName,
				Enabled:     enabled,
				Description: "Flag created from configuration",
				CreatedAt:   now,
				UpdatedAt:   now,
				Tags:        []string{"config"},
				Percentage:  100,
			}
		}
	}

	// Apply percentage settings
	for flagName, percentage := range config.Percentages {
		if flag, exists := fm.flags[flagName]; exists {
			if percentage >= 0 && percentage <= 100 {
				flag.Percentage = percentage
				flag.UpdatedAt = time.Now()
			}
		}
	}

	// Apply global overrides
	if config.EnableAll {
		for _, flag := range fm.flags {
			flag.Enabled = true
			flag.UpdatedAt = time.Now()
		}
	} else if config.DisableAll {
		for _, flag := range fm.flags {
			flag.Enabled = false
			flag.UpdatedAt = time.Now()
		}
	}

	return nil
}

// LoadFromEnvironment loads feature flags from environment variables
// Environment variables should be in format: WHATSIGNAL_FEATURE_<FLAG_NAME>=true/false
// Percentages: WHATSIGNAL_FEATURE_<FLAG_NAME>_PERCENTAGE=50
func (fm *FlagManager) LoadFromEnvironment() {
	const (
		envPrefix        = "WHATSIGNAL_FEATURE_"
		percentageSuffix = "_PERCENTAGE"
		envEnableAll     = "WHATSIGNAL_FEATURES_ENABLE_ALL"
		envDisableAll    = "WHATSIGNAL_FEATURES_DISABLE_ALL"
	)

	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Check for global enable/disable
	if envValue := os.Getenv(envEnableAll); envValue != "" {
		if enableAll, _ := strconv.ParseBool(envValue); enableAll {
			for _, flag := range fm.flags {
				flag.Enabled = true
				flag.UpdatedAt = time.Now()
			}
			return // Skip individual flag processing if all enabled
		}
	}

	if envValue := os.Getenv(envDisableAll); envValue != "" {
		if disableAll, _ := strconv.ParseBool(envValue); disableAll {
			for _, flag := range fm.flags {
				flag.Enabled = false
				flag.UpdatedAt = time.Now()
			}
			return // Skip individual flag processing if all disabled
		}
	}

	// Process all environment variables
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, envPrefix) {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envKey := parts[0]
		envValue := parts[1]

		// Extract flag name
		flagPart := strings.TrimPrefix(envKey, envPrefix)

		// Handle percentage settings
		if strings.HasSuffix(flagPart, percentageSuffix) {
			flagName := strings.TrimSuffix(flagPart, percentageSuffix)
			flagName = strings.ToLower(flagName)

			if percentage, err := strconv.Atoi(envValue); err == nil && percentage >= 0 && percentage <= 100 {
				if flag, exists := fm.flags[flagName]; exists {
					flag.Percentage = percentage
					flag.UpdatedAt = time.Now()
				}
			}
			continue
		}

		// Handle boolean flag settings
		flagName := strings.ToLower(flagPart)
		if enabled, err := strconv.ParseBool(envValue); err == nil {
			if flag, exists := fm.flags[flagName]; exists {
				flag.Enabled = enabled
				flag.UpdatedAt = time.Now()
			} else {
				// Create flag if it doesn't exist
				now := time.Now()
				fm.flags[flagName] = &Flag{
					Name:        flagName,
					Enabled:     enabled,
					Description: "Flag created from environment variable",
					CreatedAt:   now,
					UpdatedAt:   now,
					Tags:        []string{"env"},
					Percentage:  100,
				}
			}
		}
	}
}

// ToConfig exports current flag state as configuration
func (fm *FlagManager) ToConfig() FlagsConfig {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	config := FlagsConfig{
		Flags:       make(map[string]bool),
		Percentages: make(map[string]int),
	}

	for name, flag := range fm.flags {
		config.Flags[name] = flag.Enabled
		if flag.Percentage != 100 {
			config.Percentages[name] = flag.Percentage
		}
	}

	return config
}

// ValidateConfig validates feature flags configuration
func ValidateConfig(config FlagsConfig) error {
	// Validate percentages
	for flagName, percentage := range config.Percentages {
		if percentage < 0 || percentage > 100 {
			return fmt.Errorf("invalid percentage for flag %s: %d (must be 0-100)", flagName, percentage)
		}
	}

	// Check for conflicting global settings
	if config.EnableAll && config.DisableAll {
		return fmt.Errorf("cannot set both enable_all and disable_all to true")
	}

	return nil
}

// GetEnvironmentOverrides returns a list of environment variables that would override flags
func GetEnvironmentOverrides() map[string]string {
	const envPrefix = "WHATSIGNAL_FEATURE_"
	overrides := make(map[string]string)

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, envPrefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				overrides[parts[0]] = parts[1]
			}
		}
	}

	// Add global overrides
	if value := os.Getenv("WHATSIGNAL_FEATURES_ENABLE_ALL"); value != "" {
		overrides["WHATSIGNAL_FEATURES_ENABLE_ALL"] = value
	}
	if value := os.Getenv("WHATSIGNAL_FEATURES_DISABLE_ALL"); value != "" {
		overrides["WHATSIGNAL_FEATURES_DISABLE_ALL"] = value
	}

	return overrides
}
