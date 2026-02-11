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

// ContactServiceInterface defines the interface for contact operations
type ContactServiceInterface interface {
	GetContactDisplayName(ctx context.Context, phoneNumber string) string
	RefreshContact(ctx context.Context, phoneNumber string) error
	SyncAllContacts(ctx context.Context) error
	CleanupOldContacts(ctx context.Context, retentionDays int) error
}

// ContactDatabaseService defines the database operations needed by ContactService
type ContactDatabaseService interface {
	SaveContact(ctx context.Context, contact *models.Contact) error
	GetContact(ctx context.Context, contactID string) (*models.Contact, error)
	GetContactByPhone(ctx context.Context, phoneNumber string) (*models.Contact, error)
	CleanupOldContacts(ctx context.Context, retentionDays int) error
}

// ContactService provides contact caching and retrieval functionality
type ContactService struct {
	db              ContactDatabaseService
	waClient        types.WAClient
	cacheValidHours int
	logger          *errors.Logger
	circuitBreaker  *CircuitBreaker
	degradedMode    bool
}

// NewContactService creates a new contact service instance
func NewContactService(db ContactDatabaseService, waClient types.WAClient) *ContactService {
	return &ContactService{
		db:              db,
		waClient:        waClient,
		cacheValidHours: constants.DefaultContactCacheHours,
		logger:          errors.NewLogger(),
		circuitBreaker:  NewCircuitBreaker("whatsapp-contact-api", constants.ContactCBMaxFailures, time.Duration(constants.ContactCBResetTimeoutSec)*time.Second),
		degradedMode:    false,
	}
}

// NewContactServiceWithConfig creates a new contact service instance with custom cache duration
func NewContactServiceWithConfig(db ContactDatabaseService, waClient types.WAClient, cacheValidHours int) *ContactService {
	if cacheValidHours <= 0 {
		cacheValidHours = constants.DefaultContactCacheHours
	}
	return &ContactService{
		db:              db,
		waClient:        waClient,
		cacheValidHours: cacheValidHours,
		logger:          errors.NewLogger(),
		circuitBreaker:  NewCircuitBreaker("whatsapp-contact-api", constants.ContactCBMaxFailures, time.Duration(constants.ContactCBResetTimeoutSec)*time.Second),
		degradedMode:    false,
	}
}

// GetContactDisplayName retrieves the display name for a phone number/contact ID
// It first checks the cache, then fetches from WhatsApp API if needed
// For group chats, it returns the phone number directly without API calls
func (cs *ContactService) GetContactDisplayName(ctx context.Context, phoneNumber string) string {
	// Check if this is a group chat - groups use @g.us suffix
	// Groups don't have contact info in WAHA's Contacts API, only in Groups API
	if strings.HasSuffix(phoneNumber, "@g.us") || strings.Contains(phoneNumber, "@g.us") {
		cs.logger.WithContext(logrus.Fields{
			"phone_number": phoneNumber,
			"type":         "group",
		}).Debug("Skipping contact lookup for group chat")
		// Return the group ID as-is; groups don't have display names in contacts API
		return phoneNumber
	}

	// Handle LID (Linked ID) format - WhatsApp internal user identifiers
	// LIDs may not be resolvable via the standard contacts API
	if strings.HasSuffix(phoneNumber, "@lid") {
		cs.logger.WithContext(logrus.Fields{
			"phone_number": phoneNumber,
			"type":         "lid",
		}).Debug("LID format detected, using as-is")
		// Strip the @lid suffix and return just the numeric ID
		return strings.TrimSuffix(phoneNumber, "@lid")
	}

	// Try to get from cache first
	contact, err := cs.db.GetContactByPhone(ctx, phoneNumber)
	if err != nil {
		cs.logger.LogWarn(
			errors.Wrap(err, errors.ErrCodeDatabaseQuery, "failed to retrieve contact from cache"),
			"Contact cache lookup failed",
			logrus.Fields{"phone_number": phoneNumber},
		)
	}

	// If found in cache and not too old, use it
	cacheValidDuration := time.Duration(cs.cacheValidHours) * time.Hour
	if contact != nil && time.Since(contact.CachedAt) < cacheValidDuration {
		// Record cache hit
		metrics.IncrementCounter("contact_cache_hits_total", nil, "Total contact cache hits")
		return contact.GetDisplayName()
	}

	// Record cache miss - need to fetch from API
	metrics.IncrementCounter("contact_cache_misses_total", nil, "Total contact cache misses")

	// Fetch from WhatsApp API - only for individual contacts (@c.us)
	contactID := phoneNumber
	if !strings.HasSuffix(contactID, "@c.us") {
		contactID = phoneNumber + "@c.us"
	}

	// Try to fetch from WhatsApp API with circuit breaker protection
	var waContact *types.Contact
	err = cs.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		var apiErr error
		waContact, apiErr = cs.waClient.GetContact(ctx, contactID)
		return apiErr
	})

	if err != nil {
		// Check if this is a circuit breaker error (service degraded)
		if errors.GetCode(err) == errors.ErrCodeInternalError &&
			err.Error() == "circuit breaker is open" {
			cs.degradedMode = true
			cs.logger.LogWarn(err, "WhatsApp API circuit breaker open, using degraded mode",
				logrus.Fields{"contact_id": contactID, "phone_number": phoneNumber})
		} else {
			cs.logger.LogWarn(
				errors.WrapRetryable(err, errors.ErrCodeWhatsAppAPI, "failed to fetch contact from WhatsApp API"),
				"WhatsApp API contact fetch failed",
				logrus.Fields{"contact_id": contactID, "phone_number": phoneNumber},
			)
		}

		// Graceful degradation: use cached version even if old, or phone number
		if contact != nil {
			cs.logger.WithContext(logrus.Fields{
				"contact_id":       contactID,
				"phone_number":     phoneNumber,
				"cached_age_hours": time.Since(contact.CachedAt).Hours(),
			}).Info("Using cached contact in degraded mode")
			return contact.GetDisplayName()
		}
		return phoneNumber
	}

	// If contact not found in WhatsApp, return phone number
	if waContact == nil {
		return phoneNumber
	}

	// Save/update in cache
	dbContact := &models.Contact{}
	dbContact.FromWAContact(waContact)

	if err := cs.db.SaveContact(ctx, dbContact); err != nil {
		cs.logger.LogWarn(
			errors.Wrap(err, errors.ErrCodeDatabaseQuery, "failed to save contact to cache"),
			"Contact cache save failed",
			logrus.Fields{"contact_id": waContact.ID, "phone_number": phoneNumber},
		)
	} else {
		// Record successful cache refresh
		metrics.IncrementCounter("contact_cache_refreshes_total", nil, "Total contact cache refreshes")
	}

	return waContact.GetDisplayName()
}

