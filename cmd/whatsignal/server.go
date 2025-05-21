package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"whatsignal/internal/service"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	router     *mux.Router
	logger     *logrus.Logger
	msgService service.MessageService
	server     *http.Server
}

func NewServer(msgService service.MessageService, logger *logrus.Logger) *Server {
	s := &Server{
		router:     mux.NewRouter(),
		logger:     logger,
		msgService: msgService,
	}

	s.setupRoutes()
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
		// TODO: Implement WhatsApp webhook handler
		s.logger.Info("WhatsApp webhook received")
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) handleSignalWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement Signal webhook handler
		s.logger.Info("Signal webhook received")
		w.WriteHeader(http.StatusOK)
	}
}
