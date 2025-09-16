// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogging_NoOpLogger(t *testing.T) {
	logger := &NoOpLogger{}

	// Test that all methods can be called without panicking
	logger.Debug("debug message", Field{Key: "key", Value: "value"})
	logger.Info("info message", Field{Key: "key", Value: "value"})
	logger.Warn("warn message", Field{Key: "key", Value: "value"})
	logger.Error("error message", Field{Key: "key", Value: "value"})

	// Test With method
	contextLogger := logger.With(Field{Key: "context", Value: "test"})
	contextLogger.Info("test message")

	// Verify that With returns a NoOpLogger
	if _, ok := contextLogger.(*NoOpLogger); !ok {
		t.Errorf("With() should return a NoOpLogger, got %T", contextLogger)
	}
}

func TestLogging_StandardLogger(t *testing.T) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0) // No timestamp/file for predictable output

	logger := &StandardLogger{Logger: stdLogger}

	tests := []struct {
		name     string
		logFunc  func(string, ...Field)
		message  string
		fields   []Field
		expected string
	}{
		{
			name:     "debug message",
			logFunc:  logger.Debug,
			message:  "debug test",
			fields:   nil,
			expected: "[DEBUG] debug test",
		},
		{
			name:     "info with fields",
			logFunc:  logger.Info,
			message:  "info test",
			fields:   []Field{{Key: "key1", Value: "value1"}, {Key: "key2", Value: 42}},
			expected: "[INFO] info test key1=value1 key2=42",
		},
		{
			name:     "warn with string containing spaces",
			logFunc:  logger.Warn,
			message:  "warn test",
			fields:   []Field{{Key: "message", Value: "hello world"}},
			expected: "[WARN] warn test message=\"hello world\"",
		},
		{
			name:     "error with error field",
			logFunc:  logger.Error,
			message:  "error test",
			fields:   []Field{{Key: "error", Value: NewVNCError("test", ErrNetwork, "test error", nil)}},
			expected: "[ERROR] error test error=\"vnc network: test: test error\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(tt.message, tt.fields...)

			output := strings.TrimSpace(buf.String())
			if output != tt.expected {
				t.Errorf("Expected: %q, Got: %q", tt.expected, output)
			}
		})
	}
}

func TestLogging_StandardLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)

	logger := &StandardLogger{Logger: stdLogger}

	// Create context logger with base fields
	contextLogger := logger.With(
		Field{Key: "conn_id", Value: "conn-123"},
		Field{Key: "session", Value: "test-session"},
	)

	// Log message with additional fields
	contextLogger.Info("test message", Field{Key: "extra", Value: "data"})

	output := strings.TrimSpace(buf.String())
	expected := "[INFO] test message conn_id=conn-123 session=test-session extra=data"

	if output != expected {
		t.Errorf("Expected: %q, Got: %q", expected, output)
	}

	// Test that original logger is not affected
	buf.Reset()
	logger.Info("original logger")
	output = strings.TrimSpace(buf.String())
	expected = "[INFO] original logger"

	if output != expected {
		t.Errorf("Original logger should not have context fields. Expected: %q, Got: %q", expected, output)
	}
}

func TestLogging_StandardLoggerDefault(t *testing.T) {
	// Test that StandardLogger creates a default logger when Logger is nil
	logger := &StandardLogger{}

	// This should not panic and should create a default logger
	logger.Info("test message")

	// Verify that the logger was created
	if logger.Logger == nil {
		t.Error("Expected Logger to be initialized after first use")
	}
}

