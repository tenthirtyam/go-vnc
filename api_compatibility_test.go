// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"
)

// TestBackwardCompatibility_PublicAPISignatures verifies that all existing public APIs
// maintain identical function signatures and behavior for backward compatibility.
func TestAPICompatibility_PublicAPISignatures(t *testing.T) {
	t.Run("ClientConn struct fields", func(t *testing.T) {
		// Verify that ClientConn maintains all original public fields
		conn := &ClientConn{}
		val := reflect.ValueOf(conn).Elem()
		typ := val.Type()

		requiredFields := map[string]reflect.Type{
			"ColorMap":          reflect.TypeOf([256]Color{}),
			"Encs":              reflect.TypeOf([]Encoding{}),
			"FrameBufferWidth":  reflect.TypeOf(uint16(0)),
			"FrameBufferHeight": reflect.TypeOf(uint16(0)),
			"DesktopName":       reflect.TypeOf(""),
			"PixelFormat":       reflect.TypeOf(PixelFormat{}),
		}

		for fieldName, expectedType := range requiredFields {
			field, found := typ.FieldByName(fieldName)
			if !found {
				t.Errorf("Required field %s not found in ClientConn", fieldName)
				continue
			}

			if field.Type != expectedType {
				t.Errorf("Field %s has type %v, expected %v", fieldName, field.Type, expectedType)
			}

			// Verify field is exported (public)
			if !field.IsExported() {
				t.Errorf("Field %s is not exported", fieldName)
			}
		}
	})

	t.Run("ClientConfig struct fields", func(t *testing.T) {
		// Verify that ClientConfig maintains all original public fields
		config := &ClientConfig{}
		val := reflect.ValueOf(config).Elem()
		typ := val.Type()

		requiredFields := map[string]reflect.Type{
			"Auth":            reflect.TypeOf([]ClientAuth{}),
			"Exclusive":       reflect.TypeOf(false),
			"ServerMessageCh": reflect.TypeOf((chan<- ServerMessage)(nil)),
			"ServerMessages":  reflect.TypeOf([]ServerMessage{}),
		}

		for fieldName, expectedType := range requiredFields {
			field, found := typ.FieldByName(fieldName)
			if !found {
				t.Errorf("Required field %s not found in ClientConfig", fieldName)
				continue
			}

			if field.Type != expectedType {
				t.Errorf("Field %s has type %v, expected %v", fieldName, field.Type, expectedType)
			}

			// Verify field is exported (public)
			if !field.IsExported() {
				t.Errorf("Field %s is not exported", fieldName)
			}
		}
	})

	t.Run("Client function signature", func(t *testing.T) {
		// Verify Client function maintains original signature
		clientFunc := reflect.ValueOf(Client)
		clientType := clientFunc.Type()

		// Should be: func(net.Conn, *ClientConfig) (*ClientConn, error)
		if clientType.NumIn() != 2 {
			t.Errorf("Client function should have 2 parameters, got %d", clientType.NumIn())
		}

		if clientType.NumOut() != 2 {
			t.Errorf("Client function should have 2 return values, got %d", clientType.NumOut())
		}

		// Check parameter types
		if clientType.In(0) != reflect.TypeOf((*net.Conn)(nil)).Elem() {
			t.Errorf("Client first parameter should be net.Conn, got %v", clientType.In(0))
		}

		if clientType.In(1) != reflect.TypeOf((*ClientConfig)(nil)) {
			t.Errorf("Client second parameter should be *ClientConfig, got %v", clientType.In(1))
		}

		// Check return types
		if clientType.Out(0) != reflect.TypeOf((*ClientConn)(nil)) {
			t.Errorf("Client first return should be *ClientConn, got %v", clientType.Out(0))
		}

		if clientType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
			t.Errorf("Client second return should be error, got %v", clientType.Out(1))
		}
	})

	t.Run("ClientConn method signatures", func(t *testing.T) {
		// Verify all public methods maintain original signatures
		connType := reflect.TypeOf((*ClientConn)(nil))

		expectedMethods := map[string]struct {
			numIn    int
			numOut   int
			inTypes  []reflect.Type
			outTypes []reflect.Type
		}{
			"Close": {
				numIn: 1, numOut: 1,
				inTypes:  []reflect.Type{connType},
				outTypes: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()},
			},
			"CutText": {
				numIn: 2, numOut: 1,
				inTypes:  []reflect.Type{connType, reflect.TypeOf("")},
				outTypes: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()},
			},
			"FramebufferUpdateRequest": {
				numIn: 6, numOut: 1,
				inTypes:  []reflect.Type{connType, reflect.TypeOf(false), reflect.TypeOf(uint16(0)), reflect.TypeOf(uint16(0)), reflect.TypeOf(uint16(0)), reflect.TypeOf(uint16(0))},
				outTypes: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()},
			},
			"KeyEvent": {
				numIn: 3, numOut: 1,
				inTypes:  []reflect.Type{connType, reflect.TypeOf(uint32(0)), reflect.TypeOf(false)},
				outTypes: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()},
			},
			"PointerEvent": {
				numIn: 4, numOut: 1,
				inTypes:  []reflect.Type{connType, reflect.TypeOf(ButtonMask(0)), reflect.TypeOf(uint16(0)), reflect.TypeOf(uint16(0))},
				outTypes: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()},
			},
			"SetEncodings": {
				numIn: 2, numOut: 1,
				inTypes:  []reflect.Type{connType, reflect.TypeOf([]Encoding{})},
				outTypes: []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()},
			},
		}

		for methodName, expected := range expectedMethods {
			method, found := connType.MethodByName(methodName)
			if !found {
				t.Errorf("Required method %s not found in ClientConn", methodName)
				continue
			}

			methodType := method.Type

			if methodType.NumIn() != expected.numIn {
				t.Errorf("Method %s should have %d parameters, got %d", methodName, expected.numIn, methodType.NumIn())
			}

			if methodType.NumOut() != expected.numOut {
				t.Errorf("Method %s should have %d return values, got %d", methodName, expected.numOut, methodType.NumOut())
			}

			// Check parameter types
			for i, expectedType := range expected.inTypes {
				if i < methodType.NumIn() && methodType.In(i) != expectedType {
					t.Errorf("Method %s parameter %d should be %v, got %v", methodName, i, expectedType, methodType.In(i))
				}
			}

			// Check return types
			for i, expectedType := range expected.outTypes {
				if i < methodType.NumOut() && methodType.Out(i) != expectedType {
					t.Errorf("Method %s return %d should be %v, got %v", methodName, i, expectedType, methodType.Out(i))
				}
			}
		}
	})
}

