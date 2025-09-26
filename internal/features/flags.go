package features

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Flag represents a feature flag with metadata
type Flag struct {
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"`
	Percentage  int       `json:"percentage,omitempty"` // For gradual rollouts (0-100)
}

// FlagManager manages feature flags with thread-safe operations
type FlagManager struct {
	flags map[string]*Flag
	mu    sync.RWMutex
}

// NewFlagManager creates a new feature flag manager
func NewFlagManager() *FlagManager {
	return &FlagManager{
		flags: make(map[string]*Flag),
	}
}

// Define flag constants for type safety
const (
	// Core feature flags
	FlagAsyncProcessing    = "async_processing"
	FlagEnhancedLogging    = "enhanced_logging"
	FlagDistributedTracing = "distributed_tracing"
	FlagCircuitBreaker     = "circuit_breaker"

	// API features
	FlagRateLimiting     = "rate_limiting"
	FlagWebhookRetries   = "webhook_retries"
	FlagMediaCompression = "media_compression"
	FlagBatchOperations  = "batch_operations"

	// Security features
	FlagAdvancedAuth      = "advanced_auth"
	FlagEncryptionV2      = "encryption_v2"
	FlagAuditLogging      = "audit_logging"
	FlagRequestValidation = "request_validation"

	// Experimental features
	FlagNewContactSync   = "new_contact_sync"
	FlagOptimizedQueries = "optimized_queries"
	FlagCacheWarmup      = "cache_warmup"
	FlagMessageBatching  = "message_batching"
)

// FlagDefinition contains metadata about a flag
type FlagDefinition struct {
	Name         string
	Description  string
	DefaultValue bool
	Tags         []string
}

// DefaultFlags defines all available feature flags with their defaults
var DefaultFlags = []FlagDefinition{
	// Core features - generally enabled by default
	{FlagAsyncProcessing, "Enable asynchronous message processing", true, []string{"core", "performance"}},
	{FlagEnhancedLogging, "Enable detailed structured logging", true, []string{"core", "observability"}},
	{FlagDistributedTracing, "Enable OpenTelemetry distributed tracing", true, []string{"core", "observability"}},
	{FlagCircuitBreaker, "Enable circuit breaker for external services", true, []string{"core", "reliability"}},

	// API features - enabled by default
	{FlagRateLimiting, "Enable rate limiting for API endpoints", true, []string{"api", "security"}},
	{FlagWebhookRetries, "Enable automatic webhook retry logic", true, []string{"api", "reliability"}},
	{FlagMediaCompression, "Enable media compression for large files", false, []string{"api", "performance"}},
	{FlagBatchOperations, "Enable batch processing for multiple operations", false, []string{"api", "performance"}},

	// Security features - enabled by default for production safety
	{FlagAdvancedAuth, "Enable advanced authentication features", true, []string{"security"}},
	{FlagEncryptionV2, "Enable enhanced encryption algorithm", false, []string{"security", "experimental"}},
	{FlagAuditLogging, "Enable comprehensive audit logging", true, []string{"security", "compliance"}},
	{FlagRequestValidation, "Enable strict request validation", true, []string{"security", "validation"}},

	// Experimental features - disabled by default
	{FlagNewContactSync, "Use new optimized contact sync algorithm", false, []string{"experimental", "performance"}},
	{FlagOptimizedQueries, "Use optimized database queries", false, []string{"experimental", "performance"}},
	{FlagCacheWarmup, "Enable cache warming on startup", false, []string{"experimental", "performance"}},
	{FlagMessageBatching, "Enable message batching for better throughput", false, []string{"experimental", "performance"}},
}

// InitializeDefaults sets up all default flags
func (fm *FlagManager) InitializeDefaults() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	now := time.Now()
	for _, def := range DefaultFlags {
		if _, exists := fm.flags[def.Name]; !exists {
			fm.flags[def.Name] = &Flag{
				Name:        def.Name,
				Enabled:     def.DefaultValue,
				Description: def.Description,
				CreatedAt:   now,
				UpdatedAt:   now,
				Tags:        def.Tags,
				Percentage:  100, // Default to 100% when enabled
			}
		}
	}
}

// IsEnabled checks if a feature flag is enabled
func (fm *FlagManager) IsEnabled(flagName string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	flag, exists := fm.flags[flagName]
	if !exists {
		return false
	}

	return flag.Enabled
}

// Enable enables a feature flag
func (fm *FlagManager) Enable(flagName string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	flag, exists := fm.flags[flagName]
	if !exists {
		return ErrFlagNotFound{Name: flagName}
	}

	flag.Enabled = true
	flag.UpdatedAt = time.Now()
	return nil
}

