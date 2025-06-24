package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
	"whatsignal/internal/constants"
	"whatsignal/internal/models"
	"whatsignal/internal/service"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	XWahaSignatureHeader   = "X-Webhook-Hmac"
	XSignalSignatureHeader = "X-Signal-Signature-256"
)

// ValidationError represents a validation error that should return HTTP 400
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}


type Server struct {
	router     *mux.Router
	logger     *logrus.Logger
	msgService service.MessageService
	waWebhook  whatsapp.WebhookHandler
	server     *http.Server
	cfg        *models.Config
	waClient   types.WAClient
}

func convertWebhookPayloadToSignalMessage(payload *models.SignalWebhookPayload) *signaltypes.SignalMessage {
	attachments := payload.Attachments
	if attachments == nil {
		attachments = []string{}
	}

	if payload.MediaPath != "" {
		attachments = append(attachments, payload.MediaPath)
	}

	return &signaltypes.SignalMessage{
		MessageID:     payload.MessageID,
		Sender:        payload.Sender,
		Message:       payload.Message,
		Timestamp:     payload.Timestamp,
		Attachments:   attachments,
		QuotedMessage: nil,
	}
}

func NewServer(cfg *models.Config, msgService service.MessageService, logger *logrus.Logger, waClient types.WAClient) *Server {
	s := &Server{
		router:     mux.NewRouter(),
		logger:     logger,
		msgService: msgService,
		waWebhook:  whatsapp.NewWebhookHandler(),
		cfg:        cfg,
		waClient:   waClient,
	}

	s.setupRoutes()
	s.setupWebhookHandlers()
	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/health", s.handleHealth()).Methods(http.MethodGet)
	s.router.HandleFunc("/session/status", s.handleSessionStatus()).Methods(http.MethodGet)

	whatsapp := s.router.PathPrefix("/webhook/whatsapp").Subrouter()
	whatsapp.HandleFunc("", s.handleWhatsAppWebhook()).Methods(http.MethodPost)

	signal := s.router.PathPrefix("/webhook/signal").Subrouter()
	signal.HandleFunc("", s.handleSignalWebhook()).Methods(http.MethodPost)
	
}