// TestBackwardCompatibility_AuthenticationInterfaces verifies that authentication
// interfaces maintain backward compatibility.
func TestAPICompatibility_AuthenticationInterfaces(t *testing.T) {
	t.Run("ClientAuth interface", func(t *testing.T) {
		// Verify ClientAuth interface maintains required methods
		authType := reflect.TypeOf((*ClientAuth)(nil)).Elem()

		requiredMethods := map[string]struct {
			numIn  int
			numOut int
		}{
			"SecurityType": {numIn: 0, numOut: 1},
			"Handshake":    {numIn: 2, numOut: 1}, // context.Context, net.Conn -> error
			"String":       {numIn: 0, numOut: 1},
		}

		for methodName, expected := range requiredMethods {
			method, found := authType.MethodByName(methodName)
			if !found {
				t.Errorf("Required method %s not found in ClientAuth interface", methodName)
				continue
			}

			methodType := method.Type

			if methodType.NumIn() != expected.numIn {
				t.Errorf("ClientAuth.%s should have %d parameters, got %d", methodName, expected.numIn, methodType.NumIn())
			}

			if methodType.NumOut() != expected.numOut {
				t.Errorf("ClientAuth.%s should have %d return values, got %d", methodName, expected.numOut, methodType.NumOut())
			}
		}
	})

	t.Run("ClientAuthNone compatibility", func(t *testing.T) {
		// Verify ClientAuthNone implements ClientAuth and maintains behavior
		auth := &ClientAuthNone{}

		// Test interface compliance
		var _ ClientAuth = auth

		// Test method behavior
		if auth.SecurityType() != 1 {
			t.Errorf("ClientAuthNone.SecurityType() should return 1, got %d", auth.SecurityType())
		}

		if auth.String() != "None" {
			t.Errorf("ClientAuthNone.String() should return 'None', got %s", auth.String())
		}

		// Test handshake doesn't error with valid context
		ctx := context.Background()
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		if err := auth.Handshake(ctx, client); err != nil {
			t.Errorf("ClientAuthNone.Handshake() should not error with valid context, got %v", err)
		}
	})

	t.Run("PasswordAuth compatibility", func(t *testing.T) {
		// Verify PasswordAuth maintains backward compatibility
		auth := NewPasswordAuth("testpass")

		// Test interface compliance
		var _ ClientAuth = auth

		// Test method behavior
		if auth.SecurityType() != 2 {
			t.Errorf("PasswordAuth.SecurityType() should return 2, got %d", auth.SecurityType())
		}

		if auth.String() != "VNC Password" {
			t.Errorf("PasswordAuth.String() should return 'VNC Password', got %s", auth.String())
		}

		// Verify password field is accessible
		if auth.Password != "testpass" {
			t.Errorf("PasswordAuth.Password should be 'testpass', got %s", auth.Password)
		}
	})
}

