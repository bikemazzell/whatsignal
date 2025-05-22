package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
	"whatsignal/internal/models"
	"whatsignal/internal/service"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router     *mux.Router
	logger     *logrus.Logger
	msgService service.MessageService
	waWebhook  whatsapp.WebhookHandler
	server     *http.Server
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

func NewServer(msgService service.MessageService, logger *logrus.Logger) *Server {
	s := &Server{
		router:     mux.NewRouter(),
		logger:     logger,
		msgService: msgService,
		waWebhook:  whatsapp.NewWebhookHandler(),
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
		var payload WhatsAppWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode webhook payload")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate data field is present
		if payload.Data.ID == "" && payload.Data.ChatID == "" && payload.Data.Sender == "" && payload.Data.Type == "" && payload.Data.Content == "" && payload.Data.MediaPath == "" {
			http.Error(w, "Missing or invalid data field", http.StatusBadRequest)
			return
		}

		if payload.Event != "message" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Validate required fields
		if payload.Data.ID == "" || payload.Data.ChatID == "" || payload.Data.Sender == "" || payload.Data.Type == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		err := s.msgService.HandleWhatsAppMessage(
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
		// Verify webhook signature if configured
		if err := s.verifySignalWebhookSignature(r); err != nil {
			s.logger.WithError(err).Error("Invalid Signal webhook signature")
			http.Error(w, "Invalid webhook signature", http.StatusUnauthorized)
			return
		}

		var payload SignalWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.logger.WithError(err).Error("Failed to decode Signal webhook payload")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if payload.MessageID == "" || payload.Sender == "" || payload.Type == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		msg := &models.Message{
			ID:        payload.MessageID,
			Sender:    payload.Sender,
			Content:   payload.Message,
			Timestamp: time.UnixMilli(payload.Timestamp),
			Type:      models.MessageType(payload.Type),
			ThreadID:  payload.ThreadID,
			Recipient: payload.Recipient,
			MediaPath: payload.MediaPath,
			Platform:  "signal",
		}

		if len(payload.Attachments) > 0 {
			msg.Type = models.ImageMessage
			msg.MediaURL = payload.Attachments[0]
		}

		s.logger.WithFields(logrus.Fields{
			"messageId": msg.ID,
			"sender":    msg.Sender,
			"timestamp": msg.Timestamp,
		}).Info("Received Signal message")

		if err := s.msgService.HandleSignalMessage(r.Context(), msg); err != nil {
			s.logger.WithError(err).Error("Failed to handle Signal message")
			http.Error(w, "Failed to process message", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) verifySignalWebhookSignature(r *http.Request) error {
	// TODO: Implement webhook signature verification when Signal-CLI supports it
	return nil
}