func (s *Server) setupWebhookHandlers() {
	s.waWebhook.RegisterEventHandler("message.any", func(ctx context.Context, payload json.RawMessage) error {
		var msg types.MessagePayload
		if err := json.Unmarshal(payload, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal message payload: %w", err)
		}

		s.logger.WithFields(logrus.Fields{
			"messageId": msg.ID,
			"chatId":    msg.ChatID,
			"sender":    msg.Sender,
			"type":      msg.Type,
		}).Info("Received WhatsApp message")

		return s.msgService.HandleWhatsAppMessage(ctx, msg.ChatID, msg.ID, msg.Sender, msg.Content, msg.MediaURL)
	})
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

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	s.logger.Infof("Starting server on port %s", port)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Failed to get session status",
				"details": err.Error(),
			})
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
			"auto_restart_enabled": s.cfg.WhatsApp.SessionAutoRestart,
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
		
		bodyBytes, err := verifySignature(r, s.cfg.WhatsApp.WebhookSecret, XWahaSignatureHeader)
		if err != nil {
			s.logger.WithError(err).Error("WhatsApp webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload models.WhatsAppWebhookPayload
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode webhook payload after signature verification")
			s.logger.Debugf("Raw webhook body: %s", string(bodyBytes))
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
		return ValidationError{Message: "message must have either body or media"}
	}

	var mediaURL string
	if payload.Payload.HasMedia && payload.Payload.Media != nil {
		mediaURL = payload.Payload.Media.URL
	}

	chatID := payload.Payload.From
	
	return s.msgService.HandleWhatsAppMessage(
		ctx,
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

	s.logger.WithFields(logrus.Fields{
		"from":      payload.Payload.From,
		"messageId": payload.Payload.Reaction.MessageID,
		"emoji":     payload.Payload.Reaction.Text,
	}).Info("Processing WhatsApp reaction for forwarding to Signal")

	// Find the original message mapping to get the Signal message ID
	mapping, err := s.msgService.GetMessageByID(ctx, payload.Payload.Reaction.MessageID)
	if err != nil {
		s.logger.WithError(err).Warn("Could not find original message for reaction")
		return nil // Don't error out, just log and continue
	}

	if mapping == nil {
		s.logger.WithField("messageId", payload.Payload.Reaction.MessageID).Warn("No mapping found for reacted message")
		return nil // Don't error out, just log and continue
	}

	// Forward reaction to Signal as a text message (since Signal CLI doesn't support reactions yet)
	var reactionText string
	if payload.Payload.Reaction.Text == "" {
		reactionText = "❌ Removed reaction from message"
	} else {
		reactionText = fmt.Sprintf("%s Reacted with %s", payload.Payload.Reaction.Text, payload.Payload.Reaction.Text)
	}

	signalMsg := &models.Message{
		ID:       payload.Payload.ID + "_reaction",
		Platform: "signal",
		ThreadID: s.cfg.Signal.DestinationPhoneNumber,
		Content:  reactionText,
	}

	err = s.msgService.SendMessage(ctx, signalMsg)
	if err != nil {
		s.logger.WithError(err).Error("Failed to forward reaction to Signal")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"whatsappMessageId": payload.Payload.Reaction.MessageID,
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

	s.logger.WithFields(logrus.Fields{
		"from":             payload.Payload.From,
		"editedMessageId":  *payload.Payload.EditedMessageID,
		"newBody":          payload.Payload.Body,
	}).Info("Processing WhatsApp message edit")

	// Find the original message mapping
	mapping, err := s.msgService.GetMessageByID(ctx, *payload.Payload.EditedMessageID)
	if err != nil {
		s.logger.WithError(err).Warn("Could not find original message for edit")
		return nil
	}

	if mapping == nil {
		s.logger.WithField("messageId", *payload.Payload.EditedMessageID).Warn("No mapping found for edited message")
		return nil
	}

	// For now, send an edit notification to Signal as a new message
	editNotification := fmt.Sprintf("✏️ Message edited: %s", payload.Payload.Body)
	
	// Send to Signal using message service
	signalMsg := &models.Message{
		ID:       payload.Payload.ID + "_edit",
		Platform: "signal",
		ThreadID: s.cfg.Signal.DestinationPhoneNumber,
		Content:  editNotification,
	}
	
	err = s.msgService.SendMessage(ctx, signalMsg)
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
		"messageId": payload.Payload.ID,
		"from":      payload.Payload.From,
		"to":        payload.Payload.To,
		"ack":       ackStatus,
		"status":    statusText,
	}).Debug("Processing WhatsApp ACK status")

	// Update delivery status in database if we have a mapping
	mapping, err := s.msgService.GetMessageByID(ctx, payload.Payload.ID)
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
		"from":      payload.Payload.From,
		"messageId": payload.Payload.ID,
	}).Info("Processing WhatsApp waiting message event")

	// For waiting messages, we might want to send a notification to Signal
	waitingNotification := "⏳ WhatsApp is waiting for a message"
	
	signalMsg := &models.Message{
		ID:       payload.Payload.ID + "_waiting",
		Platform: "signal",
		ThreadID: s.cfg.Signal.DestinationPhoneNumber,
		Content:  waitingNotification,
	}
	
	err := s.msgService.SendMessage(ctx, signalMsg)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to send waiting notification to Signal")
		return err
	}

	return nil
}

func (s *Server) handleSignalWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("Received Signal webhook request")
		
		bodyBytes, err := verifySignature(r, s.cfg.Signal.WebhookSecret, XSignalSignatureHeader)
		if err != nil {
			s.logger.WithError(err).Error("Signal webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload models.SignalWebhookPayload
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode Signal webhook payload after signature verification")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if payload.MessageID == "" {
			http.Error(w, "Missing required field: MessageID", http.StatusBadRequest)
			return
		}
		if payload.Sender == "" {
			http.Error(w, "Missing required field: Sender", http.StatusBadRequest)
			return
		}
		if payload.Type == "" {
			http.Error(w, "Missing required field: Type", http.StatusBadRequest)
			return
		}

		if payload.Type == "text" && payload.Message == "" {
			http.Error(w, "Text messages must have content", http.StatusBadRequest)
			return
		}

		sigMsg := convertWebhookPayloadToSignalMessage(&payload)

		s.logger.WithFields(logrus.Fields{
			"messageId": sigMsg.MessageID,
			"sender":    sigMsg.Sender,
			"timestamp": sigMsg.Timestamp,
		}).Info("Received Signal message via webhook, calling ProcessIncomingSignalMessage")

		if err := s.msgService.ProcessIncomingSignalMessage(r.Context(), sigMsg); err != nil {
			s.logger.WithError(err).Error("Failed to handle Signal message from webhook")
			http.Error(w, "Failed to process message", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

