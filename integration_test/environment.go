package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"whatsignal/internal/database"
	"whatsignal/internal/models"

	"github.com/stretchr/testify/require"
)

// EnvironmentManager handles the complete lifecycle of integration test environments
type EnvironmentManager struct {
	mu            sync.RWMutex
	environments  map[string]*TestEnvironment
	globalCleanup []func()
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager() *EnvironmentManager {
	return &EnvironmentManager{
		environments:  make(map[string]*TestEnvironment),
		globalCleanup: make([]func(), 0),
	}
}

// CreateEnvironment creates a new test environment with the given name
func (em *EnvironmentManager) CreateEnvironment(t *testing.T, name string) *TestEnvironment {
	em.mu.Lock()
	defer em.mu.Unlock()

	env := NewTestEnvironment(t, name, IsolationProcess)
	em.environments[name] = env

	return env
}

// GetEnvironment retrieves an existing test environment by name
func (em *EnvironmentManager) GetEnvironment(name string) (*TestEnvironment, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	env, exists := em.environments[name]
	return env, exists
}

// CleanupEnvironment cleans up a specific environment
func (em *EnvironmentManager) CleanupEnvironment(name string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if env, exists := em.environments[name]; exists {
		env.Cleanup()
		delete(em.environments, name)
	}
}

// CleanupAll cleans up all environments and global resources
func (em *EnvironmentManager) CleanupAll() {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Cleanup all environments
	for name, env := range em.environments {
		env.Cleanup()
		delete(em.environments, name)
	}

	// Run global cleanup functions
	for i := len(em.globalCleanup) - 1; i >= 0; i-- {
		em.globalCleanup[i]()
	}
	em.globalCleanup = em.globalCleanup[:0]
}

// AddGlobalCleanup adds a cleanup function that will be called during global cleanup
func (em *EnvironmentManager) AddGlobalCleanup(cleanup func()) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.globalCleanup = append(em.globalCleanup, cleanup)
}

// Enhanced TestEnvironment with additional features
type TestEnvironment struct {
	t               *testing.T
	name            string
	dbPath          string
	db              *database.Database
	mediaDir        string
	httpServer      *httptest.Server
	fixtures        *TestFixtures
	mediaSamples    *MediaSamples
	cleanup         []func()
	isolationMode   IsolationMode
	startTime       time.Time
	mockAPIRequests map[string]int
	mockAPIFailures map[string]int
	mockAPILock     sync.RWMutex
}

// IsolationMode defines how the test environment handles isolation
type IsolationMode int

const (
	// IsolationNone means no special isolation (shared resources)
	IsolationNone IsolationMode = iota
	// IsolationProcess means each environment gets its own database and files
	IsolationProcess
	// IsolationStrict means each test gets completely isolated resources
	IsolationStrict
)

// NewTestEnvironment creates a complete test environment with enhanced features
func NewTestEnvironment(t *testing.T, name string, isolation IsolationMode) *TestEnvironment {
	env := &TestEnvironment{
		t:               t,
		name:            fmt.Sprintf("%s_%d", name, time.Now().UnixNano()),
		fixtures:        NewTestFixtures(),
		mediaSamples:    NewMediaSamples(),
		cleanup:         make([]func(), 0),
		isolationMode:   isolation,
		startTime:       time.Now(),
		mockAPIRequests: make(map[string]int),
		mockAPIFailures: make(map[string]int),
	}

	// Set up components
	env.setupDatabase()
	env.setupMediaDirectory()
	env.setupHTTPServer()

	return env
}

// SetIsolationMode sets the isolation mode for this environment
func (env *TestEnvironment) SetIsolationMode(mode IsolationMode) {
	env.isolationMode = mode
}

// setupDatabase creates a real SQLite database for testing
func (env *TestEnvironment) setupDatabase() {
	// Use the test database helper to properly handle migrations
	opts := &TestDatabaseOptions{
		UseInMemory:      env.isolationMode != IsolationStrict,
		EncryptionSecret: "test-secret-key-for-integration-tests-32bytes!!",
	}

	db, cleanup := NewTestDatabase(env.t, opts)
	env.db = db
	env.dbPath = ":memory:" // Most tests use in-memory

	// Add cleanup to our cleanup stack
	env.cleanup = append(env.cleanup, cleanup)
}

