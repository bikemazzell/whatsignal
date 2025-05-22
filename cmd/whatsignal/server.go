package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"whatsignal/internal/service"
	"whatsignal/pkg/signal"
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
		var event types.WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			s.logger.WithError(err).Error("Failed to decode webhook payload")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := s.waWebhook.Handle(r.Context(), &event); err != nil {
			s.logger.WithError(err).Error("Failed to handle webhook event")
			w.WriteHeader(http.StatusInternalServerError)
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
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var msg signal.SignalMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			s.logger.WithError(err).Error("Failed to decode Signal webhook payload")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.logger.WithFields(logrus.Fields{
			"messageId": msg.MessageID,
			"sender":    msg.Sender,
			"timestamp": msg.Timestamp,
		}).Info("Received Signal message")

		if err := s.msgService.HandleSignalMessage(r.Context(), &msg); err != nil {
			s.logger.WithError(err).Error("Failed to handle Signal message")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) verifySignalWebhookSignature(r *http.Request) error {
	// TODO: Implement webhook signature verification when Signal-CLI supports it
	return nil
}
