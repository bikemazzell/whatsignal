package errors

import (
	"context"
	"fmt"
)

type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	traceIDKey     contextKey = "trace_id"
	userIDKey      contextKey = "user_id"
	sessionNameKey contextKey = "session_name"
)

// Context helpers

// FromContext extracts error context from a context.Context if present
func FromContext(ctx context.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}

	errorCtx := make(map[string]interface{})

	// Extract common context values
	if requestID := ctx.Value(requestIDKey); requestID != nil {
		errorCtx["request_id"] = requestID
	}
	if traceID := ctx.Value(traceIDKey); traceID != nil {
		errorCtx["trace_id"] = traceID
	}
	if userID := ctx.Value(userIDKey); userID != nil {
		errorCtx["user_id"] = userID
	}
	if sessionName := ctx.Value(sessionNameKey); sessionName != nil {
		errorCtx["session_name"] = sessionName
	}

	return errorCtx
}

// WithContextFromRequest adds request context to an error
func WithContextFromRequest(err *AppError, ctx context.Context) *AppError {
	if err == nil || ctx == nil {
		return err
	}

	contextMap := FromContext(ctx)
	for k, v := range contextMap {
		err = err.WithContext(k, v)
	}

	return err
}

// HTTP helpers

// HTTPStatusCode maps error codes to appropriate HTTP status codes
func HTTPStatusCode(err error) int {
	code := GetCode(err)

	switch code {
	case ErrCodeValidationFailed, ErrCodeInvalidInput, ErrCodeInvalidConfig:
		return 400 // Bad Request
	case ErrCodeAuthentication:
		return 401 // Unauthorized
	case ErrCodeAuthorization:
		return 403 // Forbidden
	case ErrCodeNotFound:
		return 404 // Not Found
	case ErrCodeRateLimit:
		return 429 // Too Many Requests
	case ErrCodeTimeout:
		return 408 // Request Timeout
	case ErrCodeWhatsAppAPI, ErrCodeSignalAPI, ErrCodeMediaDownload:
		// If it's retryable, it's a temporary issue (502/503)
		if IsRetryable(err) {
			return 502 // Bad Gateway
		}
		return 500 // Internal Server Error
	case ErrCodeDatabaseConnection, ErrCodeDatabaseQuery, ErrCodeDatabaseMigration:
		return 503 // Service Unavailable
	default:
		return 500 // Internal Server Error
	}
}

// HTTPResponse creates a standardized HTTP error response
type HTTPErrorResponse struct {
	Error struct {
		Code    ErrorCode   `json:"code"`
		Message string      `json:"message"`
		Context interface{} `json:"context,omitempty"`
	} `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

// ToHTTPResponse converts an error to a standardized HTTP response
func ToHTTPResponse(err error, requestID string) HTTPErrorResponse {
	response := HTTPErrorResponse{
		RequestID: requestID,
	}

	if appErr, ok := err.(*AppError); ok {
		response.Error.Code = appErr.Code
		response.Error.Message = GetUserMessage(err)
		if len(appErr.Context) > 0 {
			// Only include non-sensitive context in HTTP responses
			publicContext := make(map[string]interface{})
			for k, v := range appErr.Context {
				// Exclude sensitive fields from HTTP responses
				if k != "password" && k != "token" && k != "secret" {
					publicContext[k] = v
				}
			}
			if len(publicContext) > 0 {
				response.Error.Context = publicContext
			}
		}
	} else {
		response.Error.Code = ErrCodeInternalError
		response.Error.Message = GetUserMessage(err)
	}

	return response
}

// Chain multiple errors together for complex operations
func Chain(errors ...*AppError) *AppError {
	if len(errors) == 0 {
		return nil
	}
	if len(errors) == 1 {
		return errors[0]
	}

	primary := errors[0]
	var messages []string
	var allContext = make(map[string]interface{})

	for i, err := range errors {
		if i == 0 {
			messages = append(messages, err.Message)
		} else {
			messages = append(messages, fmt.Sprintf("(%d) %s", i+1, err.Message))
		}

		// Merge context from all errors
		if err.Context != nil {
			for k, v := range err.Context {
				key := k
				if i > 0 {
					key = fmt.Sprintf("%s_%d", k, i+1)
				}
				allContext[key] = v
			}
		}
	}

	result := &AppError{
		Code:        primary.Code,
		Message:     fmt.Sprintf("multiple errors: %s", fmt.Sprintf("%v", messages)),
		Cause:       primary.Cause,
		Context:     allContext,
		Retryable:   primary.Retryable,
		UserMessage: primary.UserMessage,
	}

	return result
}