// RefreshContact forces a refresh of a specific contact from WhatsApp API
func (cs *ContactService) RefreshContact(ctx context.Context, phoneNumber string) error {
	contactID := phoneNumber
	if !strings.HasSuffix(contactID, "@c.us") {
		contactID = phoneNumber + "@c.us"
	}

	waContact, err := cs.waClient.GetContact(ctx, contactID)
	if err != nil {
		return fmt.Errorf("failed to fetch contact from WhatsApp API: %w", err)
	}

	if waContact == nil {
		return fmt.Errorf("contact not found: %s", phoneNumber)
	}

	dbContact := &models.Contact{}
	dbContact.FromWAContact(waContact)

	return cs.db.SaveContact(ctx, dbContact)
}

// SyncAllContacts fetches all contacts from WhatsApp and updates the cache
func (cs *ContactService) SyncAllContacts(ctx context.Context) error {
	sessionName := cs.waClient.GetSessionName()
	batchSize := constants.DefaultContactSyncBatchSize
	offset := 0

	cs.logger.WithContext(logrus.Fields{
		"session":    sessionName,
		"batch_size": batchSize,
	}).Info("Starting contact sync")

	for {
		cs.logger.WithContext(logrus.Fields{
			"session": sessionName,
			"offset":  offset,
			"batch":   offset/batchSize + 1,
		}).Debug("Fetching contacts batch")

		contacts, err := cs.waClient.GetAllContacts(ctx, batchSize, offset)
		if err != nil {
			contactErr := errors.WrapRetryable(err, errors.ErrCodeWhatsAppAPI, "failed to fetch contacts batch")
			cs.logger.LogError(contactErr, "Contact sync batch failed", logrus.Fields{
				"session": sessionName,
				"offset":  offset,
			})
			return contactErr
		}

		if len(contacts) == 0 {
			break // No more contacts
		}

		// Save contacts to cache
		for _, waContact := range contacts {
			dbContact := &models.Contact{}
			dbContact.FromWAContact(&waContact)

			if err := cs.db.SaveContact(ctx, dbContact); err != nil {
				cs.logger.LogWarn(
					errors.Wrap(err, errors.ErrCodeDatabaseQuery, "failed to save contact to cache during sync"),
					"Contact sync save failed",
					logrus.Fields{
						"session":    sessionName,
						"contact_id": waContact.ID,
					},
				)
				continue
			}
		}

		cs.logger.WithContext(logrus.Fields{
			"session":         sessionName,
			"synced_count":    len(contacts),
			"batch":           offset/batchSize + 1,
			"total_processed": offset + len(contacts),
		}).Info("Contact batch synced successfully")

		// If we got fewer than batch size, we're done
		if len(contacts) < batchSize {
			break
		}

		offset += batchSize

		// Add a small delay to avoid overwhelming the API
		select {
		case <-time.After(time.Duration(constants.DefaultContactSyncDelayMs) * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// CleanupOldContacts removes contacts older than the specified retention period
func (cs *ContactService) CleanupOldContacts(ctx context.Context, retentionDays int) error {
	return cs.db.CleanupOldContacts(ctx, retentionDays)
}
