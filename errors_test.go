// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrors_CodeString(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{ErrProtocol, "protocol"},
		{ErrAuthentication, "authentication"},
		{ErrEncoding, "encoding"},
		{ErrNetwork, "network"},
		{ErrConfiguration, "configuration"},
		{ErrTimeout, "timeout"},
		{ErrValidation, "validation"},
		{ErrUnsupported, "unsupported"},
		{ErrorCode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("ErrorCode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrors_VNCErrorError(t *testing.T) {
	tests := []struct {
		name     string
		vncErr   *VNCError
		expected string
	}{
		{
			name: "error with underlying error",
			vncErr: &VNCError{
				Op:      "handshake",
				Code:    ErrProtocol,
				Message: "invalid version",
				Err:     errors.New("connection refused"),
			},
			expected: "vnc protocol: handshake: invalid version: connection refused",
		},
		{
			name: "error without underlying error",
			vncErr: &VNCError{
				Op:      "authenticate",
				Code:    ErrAuthentication,
				Message: "invalid credentials",
				Err:     nil,
			},
			expected: "vnc authentication: authenticate: invalid credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vncErr.Error(); got != tt.expected {
				t.Errorf("VNCError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrors_VNCErrorUnwrap(t *testing.T) {
	underlyingErr := errors.New("underlying error")
	vncErr := &VNCError{
		Op:      "test",
		Code:    ErrNetwork,
		Message: "test message",
		Err:     underlyingErr,
	}

	if got := vncErr.Unwrap(); got != underlyingErr {
		t.Errorf("VNCError.Unwrap() = %v, want %v", got, underlyingErr)
	}

	// Test with nil underlying error
	vncErrNil := &VNCError{
		Op:      "test",
		Code:    ErrNetwork,
		Message: "test message",
		Err:     nil,
	}

	if got := vncErrNil.Unwrap(); got != nil {
		t.Errorf("VNCError.Unwrap() = %v, want nil", got)
	}
}

func TestErrors_VNCErrorIs(t *testing.T) {
	err1 := &VNCError{Op: "handshake", Code: ErrProtocol, Message: "test"}
	err2 := &VNCError{Op: "handshake", Code: ErrProtocol, Message: "different message"}
	err3 := &VNCError{Op: "authenticate", Code: ErrAuthentication, Message: "test"}
	err4 := errors.New("regular error")

	tests := []struct {
		name     string
		err      error
		target   error
		expected bool
	}{
		{"same operation and code", err1, err2, true},
		{"different operation", err1, err3, false},
		{"different error type", err1, err4, false},
		{"nil target", err1, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, tt.target); got != tt.expected {
				t.Errorf("errors.Is() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrors_NewVNCError(t *testing.T) {
	underlyingErr := errors.New("underlying")
	vncErr := NewVNCError("test_op", ErrEncoding, "test message", underlyingErr)

	if vncErr.Op != "test_op" {
		t.Errorf("NewVNCError().Op = %v, want %v", vncErr.Op, "test_op")
	}
	if vncErr.Code != ErrEncoding {
		t.Errorf("NewVNCError().Code = %v, want %v", vncErr.Code, ErrEncoding)
	}
	if vncErr.Message != "test message" {
		t.Errorf("NewVNCError().Message = %v, want %v", vncErr.Message, "test message")
	}
	if vncErr.Err != underlyingErr {
		t.Errorf("NewVNCError().Err = %v, want %v", vncErr.Err, underlyingErr)
	}
}

func TestErrors_WrapError(t *testing.T) {
	tests := []struct {
		name        string
		op          string
		code        ErrorCode
		message     string
		err         error
		expectNil   bool
		expectError bool
	}{
		{
			name:        "wrap non-nil error",
			op:          "test",
			code:        ErrNetwork,
			message:     "wrapped",
			err:         errors.New("original"),
			expectNil:   false,
			expectError: true,
		},
		{
			name:        "wrap nil error",
			op:          "test",
			code:        ErrNetwork,
			message:     "wrapped",
			err:         nil,
			expectNil:   true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(tt.op, tt.code, tt.message, tt.err)

			if tt.expectNil && result != nil {
				t.Errorf("WrapError() = %v, want nil", result)
			}

			if tt.expectError && result == nil {
				t.Errorf("WrapError() = nil, want error")
			}

			if tt.expectError {
				var vncErr *VNCError
				if !errors.As(result, &vncErr) {
					t.Errorf("WrapError() did not return VNCError")
				}
			}
		})
	}
}

func TestErrors_IsVNCError(t *testing.T) {
	vncErr := &VNCError{Code: ErrProtocol}
	regularErr := errors.New("regular error")

	tests := []struct {
		name     string
		err      error
		codes    []ErrorCode
		expected bool
	}{
		{"VNC error without code filter", vncErr, nil, true},
		{"VNC error with matching code", vncErr, []ErrorCode{ErrProtocol}, true},
		{"VNC error with non-matching code", vncErr, []ErrorCode{ErrNetwork}, false},
		{"VNC error with multiple codes, one matching", vncErr, []ErrorCode{ErrNetwork, ErrProtocol}, true},
		{"regular error", regularErr, nil, false},
		{"nil error", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsVNCError(tt.err, tt.codes...); got != tt.expected {
				t.Errorf("IsVNCError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrors_GetErrorCode(t *testing.T) {
	vncErr := &VNCError{Code: ErrAuthentication}
	regularErr := errors.New("regular error")

	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{"VNC error", vncErr, ErrAuthentication},
		{"regular error", regularErr, ErrorCode(-1)},
		{"nil error", nil, ErrorCode(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetErrorCode(tt.err); got != tt.expected {
				t.Errorf("GetErrorCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrors_Constructors(t *testing.T) {
	underlyingErr := errors.New("underlying")

	tests := []struct {
		name         string
		constructor  func(string, string, error) error
		expectedCode ErrorCode
	}{
		{"ProtocolError", func(op, msg string, err error) error { return NewVNCError(op, ErrProtocol, msg, err) }, ErrProtocol},
		{"AuthenticationError", func(op, msg string, err error) error { return NewVNCError(op, ErrAuthentication, msg, err) }, ErrAuthentication},
		{"EncodingError", func(op, msg string, err error) error { return NewVNCError(op, ErrEncoding, msg, err) }, ErrEncoding},
		{"NetworkError", func(op, msg string, err error) error { return NewVNCError(op, ErrNetwork, msg, err) }, ErrNetwork},
		{"ConfigurationError", func(op, msg string, err error) error { return NewVNCError(op, ErrConfiguration, msg, err) }, ErrConfiguration},
		{"TimeoutError", func(op, msg string, err error) error { return NewVNCError(op, ErrTimeout, msg, err) }, ErrTimeout},
		{"ValidationError", func(op, msg string, err error) error { return NewVNCError(op, ErrValidation, msg, err) }, ErrValidation},
		{"UnsupportedError", func(op, msg string, err error) error { return NewVNCError(op, ErrUnsupported, msg, err) }, ErrUnsupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor("test_op", "test message", underlyingErr)

			var vncErr *VNCError
			if !errors.As(err, &vncErr) {
				t.Errorf("%s did not return VNCError", tt.name)
				return
			}

			if vncErr.Code != tt.expectedCode {
				t.Errorf("%s code = %v, want %v", tt.name, vncErr.Code, tt.expectedCode)
			}

			if vncErr.Op != "test_op" {
				t.Errorf("%s op = %v, want %v", tt.name, vncErr.Op, "test_op")
			}

			if vncErr.Message != "test message" {
				t.Errorf("%s message = %v, want %v", tt.name, vncErr.Message, "test message")
			}

			if vncErr.Err != underlyingErr {
				t.Errorf("%s underlying error = %v, want %v", tt.name, vncErr.Err, underlyingErr)
			}
		})
	}
}

func TestErrors_WrappingChain(t *testing.T) {
	// Test error wrapping chain functionality
	originalErr := errors.New("original network error")
	wrappedErr := NewVNCError("connect", ErrNetwork, "failed to establish connection", originalErr)

	// Test that we can unwrap to the original error
	if !errors.Is(wrappedErr, originalErr) {
		t.Errorf("errors.Is() failed to find original error in chain")
	}

	// Test that we can identify it as a VNC error
	if !IsVNCError(wrappedErr, ErrNetwork) {
		t.Errorf("IsVNCError() failed to identify network error")
	}

	// Test error message formatting
	expectedMsg := "vnc network: connect: failed to establish connection: original network error"
	if wrappedErr.Error() != expectedMsg {
		t.Errorf("Error() = %v, want %v", wrappedErr.Error(), expectedMsg)
	}
}

func Example() {
	// Create a VNC error with context
	err := NewVNCError("handshake", ErrNetwork, "connection timeout", fmt.Errorf("dial tcp: timeout"))

	fmt.Println("Error:", err)
	fmt.Println("Is network error:", IsVNCError(err, ErrNetwork))
	fmt.Println("Error code:", GetErrorCode(err))

	// Output:
	// Error: vnc network: handshake: connection timeout: dial tcp: timeout
	// Is network error: true
	// Error code: network
}

// TestStructuredErrorIntegration tests that the structured error system
// works correctly in practice with error wrapping and identification.
func TestErrors_StructuredIntegration(t *testing.T) {
	// Test that we can identify specific error types from the library
	tests := []struct {
		name       string
		err        error
		expectCode ErrorCode
		expectOp   string
		expectType bool
	}{
		{
			name:       "protocol error",
			err:        NewVNCError("handshake", ErrProtocol, "invalid version", nil),
			expectCode: ErrProtocol,
			expectOp:   "handshake",
			expectType: true,
		},
		{
			name:       "authentication error",
			err:        NewVNCError("login", ErrAuthentication, "invalid credentials", nil),
			expectCode: ErrAuthentication,
			expectOp:   "login",
			expectType: true,
		},
		{
			name:       "network error",
			err:        NewVNCError("connect", ErrNetwork, "connection refused", errors.New("dial tcp: connection refused")),
			expectCode: ErrNetwork,
			expectOp:   "connect",
			expectType: true,
		},
		{
			name:       "regular error",
			err:        errors.New("regular error"),
			expectCode: ErrorCode(-1),
			expectOp:   "",
			expectType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IsVNCError
			if got := IsVNCError(tt.err); got != tt.expectType {
				t.Errorf("IsVNCError() = %v, want %v", got, tt.expectType)
			}

			// Test GetErrorCode
			if got := GetErrorCode(tt.err); got != tt.expectCode {
				t.Errorf("GetErrorCode() = %v, want %v", got, tt.expectCode)
			}

			// Test specific error code matching
			if tt.expectType {
				if !IsVNCError(tt.err, tt.expectCode) {
					t.Errorf("IsVNCError() with code filter failed for %v", tt.expectCode)
				}

				// Test that we can extract the VNCError
				var vncErr *VNCError
				if !errors.As(tt.err, &vncErr) {
					t.Errorf("errors.As() failed to extract VNCError")
				} else {
					if vncErr.Op != tt.expectOp {
						t.Errorf("VNCError.Op = %v, want %v", vncErr.Op, tt.expectOp)
					}
					if vncErr.Code != tt.expectCode {
						t.Errorf("VNCError.Code = %v, want %v", vncErr.Code, tt.expectCode)
					}
				}
			}
		})
	}
}

// TestErrorWrappingChains tests that error wrapping works correctly.
func TestErrors_WrappingChains(t *testing.T) {
	originalErr := errors.New("original network error")
	wrappedErr := NewVNCError("connect", ErrNetwork, "failed to connect", originalErr)

	// Test that we can find the original error
	if !errors.Is(wrappedErr, originalErr) {
		t.Errorf("errors.Is() failed to find original error in chain")
	}

	// Test that we can identify it as a network error
	if !IsVNCError(wrappedErr, ErrNetwork) {
		t.Errorf("IsVNCError() failed to identify network error")
	}

	// Test error message includes all context
	expectedSubstrings := []string{"vnc network", "connect", "failed to connect", "original network error"}
	errMsg := wrappedErr.Error()
	for _, substr := range expectedSubstrings {
		if !contains(errMsg, substr) {
			t.Errorf("Error message %q does not contain expected substring %q", errMsg, substr)
		}
	}
}

// Helper function to check if string contains substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
