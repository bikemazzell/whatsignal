package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsignal/internal/config"
	"whatsignal/internal/constants"
	"whatsignal/internal/database"
	"whatsignal/internal/models"
	"whatsignal/internal/service"
	"whatsignal/pkg/media"
	signalapi "whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

var (
	// Version information (set at build time)
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"

	// CLI flags
	verbose    = flag.Bool("verbose", false, "Enable verbose logging (includes sensitive information)")
	configPath = flag.String("config", "config.json", "Path to configuration file")
	version    = flag.Bool("version", false, "Show version information")
)

func main() {
	flag.Parse()
	
	if *version {
		fmt.Printf("WhatSignal %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		logrus.Fatalf("Application error: %v", err)
	}
}

func run(ctx context.Context) error {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	logger.WithFields(logrus.Fields{
		"version":   Version,
		"build":     BuildTime,
		"commit":    GitCommit,
	}).Info("Starting WhatSignal")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if *verbose {
		logger.SetLevel(logrus.DebugLevel)
		logger.Info("Verbose logging enabled - sensitive information will be logged")
	} else if cfg.LogLevel != "" {
		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err != nil {
			logger.Warnf("Invalid log level %q, defaulting to info", cfg.LogLevel)
			logger.SetLevel(logrus.InfoLevel)
		} else {
			if level > logrus.InfoLevel {
				level = logrus.InfoLevel
			}
			logger.SetLevel(level)
		}
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	var db *database.Database
	for attempts := 0; attempts < constants.DefaultDatabaseRetryAttempts; attempts++ {
		db, err = database.New(cfg.Database.Path)
		if err == nil {
			break
		}
		logger.Warnf("Failed to initialize database (attempt %d/%d): %v", attempts+1, constants.DefaultDatabaseRetryAttempts, err)
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
		cfg.Signal.IntermediaryPhoneNumber,
		cfg.Signal.DeviceName,
		nil,
	)

	if err := sigClient.InitializeDevice(ctx); err != nil {
		logger.Warnf("Failed to initialize Signal device: %v. whatsignal may not function correctly with Signal.", err)
	}

	cacheHours := cfg.WhatsApp.ContactCacheHours
	if cacheHours <= 0 {
		cacheHours = 24
	}
	contactService := service.NewContactServiceWithConfig(db, waClient, cacheHours)

	syncOnStartup := cfg.WhatsApp.ContactSyncOnStartup
	if syncOnStartup {
		logger.Info("Syncing WhatsApp contacts on startup...")
		if err := contactService.SyncAllContacts(ctx); err != nil {
			logger.Warnf("Failed to sync contacts on startup: %v. Contact names may not be available immediately.", err)
		} else {
			logger.Info("Contact sync completed successfully")
		}
	} else {
		logger.Info("Contact sync on startup is disabled")
	}

	bridge := service.NewBridge(waClient, sigClient, db, mediaHandler, models.RetryConfig{
		InitialBackoffMs: cfg.Retry.InitialBackoffMs,
		MaxBackoffMs:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:      cfg.Retry.MaxAttempts,
	}, cfg.Signal.DestinationPhoneNumber, contactService)

	messageService := service.NewMessageService(bridge, db, mediaHandler, sigClient, cfg.Signal)

	scheduler := service.NewScheduler(bridge, cfg.RetentionDays, logger)
	go scheduler.Start(ctx)

	ctxWithVerbose := context.WithValue(ctx, "verbose", *verbose)
	
	signalPoller := service.NewSignalPoller(sigClient, messageService, cfg.Signal, models.RetryConfig{
		InitialBackoffMs: cfg.Retry.InitialBackoffMs,
		MaxBackoffMs:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:      cfg.Retry.MaxAttempts,
	}, logger)

	if err := signalPoller.Start(ctxWithVerbose); err != nil {
		logger.Warnf("Failed to start Signal poller: %v", err)
	}
	defer signalPoller.Stop()

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(constants.DefaultGracefulShutdownSec)*time.Second)
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
	if cfg.Signal.IntermediaryPhoneNumber == "" {
		return fmt.Errorf("signal intermediary phone number is required")
	}
	if cfg.Signal.DestinationPhoneNumber == "" {
		return fmt.Errorf("signal destination phone number is required")
	}
	if cfg.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}
	if cfg.Media.CacheDir == "" {
		return fmt.Errorf("media cache directory is required")
	}
	return nil
}