// setupMediaDirectory creates a real media directory for testing
func (env *TestEnvironment) setupMediaDirectory() {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("whatsignal-integration-%s-media-", env.name))
	require.NoError(env.t, err)

	env.mediaDir = tmpDir

	env.cleanup = append(env.cleanup, func() {
		_ = os.RemoveAll(tmpDir)
	})
}

// setupHTTPServer creates a real HTTP server for testing
func (env *TestEnvironment) setupHTTPServer() {
	mux := http.NewServeMux()

	// Add basic routes for testing
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	})

	// Add WhatsApp API mock endpoints
	env.setupWhatsAppMockEndpoints(mux)

	// Add Signal API mock endpoints
	env.setupSignalMockEndpoints(mux)

	env.httpServer = httptest.NewServer(mux)

	env.cleanup = append(env.cleanup, func() {
		env.httpServer.Close()
	})
}

// setupWhatsAppMockEndpoints adds mock WhatsApp API endpoints
func (env *TestEnvironment) setupWhatsAppMockEndpoints(mux *http.ServeMux) {
	// Session status endpoint
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"name": "personal", "status": "WORKING"}, {"name": "business", "status": "WORKING"}]`))
	})

	// Get contacts endpoint
	mux.HandleFunc("/api/contacts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"contacts": []}`)) // Simplified for now
	})

	// Note: /api/sendText endpoint is registered in StartMessageFlowServer if needed
}

