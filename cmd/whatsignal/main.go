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

const (
	// MaxDatabaseRetryAttempts is the maximum number of times to retry database initialization
	MaxDatabaseRetryAttempts = 3
	// GracefulShutdownTimeout is the maximum time to wait for graceful shutdown
	GracefulShutdownTimeout = 30 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		logrus.Fatalf("Application error: %v", err)
	}
}

func run(ctx context.Context) error {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if cfg.LogLevel != "" {
		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err != nil {
			logger.Warnf("Invalid log level %q, defaulting to info", cfg.LogLevel)
			logger.SetLevel(logrus.InfoLevel)
		} else {
			logger.SetLevel(level)
		}
	}

	var db *database.Database
	for attempts := 0; attempts < MaxDatabaseRetryAttempts; attempts++ {
		db, err = database.New(cfg.Database.Path)
		if err == nil {
			break
		}
		logger.Warnf("Failed to initialize database (attempt %d/%d): %v", attempts+1, MaxDatabaseRetryAttempts, err)
		time.Sleep(time.Second * time.Duration(attempts+1))
	}
	if err != nil {
		return fmt.Errorf("failed to initialize database after retries: %w", err)
	}
	defer db.Close()

	mediaHandler, err := media.NewHandler(cfg.Media.CacheDir, cfg.Media)
	if err != nil {
		return fmt.Errorf("failed to initialize media handler: %w", err)
	}

	apiKey := os.Getenv("WHATSAPP_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("WHATSAPP_API_KEY environment variable is required")
	}

	waClient := whatsapp.NewClient(types.ClientConfig{
		BaseURL:     cfg.WhatsApp.APIBaseURL,
		APIKey:      apiKey,
		SessionName: cfg.WhatsApp.SessionName,
		Timeout:     cfg.WhatsApp.Timeout,
		RetryCount:  cfg.WhatsApp.RetryCount,
	})

	sigClient := signalapi.NewClient(
		cfg.Signal.RPCURL,
		cfg.Signal.AuthToken,
		cfg.Signal.PhoneNumber,
		cfg.Signal.DeviceName,
		nil,
	)

	if err := sigClient.InitializeDevice(ctx); err != nil {
		logger.Warnf("Failed to initialize Signal device: %v. whatsignal may not function correctly with Signal.", err)
	}

	bridge := service.NewBridge(waClient, sigClient, db, mediaHandler, models.RetryConfig{
		InitialBackoffMs: cfg.Retry.InitialBackoffMs,
		MaxBackoffMs:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:      cfg.Retry.MaxAttempts,
	})

	messageService := service.NewMessageService(bridge, db, mediaHandler)

	scheduler := service.NewScheduler(bridge, cfg.RetentionDays, logger)
	go scheduler.Start(ctx)

	server := NewServer(cfg, messageService, logger)
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErrCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("Received shutdown signal")
	case err := <-serverErrCh:
		logger.Error(err)
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server gracefully: %w", err)
	}

	logger.Info("Server shutdown completed")
	return nil
}

func validateConfig(cfg *models.Config) error {
	if cfg.WhatsApp.APIBaseURL == "" {
		return fmt.Errorf("whatsApp API base URL is required")
	}
	if cfg.Signal.PhoneNumber == "" {
		return fmt.Errorf("signal phone number is required")
	}
	if cfg.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}
	if cfg.Media.CacheDir == "" {
		return fmt.Errorf("media cache directory is required")
	}
	return nil
}