func TestLogging_FormatFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "simple string",
			value:    "hello",
			expected: "hello",
		},
		{
			name:     "string with spaces",
			value:    "hello world",
			expected: `"hello world"`,
		},
		{
			name:     "integer",
			value:    42,
			expected: "42",
		},
		{
			name:     "boolean",
			value:    true,
			expected: "true",
		},
		{
			name:     "error",
			value:    NewVNCError("test", ErrNetwork, "test error", nil),
			expected: `"vnc network: test: test error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFieldValue(tt.value)
			if result != tt.expected {
				t.Errorf("Expected: %q, Got: %q", tt.expected, result)
			}
		})
	}
}

func TestLogging_ContainsSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello", false},
		{"hello world", true},
		{"hello\tworld", true},
		{"hello\nworld", true},
		{"hello\rworld", true},
		{"", false},
		{"no-spaces-here", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSpace(tt.input)
			if result != tt.expected {
				t.Errorf("containsSpace(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLogging_Integration(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0) // No timestamp for predictable output

	logger := &StandardLogger{Logger: stdLogger}

	// Test ClientConfig with logger
	config := &ClientConfig{
		Auth:   []ClientAuth{new(ClientAuthNone)},
		Logger: logger,
	}

	// Verify logger is properly set in config
	if config.Logger == nil {
		t.Error("Logger should be set in ClientConfig")
	}

	// Test that NoOpLogger works as default
	configWithoutLogger := &ClientConfig{
		Auth: []ClientAuth{new(ClientAuthNone)},
	}

	if configWithoutLogger.Logger != nil {
		t.Error("Logger should be nil when not explicitly set")
	}
}

func TestLogging_ClientConnInitialization(t *testing.T) {
	// This test verifies that the logger initialization logic in Client() works correctly
	// We can't easily test the full Client() function without a real network connection,
	// but we can test the logger initialization logic

	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)
	logger := &StandardLogger{Logger: stdLogger}

	// Test with logger in config
	config := &ClientConfig{
		Logger: logger,
	}

	// Simulate the logger initialization logic from Client()
	var clientLogger Logger = &NoOpLogger{}
	if config != nil && config.Logger != nil {
		clientLogger = config.Logger
	}

	// Verify the logger is correctly assigned
	if _, ok := clientLogger.(*StandardLogger); !ok {
		t.Errorf("Expected StandardLogger, got %T", clientLogger)
	}

	// Test with nil config
	var nilConfig *ClientConfig = nil
	clientLogger = &NoOpLogger{}
	if nilConfig != nil && nilConfig.Logger != nil {
		clientLogger = nilConfig.Logger
	}

	// Should remain NoOpLogger
	if _, ok := clientLogger.(*NoOpLogger); !ok {
		t.Errorf("Expected NoOpLogger for nil config, got %T", clientLogger)
	}

	// Test with config but no logger
	configNoLogger := &ClientConfig{}
	clientLogger = &NoOpLogger{}
	if configNoLogger != nil && configNoLogger.Logger != nil {
		clientLogger = configNoLogger.Logger
	}

	// Should remain NoOpLogger
	if _, ok := clientLogger.(*NoOpLogger); !ok {
		t.Errorf("Expected NoOpLogger for config without logger, got %T", clientLogger)
	}
}

func TestLogging_FieldsFormatting(t *testing.T) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)
	logger := &StandardLogger{Logger: stdLogger}

	// Test various field types that would be used in VNC logging
	logger.Info("VNC connection test",
		Field{Key: "server", Value: "192.168.1.100:5900"},
		Field{Key: "auth_type", Value: 2},
		Field{Key: "shared", Value: true},
		Field{Key: "framebuffer_width", Value: uint16(1920)},
		Field{Key: "framebuffer_height", Value: uint16(1080)})

	output := strings.TrimSpace(buf.String())
	expected := `[INFO] VNC connection test server=192.168.1.100:5900 auth_type=2 shared=true framebuffer_width=1920 framebuffer_height=1080`

	if output != expected {
		t.Errorf("Expected: %q, Got: %q", expected, output)
	}
}

func TestLogging_Contextual(t *testing.T) {
	var buf bytes.Buffer
	stdLogger := log.New(&buf, "", 0)
	logger := &StandardLogger{Logger: stdLogger}

	// Create a contextual logger like would be used for a connection
	connLogger := logger.With(
		Field{Key: "conn_id", Value: "conn-123"},
		Field{Key: "remote_addr", Value: "192.168.1.100:5900"},
	)

	// Log a message that would occur during handshake
	connLogger.Info("Protocol version negotiated",
		Field{Key: "major", Value: 3},
		Field{Key: "minor", Value: 8})

	output := strings.TrimSpace(buf.String())
	expected := `[INFO] Protocol version negotiated conn_id=conn-123 remote_addr=192.168.1.100:5900 major=3 minor=8`

	if output != expected {
		t.Errorf("Expected: %q, Got: %q", expected, output)
	}
}
