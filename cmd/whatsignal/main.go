package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsignal/internal/config"
	"whatsignal/internal/database"
	"whatsignal/internal/models"
	"whatsignal/internal/service"
	"whatsignal/pkg/media"
	signalapi "whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

func main() {
	// Create context that listens for the interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		logrus.Fatalf("Application error: %v", err)
	}
}

func run(ctx context.Context) error {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate required configuration
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Set log level from config
	if cfg.LogLevel != "" {
		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err != nil {
			logger.Warnf("Invalid log level %q, defaulting to info", cfg.LogLevel)
			logger.SetLevel(logrus.InfoLevel)
		} else {
			logger.SetLevel(level)
		}
	}

	// Initialize database with retry
	var db *database.Database
	for attempts := 0; attempts < 3; attempts++ {
		db, err = database.New(cfg.Database.Path)
		if err == nil {
			break
		}
		logger.Warnf("Failed to initialize database (attempt %d/3): %v", attempts+1, err)
		time.Sleep(time.Second * time.Duration(attempts+1))
	}
	if err != nil {
		return fmt.Errorf("failed to initialize database after retries: %w", err)
	}
	defer db.Close()

	// Initialize media handler
	mediaHandler, err := media.NewHandler(cfg.Media.CacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize media handler: %w", err)
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
		cfg.Signal.DeviceName,
	)

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
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErrCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		logger.Info("Received shutdown signal")
	case err := <-serverErrCh:
		logger.Error(err)
		return err
	}

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server gracefully: %w", err)
	}

	logger.Info("Server shutdown completed")
	return nil
}

func validateConfig(cfg *models.Config) error {
	if cfg.WhatsApp.APIKey == "" {
		return fmt.Errorf("WhatsApp API key is required")
	}
	if cfg.WhatsApp.APIBaseURL == "" {
		return fmt.Errorf("WhatsApp API base URL is required")
	}
	if cfg.Signal.PhoneNumber == "" {
		return fmt.Errorf("Signal phone number is required")
	}
	if cfg.Database.Path == "" {
		return fmt.Errorf("Database path is required")
	}
	if cfg.Media.CacheDir == "" {
		return fmt.Errorf("Media cache directory is required")
	}
	return nil
}
