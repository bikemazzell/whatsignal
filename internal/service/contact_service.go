package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/models"
	"whatsignal/pkg/whatsapp/types"
)

// ContactServiceInterface defines the interface for contact operations
type ContactServiceInterface interface {
	GetContactDisplayName(ctx context.Context, phoneNumber string) string
	RefreshContact(ctx context.Context, phoneNumber string) error
	SyncAllContacts(ctx context.Context) error
	CleanupOldContacts(retentionDays int) error
}

// ContactDatabaseService defines the database operations needed by ContactService
type ContactDatabaseService interface {
	SaveContact(ctx context.Context, contact *models.Contact) error
	GetContact(ctx context.Context, contactID string) (*models.Contact, error)
	GetContactByPhone(ctx context.Context, phoneNumber string) (*models.Contact, error)
	CleanupOldContacts(retentionDays int) error
}

// ContactService provides contact caching and retrieval functionality
type ContactService struct {
	db             ContactDatabaseService
	waClient       types.WAClient
	cacheValidHours int
}

// NewContactService creates a new contact service instance
func NewContactService(db ContactDatabaseService, waClient types.WAClient) *ContactService {
	return &ContactService{
		db:             db,
		waClient:       waClient,
		cacheValidHours: 24, // Default to 24 hours
	}
}

// NewContactServiceWithConfig creates a new contact service instance with custom cache duration
func NewContactServiceWithConfig(db ContactDatabaseService, waClient types.WAClient, cacheValidHours int) *ContactService {
	if cacheValidHours <= 0 {
		cacheValidHours = 24 // Default fallback
	}
	return &ContactService{
		db:             db,
		waClient:       waClient,
		cacheValidHours: cacheValidHours,
	}
}

// GetContactDisplayName retrieves the display name for a phone number/contact ID
// It first checks the cache, then fetches from WhatsApp API if needed
func (cs *ContactService) GetContactDisplayName(ctx context.Context, phoneNumber string) string {
	// Try to get from cache first
	contact, err := cs.db.GetContactByPhone(ctx, phoneNumber)
	if err != nil {
		log.Printf("Error retrieving contact from cache: %v", err)
	}

	// If found in cache and not too old, use it
	cacheValidDuration := time.Duration(cs.cacheValidHours) * time.Hour
	if contact != nil && time.Since(contact.CachedAt) < cacheValidDuration {
		return contact.GetDisplayName()
	}

	// Fetch from WhatsApp API
	contactID := phoneNumber
	if !strings.HasSuffix(contactID, "@c.us") {
		contactID = phoneNumber + "@c.us"
	}

	waContact, err := cs.waClient.GetContact(ctx, contactID)
	if err != nil {
		log.Printf("Error fetching contact from WhatsApp API: %v", err)
		// Fallback to cached version even if old, or phone number
		if contact != nil {
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
		log.Printf("Error saving contact to cache: %v", err)
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

	for {
		contacts, err := cs.waClient.GetAllContacts(ctx, batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch contacts batch (offset %d): %w", offset, err)
		}

		if len(contacts) == 0 {
			break // No more contacts
		}

		// Save contacts to cache
		for _, waContact := range contacts {
			dbContact := &models.Contact{}
			dbContact.FromWAContact(&waContact)
			
			if err := cs.db.SaveContact(ctx, dbContact); err != nil {
				log.Printf("[%s] Error saving contact %s to cache: %v", sessionName, waContact.ID, err)
				continue
			}
		}

		log.Printf("[%s] Synced %d contacts (batch %d)", sessionName, len(contacts), offset/batchSize+1)

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
func (cs *ContactService) CleanupOldContacts(retentionDays int) error {
	return cs.db.CleanupOldContacts(retentionDays)
}