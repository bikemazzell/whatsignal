package features

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlagManager(t *testing.T) {
	fm := NewFlagManager()
	assert.NotNil(t, fm)
	assert.NotNil(t, fm.flags)
	assert.Len(t, fm.flags, 0)
}

func TestFlagManager_InitializeDefaults(t *testing.T) {
	fm := NewFlagManager()
	fm.InitializeDefaults()

	// Check that all default flags are created
	flags := fm.ListFlags()
	assert.Greater(t, len(flags), 0)

	// Check specific flags exist
	assert.True(t, fm.IsEnabled(FlagAsyncProcessing))
	assert.True(t, fm.IsEnabled(FlagEnhancedLogging))
	assert.False(t, fm.IsEnabled(FlagNewContactSync)) // experimental flag

	// Verify flag properties
	flag, err := fm.GetFlag(FlagAsyncProcessing)
	require.NoError(t, err)
	assert.Equal(t, FlagAsyncProcessing, flag.Name)
	assert.Contains(t, flag.Tags, "core")
	assert.Equal(t, 100, flag.Percentage)
}

func TestFlagManager_IsEnabled(t *testing.T) {
	fm := NewFlagManager()

	// Non-existent flag should return false
	assert.False(t, fm.IsEnabled("nonexistent"))

	// Create a flag
	err := fm.CreateFlag("test_flag", "Test flag", true, []string{"test"})
	require.NoError(t, err)

	assert.True(t, fm.IsEnabled("test_flag"))

	// Disable the flag
	err = fm.Disable("test_flag")
	require.NoError(t, err)

	assert.False(t, fm.IsEnabled("test_flag"))
}