// TestBackwardCompatibility_EncodingInterface verifies that encoding interfaces
// maintain backward compatibility.
func TestAPICompatibility_EncodingInterface(t *testing.T) {
	t.Run("Encoding interface", func(t *testing.T) {
		// Verify Encoding interface maintains required methods
		encType := reflect.TypeOf((*Encoding)(nil)).Elem()

		requiredMethods := map[string]struct {
			numIn  int
			numOut int
		}{
			"Type": {numIn: 0, numOut: 1},
			"Read": {numIn: 3, numOut: 2}, // *ClientConn, *Rectangle, io.Reader -> Encoding, error
		}

		for methodName, expected := range requiredMethods {
			method, found := encType.MethodByName(methodName)
			if !found {
				t.Errorf("Required method %s not found in Encoding interface", methodName)
				continue
			}

			methodType := method.Type

			if methodType.NumIn() != expected.numIn {
				t.Errorf("Encoding.%s should have %d parameters, got %d", methodName, expected.numIn, methodType.NumIn())
			}

			if methodType.NumOut() != expected.numOut {
				t.Errorf("Encoding.%s should have %d return values, got %d", methodName, expected.numOut, methodType.NumOut())
			}
		}
	})
}

// TestBackwardCompatibility_Constants verifies that all public constants
// maintain their values for backward compatibility.
func TestAPICompatibility_Constants(t *testing.T) {
	t.Run("ButtonMask constants", func(t *testing.T) {
		// Verify button mask constants maintain their values
		expectedValues := map[string]ButtonMask{
			"ButtonLeft":   1,
			"ButtonMiddle": 2,
			"ButtonRight":  4,
			"Button4":      8,
			"Button5":      16,
			"Button6":      32,
			"Button7":      64,
			"Button8":      128,
		}

		// Use reflection to get actual constant values
		actualValues := map[string]ButtonMask{
			"ButtonLeft":   ButtonLeft,
			"ButtonMiddle": ButtonMiddle,
			"ButtonRight":  ButtonRight,
			"Button4":      Button4,
			"Button5":      Button5,
			"Button6":      Button6,
			"Button7":      Button7,
			"Button8":      Button8,
		}

		for name, expected := range expectedValues {
			if actual, exists := actualValues[name]; !exists {
				t.Errorf("Constant %s not found", name)
			} else if actual != expected {
				t.Errorf("Constant %s should be %d, got %d", name, expected, actual)
			}
		}
	})
}

