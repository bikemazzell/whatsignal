package signal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"whatsignal/internal/constants"
	"whatsignal/internal/security"
	"whatsignal/pkg/circuitbreaker"
	"whatsignal/pkg/signal/types"

	"github.com/sirupsen/logrus"
)

type Client interface {
	SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error)
	ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]types.SignalMessage, error)
	InitializeDevice(ctx context.Context) error
	DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error)
	ListAttachments(ctx context.Context) ([]string, error)
}

type SignalClient struct {
	baseURL        string
	client         *http.Client
	phoneNumber    string
	deviceName     string
	attachmentsDir string
	logger         *logrus.Logger
	circuitBreaker *circuitbreaker.CircuitBreaker
	initialized    bool   // Tracks whether InitializeDevice succeeded
	initError      string // Stores initialization error message if any
}

func NewClient(baseURL, phoneNumber, deviceName, attachmentsDir string, httpClient *http.Client) Client {
	return NewClientWithLogger(baseURL, phoneNumber, deviceName, attachmentsDir, httpClient, nil)
}

func NewClientWithLogger(baseURL, phoneNumber, deviceName, attachmentsDir string, httpClient *http.Client, logger *logrus.Logger) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Duration(constants.DefaultSignalHTTPTimeoutSec) * time.Second}
	}

	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel) // Default to warn level to reduce noise
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	return &SignalClient{
		baseURL:        baseURL,
		phoneNumber:    phoneNumber,
		deviceName:     deviceName,
		attachmentsDir: attachmentsDir,
		client:         httpClient,
		logger:         logger,
		circuitBreaker: circuitbreaker.NewWithLogger("signal-api", 5, 30*time.Second, logger),
	}
}

// doRequestWithCircuitBreaker wraps HTTP requests with circuit breaker protection
func (c *SignalClient) doRequestWithCircuitBreaker(ctx context.Context, req *http.Request) (*http.Response, error) {
	// If circuit breaker is not initialized (e.g., in tests), fall back to direct call
	if c.circuitBreaker == nil {
		return c.client.Do(req)
	}

	var resp *http.Response
	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		var httpErr error
		resp, httpErr = c.client.Do(req)
		return httpErr
	})
	return resp, err
}

