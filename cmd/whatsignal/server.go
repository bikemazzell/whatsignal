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
	"whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	// XWahaSignatureHeader is the expected header name for WhatsApp (WAHA) webhook signatures.
	XWahaSignatureHeader = "X-Waha-Signature-256"
	// XSignalSignatureHeader is the expected header name for Signal webhook signatures.
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

type WhatsAppWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		ID        string `json:"id"`
		ChatID    string `json:"chatId"`
		Sender    string `json:"sender"`
		Type      string `json:"type"`
		Content   string `json:"content"`
		MediaPath string `json:"mediaPath,omitempty"`
	} `json:"data"`
}

type SignalWebhookPayload struct {
	MessageID   string   `json:"messageId"`
	Sender      string   `json:"sender"`
	Message     string   `json:"message"`
	Timestamp   int64    `json:"timestamp"`
	Type        string   `json:"type"`
	ThreadID    string   `json:"threadId"`
	Recipient   string   `json:"recipient"`
	MediaPath   string   `json:"mediaPath,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
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

	// Signal webhook
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
	return s.server.Shutdown(ctx)
}

// Handler implementations
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func (s *Server) handleWhatsAppWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify signature first
		bodyBytes, err := verifySignature(r, s.cfg.WhatsApp.WebhookSecret, XWahaSignatureHeader)
		if err != nil {
			s.logger.WithError(err).Error("WhatsApp webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload WhatsAppWebhookPayload
		// Decode from the bodyBytes we got from verifySignature, as r.Body was replaced
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode webhook payload after signature verification")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate data field is present
		if payload.Data.ID == "" && payload.Data.ChatID == "" && payload.Data.Sender == "" && payload.Data.Type == "" && payload.Data.Content == "" && payload.Data.MediaPath == "" {
			http.Error(w, "Missing or invalid data field", http.StatusBadRequest)
			return
		}

		if payload.Event != "message" {
			s.logger.Infof("Skipping non-message WhatsApp event: %s", payload.Event)
			w.WriteHeader(http.StatusOK)
			return
		}

		// Validate required fields
		if payload.Data.ID == "" || payload.Data.ChatID == "" || payload.Data.Sender == "" || payload.Data.Type == "" {
			http.Error(w, "Missing required fields for message event", http.StatusBadRequest)
			return
		}

		err = s.msgService.HandleWhatsAppMessage(
			r.Context(),
			payload.Data.ChatID,
			payload.Data.ID,
			payload.Data.Sender,
			payload.Data.Content,
			payload.Data.MediaPath,
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
		// Verify signature first
		bodyBytes, err := verifySignature(r, s.cfg.Signal.WebhookSecret, XSignalSignatureHeader)
		if err != nil {
			s.logger.WithError(err).Error("Signal webhook signature verification failed")
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		var payload SignalWebhookPayload
		// Decode from the bodyBytes we got from verifySignature
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode Signal webhook payload after signature verification")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if payload.MessageID == "" || payload.Sender == "" || payload.Type == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		// Convert SignalWebhookPayload to models.Message as before
		// Note: The original HandleSignalMessage expected a models.Message,
		// but the service layer HandleSignalMessage expects signal.SignalMessage.
		// This part needs to be reconciled with how signal messages are actually processed.
		// For now, assuming the server constructs a signal.SignalMessage if that's what the service needs.
		// Or, the service.HandleSignalMessage is adapted.

		// Let's assume for now the webhook payload IS the signal.SignalMessage structure
		// or can be directly mapped to it or a subset needed by HandleSignalMessage service method.
		// The current SignalWebhookPayload is quite different from signal.SignalMessage.
		// This indicates a potential mismatch between what the /webhook/signal expects and what service.HandleSignalMessage processes.

		// For the purpose of this example, let's create a signal.SignalMessage from SignalWebhookPayload
		// This part needs careful review based on actual data flow for Signal messages.
		sigMsg := &signal.SignalMessage{
			MessageID:   payload.MessageID,
			Sender:      payload.Sender,
			Message:     payload.Message,
			Timestamp:   payload.Timestamp,
			Attachments: payload.Attachments, // Assuming SignalWebhookPayload.Attachments maps directly
			// QuotedMessage *struct { ... } // Not present in SignalWebhookPayload, would be nil
		}

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