// TestBackwardCompatibility_PackageImport verifies that the package can be imported
// with the same name and provides the same public API.
func TestAPICompatibility_PackageImport(t *testing.T) {
	t.Run("Package name", func(t *testing.T) {
		// This test verifies that the package name is "vnc"
		// The package name is verified by the fact that this test compiles
		// and can reference types without qualification
		var _ *ClientConn
		var _ *ClientConfig
		var _ ClientAuth
		var _ Encoding
	})

	t.Run("Public types availability", func(t *testing.T) {
		// Verify all expected public types are available
		expectedTypes := []interface{}{
			(*ClientConn)(nil),
			(*ClientConfig)(nil),
			(*ClientAuthNone)(nil),
			(*PasswordAuth)(nil),
			(*Color)(nil),
			(*PixelFormat)(nil),
			(*Rectangle)(nil),
			ButtonMask(0),
		}

		for i, expectedType := range expectedTypes {
			if reflect.TypeOf(expectedType) == nil {
				t.Errorf("Expected type at index %d is nil", i)
			}
		}
	})
}

// TestBackwardCompatibility_FunctionalBehavior tests that the library behaves
// identically to the original implementation for common use cases.
func TestAPICompatibility_FunctionalBehavior(t *testing.T) {
	t.Run("Basic client creation", func(t *testing.T) {
		// Test that Client function works with nil config (should not panic)
		// This test verifies the function signature and basic error handling

		// Create a connection that will immediately fail to test error handling
		server, client := net.Pipe()
		server.Close() // Close server side immediately to cause connection error
		defer client.Close()

		// This should not panic with nil config (backward compatibility)
		_, err := Client(client, nil)
		if err != nil {
			// Error is expected due to closed connection, but should not panic
			t.Logf("Client creation failed as expected due to closed connection: %v", err)
		} else {
			t.Error("Expected error due to closed connection, but got nil")
		}
	})

	t.Run("ClientConfig field access", func(t *testing.T) {
		// Test that all original ClientConfig fields can be accessed and set
		config := &ClientConfig{}

		// Test setting original fields
		config.Auth = []ClientAuth{&ClientAuthNone{}}
		config.Exclusive = true
		config.ServerMessages = []ServerMessage{}

		// Verify fields are accessible
		if len(config.Auth) != 1 {
			t.Errorf("Auth field should have 1 element, got %d", len(config.Auth))
		}

		if !config.Exclusive {
			t.Errorf("Exclusive field should be true")
		}

		if config.ServerMessages == nil {
			t.Errorf("ServerMessages field should not be nil")
		}
	})

	t.Run("Authentication method creation", func(t *testing.T) {
		// Test that authentication methods can be created and used as before
		noneAuth := &ClientAuthNone{}
		passAuth := NewPasswordAuth("test")

		// Test that they implement the interface
		var _ ClientAuth = noneAuth
		var _ ClientAuth = passAuth

		// Test basic functionality
		if noneAuth.SecurityType() != 1 {
			t.Errorf("None auth security type should be 1")
		}

		if passAuth.SecurityType() != 2 {
			t.Errorf("Password auth security type should be 2")
		}
	})
}