// Disable disables a feature flag
func (fm *FlagManager) Disable(flagName string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	flag, exists := fm.flags[flagName]
	if !exists {
		return ErrFlagNotFound{Name: flagName}
	}

	flag.Enabled = false
	flag.UpdatedAt = time.Now()
	return nil
}

// SetPercentage sets the rollout percentage for a flag (0-100)
func (fm *FlagManager) SetPercentage(flagName string, percentage int) error {
	if percentage < 0 || percentage > 100 {
		return ErrInvalidPercentage{Percentage: percentage}
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	flag, exists := fm.flags[flagName]
	if !exists {
		return ErrFlagNotFound{Name: flagName}
	}

	flag.Percentage = percentage
	flag.UpdatedAt = time.Now()
	return nil
}

// CreateFlag creates a new feature flag
func (fm *FlagManager) CreateFlag(name, description string, enabled bool, tags []string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.flags[name]; exists {
		return ErrFlagExists{Name: name}
	}

	now := time.Now()
	fm.flags[name] = &Flag{
		Name:        name,
		Enabled:     enabled,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        tags,
		Percentage:  100,
	}

	return nil
}

// DeleteFlag removes a feature flag
func (fm *FlagManager) DeleteFlag(flagName string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.flags[flagName]; !exists {
		return ErrFlagNotFound{Name: flagName}
	}

	delete(fm.flags, flagName)
	return nil
}

// GetFlag returns a copy of the flag information
func (fm *FlagManager) GetFlag(flagName string) (*Flag, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	flag, exists := fm.flags[flagName]
	if !exists {
		return nil, ErrFlagNotFound{Name: flagName}
	}

	// Return a copy to prevent external modification
	flagCopy := *flag
	if flag.Tags != nil {
		flagCopy.Tags = make([]string, len(flag.Tags))
		copy(flagCopy.Tags, flag.Tags)
	}

	return &flagCopy, nil
}

// ListFlags returns all flags, optionally filtered by tags
func (fm *FlagManager) ListFlags(filterTags ...string) []*Flag {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var result []*Flag

	for _, flag := range fm.flags {
		// If no filter tags, include all flags
		if len(filterTags) == 0 {
			flagCopy := *flag
			if flag.Tags != nil {
				flagCopy.Tags = make([]string, len(flag.Tags))
				copy(flagCopy.Tags, flag.Tags)
			}
			result = append(result, &flagCopy)
			continue
		}

		// Check if flag has any of the filter tags
		for _, filterTag := range filterTags {
			for _, flagTag := range flag.Tags {
				if flagTag == filterTag {
					flagCopy := *flag
					if flag.Tags != nil {
						flagCopy.Tags = make([]string, len(flag.Tags))
						copy(flagCopy.Tags, flag.Tags)
					}
					result = append(result, &flagCopy)
					goto nextFlag
				}
			}
		}
	nextFlag:
	}

	return result
}

// ExportJSON exports all flags as JSON
func (fm *FlagManager) ExportJSON() ([]byte, error) {
	flags := fm.ListFlags()
	return json.MarshalIndent(flags, "", "  ")
}

// ImportJSON imports flags from JSON
func (fm *FlagManager) ImportJSON(data []byte) error {
	var flags []*Flag
	if err := json.Unmarshal(data, &flags); err != nil {
		return err
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	for _, flag := range flags {
		// Validate flag before importing
		if flag.Name == "" {
			continue
		}

		if flag.Percentage < 0 || flag.Percentage > 100 {
			flag.Percentage = 100
		}

		fm.flags[flag.Name] = flag
	}

	return nil
}

// Global flag manager instance
var globalFlagManager = NewFlagManager()

// Initialize sets up the global flag manager with defaults
func Initialize() {
	globalFlagManager.InitializeDefaults()
}

// IsEnabled checks if a feature flag is enabled globally
func IsEnabled(flagName string) bool {
	return globalFlagManager.IsEnabled(flagName)
}

// Enable enables a feature flag globally
func Enable(flagName string) error {
	return globalFlagManager.Enable(flagName)
}

// Disable disables a feature flag globally
func Disable(flagName string) error {
	return globalFlagManager.Disable(flagName)
}

// GetGlobalManager returns the global flag manager
func GetGlobalManager() *FlagManager {
	return globalFlagManager
}

// Custom errors
type ErrFlagNotFound struct {
	Name string
}

func (e ErrFlagNotFound) Error() string {
	return "feature flag not found: " + e.Name
}

type ErrFlagExists struct {
	Name string
}

func (e ErrFlagExists) Error() string {
	return "feature flag already exists: " + e.Name
}

type ErrInvalidPercentage struct {
	Percentage int
}

func (e ErrInvalidPercentage) Error() string {
	return fmt.Sprintf("invalid percentage: %d (must be 0-100)", e.Percentage)
}
