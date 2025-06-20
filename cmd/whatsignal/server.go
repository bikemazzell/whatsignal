package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
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

type Server struct {
	router     *mux.Router
	logger     *logrus.Logger
	msgService service.MessageService
	waWebhook  whatsapp.WebhookHandler
	server     *http.Server
	cfg        *models.Config
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

func NewServer(cfg *models.Config, msgService service.MessageService, logger *logrus.Logger) *Server {
	s := &Server{
		router:     mux.NewRouter(),
		logger:     logger,
		msgService: msgService,
		waWebhook:  whatsapp.NewWebhookHandler(),
		cfg:        cfg,
	}

	s.setupRoutes()
	s.setupWebhookHandlers()
	return s
}

func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.handleHealth()).Methods(http.MethodGet)

	// WhatsApp webhook
	whatsapp := s.router.PathPrefix("/webhook/whatsapp").Subrouter()
	whatsapp.HandleFunc("", s.handleWhatsAppWebhook()).Methods(http.MethodPost)

	// Signal webhook (for future versions that support it)
	signal := s.router.PathPrefix("/webhook/signal").Subrouter()
	signal.HandleFunc("", s.handleSignalWebhook()).Methods(http.MethodPost)
	
}

func (s *Server) setupWebhookHandlers() {
	// Register WhatsApp webhook handlers
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
		port = "8082"
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Infof("Starting server on port %s", port)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil // Server was never started
	}
	return s.server.Shutdown(ctx)
}

// Handler implementations
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			s.logger.WithError(err).Error("Failed to write health check response")
		}
	}
}

func (s *Server) handleWhatsAppWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debug("Processing WhatsApp webhook request")
		
		// Verify signature first
		bodyBytes, err := verifySignature(r, s.cfg.WhatsApp.WebhookSecret, XWahaSignatureHeader)
		if err != nil {
			s.logger.WithError(err).Error("WhatsApp webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload models.WhatsAppWebhookPayload
		// Decode from the bodyBytes we got from verifySignature, as r.Body was replaced
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode webhook payload after signature verification")
			// Log the raw body for debugging
			s.logger.Debugf("Raw webhook body: %s", string(bodyBytes))
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		s.logger.Debug("Received WhatsApp webhook payload")

		// Only process "message" events, skip "message.any" to avoid duplicates
		if payload.Event != "message" {
			s.logger.Debug("Skipping non-message WhatsApp event")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Skip messages from ourselves (fromMe: true) to avoid loops
		if payload.Payload.FromMe {
			s.logger.Debug("Skipping message from ourselves")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Validate required fields for message events
		if payload.Payload.ID == "" {
			s.logger.Error("Missing required field: Payload.ID")
			http.Error(w, "Missing required field: Payload.ID", http.StatusBadRequest)
			return
		}
		if payload.Payload.From == "" {
			s.logger.Error("Missing required field: Payload.From")
			http.Error(w, "Missing required field: Payload.From", http.StatusBadRequest)
			return
		}
		if payload.Payload.Body == "" && !payload.Payload.HasMedia {
			s.logger.Error("Message must have either body or media")
			http.Error(w, "Message must have either body or media", http.StatusBadRequest)
			return
		}

		// Extract media URL if present
		var mediaURL string
		if payload.Payload.HasMedia && payload.Payload.Media != nil {
			mediaURL = payload.Payload.Media.URL
		}

		// The "from" field is the chat ID in WhatsApp
		chatID := payload.Payload.From
		
		err = s.msgService.HandleWhatsAppMessage(
			r.Context(),
			chatID,
			payload.Payload.ID,
			payload.Payload.From,
			payload.Payload.Body,
			mediaURL,
		)
		if err != nil {
			s.logger.WithError(err).Error("Failed to handle WhatsApp message")
			http.Error(w, "Failed to process message", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) handleSignalWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("Received Signal webhook request")
		
		// Verify signature first
		bodyBytes, err := verifySignature(r, s.cfg.Signal.WebhookSecret, XSignalSignatureHeader)
		if err != nil {
			s.logger.WithError(err).Error("Signal webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload models.SignalWebhookPayload
		// Decode from the bodyBytes we got from verifySignature
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode Signal webhook payload after signature verification")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
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

		// Additional validation for message content
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

		if err := s.msgService.ProcessIncomingSignalMessage(r.Context(), sigMsg); err != nil { // Use ProcessIncomingSignalMessage
			s.logger.WithError(err).Error("Failed to handle Signal message from webhook")
			http.Error(w, "Failed to process message", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