// setupSignalMockEndpoints adds mock Signal API endpoints
func (env *TestEnvironment) setupSignalMockEndpoints(mux *http.ServeMux) {
	// Signal about endpoint
	mux.HandleFunc("/v1/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version": "test", "status": "ok"}`))
	})

	// Note: /v1/send endpoint is registered in StartMessageFlowServer if needed
}

// Cleanup tears down all test resources
func (env *TestEnvironment) Cleanup() {
	for i := len(env.cleanup) - 1; i >= 0; i-- {
		env.cleanup[i]()
	}
}

// GetDatabase returns the test database service
func (env *TestEnvironment) GetDatabase() *database.Database {
	return env.db
}

// GetMediaDirectory returns the test media directory path
func (env *TestEnvironment) GetMediaDirectory() string {
	return env.mediaDir
}

// GetHTTPServer returns the test HTTP server
func (env *TestEnvironment) GetHTTPServer() *httptest.Server {
	return env.httpServer
}

// GetFixtures returns the test fixtures
func (env *TestEnvironment) GetFixtures() *TestFixtures {
	return env.fixtures
}

// GetMediaSamples returns the media samples
func (env *TestEnvironment) GetMediaSamples() *MediaSamples {
	return env.mediaSamples
}

// GetConfig creates a test configuration using the environment's resources
func (env *TestEnvironment) GetConfig() *models.Config {
	return &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL:            env.httpServer.URL,
			WebhookSecret:         "test-webhook-secret",
			ContactSyncOnStartup:  true,
			ContactCacheHours:     24,
			SessionHealthCheckSec: 300,
			SessionAutoRestart:    true,
		},
		Signal: models.SignalConfig{
			RPCURL:                  env.httpServer.URL,
			IntermediaryPhoneNumber: "+1234567890",
			DeviceName:              "test-device",
			HTTPTimeoutSec:          30,
		},
		Database: models.DatabaseConfig{
			Path:               env.dbPath,
			MaxOpenConnections: 10,
			MaxIdleConnections: 5,
			ConnMaxLifetimeSec: 300,
			ConnMaxIdleTimeSec: 60,
		},
		Media: models.MediaConfig{
			CacheDir: env.mediaDir,
			MaxSizeMB: models.MediaSizeLimits{
				Image:    10,
				Video:    50,
				Document: 20,
				Voice:    5,
			},
			AllowedTypes: models.MediaAllowedTypes{
				Image:    []string{".jpg", ".jpeg", ".png", ".gif"},
				Video:    []string{".mp4", ".avi", ".mov"},
				Document: []string{".pdf", ".doc", ".docx"},
				Voice:    []string{".mp3", ".wav", ".ogg"},
			},
			DownloadTimeout: 60,
		},
		Server: models.ServerConfig{
			ReadTimeoutSec:          30,
			WriteTimeoutSec:         30,
			IdleTimeoutSec:          60,
			WebhookMaxSkewSec:       300,
			WebhookMaxBytes:         1048576,
			RateLimitPerMinute:      100,
			RateLimitCleanupMinutes: 60,
			CleanupIntervalHours:    24,
		},
		Channels: []models.Channel{
			{
				WhatsAppSessionName:          "personal",
				SignalDestinationPhoneNumber: "+1111111111",
			},
			{
				WhatsAppSessionName:          "business",
				SignalDestinationPhoneNumber: "+2222222222",
			},
		},
		RetentionDays: 30,
		LogLevel:      "info",
	}
}

// Database test helpers

// PopulateWithFixtures populates the database with standard test fixtures
func (env *TestEnvironment) PopulateWithFixtures() error {
	ctx := context.Background()

	// Add contacts
	contacts := env.fixtures.Contacts()
	for _, contact := range contacts {
		if err := env.db.SaveContact(ctx, &contact); err != nil {
			return fmt.Errorf("failed to save contact: %w", err)
		}
	}

	// Add message mappings
	mappings := env.fixtures.MessageMappings()
	for _, mapping := range mappings {
		if err := env.db.SaveMessageMapping(ctx, &mapping); err != nil {
			return fmt.Errorf("failed to save message mapping: %w", err)
		}
	}

	return nil
}

// CreateTestContact creates a test contact in the database
func (env *TestEnvironment) CreateTestContact(contactID, phoneNumber, name string) *models.Contact {
	contact := &models.Contact{
		ContactID:   contactID,
		PhoneNumber: phoneNumber,
		Name:        name,
		CachedAt:    time.Now(),
	}

	ctx := context.Background()
	err := env.db.SaveContact(ctx, contact)
	require.NoError(env.t, err)

	return contact
}

// CreateTestMessageMapping creates a test message mapping in the database
func (env *TestEnvironment) CreateTestMessageMapping(whatsappID, signalID, sessionName string) *models.MessageMapping {
	mapping := &models.MessageMapping{
		WhatsAppChatID:  sessionName + "@c.us",
		WhatsAppMsgID:   whatsappID,
		SignalMsgID:     signalID,
		SessionName:     sessionName,
		DeliveryStatus:  models.DeliveryStatusPending,
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	ctx := context.Background()
	err := env.db.SaveMessageMapping(ctx, mapping)
	require.NoError(env.t, err)

	return mapping
}

// VerifyDatabaseConnection ensures the database is accessible
func (env *TestEnvironment) VerifyDatabaseConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return env.db.HealthCheck(ctx)
}

// Media test helpers

// CreateTestMediaFile creates a test media file with specified content
func (env *TestEnvironment) CreateTestMediaFile(filename, content string) string {
	filePath := filepath.Join(env.mediaDir, filename)

	err := os.WriteFile(filePath, []byte(content), 0600)
	require.NoError(env.t, err)

	return filePath
}

// CreateTestImageFile creates a test image file (PNG)
func (env *TestEnvironment) CreateTestImageFile(filename string) string {
	pngData := env.mediaSamples.SmallImage()
	filePath := filepath.Join(env.mediaDir, filename)
	err := os.WriteFile(filePath, pngData, 0600)
	require.NoError(env.t, err)

	return filePath
}

// VerifyFileExists checks if a file exists at the given path
func (env *TestEnvironment) VerifyFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// GetFileSize returns the size of a file
func (env *TestEnvironment) GetFileSize(filePath string) int64 {
	info, err := os.Stat(filePath)
	require.NoError(env.t, err)
	return info.Size()
}

// HTTP test helpers

// AddHTTPHandler adds a custom handler to the test HTTP server
func (env *TestEnvironment) AddHTTPHandler(pattern string, handler http.HandlerFunc) {
	mux := env.httpServer.Config.Handler.(*http.ServeMux)
	mux.HandleFunc(pattern, handler)
}

// MakeHTTPRequest makes an HTTP request to the test server
func (env *TestEnvironment) MakeHTTPRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := env.httpServer.URL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

// VerifyHTTPEndpoint checks if an endpoint returns the expected status code
func (env *TestEnvironment) VerifyHTTPEndpoint(path string, expectedStatus int) {
	resp, err := env.MakeHTTPRequest("GET", path, nil)
	require.NoError(env.t, err)
	defer resp.Body.Close()

	require.Equal(env.t, expectedStatus, resp.StatusCode)
}

// Performance and monitoring helpers

// GetEnvironmentStats returns statistics about the test environment
func (env *TestEnvironment) GetEnvironmentStats() map[string]interface{} {
	stats := map[string]interface{}{
		"name":       env.name,
		"uptime":     time.Since(env.startTime),
		"database":   env.dbPath,
		"media_dir":  env.mediaDir,
		"server_url": env.httpServer.URL,
		"isolation":  env.isolationMode,
	}

	// Add database stats if available
	if env.db != nil {
		stats["database_healthy"] = env.VerifyDatabaseConnection() == nil
	}

	// Add media directory stats
	if env.mediaDir != "" {
		if info, err := os.Stat(env.mediaDir); err == nil {
			stats["media_dir_exists"] = info.IsDir()
		}
	}

	return stats
}

// WaitForCondition waits for a condition to become true within a timeout
func (env *TestEnvironment) WaitForCondition(condition func() bool, timeout time.Duration, checkInterval time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(checkInterval)
	}

	return false
}

// Message flow test helpers

// StartMessageFlowServer starts a mock server for message flow testing
func (env *TestEnvironment) StartMessageFlowServer() {
	env.mockAPILock.Lock()
	defer env.mockAPILock.Unlock()

	// Reset counters
	env.mockAPIRequests = make(map[string]int)
	env.mockAPIFailures = make(map[string]int)

	// Add webhook endpoints
	mux := env.httpServer.Config.Handler.(*http.ServeMux)

	// WhatsApp webhook endpoint
	mux.HandleFunc("/webhook/whatsapp", func(w http.ResponseWriter, r *http.Request) {
		env.incrementMockAPIRequest("webhook_whatsapp")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "received"}`))
	})

	// Signal webhook endpoint
	mux.HandleFunc("/webhook/signal", func(w http.ResponseWriter, r *http.Request) {
		env.incrementMockAPIRequest("webhook_signal")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "received"}`))
	})

	// WhatsApp ACK endpoint
	mux.HandleFunc("/api/ack", func(w http.ResponseWriter, r *http.Request) {
		env.incrementMockAPIRequest("ack")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "acked"}`))
	})

	// Signal send endpoint (override the basic one with tracking)
	mux.HandleFunc("/v1/send", func(w http.ResponseWriter, r *http.Request) {
		failures := env.getMockAPIFailures("send")
		if failures > 0 {
			env.decrementMockAPIFailures("send")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "temporary failure"}`))
			return
		}

		env.incrementMockAPIRequest("send")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"timestamp": 123456789, "message": "sent"}`))
	})

	// WhatsApp send endpoint
	mux.HandleFunc("/api/sendText", func(w http.ResponseWriter, r *http.Request) {
		failures := env.getMockAPIFailures("whatsapp_send")
		if failures > 0 {
			env.decrementMockAPIFailures("whatsapp_send")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "temporary failure"}`))
			return
		}

		env.incrementMockAPIRequest("whatsapp_send")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "wamid.test123", "status": "sent"}`))
	})
}

