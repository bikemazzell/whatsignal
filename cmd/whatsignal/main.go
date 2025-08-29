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
		fmt.Printf("WhatSignal %s\nBuild Time: %s\nGit Commit: %s\n", Version, BuildTime, GitCommit)
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

	apiKey := os.Getenv("WHATSAPP_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("WHATSAPP_API_KEY environment variable is required")
	}

	mediaHandler, err := media.NewHandlerWithWAHA(cfg.Media.CacheDir, cfg.Media, cfg.WhatsApp.APIBaseURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to initialize media handler: %w", err)
	}

	// Create channel manager
	channelManager, err := service.NewChannelManager(cfg.Channels)
	if err != nil {
		return fmt.Errorf("failed to create channel manager: %w", err)
	}

	// Use the first configured session explicitly from config for client operations
	if len(cfg.Channels) == 0 {
		return fmt.Errorf("no channels configured")
	}
	defaultSessionName := cfg.Channels[0].WhatsAppSessionName

	waClient := whatsapp.NewClient(types.ClientConfig{
		BaseURL:      cfg.WhatsApp.APIBaseURL,
		APIKey:       apiKey,
		SessionName:  defaultSessionName,
		Timeout:      cfg.WhatsApp.Timeout,
		RetryCount:   cfg.WhatsApp.RetryCount,
	})

	sigClient := signalapi.NewClientWithLogger(
		cfg.Signal.RPCURL,
		cfg.Signal.IntermediaryPhoneNumber,
		cfg.Signal.DeviceName,
		cfg.Signal.AttachmentsDir,
		nil,
		logger,
	)

	if err := sigClient.InitializeDevice(ctx); err != nil {
		logger.Warnf("Failed to initialize Signal device: %v. whatsignal may not function correctly with Signal.", err)
	}

	cacheHours := cfg.WhatsApp.ContactCacheHours
	if cacheHours <= 0 {
		cacheHours = constants.DefaultContactCacheHours
	}
	contactService := service.NewContactServiceWithConfig(db, waClient, cacheHours)

	syncOnStartup := cfg.WhatsApp.ContactSyncOnStartup
	if syncOnStartup {
		// Sync contacts for all configured sessions
		for _, channel := range cfg.Channels {
			sessionName := channel.WhatsAppSessionName
			logger.WithField("session", sessionName).Info("Waiting for WhatsApp session to be ready...")
			
			// Create a client for this specific session
			sessionClient := whatsapp.NewClient(types.ClientConfig{
				BaseURL:      cfg.WhatsApp.APIBaseURL,
				APIKey:       apiKey,
				SessionName:  sessionName,
				Timeout:      cfg.WhatsApp.Timeout,
				RetryCount:   cfg.WhatsApp.RetryCount,
			})
			
			// Wait for session to be ready
			sessionReadyTimeout := time.Duration(constants.DefaultSessionReadyTimeoutSec) * time.Second
			if err := sessionClient.WaitForSessionReady(ctx, sessionReadyTimeout); err != nil {
				logger.WithField("session", sessionName).Warnf("Failed to wait for session ready: %v. Skipping contact sync.", err)
				continue
			}
			
			logger.WithField("session", sessionName).Info("WhatsApp session is ready. Syncing contacts...")
			
			// Create a contact service for this session
			sessionContactService := service.NewContactServiceWithConfig(db, sessionClient, cacheHours)
			if err := sessionContactService.SyncAllContacts(ctx); err != nil {
				logger.WithField("session", sessionName).Warnf("Failed to sync contacts on startup: %v. Contact names may not be available immediately.", err)
			} else {
				logger.WithField("session", sessionName).Info("Contact sync completed successfully")
			}
		}
	} else {
		logger.Info("Contact sync on startup is disabled")
	}

	// Channel manager was already created above

	bridge := service.NewBridge(waClient, sigClient, db, mediaHandler, models.RetryConfig{
		InitialBackoffMs: cfg.Retry.InitialBackoffMs,
		MaxBackoffMs:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:      cfg.Retry.MaxAttempts,
	}, cfg.Media, channelManager, contactService)

	logger.WithField("channels", len(cfg.Channels)).Info("Multi-channel bridge initialized")

	messageService := service.NewMessageService(bridge, db, mediaHandler, sigClient, cfg.Signal, channelManager)

	scheduler := service.NewScheduler(bridge, cfg.RetentionDays, logger)
	go scheduler.Start(ctx)

	// Start session monitor if auto-restart is enabled
	if cfg.WhatsApp.SessionAutoRestart {
		checkInterval := time.Duration(cfg.WhatsApp.SessionHealthCheckSec) * time.Second
		if checkInterval <= 0 {
			checkInterval = time.Duration(constants.DefaultSessionHealthCheckSec) * time.Second
		}
		sessionMonitor := service.NewSessionMonitor(waClient, logger, checkInterval)
		sessionMonitor.Start(ctx)
		defer sessionMonitor.Stop()
		logger.WithField("interval", checkInterval).Info("Session health monitor started")
	}

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

	server := NewServer(cfg, messageService, logger, waClient, channelManager)
	serverErrCh := make(chan error, constants.ServerErrorChannelSize)
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
	// Signal destination phone numbers are now validated in the channels configuration
	if cfg.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}
	if cfg.Media.CacheDir == "" {
		return fmt.Errorf("media cache directory is required")
	}
	return nil
}
