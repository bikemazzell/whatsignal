package errors

import (
	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Logger with structured error logging
type Logger struct {
	*logrus.Logger
}

// NewLogger creates a new structured logger
func NewLogger() *Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	return &Logger{Logger: logger}
}

// LogError logs an error with structured context
func (l *Logger) LogError(err error, message string, fields ...logrus.Fields) {
	entry := l.Logger.WithError(err)

	// Add structured context from AppError
	if appErr, ok := err.(*AppError); ok {
		entry = entry.WithFields(logrus.Fields{
			"error_code": appErr.Code,
			"retryable":  appErr.Retryable,
		})

		// Add custom context
		if appErr.Context != nil {
			for k, v := range appErr.Context {
				entry = entry.WithField(k, v)
			}
		}
	}

	// Add additional fields
	for _, field := range fields {
		entry = entry.WithFields(field)
	}

	entry.Error(message)
}

// LogWarn logs a warning with structured context
func (l *Logger) LogWarn(err error, message string, fields ...logrus.Fields) {
	entry := l.Logger.WithError(err)

	if appErr, ok := err.(*AppError); ok {
		entry = entry.WithFields(logrus.Fields{
			"error_code": appErr.Code,
			"retryable":  appErr.Retryable,
		})

		if appErr.Context != nil {
			for k, v := range appErr.Context {
				entry = entry.WithField(k, v)
			}
		}
	}

	for _, field := range fields {
		entry = entry.WithFields(field)
	}

	entry.Warn(message)
}

// LogRetryableError logs a retryable error at warn level, non-retryable at error level
func (l *Logger) LogRetryableError(err error, message string, fields ...logrus.Fields) {
	if IsRetryable(err) {
		l.LogWarn(err, message, fields...)
	} else {
		l.LogError(err, message, fields...)
	}
}

// WithContext adds context fields to subsequent log entries
func (l *Logger) WithContext(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithError adds an error to subsequent log entries
func (l *Logger) WithError(err error) *logrus.Entry {
	entry := l.Logger.WithError(err)

	if appErr, ok := err.(*AppError); ok {
		entry = entry.WithFields(logrus.Fields{
			"error_code": appErr.Code,
			"retryable":  appErr.Retryable,
		})

		if appErr.Context != nil {
			for k, v := range appErr.Context {
				entry = entry.WithField(k, v)
			}
		}
	}

	return entry
}
