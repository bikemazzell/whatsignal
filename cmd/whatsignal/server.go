package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"whatsignal/internal/constants"
	"whatsignal/internal/middleware"
	"whatsignal/internal/models"
	"whatsignal/internal/service"
	"whatsignal/pkg/whatsapp/types"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	XWahaSignatureHeader = "X-Webhook-Hmac"
)

// ValidationError represents a validation error that should return HTTP 400
type ValidationError struct {
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Message
}

type Server struct {
	router     *mux.Router
	logger     *logrus.Logger
	msgService service.MessageService

	serverMu       sync.RWMutex
	server         *http.Server
	cfg            *models.Config
	waClient       types.WAClient
	channelManager *service.ChannelManager
	rateLimiter    *RateLimiter
}

func NewServer(cfg *models.Config, msgService service.MessageService, logger *logrus.Logger, waClient types.WAClient, channelManager *service.ChannelManager) *Server {
	// Use configured rate limit or default
	rateLimit := cfg.Server.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = constants.DefaultRateLimitPerMinute
	}

	// Use configured cleanup period or default
	cleanupMinutes := cfg.Server.RateLimitCleanupMinutes
	if cleanupMinutes <= 0 {
		cleanupMinutes = constants.DefaultRateLimitCleanupMinutes
	}

	s := &Server{
		router:         mux.NewRouter(),
		logger:         logger,
		msgService:     msgService,
		cfg:            cfg,
		waClient:       waClient,
		channelManager: channelManager,
		rateLimiter:    NewRateLimiter(rateLimit, time.Minute, time.Duration(cleanupMinutes)*time.Minute),
	}

	s.setupRoutes()

	return s
}

func (s *Server) setupRoutes() {
	// Add global observability middleware for all routes
	s.router.Use(middleware.ObservabilityMiddleware(s.logger))

	// Public endpoints (no rate limiting for health checks)
	s.router.HandleFunc("/health", s.handleHealth()).Methods(http.MethodGet)
	s.router.HandleFunc("/session/status", s.handleSessionStatus()).Methods(http.MethodGet)
	s.router.HandleFunc("/metrics", s.handleMetrics()).Methods(http.MethodGet)

	// Webhook endpoints with security middleware and webhook-specific observability
	whatsapp := s.router.PathPrefix("/webhook/whatsapp").Subrouter()
	whatsapp.Use(s.securityMiddleware)
	whatsapp.Use(middleware.WebhookObservabilityMiddleware(s.logger, "whatsapp"))
	whatsapp.HandleFunc("", s.handleWhatsAppWebhook()).Methods(http.MethodPost)

}

func (s *Server) Start() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(constants.DefaultServerPort)
	}

	// Use configured timeouts or defaults
	readTimeout := time.Duration(s.cfg.Server.ReadTimeoutSec) * time.Second
	if readTimeout <= 0 {
		readTimeout = time.Duration(constants.DefaultServerReadTimeoutSec) * time.Second
	}

	writeTimeout := time.Duration(s.cfg.Server.WriteTimeoutSec) * time.Second
	if writeTimeout <= 0 {
		writeTimeout = time.Duration(constants.DefaultServerWriteTimeoutSec) * time.Second
	}

	idleTimeout := time.Duration(s.cfg.Server.IdleTimeoutSec) * time.Second
	if idleTimeout <= 0 {
		idleTimeout = time.Duration(constants.DefaultServerIdleTimeoutSec) * time.Second
	}

	s.serverMu.Lock()
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	s.serverMu.Unlock()

	s.logger.Infof("Starting server on port %s", port)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	// Stop the rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}

	s.serverMu.RLock()
	server := s.server
	s.serverMu.RUnlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

// securityMiddleware applies security measures to webhook endpoints
func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting
		clientIP := GetClientIP(r)
		rateLimitInfo := s.rateLimiter.AllowWithInfo(clientIP)

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimitInfo.Limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", rateLimitInfo.Remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", rateLimitInfo.ResetTime.Unix()))

		if !rateLimitInfo.Allowed {
			s.logger.WithField("remote_ip", clientIP).Warn("Rate limit exceeded")
			// Add Retry-After header indicating when they can try again
			retryAfter := int(time.Until(rateLimitInfo.ResetTime).Seconds())
			if retryAfter > 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			}
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content-Type validation and size limit for requests with bodies
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			// Enforce max request size for all endpoints with bodies
			maxBytes := s.cfg.Server.WebhookMaxBytes
			if maxBytes <= 0 {
				maxBytes = constants.DefaultWebhookMaxBytes
			}
			r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

			// Additional validation for webhook endpoints (POST to webhook paths)
			if r.Method == http.MethodPost && (strings.Contains(r.URL.Path, "webhook") || strings.Contains(r.URL.Path, "whatsapp") || strings.Contains(r.URL.Path, "signal")) {
				contentType := r.Header.Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					s.logger.WithFields(logrus.Fields{
						"remote_ip":    clientIP,
						"content_type": contentType,
					}).Warn("Invalid content type for webhook")
					http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
					return
				}
			}
		}

		// Log security-relevant information
		s.logger.WithFields(logrus.Fields{
			"remote_ip":  clientIP,
			"method":     r.Method,
			"path":       r.URL.Path,
			"user_agent": r.Header.Get("User-Agent"),
		}).Debug("Webhook request received")

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":  "healthy",
			"version": Version,
			"build": map[string]string{
				"time":   BuildTime,
				"commit": GitCommit,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(health); err != nil {
			s.logger.WithError(err).Error("Failed to write health check response")
		}
	}
}