// CountMockAPIRequests returns the number of requests made to a specific mock API endpoint
func (env *TestEnvironment) CountMockAPIRequests(endpoint string) int {
	env.mockAPILock.RLock()
	defer env.mockAPILock.RUnlock()
	return env.mockAPIRequests[endpoint]
}

// SetMockAPIFailures sets the number of failures for a specific endpoint
func (env *TestEnvironment) SetMockAPIFailures(endpoint string, failures int) {
	env.mockAPILock.Lock()
	defer env.mockAPILock.Unlock()
	env.mockAPIFailures[endpoint] = failures
}

// incrementMockAPIRequest increments the request counter for an endpoint
func (env *TestEnvironment) incrementMockAPIRequest(endpoint string) {
	env.mockAPILock.Lock()
	defer env.mockAPILock.Unlock()
	env.mockAPIRequests[endpoint]++
}

// getMockAPIFailures gets the current failure count for an endpoint
func (env *TestEnvironment) getMockAPIFailures(endpoint string) int {
	env.mockAPILock.RLock()
	defer env.mockAPILock.RUnlock()
	return env.mockAPIFailures[endpoint]
}

// decrementMockAPIFailures decrements the failure count for an endpoint
func (env *TestEnvironment) decrementMockAPIFailures(endpoint string) {
	env.mockAPILock.Lock()
	defer env.mockAPILock.Unlock()
	if env.mockAPIFailures[endpoint] > 0 {
		env.mockAPIFailures[endpoint]--
	}
}

// GetMemoryUsage returns current memory usage statistics
func (env *TestEnvironment) GetMemoryUsage() MemorySnapshot {
	return TakeMemorySnapshot()
}
