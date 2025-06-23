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
	"sync"
	"time"
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
	authToken      string
	client         *http.Client
	phoneNumber    string
	deviceName     string
	attachmentsDir string
	logger         *logrus.Logger
	mu             sync.Mutex // Prevent concurrent Signal-CLI operations
}

func NewClient(baseURL, authToken, phoneNumber, deviceName, attachmentsDir string, httpClient *http.Client) Client {
	return NewClientWithLogger(baseURL, authToken, phoneNumber, deviceName, attachmentsDir, httpClient, nil)
}

func NewClientWithLogger(baseURL, authToken, phoneNumber, deviceName, attachmentsDir string, httpClient *http.Client, logger *logrus.Logger) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second} // Increased timeout for polling
	}

	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel) // Default to warn level to reduce noise
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	return &SignalClient{
		baseURL:        baseURL,
		authToken:      authToken,
		phoneNumber:    phoneNumber,
		deviceName:     deviceName,
		attachmentsDir: attachmentsDir,
		client:         httpClient,
		logger:         logger,
	}
}

func (c *SignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*types.SendMessageResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	// Debug logging for troubleshooting
	c.logger.WithFields(logrus.Fields{
		"endpoint": fmt.Sprintf("%s/v2/send", c.baseURL),
		"payload":  string(jsonData),
	}).Debug("Sending Signal message request")

	endpoint := fmt.Sprintf("%s/v2/send", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	endpoint := fmt.Sprintf("%s/v1/receive/%s", c.baseURL, url.QueryEscape(c.phoneNumber))

	if timeoutSeconds > 0 {
		endpoint += fmt.Sprintf("?timeout=%d", timeoutSeconds)
	}

	c.logger.WithFields(logrus.Fields{
		"endpoint": endpoint,
		"timeout":  timeoutSeconds,
	}).Debug("Polling Signal messages")

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.WithError(err).Error("Failed to send Signal polling request")
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(bodyBytes),
		}).Error("Signal API returned error status")
		return nil, fmt.Errorf("signal API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
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
			Attachments: c.extractAttachmentPaths(msg.Envelope.DataMessage.Attachments),
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
		
		result = append(result, sigMsg)
	}

	return result, nil
}

func (c *SignalClient) extractAttachmentPaths(attachments []types.RestMessageAttachment) []string {
	if len(attachments) == 0 {
		return nil
	}

	var paths []string
	for _, att := range attachments {
		// First try to find the file using the configured path (direct filesystem access)
		directPath := c.getDirectAttachmentPath(att)
		if _, err := os.Stat(directPath); err == nil {
			// File exists, use direct path
			c.logger.WithFields(logrus.Fields{
				"attachmentID": att.ID,
				"path":         directPath,
			}).Debug("Found attachment file directly")
			paths = append(paths, directPath)
			continue
		}

		// Try fallback path without extension
		fallbackPath := c.fallbackAttachmentPath(att)
		if _, err := os.Stat(fallbackPath); err == nil {
			c.logger.WithFields(logrus.Fields{
				"attachmentID": att.ID,
				"path":         fallbackPath,
			}).Debug("Found attachment file at fallback path")
			paths = append(paths, fallbackPath)
			continue
		}

		// If files don't exist locally and we have an HTTP client, try downloading via API
		if c.client != nil {
			c.logger.WithFields(logrus.Fields{
				"attachmentID": att.ID,
			}).Debug("File not found locally, attempting download via API")

			// Use a goroutine with timeout to prevent blocking the entire polling operation
			downloadChan := make(chan string, 1)
			errorChan := make(chan error, 1)

			go func() {
				filePath, err := c.downloadAndSaveAttachment(att)
				if err != nil {
					errorChan <- err
				} else {
					downloadChan <- filePath
				}
			}()

			// Wait for download with timeout
			select {
			case filePath := <-downloadChan:
				paths = append(paths, filePath)
			case err := <-errorChan:
				// Log error but don't add non-existent file to paths
				c.logger.WithFields(logrus.Fields{
					"attachmentID": att.ID,
					"error":        err,
				}).Warn("Failed to download attachment, skipping")
				// Don't add non-existent paths that will cause media processing to fail
			case <-time.After(15 * time.Second):
				// Download timeout - don't block polling
				c.logger.WithFields(logrus.Fields{
					"attachmentID": att.ID,
				}).Warn("Attachment download timed out, skipping")
				// Don't add non-existent paths that will cause media processing to fail
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

func (c *SignalClient) downloadAndSaveAttachment(att types.RestMessageAttachment) (string, error) {
	// Create context with shorter timeout for download to avoid blocking polling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.logger.WithFields(logrus.Fields{
		"attachmentID": att.ID,
		"contentType":  att.ContentType,
		"filename":     att.Filename,
	}).Debug("Starting attachment download")

	// Download attachment data
	data, err := c.DownloadAttachment(ctx, att.ID)
	if err != nil {
		return "", fmt.Errorf("failed to download attachment: %w", err)
	}

	// Ensure attachments directory exists
	if c.attachmentsDir != "" {
		if err := os.MkdirAll(c.attachmentsDir, 0755); err != nil {
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

	// Write attachment data to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save attachment: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"attachmentID": att.ID,
		"filePath":     filePath,
		"size":         len(data),
	}).Debug("Successfully downloaded and saved attachment")

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
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/mov":
		return ".mov"
	case "audio/ogg":
		return ".ogg"
	case "audio/aac":
		return ".aac"
	case "audio/mp4":
		return ".m4a"
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	default:
		return ""
	}
}

func (c *SignalClient) InitializeDevice(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/v1/about", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create initialize device request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send initialize device request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device initialization failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var aboutResponse types.AboutResponse
	if err := json.NewDecoder(resp.Body).Decode(&aboutResponse); err != nil {
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
		return fmt.Errorf("signal-cli-rest-api service does not support required API versions (v1, v2)")
	}

	return nil
}

func (c *SignalClient) DownloadAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/v1/attachments/%s", c.baseURL, url.QueryEscape(attachmentID))
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create attachment download request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("attachment download failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment data: %w", err)
	}

	return data, nil
}

// EncodeAttachment is a public method for testing purposes
func (c *SignalClient) EncodeAttachment(filePath string) (string, string, string, error) {
	return c.encodeAttachment(filePath)
}

func (c *SignalClient) encodeAttachment(filePath string) (string, string, string, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
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
	switch ext {
	case ".jpg", ".jpeg", ".jfif":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".ogg":
		return "audio/ogg"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func (c *SignalClient) ListAttachments(ctx context.Context) ([]string, error) {
	endpoint := fmt.Sprintf("%s/v1/attachments", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list attachments request: %w", err)
	}

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list attachments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list attachments failed with status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var attachments []string
	if err := json.NewDecoder(resp.Body).Decode(&attachments); err != nil {
		return nil, fmt.Errorf("failed to decode attachments list: %w", err)
	}

	return attachments, nil
}
