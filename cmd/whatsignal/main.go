package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"whatsignal/internal/config"
	"whatsignal/internal/constants"
	"whatsignal/internal/database"
	"whatsignal/internal/models"
	"whatsignal/internal/retry"
	"whatsignal/internal/service"
	"whatsignal/internal/tracing"
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
		"version": Version,
		"build":   BuildTime,
		"commit":  GitCommit,
	}).Info("Starting WhatSignal")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Configure log level: verbose flag > config > default (info)
	if *verbose {
		logger.SetLevel(logrus.DebugLevel)
		logger.Info("Verbose logging enabled - sensitive information will be logged")
	} else {
		setLogLevel(logger, cfg.LogLevel)
	}

	// Initialize OpenTelemetry tracing
	tracingManager := tracing.NewTracingManager(tracing.TracingConfig{
		ServiceName:        cfg.Tracing.ServiceName,
		ServiceVersion:     cfg.Tracing.ServiceVersion,
		Environment:        cfg.Tracing.Environment,
		OTLPEndpoint:       cfg.Tracing.OTLPEndpoint,
		SampleRate:         cfg.Tracing.SampleRate,
		Enabled:            cfg.Tracing.Enabled,
		UseStdout:          cfg.Tracing.UseStdout,
		ShutdownTimeoutSec: cfg.Tracing.ShutdownTimeoutSec,
	}, logger)

	if err := tracingManager.Initialize(ctx); err != nil {
		logger.Warnf("Failed to initialize tracing: %v", err)
	}
	defer func() {
		if err := tracingManager.Shutdown(context.Background()); err != nil {
			logger.Warnf("Failed to shutdown tracing: %v", err)
		}
	}()

	// Initialize database with exponential backoff retry
	var db *database.Database
	backoffConfig := retry.BackoffConfig{
		InitialDelay: time.Duration(cfg.Retry.InitialBackoffMs) * time.Millisecond,
		MaxDelay:     time.Duration(cfg.Retry.MaxBackoffMs) * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  constants.DefaultDatabaseRetryAttempts,
		Jitter:       true,
	}
	backoff := retry.NewBackoff(backoffConfig)

	err = backoff.Retry(ctx, func() error {
		var initErr error
		db, initErr = database.New(cfg.Database.Path, &cfg.Database)
		if initErr != nil {
			logger.Warnf("Failed to initialize database: %v", initErr)
		}
		return initErr
	})
	if err != nil {
		return fmt.Errorf("failed to initialize database after retries: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Warnf("Failed to close database: %v", closeErr)
		}
	}()

	// Validate required environment variables
	apiKey := os.Getenv("WHATSAPP_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("WHATSAPP_API_KEY environment variable is required")
	}

	mediaHandler, err := media.NewHandlerWithServices(cfg.Media.CacheDir, cfg.Media, cfg.WhatsApp.APIBaseURL, apiKey, cfg.Signal.RPCURL)
	if err != nil {
		return fmt.Errorf("failed to initialize media handler: %w", err)
	}

	// Create channel manager
	channelManager, err := service.NewChannelManager(cfg.Channels)
	if err != nil {
		return fmt.Errorf("failed to create channel manager: %w", err)
	}

	// Use the first configured session explicitly from config for client operations
	// (already validated in validateConfig)
	defaultSessionName := cfg.Channels[0].WhatsAppSessionName

	waClient := whatsapp.NewClient(types.ClientConfig{
		BaseURL:     cfg.WhatsApp.APIBaseURL,
		APIKey:      apiKey,
		SessionName: defaultSessionName,
		Timeout:     cfg.WhatsApp.Timeout,
		RetryCount:  cfg.WhatsApp.RetryCount,
	})

	// Use configured Signal HTTP timeout or default
	signalHTTPClient := &http.Client{
		Timeout: getTimeoutDuration(cfg.Signal.HTTPTimeoutSec, constants.DefaultSignalHTTPTimeoutSec),
	}

	sigClient := signalapi.NewClientWithLogger(
		cfg.Signal.RPCURL,
		cfg.Signal.IntermediaryPhoneNumber,
		cfg.Signal.DeviceName,
		cfg.Signal.AttachmentsDir,
		signalHTTPClient,
		logger,
	)

	if err := sigClient.InitializeDevice(ctx); err != nil {
		if cfg.Signal.StrictInit {
			logger.Fatalf("Failed to initialize Signal device (strict mode enabled): %v", err)
		}
		logger.Warnf("Failed to initialize Signal device: %v. whatsignal may not function correctly with Signal.", err)
	}

	// Use configured contact cache hours or default
	cacheHours := cfg.WhatsApp.ContactCacheHours
	if cacheHours <= 0 {
		cacheHours = constants.DefaultContactCacheHours
	}
	contactService := service.NewContactServiceWithConfig(db, waClient, cacheHours)

	syncOnStartup := cfg.WhatsApp.ContactSyncOnStartup
	if syncOnStartup {
		// Sync contacts for all configured sessions in parallel
		syncParallelContacts(ctx, cfg, db, apiKey, cacheHours, logger)
	} else {
		logger.Info("Contact sync on startup is disabled")
	}

	// Initialize GroupService (always enabled)
	groupCacheHours := cfg.WhatsApp.Groups.CacheHours
	if groupCacheHours <= 0 {
		groupCacheHours = constants.DefaultContactCacheHours
	}
	groupService := service.NewGroupServiceWithConfig(db, waClient, groupCacheHours)
	logger.WithField("cache_hours", groupCacheHours).Info("GroupService initialized")

	// Optionally sync all groups on startup if configured
	if cfg.WhatsApp.Groups.SyncOnStartup {
		// Sync groups for all configured sessions in parallel
		syncParallelGroups(ctx, cfg, db, apiKey, groupCacheHours, logger)
	} else {
		logger.Info("Group sync on startup is disabled")
	}

	bridge := service.NewBridge(waClient, sigClient, db, mediaHandler, models.RetryConfig{
		InitialBackoffMs: cfg.Retry.InitialBackoffMs,
		MaxBackoffMs:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:      cfg.Retry.MaxAttempts,
	}, cfg.Media, channelManager, contactService, groupService, cfg.Signal.AttachmentsDir, logger)

	logger.WithField("channels", len(cfg.Channels)).Info("Multi-channel bridge initialized")

	messageService := service.NewMessageService(bridge, db, mediaHandler, sigClient, cfg.Signal, channelManager)

	scheduler := service.NewScheduler(bridge, cfg.RetentionDays, cfg.Server.CleanupIntervalHours, logger)
	go scheduler.Start(ctx)
	defer scheduler.Stop()

	deliveryMonitor := service.NewDeliveryMonitor(db, time.Duration(constants.DefaultDeliveryMonitorIntervalMin)*time.Minute, time.Duration(constants.DefaultDeliveryMonitorStaleThresholdMin)*time.Minute, logger)
	go deliveryMonitor.Start(ctx)
	defer deliveryMonitor.Stop()

	// Start session monitor if auto-restart is enabled
	if cfg.WhatsApp.SessionAutoRestart {
		checkInterval := getTimeoutDuration(cfg.WhatsApp.SessionHealthCheckSec, constants.DefaultSessionHealthCheckSec)

		// Get startup timeout from config or environment variable (env var takes precedence)
		startupTimeoutSec := cfg.WhatsApp.SessionStartupTimeoutSec
		if envTimeout := getEnvInt("WHATSAPP_SESSION_STARTUP_TIMEOUT_SEC"); envTimeout > 0 {
			startupTimeoutSec = envTimeout
			logger.WithField("timeout_sec", envTimeout).Info("Using session startup timeout from environment variable")
		}
		startupTimeout := getTimeoutDuration(startupTimeoutSec, constants.DefaultSessionStartupTimeoutSec)

		sessionMonitor := service.NewSessionMonitorWithStartupTimeout(
			waClient,
			logger,
			checkInterval,
			startupTimeout,
		)
		sessionMonitor.Start(ctx)
		defer sessionMonitor.Stop()

		logger.WithFields(logrus.Fields{
			"interval":        checkInterval,
			"startup_timeout": startupTimeout,
		}).Info("Session health monitor started")
	}

	ctxWithVerbose := context.WithValue(ctx, service.VerboseContextKey, *verbose)

	signalPoller := service.NewSignalPoller(sigClient, messageService, cfg.Signal, models.RetryConfig{
		InitialBackoffMs: cfg.Retry.InitialBackoffMs,
		MaxBackoffMs:     cfg.Retry.MaxBackoffMs,
		MaxAttempts:      cfg.Retry.MaxAttempts,
	}, logger)

	if err := signalPoller.Start(ctxWithVerbose); err != nil {
		logger.Warnf("Failed to start Signal poller: %v", err)
	} else {
		// Only defer Stop() if Start() succeeded
		defer signalPoller.Stop()
	}

	// Safe type assertion for SignalClient
	signalClient, ok := sigClient.(*signalapi.SignalClient)
	if !ok {
		return fmt.Errorf("failed to assert sigClient as *signalapi.SignalClient")
	}

	server := NewServer(cfg, messageService, logger, waClient, channelManager, db, signalClient)
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
	if len(cfg.Channels) == 0 {
		return fmt.Errorf("at least one channel must be configured")
	}
	// Validate first channel has a session name
	if cfg.Channels[0].WhatsAppSessionName == "" {
		return fmt.Errorf("first channel must have a WhatsApp session name")
	}
	return nil
}