func TestFlagManager_Enable_Disable(t *testing.T) {
	fm := NewFlagManager()

	// Test enabling non-existent flag
	err := fm.Enable("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, ErrFlagNotFound{}, err)

	// Create a flag
	err = fm.CreateFlag("test_flag", "Test flag", false, nil)
	require.NoError(t, err)

	// Enable it
	err = fm.Enable("test_flag")
	require.NoError(t, err)
	assert.True(t, fm.IsEnabled("test_flag"))

	// Disable it
	err = fm.Disable("test_flag")
	require.NoError(t, err)
	assert.False(t, fm.IsEnabled("test_flag"))

	// Test disabling non-existent flag
	err = fm.Disable("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, ErrFlagNotFound{}, err)
}

func TestFlagManager_SetPercentage(t *testing.T) {
	fm := NewFlagManager()

	// Create a flag
	err := fm.CreateFlag("test_flag", "Test flag", true, nil)
	require.NoError(t, err)

	// Set valid percentage
	err = fm.SetPercentage("test_flag", 50)
	require.NoError(t, err)

	flag, err := fm.GetFlag("test_flag")
	require.NoError(t, err)
	assert.Equal(t, 50, flag.Percentage)

	// Test invalid percentages
	err = fm.SetPercentage("test_flag", -1)
	assert.Error(t, err)
	assert.IsType(t, ErrInvalidPercentage{}, err)

	err = fm.SetPercentage("test_flag", 101)
	assert.Error(t, err)
	assert.IsType(t, ErrInvalidPercentage{}, err)

	// Test non-existent flag
	err = fm.SetPercentage("nonexistent", 50)
	assert.Error(t, err)
	assert.IsType(t, ErrFlagNotFound{}, err)
}

func TestFlagManager_CreateFlag(t *testing.T) {
	fm := NewFlagManager()

	// Create a new flag
	err := fm.CreateFlag("new_flag", "A new test flag", true, []string{"test", "new"})
	require.NoError(t, err)

	// Verify it was created
	flag, err := fm.GetFlag("new_flag")
	require.NoError(t, err)
	assert.Equal(t, "new_flag", flag.Name)
	assert.Equal(t, "A new test flag", flag.Description)
	assert.True(t, flag.Enabled)
	assert.Contains(t, flag.Tags, "test")
	assert.Contains(t, flag.Tags, "new")
	assert.Equal(t, 100, flag.Percentage)

	// Try to create duplicate flag
	err = fm.CreateFlag("new_flag", "Duplicate", false, nil)
	assert.Error(t, err)
	assert.IsType(t, ErrFlagExists{}, err)
}

func TestFlagManager_DeleteFlag(t *testing.T) {
	fm := NewFlagManager()

	// Try to delete non-existent flag
	err := fm.DeleteFlag("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, ErrFlagNotFound{}, err)

	// Create and delete a flag
	err = fm.CreateFlag("temp_flag", "Temporary flag", true, nil)
	require.NoError(t, err)

	// Verify it exists
	_, err = fm.GetFlag("temp_flag")
	require.NoError(t, err)

	// Delete it
	err = fm.DeleteFlag("temp_flag")
	require.NoError(t, err)

	// Verify it's gone
	_, err = fm.GetFlag("temp_flag")
	assert.Error(t, err)
	assert.IsType(t, ErrFlagNotFound{}, err)
}

func TestFlagManager_GetFlag(t *testing.T) {
	fm := NewFlagManager()

	// Test non-existent flag
	_, err := fm.GetFlag("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, ErrFlagNotFound{}, err)

	// Create a flag with tags
	originalTags := []string{"test", "example"}
	err = fm.CreateFlag("test_flag", "Test flag", true, originalTags)
	require.NoError(t, err)

	// Get the flag
	flag, err := fm.GetFlag("test_flag")
	require.NoError(t, err)

	// Verify it's a copy (modify returned tags shouldn't affect original)
	flag.Tags[0] = "modified"

	flag2, err := fm.GetFlag("test_flag")
	require.NoError(t, err)
	assert.Equal(t, "test", flag2.Tags[0]) // Should be unchanged
}

func TestFlagManager_ListFlags(t *testing.T) {
	fm := NewFlagManager()

	// Create flags with different tags
	err := fm.CreateFlag("flag1", "Flag 1", true, []string{"core", "stable"})
	require.NoError(t, err)

	err = fm.CreateFlag("flag2", "Flag 2", false, []string{"experimental"})
	require.NoError(t, err)

	err = fm.CreateFlag("flag3", "Flag 3", true, []string{"core", "experimental"})
	require.NoError(t, err)

	// List all flags
	allFlags := fm.ListFlags()
	assert.Len(t, allFlags, 3)

	// Filter by core tag
	coreFlags := fm.ListFlags("core")
	assert.Len(t, coreFlags, 2)

	// Filter by experimental tag
	expFlags := fm.ListFlags("experimental")
	assert.Len(t, expFlags, 2)

	// Filter by stable tag
	stableFlags := fm.ListFlags("stable")
	assert.Len(t, stableFlags, 1)

	// Filter by non-existent tag
	noneFlags := fm.ListFlags("nonexistent")
	assert.Len(t, noneFlags, 0)
}

func TestFlagManager_ExportImportJSON(t *testing.T) {
	fm := NewFlagManager()

	// Create some flags
	err := fm.CreateFlag("flag1", "Flag 1", true, []string{"test"})
	require.NoError(t, err)

	err = fm.CreateFlag("flag2", "Flag 2", false, []string{"test", "experimental"})
	require.NoError(t, err)

	err = fm.SetPercentage("flag1", 75)
	require.NoError(t, err)

	// Export to JSON
	jsonData, err := fm.ExportJSON()
	require.NoError(t, err)

	// Verify JSON structure
	var exported []*Flag
	err = json.Unmarshal(jsonData, &exported)
	require.NoError(t, err)
	assert.Len(t, exported, 2)

	// Create new manager and import
	fm2 := NewFlagManager()
	err = fm2.ImportJSON(jsonData)
	require.NoError(t, err)

	// Verify flags were imported
	assert.True(t, fm2.IsEnabled("flag1"))
	assert.False(t, fm2.IsEnabled("flag2"))

	flag1, err := fm2.GetFlag("flag1")
	require.NoError(t, err)
	assert.Equal(t, 75, flag1.Percentage)
}

func TestFlagManager_ThreadSafety(t *testing.T) {
	fm := NewFlagManager()
	fm.InitializeDefaults()

	// Run concurrent operations
	done := make(chan bool, 10)

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				fm.IsEnabled(FlagAsyncProcessing)
				fm.ListFlags()
			}
			done <- true
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				flagName := fmt.Sprintf("test_flag_%d_%d", id, j)
				_ = fm.CreateFlag(flagName, "Test flag", true, []string{"test"})
				_ = fm.Enable(flagName)
				_ = fm.Disable(flagName)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify manager is still functional
	flags := fm.ListFlags()
	assert.Greater(t, len(flags), 0)
}

func TestDefaultFlags_Consistency(t *testing.T) {
	// Verify all flag constants have corresponding definitions
	expectedFlags := []string{
		FlagAsyncProcessing,
		FlagEnhancedLogging,
		FlagDistributedTracing,
		FlagCircuitBreaker,
		FlagRateLimiting,
		FlagWebhookRetries,
		FlagMediaCompression,
		FlagBatchOperations,
		FlagAdvancedAuth,
		FlagEncryptionV2,
		FlagAuditLogging,
		FlagRequestValidation,
		FlagNewContactSync,
		FlagOptimizedQueries,
		FlagCacheWarmup,
		FlagMessageBatching,
	}

	// Create map of defined flags
	definedFlags := make(map[string]bool)
	for _, def := range DefaultFlags {
		definedFlags[def.Name] = true
	}

	// Check that all constants have definitions
	for _, flagName := range expectedFlags {
		assert.True(t, definedFlags[flagName], "Flag constant %s missing from DefaultFlags", flagName)
	}

	// Check that all definitions have valid properties
	for _, def := range DefaultFlags {
		assert.NotEmpty(t, def.Name, "Flag definition missing name")
		assert.NotEmpty(t, def.Description, "Flag %s missing description", def.Name)
		assert.NotEmpty(t, def.Tags, "Flag %s missing tags", def.Name)
	}
}

func TestGlobalFunctions(t *testing.T) {
	// Reset global manager for testing
	globalFlagManager = NewFlagManager()

	// Initialize defaults
	Initialize()

	// Test global functions
	assert.True(t, IsEnabled(FlagAsyncProcessing))

	err := Disable(FlagAsyncProcessing)
	require.NoError(t, err)
	assert.False(t, IsEnabled(FlagAsyncProcessing))

	err = Enable(FlagAsyncProcessing)
	require.NoError(t, err)
	assert.True(t, IsEnabled(FlagAsyncProcessing))

	// Test getting global manager
	manager := GetGlobalManager()
	assert.NotNil(t, manager)
	assert.Equal(t, globalFlagManager, manager)
}

func TestCustomErrors(t *testing.T) {
	// Test ErrFlagNotFound
	err := ErrFlagNotFound{Name: "test_flag"}
	assert.Equal(t, "feature flag not found: test_flag", err.Error())

	// Test ErrFlagExists
	err2 := ErrFlagExists{Name: "existing_flag"}
	assert.Equal(t, "feature flag already exists: existing_flag", err2.Error())

	// Test ErrInvalidPercentage
	err3 := ErrInvalidPercentage{Percentage: 150}
	assert.Equal(t, "invalid percentage: 150 (must be 0-100)", err3.Error())
}

func TestFlag_Timestamps(t *testing.T) {
	fm := NewFlagManager()

	beforeCreate := time.Now()
	err := fm.CreateFlag("time_test", "Time test flag", false, nil)
	require.NoError(t, err)
	afterCreate := time.Now()

	flag, err := fm.GetFlag("time_test")
	require.NoError(t, err)

	// Check creation time
	assert.True(t, flag.CreatedAt.After(beforeCreate) || flag.CreatedAt.Equal(beforeCreate))
	assert.True(t, flag.CreatedAt.Before(afterCreate) || flag.CreatedAt.Equal(afterCreate))
	assert.Equal(t, flag.CreatedAt, flag.UpdatedAt) // Should be same initially

	// Update the flag
	time.Sleep(1 * time.Millisecond) // Ensure time difference
	beforeUpdate := time.Now()
	err = fm.Enable("time_test")
	require.NoError(t, err)
	afterUpdate := time.Now()

	flag, err = fm.GetFlag("time_test")
	require.NoError(t, err)

	// Check update time
	assert.True(t, flag.UpdatedAt.After(beforeUpdate) || flag.UpdatedAt.Equal(beforeUpdate))
	assert.True(t, flag.UpdatedAt.Before(afterUpdate) || flag.UpdatedAt.Equal(afterUpdate))
	assert.True(t, flag.UpdatedAt.After(flag.CreatedAt))
}
