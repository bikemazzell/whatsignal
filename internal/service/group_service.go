package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/errors"
	"whatsignal/internal/metrics"
	"whatsignal/internal/models"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

// GroupServiceInterface defines the interface for group operations
type GroupServiceInterface interface {
	GetGroupName(ctx context.Context, groupID, sessionName string) string
	RefreshGroup(ctx context.Context, groupID, sessionName string) error
	SyncAllGroups(ctx context.Context, sessionName string) error
	CleanupOldGroups(ctx context.Context, retentionDays int) error
}

// GroupDatabaseService defines the database operations needed by GroupService
type GroupDatabaseService interface {
	SaveGroup(ctx context.Context, group *models.Group) error
	GetGroup(ctx context.Context, groupID, sessionName string) (*models.Group, error)
	CleanupOldGroups(ctx context.Context, retentionDays int) error
}

// GroupService provides group caching and retrieval functionality
type GroupService struct {
	db              GroupDatabaseService
	waClient        types.WAClient
	cacheValidHours int
	logger          *errors.Logger
	circuitBreaker  *CircuitBreaker
	degradedMode    bool
}

// NewGroupService creates a new group service instance
func NewGroupService(db GroupDatabaseService, waClient types.WAClient) *GroupService {
	return &GroupService{
		db:              db,
		waClient:        waClient,
		cacheValidHours: 24, // Default to 24 hours
		logger:          errors.NewLogger(),
		circuitBreaker:  NewCircuitBreaker("whatsapp-groups-api", 5, 30*time.Second),
		degradedMode:    false,
	}
}

// NewGroupServiceWithConfig creates a new group service instance with custom cache duration
func NewGroupServiceWithConfig(db GroupDatabaseService, waClient types.WAClient, cacheValidHours int) *GroupService {
	if cacheValidHours <= 0 {
		cacheValidHours = 24 // Default fallback
	}
	return &GroupService{
		db:              db,
		waClient:        waClient,
		cacheValidHours: cacheValidHours,
		logger:          errors.NewLogger(),
		circuitBreaker:  NewCircuitBreaker("whatsapp-groups-api", 5, 30*time.Second),
		degradedMode:    false,
	}
}

// GetGroupName retrieves the display name for a group ID
// It first checks the cache, then fetches from WhatsApp API if needed
// Returns the group ID as fallback if the API fails
func (gs *GroupService) GetGroupName(ctx context.Context, groupID, sessionName string) string {
	// Validate that this is a group ID
	if !strings.HasSuffix(groupID, "@g.us") {
		gs.logger.WithContext(logrus.Fields{
			"group_id": groupID,
		}).Warn("Invalid group ID format (missing @g.us suffix)")
		return groupID
	}

	// Try to get from cache first
	group, err := gs.db.GetGroup(ctx, groupID, sessionName)
	if err != nil {
		gs.logger.LogWarn(
			errors.Wrap(err, errors.ErrCodeDatabaseQuery, "failed to retrieve group from cache"),
			"Group cache lookup failed",
			logrus.Fields{"group_id": groupID, "session": sessionName},
		)
	}

	// If found in cache and not too old, use it
	cacheValidDuration := time.Duration(gs.cacheValidHours) * time.Hour
	if group != nil && time.Since(group.CachedAt) < cacheValidDuration {
		// Record cache hit
		metrics.IncrementCounter("group_cache_hits_total", nil, "Total group cache hits")
		return group.GetDisplayName()
	}

	// Record cache miss - need to fetch from API
	metrics.IncrementCounter("group_cache_misses_total", nil, "Total group cache misses")

	// Try to fetch from WhatsApp API with circuit breaker protection
	var waGroup *types.Group
	err = gs.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		var apiErr error
		waGroup, apiErr = gs.waClient.GetGroup(ctx, groupID)
		return apiErr
	})

	if err != nil {
		// Check if this is a circuit breaker error (service degraded)
		if errors.GetCode(err) == errors.ErrCodeInternalError &&
			err.Error() == "circuit breaker is open" {
			gs.degradedMode = true
			gs.logger.LogWarn(err, "WhatsApp Groups API circuit breaker open, using degraded mode",
				logrus.Fields{"group_id": groupID, "session": sessionName})
		} else {
			gs.logger.LogWarn(
				errors.WrapRetryable(err, errors.ErrCodeWhatsAppAPI, "failed to fetch group from WhatsApp API"),
				"WhatsApp API group fetch failed",
				logrus.Fields{"group_id": groupID, "session": sessionName},
			)
		}

		// Graceful degradation: use cached version even if old, or group ID
		if group != nil {
			gs.logger.WithContext(logrus.Fields{
				"group_id":         groupID,
				"session":          sessionName,
				"cached_age_hours": time.Since(group.CachedAt).Hours(),
			}).Info("Using cached group in degraded mode")
			return group.GetDisplayName()
		}
		return groupID
	}

	// If group not found in WhatsApp, return group ID
	if waGroup == nil {
		return groupID
	}

	// Save/update in cache
	dbGroup := &models.Group{}
	dbGroup.FromWAGroup(waGroup, sessionName)

	if err := gs.db.SaveGroup(ctx, dbGroup); err != nil {
		gs.logger.LogWarn(
			errors.Wrap(err, errors.ErrCodeDatabaseQuery, "failed to save group to cache"),
			"Group cache save failed",
			logrus.Fields{"group_id": waGroup.ID, "session": sessionName},
		)
	} else {
		// Record successful cache refresh
		metrics.IncrementCounter("group_cache_refreshes_total", nil, "Total group cache refreshes")
	}

	return waGroup.GetDisplayName()
}

