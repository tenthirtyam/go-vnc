// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"errors"
	"fmt"
)

// ErrorCode represents specific error categories for VNC operations.
type ErrorCode int

const (
	// ErrProtocol indicates a protocol-level error.
	ErrProtocol ErrorCode = iota
	// ErrAuthentication indicates an authentication failure.
	ErrAuthentication
	// ErrEncoding indicates an encoding/decoding error.
	ErrEncoding
	// ErrNetwork indicates a network-related error.
	ErrNetwork
	// ErrConfiguration indicates a configuration error.
	ErrConfiguration
	// ErrTimeout indicates a timeout error.
	ErrTimeout
	// ErrValidation indicates input validation failure.
	ErrValidation
	// ErrUnsupported indicates an unsupported feature or operation.
	ErrUnsupported
)

// String returns the string representation of the error code.
func (e ErrorCode) String() string {
	switch e {
	case ErrProtocol:
		return "protocol"
	case ErrAuthentication:
		return "authentication"
	case ErrEncoding:
		return "encoding"
	case ErrNetwork:
		return "network"
	case ErrConfiguration:
		return "configuration"
	case ErrTimeout:
		return "timeout"
	case ErrValidation:
		return "validation"
	case ErrUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

// VNCError provides structured error information with operation context,
// error codes, and message wrapping for comprehensive error handling.
type VNCError struct {
	Op      string
	Code    ErrorCode
	Message string
	Err     error
}

// Error returns the formatted error message.
func (e *VNCError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("vnc %s: %s: %s: %v", e.Code.String(), e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("vnc %s: %s: %s", e.Code.String(), e.Op, e.Message)
}

// Unwrap returns the underlying error for error chain unwrapping.
func (e *VNCError) Unwrap() error {
	return e.Err
}

// Is reports whether this error matches the target error.
func (e *VNCError) Is(target error) bool {
	var vncErr *VNCError
	if errors.As(target, &vncErr) {
		return e.Code == vncErr.Code && e.Op == vncErr.Op
	}
	return false
}

// NewVNCError creates a new VNCError with the specified parameters.
// This is the primary constructor for structured VNC errors.
func NewVNCError(op string, code ErrorCode, message string, err error) *VNCError {
	return &VNCError{
		Op:      op,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// WrapError wraps an existing error with VNC-specific context.
// Returns nil if the input error is nil, otherwise creates a new VNCError.
func WrapError(op string, code ErrorCode, message string, err error) error {
	if err == nil {
		return nil
	}
	return &VNCError{
		Op:      op,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsVNCError checks if an error is a VNCError and optionally matches specific error codes.
// If no codes are provided, returns true for any VNCError. If codes are provided,
// returns true only if the error matches one of the specified codes.
func IsVNCError(err error, code ...ErrorCode) bool {
	var vncErr *VNCError
	if !errors.As(err, &vncErr) {
		return false
	}

	if len(code) == 0 {
		return true
	}

	for _, c := range code {
		if vncErr.Code == c {
			return true
		}
	}
	return false
}

// GetErrorCode extracts the error code from a VNCError.
// Returns the error code if the error is a VNCError, otherwise returns -1.
func GetErrorCode(err error) ErrorCode {
	var vncErr *VNCError
	if errors.As(err, &vncErr) {
		return vncErr.Code
	}
	return ErrorCode(-1)
}

// protocolError creates a new protocol error.
func protocolError(op, message string, err error) error {
	return NewVNCError(op, ErrProtocol, message, err)
}

// authenticationError creates a new authentication error.
func authenticationError(op, message string, err error) error {
	return NewVNCError(op, ErrAuthentication, message, err)
}

// encodingError creates a new encoding error.
func encodingError(op, message string, err error) error {
	return NewVNCError(op, ErrEncoding, message, err)
}

// networkError creates a new network error.
func networkError(op, message string, err error) error {
	return NewVNCError(op, ErrNetwork, message, err)
}

// configurationError creates a new configuration error.
func configurationError(op, message string, err error) error {
	return NewVNCError(op, ErrConfiguration, message, err)
}

// timeoutError creates a new timeout error.
func timeoutError(op, message string, err error) error {
	return NewVNCError(op, ErrTimeout, message, err)
}

// validationError creates a new validation error.
func validationError(op, message string, err error) error {
	return NewVNCError(op, ErrValidation, message, err)
}

// unsupportedError creates a new unsupported operation error.
func unsupportedError(op, message string, err error) error {
	return NewVNCError(op, ErrUnsupported, message, err)
}