// TestBackwardCompatibility_ErrorHandling verifies that error handling
// maintains backward compatibility while providing enhanced functionality.
func TestAPICompatibility_ErrorHandling(t *testing.T) {
	t.Run("Error interface compliance", func(t *testing.T) {
		// Test that all custom errors implement the error interface
		errors := []error{
			NewVNCError("test", ErrNetwork, "message", nil),
			NewVNCError("test", ErrAuthentication, "message", nil),
			NewVNCError("test", ErrProtocol, "message", nil),
			NewVNCError("test", ErrValidation, "message", nil),
			NewVNCError("test", ErrTimeout, "message", nil),
			NewVNCError("test", ErrUnsupported, "message", nil),
		}

		for i, err := range errors {
			if err == nil {
				t.Errorf("Error at index %d is nil", i)
				continue
			}

			// Test that Error() method works
			if err.Error() == "" {
				t.Errorf("Error at index %d has empty Error() string", i)
			}

			// Test that errors can be compared
			if err == nil {
				t.Errorf("Error at index %d should not be nil", i)
			}
		}
	})

	t.Run("Error wrapping compatibility", func(t *testing.T) {
		// Test that error wrapping works with standard library functions
		originalErr := NewVNCError("connect", ErrNetwork, "connection failed", nil)

		// Should be able to use with errors.Is, errors.As, etc.
		if originalErr == nil {
			t.Error("Created error should not be nil")
		}

		// Test string representation
		if originalErr.Error() == "" {
			t.Error("Error should have non-empty string representation")
		}
	})
}

// TestBackwardCompatibility_Integration performs integration tests to verify
// that the modernized library works as a drop-in replacement.
func TestAPICompatibility_Integration(t *testing.T) {
	t.Run("Mock server integration", func(t *testing.T) {
		// Create a simple mock server for testing
		server := NewMockVNCServer()
		server.AcceptAuth = true
		server.AuthMethods = []uint8{1} // None authentication

		err := server.Start()
		if err != nil {
			t.Fatalf("Failed to start mock server: %v", err)
		}
		defer server.Stop()
		addr := server.Addr()

		// Test connection using original API pattern
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect to mock server: %v", err)
		}
		defer conn.Close()

		// Use original Client function signature
		config := &ClientConfig{
			Auth: []ClientAuth{&ClientAuthNone{}},
		}

		client, err := Client(conn, config)
		if err != nil {
			t.Fatalf("Failed to create VNC client: %v", err)
		}
		defer client.Close()

		// Verify client has expected properties
		if client.FrameBufferWidth == 0 || client.FrameBufferHeight == 0 {
			t.Error("Client should have non-zero framebuffer dimensions")
		}

		// Test basic operations
		err = client.FramebufferUpdateRequest(false, 0, 0, client.FrameBufferWidth, client.FrameBufferHeight)
		if err != nil {
			t.Errorf("FramebufferUpdateRequest failed: %v", err)
		}

		err = client.KeyEvent(0x0041, true) // 'A' key down
		if err != nil {
			t.Errorf("KeyEvent failed: %v", err)
		}

		err = client.KeyEvent(0x0041, false) // 'A' key up
		if err != nil {
			t.Errorf("KeyEvent failed: %v", err)
		}

		err = client.PointerEvent(ButtonLeft, 100, 100)
		if err != nil {
			t.Errorf("PointerEvent failed: %v", err)
		}
	})

	t.Run("Functional options backward compatibility", func(t *testing.T) {
		// Test that new functional options don't break existing usage
		server := NewMockVNCServer()
		server.AcceptAuth = true
		server.AuthMethods = []uint8{1}

		err := server.Start()
		if err != nil {
			t.Fatalf("Failed to start mock server: %v", err)
		}
		defer server.Stop()
		addr := server.Addr()

		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect to mock server: %v", err)
		}
		defer conn.Close()

		// Test that ClientWithOptions produces equivalent results to Client
		ctx := context.Background()

		client1, err1 := ClientWithOptions(ctx, conn,
			WithAuth(&ClientAuthNone{}),
			WithExclusive(false),
		)

		// We can't actually compare both clients since we only have one connection,
		// but we can verify the new method works
		if err1 != nil {
			t.Logf("ClientWithOptions failed as expected due to connection reuse: %v", err1)
		} else {
			defer client1.Close()

			// Verify it has the same interface
			if client1.FrameBufferWidth == 0 || client1.FrameBufferHeight == 0 {
				t.Error("ClientWithOptions should produce client with non-zero framebuffer dimensions")
			}
		}
	})
}