// RefreshGroup forces a refresh of a specific group from WhatsApp API
func (gs *GroupService) RefreshGroup(ctx context.Context, groupID, sessionName string) error {
	if !strings.HasSuffix(groupID, "@g.us") {
		return fmt.Errorf("invalid group ID format: %s", groupID)
	}

	waGroup, err := gs.waClient.GetGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("failed to fetch group from WhatsApp API: %w", err)
	}

	if waGroup == nil {
		return fmt.Errorf("group not found: %s", groupID)
	}

	dbGroup := &models.Group{}
	dbGroup.FromWAGroup(waGroup, sessionName)

	return gs.db.SaveGroup(ctx, dbGroup)
}

// SyncAllGroups fetches all groups from WhatsApp and updates the cache
func (gs *GroupService) SyncAllGroups(ctx context.Context, sessionName string) error {
	batchSize := constants.DefaultContactSyncBatchSize // Reuse same constant for groups
	offset := 0

	gs.logger.WithContext(logrus.Fields{
		"session":    sessionName,
		"batch_size": batchSize,
	}).Info("Starting group sync")

	totalGroups := 0
	for {
		groups, err := gs.waClient.GetAllGroups(ctx, batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch groups at offset %d: %w", offset, err)
		}

		if len(groups) == 0 {
			break
		}

		// Save each group to cache
		for _, waGroup := range groups {
			dbGroup := &models.Group{}
			dbGroup.FromWAGroup(&waGroup, sessionName)

			if err := gs.db.SaveGroup(ctx, dbGroup); err != nil {
				gs.logger.LogWarn(
					errors.Wrap(err, errors.ErrCodeDatabaseQuery, "failed to save group during sync"),
					"Group sync save failed",
					logrus.Fields{"group_id": waGroup.ID, "session": sessionName},
				)
			}
		}

		totalGroups += len(groups)
		offset += batchSize

		// If we got fewer groups than batch size, we've reached the end
		if len(groups) < batchSize {
			break
		}
	}

	gs.logger.WithContext(logrus.Fields{
		"session":      sessionName,
		"total_groups": totalGroups,
	}).Info("Group sync completed")

	return nil
}

// CleanupOldGroups removes groups older than the specified retention period
func (gs *GroupService) CleanupOldGroups(ctx context.Context, retentionDays int) error {
	if err := gs.db.CleanupOldGroups(ctx, retentionDays); err != nil {
		return fmt.Errorf("failed to cleanup old groups: %w", err)
	}

	gs.logger.WithContext(logrus.Fields{
		"retention_days": retentionDays,
	}).Info("Old groups cleaned up")

	return nil
}
