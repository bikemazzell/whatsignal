package service

// Logging Standards for WhatSignal
//
// This file defines standard field names, log levels, and patterns
// to ensure consistent logging across the application.

// Standard Field Names
// Use these exact field names for consistency across all logging calls
const (
	// Core identifiers
	LogFieldSession   = "session"
	LogFieldMessageID = "message_id"
	LogFieldChatID    = "chat_id"
	LogFieldUserID    = "user_id"
	LogFieldContactID = "contact_id"

	// Service and operation fields
	LogFieldService   = "service"
	LogFieldOperation = "operation"
	LogFieldComponent = "component"
	LogFieldMethod    = "method"

	// Message and event fields
	LogFieldEvent       = "event"
	LogFieldMessageType = "message_type"
	LogFieldPlatform    = "platform"
	LogFieldDirection   = "direction" // "incoming" or "outgoing"

	// Performance and metrics
	LogFieldDuration = "duration_ms"
	LogFieldCount    = "count"
	LogFieldSize     = "size_bytes"

	// Network and external services
	LogFieldURL        = "url"
	LogFieldEndpoint   = "endpoint"
	LogFieldStatusCode = "status_code"
	LogFieldRemoteIP   = "remote_ip"

	// File and media
	LogFieldFilePath  = "file_path"
	LogFieldFileName  = "file_name"
	LogFieldMediaType = "media_type"
	LogFieldFileSize  = "file_size"

	// Error and debugging
	LogFieldErrorCode  = "error_code"
	LogFieldErrorType  = "error_type"
	LogFieldRetryCount = "retry_count"
	LogFieldAttempt    = "attempt"
)

// Log Level Usage Guidelines
//
// DEBUG: Detailed information for diagnosing problems. Only use in development or verbose mode.
//   - Function entry/exit
//   - Variable values
//   - Detailed flow information
//   - Raw request/response data (sanitized)
//
// INFO: General information about application flow and key events.
//   - Application startup/shutdown
//   - Major state changes
//   - Successful operations
//   - Configuration loaded
//   - Services started/stopped
//
// WARN: Something unexpected happened, but the application can continue.
//   - Retryable errors
//   - Fallback behavior used
//   - Configuration issues (using defaults)
//   - Rate limiting triggered
//   - External service temporarily unavailable
//
// ERROR: Error events that might still allow the application to continue.
//   - Failed operations
//   - External service errors
//   - Data validation failures
//   - Authentication failures
//
// FATAL: Very severe error events that will presumably lead the application to abort.
//   - Configuration required for startup is missing
//   - Critical resources unavailable
//   - Database connection failed and cannot be recovered

// Standard Log Message Patterns
//
// Use these patterns for consistent messaging:
//
// Starting operations: "Starting [operation]"
// Completed operations: "Completed [operation]" or "[Operation] completed successfully"
// Failed operations: "Failed to [operation]"
// Retrying operations: "Retrying [operation] (attempt X/Y)"
// Skipping operations: "Skipping [operation]: [reason]"
// Configuration: "Loaded [config type] configuration" / "Using default [setting]"
// External services: "[Service] request completed" / "Failed to connect to [service]"

// Example Usage:
//
// logger.WithFields(logrus.Fields{
//     LogFieldSession:   sessionName,
//     LogFieldMessageID: messageID,
//     LogFieldPlatform:  "whatsapp",
//     LogFieldDirection: "incoming",
// }).Info("Processing WhatsApp message")
//
// logger.WithFields(logrus.Fields{
//     LogFieldService:   "signal",
//     LogFieldOperation: "send_message",
//     LogFieldDuration:  duration.Milliseconds(),
//     LogFieldAttempt:   retryCount,
// }).Debug("Message send attempt completed")