func (c *SignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error) {
	payload := types.SendMessageRequest{
		Message:    message,
		Number:     c.phoneNumber,
		Recipients: []string{recipient},
	}

	if len(attachments) > 0 {
		payload.Base64Attachments = make([]string, len(attachments))
		for i, attachment := range attachments {
			// Read and encode the attachment file
			encodedData, _, _, err := c.encodeAttachment(attachment)
			if err != nil {
				return nil, fmt.Errorf("failed to encode attachment %s: %w", attachment, err)
			}

			payload.Base64Attachments[i] = encodedData
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v2/send", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Signal CLI REST API typically doesn't require authentication headers

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("signal API error: status %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result types.SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	timestamp := result.Timestamp.Int64()
	response := &types.SendMessageResponse{
		Timestamp: timestamp,
		MessageID: fmt.Sprintf("%d", timestamp),
	}

	return response, nil
}

func (c *SignalClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]types.SignalMessage, error) {
	endpoint := fmt.Sprintf("%s/v1/receive/%s", c.baseURL, url.QueryEscape(c.phoneNumber))

	if timeoutSeconds > 0 {
		endpoint += fmt.Sprintf("?timeout=%d", timeoutSeconds)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Signal CLI REST API typically doesn't require authentication headers

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		c.logger.WithError(err).Error("Failed to send Signal polling request")
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			c.logger.WithError(readErr).WithField("status", resp.StatusCode).Error("Failed to read Signal API error response body")
			return nil, fmt.Errorf("signal API error: status %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		bodyStr := string(bodyBytes)

		// Check if this is a transient connection error that can be retried
		isTransientError := strings.Contains(bodyStr, "Closed unexpectedly") ||
			strings.Contains(bodyStr, "connection") ||
			strings.Contains(bodyStr, "timeout") ||
			strings.Contains(bodyStr, "TimeoutException")

		if isTransientError {
			c.logger.WithFields(logrus.Fields{
				"status": resp.StatusCode,
				"error":  bodyStr,
			}).Warn("Signal API connection issue (will retry)")
		} else {
			c.logger.WithFields(logrus.Fields{
				"status": resp.StatusCode,
				"body":   bodyStr,
			}).Error("Signal API returned error status")
		}

		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, bodyStr)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var messages []types.RestMessage
	if err := json.Unmarshal(bodyBytes, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := make([]types.SignalMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Envelope.DataMessage == nil {
			continue
		}

		sigMsg := types.SignalMessage{
			Timestamp:   msg.Envelope.Timestamp,
			Sender:      msg.Envelope.Source,
			MessageID:   fmt.Sprintf("%d", msg.Envelope.Timestamp),
			Message:     msg.Envelope.DataMessage.Message,
			Attachments: c.extractAttachmentPaths(ctx, msg.Envelope.DataMessage.Attachments),
		}

		// Handle remote deletion
		if msg.Envelope.DataMessage.RemoteDelete != nil {
			sigMsg.Deletion = &types.SignalDeletion{
				TargetMessageID: fmt.Sprintf("%d", msg.Envelope.DataMessage.RemoteDelete.Timestamp),
				TargetTimestamp: msg.Envelope.DataMessage.RemoteDelete.Timestamp,
			}
		}

		if msg.Envelope.DataMessage.Quote != nil {
			sigMsg.QuotedMessage = &struct {
				ID        string `json:"id"`
				Author    string `json:"author"`
				Text      string `json:"text"`
				Timestamp int64  `json:"timestamp"`
			}{
				ID:        fmt.Sprintf("%d", msg.Envelope.DataMessage.Quote.ID),
				Author:    msg.Envelope.DataMessage.Quote.Author,
				Text:      msg.Envelope.DataMessage.Quote.Text,
				Timestamp: msg.Envelope.DataMessage.Quote.ID,
			}
		}

		if msg.Envelope.DataMessage.Reaction != nil {
			sigMsg.Reaction = &types.SignalReaction{
				Emoji:           msg.Envelope.DataMessage.Reaction.Emoji,
				TargetAuthor:    msg.Envelope.DataMessage.Reaction.TargetAuthor,
				TargetTimestamp: msg.Envelope.DataMessage.Reaction.TargetTimestamp,
				IsRemove:        msg.Envelope.DataMessage.Reaction.IsRemove,
			}
			// For reactions, we might not have a regular message text
			if sigMsg.Message == "" && sigMsg.Reaction != nil {
				sigMsg.Message = sigMsg.Reaction.Emoji // Store emoji as message for easy access
			}
		}

		result = append(result, sigMsg)
	}

	return result, nil
}

func (c *SignalClient) extractAttachmentPaths(ctx context.Context, attachments []types.RestMessageAttachment) []string {
	if len(attachments) == 0 {
		return nil
	}

	var paths []string
	for _, att := range attachments {
		// First try to find the file using the configured path (direct filesystem access)
		directPath := c.getDirectAttachmentPath(att)
		if _, err := os.Stat(directPath); err == nil {
			// File exists, use direct path
			paths = append(paths, directPath)
			continue
		}

		// Try fallback path without extension
		fallbackPath := c.fallbackAttachmentPath(att)
		if _, err := os.Stat(fallbackPath); err == nil {
			paths = append(paths, fallbackPath)
			continue
		}

		// If files don't exist locally and we have an HTTP client, try downloading via API
		if c.client != nil {
			// Use a goroutine with timeout to prevent blocking the entire polling operation
			downloadChan := make(chan string, 1)
			errorChan := make(chan error, 1)
			downloadCtx, downloadCancel := context.WithTimeout(ctx, time.Duration(constants.AttachmentDownloadTimeoutSec)*time.Second)

			// Capture attachment in closure to avoid loop variable issues
			go func(attachment types.RestMessageAttachment) {
				filePath, err := c.downloadAndSaveAttachment(downloadCtx, attachment)
				if err != nil {
					select {
					case errorChan <- err:
					case <-downloadCtx.Done():
						// Context cancelled, exit gracefully
					}
				} else {
					select {
					case downloadChan <- filePath:
					case <-downloadCtx.Done():
						// Context cancelled, exit gracefully
					}
				}
			}(att)

			// Wait for download with timeout
			select {
			case filePath := <-downloadChan:
				paths = append(paths, filePath)
				downloadCancel() // Cleanup after successful download
			case err := <-errorChan:
				// Log error but don't add non-existent file to paths
				c.logger.WithFields(logrus.Fields{
					"attachmentID": att.ID,
					"error":        err,
				}).Warn("Failed to download attachment, skipping")
				downloadCancel() // Cleanup after error
			case <-downloadCtx.Done():
				// Download timeout or context cancelled - don't block polling
				c.logger.WithFields(logrus.Fields{
					"attachmentID": att.ID,
				}).Warn("Attachment download timed out or cancelled, skipping")
				// downloadCancel already called by context timeout, but calling again is safe
				downloadCancel()
			}
		} else {
			// No HTTP client available - skip attachment
			c.logger.WithFields(logrus.Fields{
				"attachmentID": att.ID,
			}).Warn("No HTTP client available and file not found locally, skipping attachment")
			// Don't add non-existent paths that will cause media processing to fail
		}
	}

	// Return empty slice instead of nil if no paths were found
	if len(paths) == 0 {
		return []string{}
	}
	return paths
}

func (c *SignalClient) getDirectAttachmentPath(att types.RestMessageAttachment) string {
	// Try with file extension from filename or content type
	ext := c.getFileExtension(att.ContentType, att.Filename)
	filename := att.ID + ext

	if c.attachmentsDir != "" {
		// First try with extension
		pathWithExt := filepath.Join(c.attachmentsDir, filename)
		if _, err := os.Stat(pathWithExt); err == nil {
			return pathWithExt
		}

		// Try without extension (original ID only)
		pathWithoutExt := filepath.Join(c.attachmentsDir, att.ID)
		return pathWithoutExt
	}

	return att.ID
}

func (c *SignalClient) fallbackAttachmentPath(att types.RestMessageAttachment) string {
	if c.attachmentsDir != "" {
		return filepath.Join(c.attachmentsDir, att.ID)
	}
	return att.ID
}

func (c *SignalClient) downloadAndSaveAttachment(ctx context.Context, att types.RestMessageAttachment) (string, error) {
	// Ensure attachments directory exists
	if c.attachmentsDir != "" {
		// Use more restrictive permissions for security
		if err := os.MkdirAll(c.attachmentsDir, constants.DefaultDirectoryPermissions); err != nil {
			return "", fmt.Errorf("failed to create attachments directory: %w", err)
		}
	}

	// Determine file extension from content type or filename
	ext := c.getFileExtension(att.ContentType, att.Filename)

	// Create unique filename using attachment ID and extension
	filename := att.ID + ext
	var filePath string
	if c.attachmentsDir != "" {
		filePath = filepath.Join(c.attachmentsDir, filename)
	} else {
		filePath = filename
	}

	// Stream attachment directly to file (avoids loading entire file into memory)
	if err := c.DownloadAttachmentToFile(ctx, att.ID, filePath); err != nil {
		return "", fmt.Errorf("failed to download attachment: %w", err)
	}

	return filePath, nil
}

func (c *SignalClient) getFileExtension(contentType, filename string) string {
	// First try to get extension from filename
	if filename != "" {
		ext := filepath.Ext(filename)
		if ext != "" {
			return ext
		}
	}

	// Fallback to content type mapping
	if ext, ok := constants.MimeTypeToExtension[contentType]; ok {
		return ext
	}
	return ""
}

func (c *SignalClient) InitializeDevice(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/v1/about", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		c.initError = fmt.Sprintf("failed to create initialize device request: %v", err)
		return fmt.Errorf("failed to create initialize device request: %w", err)
	}

	// Signal CLI REST API typically doesn't require authentication headers

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		c.initError = fmt.Sprintf("failed to send initialize device request: %v", err)
		return fmt.Errorf("failed to send initialize device request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			c.initError = fmt.Sprintf("device initialization failed with status: %d (failed to read body: %v)", resp.StatusCode, readErr)
			return fmt.Errorf("device initialization failed with status: %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		c.initError = fmt.Sprintf("device initialization failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("device initialization failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var aboutResponse types.AboutResponse
	if err := json.NewDecoder(resp.Body).Decode(&aboutResponse); err != nil {
		c.initError = fmt.Sprintf("failed to decode about response: %v", err)
		return fmt.Errorf("failed to decode about response: %w", err)
	}

	hasV1 := false
	hasV2 := false
	for _, version := range aboutResponse.Versions {
		if version == "v1" {
			hasV1 = true
		}
		if version == "v2" {
			hasV2 = true
		}
	}

	if !hasV1 || !hasV2 {
		initErr := fmt.Errorf("signal-cli-rest-api service does not support required API versions (v1, v2)")
		c.initError = initErr.Error()
		return initErr
	}

	c.initialized = true
	c.initError = ""
	return nil
}

func (c *SignalClient) DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/v1/attachments/%s", c.baseURL, url.QueryEscape(attachmentID))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create attachment download request: %w", err)
	}

	// Signal CLI REST API typically doesn't require authentication headers

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("attachment download failed with status: %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("attachment download failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment data: %w", err)
	}

	return data, nil
}

func (c *SignalClient) DownloadAttachmentToFile(ctx context.Context, attachmentID, destPath string) error {
	// Validate destination path to prevent directory traversal
	if err := security.ValidateFilePath(destPath); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1/attachments/%s", c.baseURL, url.QueryEscape(attachmentID))

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create attachment download request: %w", err)
	}

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to download attachment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("attachment download failed with status: %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("attachment download failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, constants.DefaultFilePermissions) // #nosec G304 - path validated above
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("failed to write attachment data: %w", err)
	}

	return nil
}

// EncodeAttachment is a public method for testing purposes
func (c *SignalClient) EncodeAttachment(filePath string) (string, string, string, error) {
	return c.encodeAttachment(filePath)
}

func (c *SignalClient) encodeAttachment(filePath string) (string, string, string, error) {
	// Validate file path to prevent directory traversal
	if err := security.ValidateFilePath(filePath); err != nil {
		return "", "", "", fmt.Errorf("invalid attachment path: %w", err)
	}

	// Read the file
	data, err := os.ReadFile(filePath) // #nosec G304 - Path validated by security.ValidateFilePath above
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read attachment file: %w", err)
	}

	// Base64 encode the file data
	encodedData := base64.StdEncoding.EncodeToString(data)

	// Detect content type from file extension
	contentType := c.detectContentType(filePath)

	// Extract just the filename from the path
	filename := filepath.Base(filePath)

	return encodedData, contentType, filename, nil
}

