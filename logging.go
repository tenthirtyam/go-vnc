// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"fmt"
	"log"
	"os"
)

// Field represents a structured logging field with a key-value pair.
type Field struct {
	Key   string
	Value interface{}
}

// Logger defines the interface for structured logging throughout the VNC library.
type Logger interface {
	// Debug logs debug-level messages with optional structured fields.
	Debug(msg string, fields ...Field)

	// Info logs info-level messages with optional structured fields.
	Info(msg string, fields ...Field)

	// Warn logs warning-level messages with optional structured fields.
	Warn(msg string, fields ...Field)

	// Error logs error-level messages with optional structured fields.
	Error(msg string, fields ...Field)

	// With creates a new logger instance with the provided fields pre-populated.
	With(fields ...Field) Logger
}

// NoOpLogger is a Logger implementation that discards all log messages.
type NoOpLogger struct{}

// Debug discards debug-level log messages.
func (l *NoOpLogger) Debug(msg string, fields ...Field) {
}

// Info discards info-level log messages.
func (l *NoOpLogger) Info(msg string, fields ...Field) {
}

// Warn discards warning-level log messages.
func (l *NoOpLogger) Warn(msg string, fields ...Field) {
}

// Error discards error-level log messages.
func (l *NoOpLogger) Error(msg string, fields ...Field) {
}

// With returns a new NoOpLogger instance (ignores fields).
func (l *NoOpLogger) With(fields ...Field) Logger {
	return &NoOpLogger{}
}

// StandardLogger wraps Go's standard log package to implement the Logger interface.
type StandardLogger struct {
	// Logger is the underlying standard library logger.
	Logger *log.Logger

	// contextFields holds fields that should be included in all log messages
	contextFields []Field
}

// ensureLogger initializes the logger if it's nil.
func (l *StandardLogger) ensureLogger() *log.Logger {
	if l.Logger == nil {
		l.Logger = log.New(os.Stderr, "VNC: ", log.LstdFlags|log.Lshortfile)
	}
	return l.Logger
}

// formatMessage formats a log message with structured fields.
func (l *StandardLogger) formatMessage(level, msg string, fields ...Field) string {
	allFields := make([]Field, 0, len(l.contextFields)+len(fields))
	allFields = append(allFields, l.contextFields...)
	allFields = append(allFields, fields...)

	if len(allFields) == 0 {
		return level + " " + msg
	}
	formatted := level + " " + msg
	for _, field := range allFields {
		formatted += " " + field.Key + "=" + formatFieldValue(field.Value)
	}
	return formatted
}

// formatFieldValue converts a field value to a string representation for logging.
// Strings containing spaces are quoted, errors are quoted, other values use default formatting.
func formatFieldValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		if containsSpace(v) {
			return `"` + v + `"`
		}
		return v
	case error:
		return `"` + v.Error() + `"`
	default:
		return fmt.Sprintf("%v", v)
	}
}

// containsSpace checks if a string contains any whitespace characters.
// Returns true if the string contains spaces, tabs, newlines, or carriage returns.
func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return true
		}
	}
	return false
}

// Debug logs a debug-level message with structured fields.
func (l *StandardLogger) Debug(msg string, fields ...Field) {
	logger := l.ensureLogger()
	formatted := l.formatMessage("[DEBUG]", msg, fields...)
	logger.Print(formatted)
}

// Info logs an info-level message with structured fields.
func (l *StandardLogger) Info(msg string, fields ...Field) {
	logger := l.ensureLogger()
	formatted := l.formatMessage("[INFO]", msg, fields...)
	logger.Print(formatted)
}

// Warn logs a warning-level message with structured fields.
func (l *StandardLogger) Warn(msg string, fields ...Field) {
	logger := l.ensureLogger()
	formatted := l.formatMessage("[WARN]", msg, fields...)
	logger.Print(formatted)
}

// Error logs an error-level message with structured fields.
func (l *StandardLogger) Error(msg string, fields ...Field) {
	logger := l.ensureLogger()
	formatted := l.formatMessage("[ERROR]", msg, fields...)
	logger.Print(formatted)
}

// With creates a new StandardLogger instance with additional context fields.
// The returned logger will include the provided fields in all subsequent log messages.
func (l *StandardLogger) With(fields ...Field) Logger {
	newContextFields := make([]Field, 0, len(l.contextFields)+len(fields))
	newContextFields = append(newContextFields, l.contextFields...)
	newContextFields = append(newContextFields, fields...)

	return &StandardLogger{
		Logger:        l.Logger,
		contextFields: newContextFields,
	}
}