// getTimeoutDuration returns a duration from config value (in seconds), falling back to default if <= 0
func getTimeoutDuration(configValueSec int, defaultSec int) time.Duration {
	if configValueSec <= 0 {
		return time.Duration(defaultSec) * time.Second
	}
	return time.Duration(configValueSec) * time.Second
}

// getEnvInt reads an integer from environment variable, returning 0 if not set or invalid
func getEnvInt(key string) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return 0
}

// setLogLevel configures the logger's log level from config string, defaulting to Info
func setLogLevel(logger *logrus.Logger, configLevel string) {
	if configLevel == "" {
		logger.SetLevel(logrus.InfoLevel)
		return
	}

	level, err := logrus.ParseLevel(configLevel)
	if err != nil {
		logger.Warnf("Invalid log level %q, defaulting to info", configLevel)
		logger.SetLevel(logrus.InfoLevel)
		return
	}

	// Cap at InfoLevel unless explicitly set to debug/trace (verbose flag handles debug)
	if level > logrus.InfoLevel {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
}

// syncParallelContacts performs contact sync for all sessions in parallel with bounded concurrency
func syncParallelContacts(ctx context.Context, cfg *models.Config, db *database.Database, apiKey string, cacheHours int, logger *logrus.Logger) {
	channels := cfg.Channels
	if len(channels) == 0 {
		return
	}

	// Use bounded concurrency to avoid overwhelming the system
	maxConcurrency := constants.DefaultContactSyncBatchSize / constants.DefaultContactSyncConcurrencyDivisor
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	if maxConcurrency > constants.DefaultContactSyncMaxConcurrency {
		maxConcurrency = constants.DefaultContactSyncMaxConcurrency
	}

	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	logger.WithField("sessions", len(channels)).WithField("max_concurrent", maxConcurrency).Info("Starting parallel contact sync")

	for _, channel := range channels {
		wg.Add(1)
		go func(sessionName string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			syncSessionContacts(ctx, cfg, db, apiKey, sessionName, cacheHours, logger)
		}(channel.WhatsAppSessionName)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	logger.Info("Parallel contact sync completed")
}

// syncSessionContacts handles contact sync for a single session
func syncSessionContacts(ctx context.Context, cfg *models.Config, db *database.Database, apiKey, sessionName string, cacheHours int, logger *logrus.Logger) {
	sessionLogger := logger.WithField("session", sessionName)
	sessionLogger.Info("Waiting for WhatsApp session to be ready...")

	// Create a client for this specific session
	sessionClient := whatsapp.NewClient(types.ClientConfig{
		BaseURL:     cfg.WhatsApp.APIBaseURL,
		APIKey:      apiKey,
		SessionName: sessionName,
		Timeout:     cfg.WhatsApp.Timeout,
		RetryCount:  cfg.WhatsApp.RetryCount,
	})

	// Wait for session to be ready
	sessionReadyTimeout := time.Duration(constants.DefaultSessionReadyTimeoutSec) * time.Second
	if err := sessionClient.WaitForSessionReady(ctx, sessionReadyTimeout); err != nil {
		sessionLogger.Warnf("Failed to wait for session ready: %v. Skipping contact sync.", err)
		return
	}

	sessionLogger.Info("WhatsApp session is ready. Syncing contacts...")

	// Check session status before attempting contact sync
	sessionStatus, err := sessionClient.GetSessionStatus(ctx)
	if err != nil {
		sessionLogger.Warnf("Failed to get session status before contact sync: %v. This may indicate missing WHATSAPP_API_KEY or WAHA service issues. Skipping contact sync.", err)
		return
	}

	if sessionStatus == nil || sessionStatus.Status != "WORKING" {
		sessionLogger.Warnf("Session status is %v, not WORKING. Skipping contact sync.", sessionStatus)
		return
	}

	// Create a contact service for this session
	sessionContactService := service.NewContactServiceWithConfig(db, sessionClient, cacheHours)
	if err := sessionContactService.SyncAllContacts(ctx); err != nil {
		sessionLogger.Warnf("Failed to sync contacts on startup: %v. Contact names may not be available immediately.", err)
	} else {
		sessionLogger.Info("Contact sync completed successfully")
	}
}

// syncParallelGroups performs group sync for all sessions in parallel with bounded concurrency
func syncParallelGroups(ctx context.Context, cfg *models.Config, db *database.Database, apiKey string, cacheHours int, logger *logrus.Logger) {
	channels := cfg.Channels
	if len(channels) == 0 {
		return
	}

	// Use bounded concurrency to avoid overwhelming the system
	maxConcurrency := constants.DefaultContactSyncBatchSize / constants.DefaultContactSyncConcurrencyDivisor
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	if maxConcurrency > constants.DefaultContactSyncMaxConcurrency {
		maxConcurrency = constants.DefaultContactSyncMaxConcurrency
	}

	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	logger.WithField("sessions", len(channels)).WithField("max_concurrent", maxConcurrency).Info("Starting parallel group sync")

	for _, channel := range channels {
		wg.Add(1)
		go func(sessionName string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			syncSessionGroups(ctx, cfg, db, apiKey, sessionName, cacheHours, logger)
		}(channel.WhatsAppSessionName)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	logger.Info("Parallel group sync completed")
}

// syncSessionGroups handles group sync for a single session
func syncSessionGroups(ctx context.Context, cfg *models.Config, db *database.Database, apiKey, sessionName string, cacheHours int, logger *logrus.Logger) {
	sessionLogger := logger.WithField("session", sessionName)
	sessionLogger.Info("Waiting for WhatsApp session to be ready for group sync...")

	// Create a client for this specific session
	sessionClient := whatsapp.NewClient(types.ClientConfig{
		BaseURL:     cfg.WhatsApp.APIBaseURL,
		APIKey:      apiKey,
		SessionName: sessionName,
		Timeout:     cfg.WhatsApp.Timeout,
		RetryCount:  cfg.WhatsApp.RetryCount,
	})

	// Wait for session to be ready
	sessionReadyTimeout := time.Duration(constants.DefaultSessionReadyTimeoutSec) * time.Second
	if err := sessionClient.WaitForSessionReady(ctx, sessionReadyTimeout); err != nil {
		sessionLogger.Warnf("Failed to wait for session ready: %v. Skipping group sync.", err)
		return
	}

	sessionLogger.Info("WhatsApp session is ready. Syncing groups...")

	// Check session status before attempting group sync
	sessionStatus, err := sessionClient.GetSessionStatus(ctx)
	if err != nil {
		sessionLogger.Warnf("Failed to get session status before group sync: %v. This may indicate missing WHATSAPP_API_KEY or WAHA service issues. Skipping group sync.", err)
		return
	}

	if sessionStatus == nil || sessionStatus.Status != "WORKING" {
		sessionLogger.Warnf("Session status is %v, not WORKING. Skipping group sync.", sessionStatus)
		return
	}

	// Create a group service for this session
	sessionGroupService := service.NewGroupServiceWithConfig(db, sessionClient, cacheHours)
	if err := sessionGroupService.SyncAllGroups(ctx, sessionName); err != nil {
		sessionLogger.Warnf("Failed to sync groups on startup: %v. Group names may not be available immediately.", err)
	} else {
		sessionLogger.Info("Group sync completed successfully")
	}
}