func (c *SignalClient) detectContentType(filePath string) string {
	// First try to detect from file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Use MIME type detection
	contentType := mime.TypeByExtension(ext)
	if contentType != "" {
		return contentType
	}

	// Fallback to manual mapping for common types
	if mimeType, ok := constants.MimeTypes[ext]; ok {
		return mimeType
	}
	return constants.DefaultMimeType
}

func (c *SignalClient) ListAttachments(ctx context.Context) ([]string, error) {
	endpoint := fmt.Sprintf("%s/v1/attachments", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list attachments request: %w", err)
	}

	// Signal CLI REST API typically doesn't require authentication headers

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("list attachments failed with status: %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("list attachments failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var attachments []string
	if err := json.NewDecoder(resp.Body).Decode(&attachments); err != nil {
		return nil, fmt.Errorf("failed to decode attachments list: %w", err)
	}

	return attachments, nil
}

// HealthCheck performs a health check on the Signal API
func (c *SignalClient) HealthCheck(ctx context.Context) error {
	// Try to receive messages as a health check (this is a lightweight operation)
	endpoint := fmt.Sprintf("%s/v1/receive/%s", c.baseURL, c.phoneNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create Signal health check request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequestWithCircuitBreaker(ctx, req)
	if err != nil {
		return fmt.Errorf("signal API health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check if we got a successful response (2xx)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Read the response body for error details
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("signal API health check returned status %d (failed to read body: %v)", resp.StatusCode, readErr)
	}
	return fmt.Errorf("signal API health check returned status %d: %s", resp.StatusCode, string(body))
}

// IsInitialized returns whether the Signal client was successfully initialized
func (c *SignalClient) IsInitialized() bool {
	return c.initialized
}

// InitializationError returns the initialization error message, if any
func (c *SignalClient) InitializationError() string {
	return c.initError
}
