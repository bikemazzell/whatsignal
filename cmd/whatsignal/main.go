package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"whatsignal/internal/config"
	"whatsignal/internal/database"
	"whatsignal/internal/service"
	"whatsignal/pkg/media"
	signalapi "whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Create context that listens for the interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Set log level from config
	if cfg.LogLevel != "" {
		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err != nil {
			logger.Warnf("Invalid log level %q, defaulting to info", cfg.LogLevel)
		} else {
			logger.SetLevel(level)
		}
	}

	// Initialize database
	db, err := database.New("whatsignal.db")
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize clients
	waClient := whatsapp.NewClient(types.ClientConfig{
		BaseURL:     cfg.WhatsApp.APIBaseURL,
		APIKey:      cfg.WhatsApp.APIKey,
		SessionName: cfg.WhatsApp.SessionName,
		Timeout:     cfg.WhatsApp.Timeout,
		RetryCount:  cfg.WhatsApp.RetryCount,
	})
	sigClient := signalapi.NewClient(
		cfg.Signal.RPCURL,
		cfg.Signal.AuthToken,
		cfg.Signal.PhoneNumber,
		"whatsignal-bridge", // Default device name
	)

	// Initialize media handler
	mediaHandler, err := media.NewHandler("cache")
	if err != nil {
		logger.Fatalf("Failed to initialize media handler: %v", err)
	}

	// Initialize message bridge
	bridge := service.NewBridge(waClient, sigClient, db, mediaHandler, service.RetryConfig{
		InitialBackoff: cfg.Retry.InitialBackoffMs,
		MaxBackoff:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:    cfg.Retry.MaxAttempts,
	})

	// Initialize message service
	messageService := service.NewMessageService(bridge, db, mediaHandler)

	// Start HTTP server
	server := NewServer(messageService, logger)
	go func() {
		if err := server.Start(); err != nil {
			logger.Errorf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info("Shutting down gracefully...")

	// Perform cleanup
	if err := server.Shutdown(context.Background()); err != nil {
		logger.Errorf("Error during shutdown: %v", err)
	}
}