func (s *Server) handleSessionStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(constants.DefaultSessionStatusTimeoutSec)*time.Second)
		defer cancel()

		session, err := s.waClient.GetSessionStatus(ctx)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get session status")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Failed to get session status",
				"details": err.Error(),
			}); err != nil {
				s.logger.WithError(err).Error("Failed to write error response")
			}
			return
		}

		sessionStatus := map[string]interface{}{
			"name":       session.Name,
			"status":     string(session.Status),
			"healthy":    string(session.Status) == "WORKING",
			"updated_at": session.UpdatedAt,
		}

		// Add config info
		sessionStatus["config"] = map[string]interface{}{
			"auto_restart_enabled":      s.cfg.WhatsApp.SessionAutoRestart,
			"health_check_interval_sec": s.cfg.WhatsApp.SessionHealthCheckSec,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(sessionStatus); err != nil {
			s.logger.WithError(err).Error("Failed to write session status response")
		}
	}
}

func (s *Server) handleWhatsAppWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug("Processing WhatsApp webhook request")

		maxSkewSec := s.cfg.Server.WebhookMaxSkewSec
		if maxSkewSec <= 0 {
			maxSkewSec = constants.DefaultWebhookMaxSkewSec
		}
		maxSkew := time.Duration(maxSkewSec) * time.Second
		bodyBytes, err := verifySignatureWithSkew(r, s.cfg.WhatsApp.WebhookSecret, XWahaSignatureHeader, maxSkew)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "body too large") {
				s.logger.WithError(err).Warn("Webhook request body too large")
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}
			s.logger.WithError(err).Error("WhatsApp webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload models.WhatsAppWebhookPayload
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode webhook payload after signature verification")
			// Do not log raw body to avoid leaking PII; log size instead
			s.logger.WithField("body_len", len(bodyBytes)).Debug("Invalid webhook JSON payload")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		s.logger.WithField("event", payload.Event).Debug("Received WhatsApp webhook payload")

		if payload.Payload.FromMe {
			s.logger.Debug("Skipping message from ourselves")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle different event types
		switch payload.Event {
		case models.EventMessage:
			err = s.handleWhatsAppMessage(r.Context(), &payload)
		case models.EventMessageReaction:
			err = s.handleWhatsAppReaction(r.Context(), &payload)
		case models.EventMessageEdited:
			err = s.handleWhatsAppEditedMessage(r.Context(), &payload)
		case models.EventMessageACK:
			err = s.handleWhatsAppACK(r.Context(), &payload)
		case models.EventMessageWaiting:
			err = s.handleWhatsAppWaitingMessage(r.Context(), &payload)
		default:
			s.logger.WithField("event", payload.Event).Debug("Skipping unsupported WhatsApp event")
			w.WriteHeader(http.StatusOK)
			return
		}

		if err != nil {
			s.logger.WithError(err).WithField("event", payload.Event).Error("Failed to handle WhatsApp event")
			if _, isValidationError := err.(ValidationError); isValidationError {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, "Failed to process event", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) handleWhatsAppMessage(ctx context.Context, payload *models.WhatsAppWebhookPayload) error {
	if payload.Payload.ID == "" {
		return ValidationError{Message: "missing required field: Payload.ID"}
	}
	if payload.Payload.From == "" {
		return ValidationError{Message: "missing required field: Payload.From"}
	}
	if payload.Payload.Body == "" && !payload.Payload.HasMedia {
		// Skip empty system messages (status updates, typing indicators, etc.)
		s.logger.WithField("messageID", service.SanitizeMessageID(payload.Payload.ID)).Debug("Ignoring empty system message")
		return nil
	}

	// Skip WhatsApp status/broadcast messages
	if strings.Contains(payload.Payload.From, "status@broadcast") {
		s.logger.WithFields(logrus.Fields{
			"messageID": service.SanitizeMessageID(payload.Payload.ID),
			"from":      payload.Payload.From,
		}).Debug("Ignoring WhatsApp status/broadcast message")
		return nil
	}

	// Enhanced input validation
	if err := service.ValidateMessageID(payload.Payload.ID); err != nil {
		return ValidationError{Message: fmt.Sprintf("invalid message ID: %v", err)}
	}
	if err := service.ValidatePhoneNumber(payload.Payload.From); err != nil {
		return ValidationError{Message: fmt.Sprintf("invalid sender phone number: %v", err)}
	}

	var mediaURL string
	if payload.Payload.HasMedia && payload.Payload.Media != nil {
		mediaURL = payload.Payload.Media.URL
	}

	chatID := payload.Payload.From

	// Use session from webhook payload
	sessionName := payload.Session
	if sessionName == "" {
		return ValidationError{Message: "session name is required but was not provided"}
	}

	// Validate session name
	if err := service.ValidateSessionName(sessionName); err != nil {
		return ValidationError{Message: fmt.Sprintf("invalid session name: %v", err)}
	}

	// Check if session is configured
	if !s.channelManager.IsValidSession(sessionName) {
		s.logger.WithField("session", sessionName).Debug("Ignoring message from unconfigured session")
		return nil
	}

	return s.msgService.HandleWhatsAppMessageWithSession(
		ctx,
		sessionName,
		chatID,
		payload.Payload.ID,
		payload.Payload.From,
		payload.Payload.Body,
		mediaURL,
	)
}

func (s *Server) handleWhatsAppReaction(ctx context.Context, payload *models.WhatsAppWebhookPayload) error {
	if payload.Payload.Reaction == nil {
		return ValidationError{Message: "missing reaction data"}
	}
	if payload.Payload.From == "" {
		return ValidationError{Message: "missing required field: Payload.From"}
	}

	// Check if session is configured
	sessionName := payload.Session
	if sessionName == "" {
		return ValidationError{Message: "session name is required but was not provided"}
	}
	if !s.channelManager.IsValidSession(sessionName) {
		s.logger.WithField("session", sessionName).Debug("Ignoring reaction from unconfigured session")
		return nil
	}

	s.logger.WithFields(logrus.Fields{
		"from":      service.SanitizePhoneNumber(payload.Payload.From),
		"messageId": service.SanitizeWhatsAppMessageID(payload.Payload.Reaction.MessageID),
		"emoji":     payload.Payload.Reaction.Text,
	}).Info("Processing WhatsApp reaction for forwarding to Signal")

	// Find the original message mapping to get the Signal message ID
	mapping, err := s.msgService.GetMessageMappingByWhatsAppID(ctx, payload.Payload.Reaction.MessageID)
	if err != nil {
		s.logger.WithError(err).Warn("Could not find original message for reaction")
		return nil // Don't error out, just log and continue
	}

	if mapping == nil {
		s.logger.WithField("messageId", service.SanitizeWhatsAppMessageID(payload.Payload.Reaction.MessageID)).Warn("No mapping found for reacted message")
		return nil // Don't error out, just log and continue
	}

	// Forward reaction to Signal as a text message (since Signal CLI doesn't support reactions yet)
	var reactionText string
	if payload.Payload.Reaction.Text == "" {
		reactionText = "❌ Removed reaction from message"
	} else {
		reactionText = fmt.Sprintf("%s Reacted with %s", payload.Payload.Reaction.Text, payload.Payload.Reaction.Text)
	}

	// Send reaction notification to the appropriate Signal destination
	// Use the session from the mapping to determine the destination
	reactionSessionName := mapping.SessionName
	if reactionSessionName == "" {
		s.logger.Error("Session name missing in message mapping for reaction")
		return ValidationError{Message: "session name required but not found in message mapping"}
	}

	// Use the message service to send via the bridge with session context
	err = s.msgService.SendSignalNotification(ctx, reactionSessionName, reactionText)
	if err != nil {
		s.logger.WithError(err).Error("Failed to forward reaction to Signal")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"whatsappMessageId": service.SanitizeWhatsAppMessageID(payload.Payload.Reaction.MessageID),
		"signalMessageId":   mapping.ID,
		"emoji":             payload.Payload.Reaction.Text,
	}).Info("Successfully forwarded WhatsApp reaction to Signal")

	return nil
}

func (s *Server) handleWhatsAppEditedMessage(ctx context.Context, payload *models.WhatsAppWebhookPayload) error {
	if payload.Payload.EditedMessageID == nil {
		return ValidationError{Message: "missing editedMessageId for edited message event"}
	}
	if payload.Payload.From == "" {
		return ValidationError{Message: "missing required field: Payload.From"}
	}

	// Check if session is configured
	sessionName := payload.Session
	if sessionName == "" {
		return ValidationError{Message: "session name is required but was not provided"}
	}
	if !s.channelManager.IsValidSession(sessionName) {
		s.logger.WithField("session", sessionName).Debug("Ignoring edited message from unconfigured session")
		return nil
	}

	s.logger.WithFields(logrus.Fields{
		"from":            service.SanitizePhoneNumber(payload.Payload.From),
		"editedMessageId": service.SanitizeWhatsAppMessageID(*payload.Payload.EditedMessageID),
		"newBody":         service.SanitizeContent(payload.Payload.Body),
	}).Info("Processing WhatsApp message edit")

	// Find the original message mapping
	mapping, err := s.msgService.GetMessageMappingByWhatsAppID(ctx, *payload.Payload.EditedMessageID)
	if err != nil {
		s.logger.WithError(err).Warn("Could not find original message for edit")
		return nil
	}

	if mapping == nil {
		s.logger.WithField("messageId", service.SanitizeWhatsAppMessageID(*payload.Payload.EditedMessageID)).Warn("No mapping found for edited message")
		return nil
	}

	// For now, send an edit notification to Signal as a new message
	editNotification := fmt.Sprintf("✏️ Message edited: %s", payload.Payload.Body)

	// Send to Signal using message service
	// Send edit notification to the appropriate Signal destination
	editSessionName := mapping.SessionName
	if editSessionName == "" {
		s.logger.Error("Session name missing in message mapping for edited message")
		return ValidationError{Message: "session name required but not found in message mapping"}
	}

	// Use the message service to send via the bridge with session context
	err = s.msgService.SendSignalNotification(ctx, editSessionName, editNotification)
	if err != nil {
		s.logger.WithError(err).Error("Failed to send edit notification to Signal")
		return err
	}

	s.logger.Info("Successfully forwarded message edit notification to Signal")
	return nil
}

func (s *Server) handleWhatsAppACK(ctx context.Context, payload *models.WhatsAppWebhookPayload) error {
	if payload.Payload.ACK == nil {
		return ValidationError{Message: "missing ACK data"}
	}

	ackStatus := *payload.Payload.ACK
	var statusText string
	switch ackStatus {
	case models.ACKError:
		statusText = "Error"
	case models.ACKPending:
		statusText = "Pending"
	case models.ACKServer:
		statusText = "Sent"
	case models.ACKDevice:
		statusText = "Delivered"
	case models.ACKRead:
		statusText = "Read"
	case models.ACKPlayed:
		statusText = "Played"
	default:
		statusText = "Unknown"
	}

	s.logger.WithFields(logrus.Fields{
		"messageId": service.SanitizeWhatsAppMessageID(payload.Payload.ID),
		"from":      service.SanitizePhoneNumber(payload.Payload.From),
		"to":        service.SanitizePhoneNumber(payload.Payload.To),
		"ack":       ackStatus,
		"status":    statusText,
	}).Debug("Processing WhatsApp ACK status")

	// Update delivery status in database if we have a mapping
	mapping, err := s.msgService.GetMessageMappingByWhatsAppID(ctx, payload.Payload.ID)
	if err != nil || mapping == nil {
		// No mapping found, might be a message we don't track
		return nil
	}

	// Map WhatsApp ACK to our delivery status
	var deliveryStatus string
	switch ackStatus {
	case models.ACKError:
		deliveryStatus = "failed"
	case models.ACKPending, models.ACKServer:
		deliveryStatus = "sent"
	case models.ACKDevice:
		deliveryStatus = "delivered"
	case models.ACKRead, models.ACKPlayed:
		deliveryStatus = "read"
	}

	if deliveryStatus != "" {
		err = s.msgService.UpdateDeliveryStatus(ctx, payload.Payload.ID, deliveryStatus)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to update delivery status")
		}
	}

	return nil
}

func (s *Server) handleWhatsAppWaitingMessage(ctx context.Context, payload *models.WhatsAppWebhookPayload) error {
	s.logger.WithFields(logrus.Fields{
		"from":      service.SanitizePhoneNumber(payload.Payload.From),
		"messageId": service.SanitizeWhatsAppMessageID(payload.Payload.ID),
	}).Info("Processing WhatsApp waiting message event")

	waitingNotification := "⏳ WhatsApp is waiting for a message"

	// Require session context for waiting messages
	sessionName := payload.Session
	if sessionName == "" {
		return ValidationError{Message: "session name is required for waiting message events"}
	}

	err := s.msgService.SendSignalNotification(ctx, sessionName, waitingNotification)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to send waiting notification to Signal")
		return err
	}

	return nil
}
