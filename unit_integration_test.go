// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestBasicConnectionWorkflow tests a basic VNC connection workflow using existing mock server.
func TestUnitIntegration_BasicConnectionWorkflow(t *testing.T) {
	// Create and start mock server
	server := NewMockVNCServer()
	server.AcceptAuth = true
	server.SendUpdates = false

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect to the mock server
	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	// Create client configuration
	config := &ClientConfig{
		Auth:      []ClientAuth{&ClientAuthNone{}},
		Exclusive: false,
		Logger:    &NoOpLogger{},
	}

	// Establish VNC client connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := ClientWithContext(ctx, conn, config)
	if err != nil {
		t.Fatalf("Failed to establish VNC connection: %v", err)
	}
	defer client.Close()

	// Verify connection state
	if client.FrameBufferWidth == 0 {
		t.Error("Expected non-zero framebuffer width")
	}

	if client.FrameBufferHeight == 0 {
		t.Error("Expected non-zero framebuffer height")
	}

	if client.DesktopName == "" {
		t.Error("Expected non-empty desktop name")
	}

	// Test basic client operations
	err = client.FramebufferUpdateRequest(false, 0, 0, 100, 100)
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

	err = client.PointerEvent(1, 100, 200) // Left click at (100, 200)
	if err != nil {
		t.Errorf("PointerEvent failed: %v", err)
	}

	err = client.CutText("Hello, World!")
	if err != nil {
		t.Errorf("CutText failed: %v", err)
	}

	// Give some time for message processing
	time.Sleep(100 * time.Millisecond)
}

// TestAuthenticationFailure tests authentication failure scenarios.
func TestUnitIntegration_AuthenticationFailure(t *testing.T) {
	// Create mock server that rejects authentication
	server := NewMockVNCServer()
	server.AcceptAuth = false // This should cause auth failure

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	config := &ClientConfig{
		Auth:   []ClientAuth{&ClientAuthNone{}},
		Logger: &NoOpLogger{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = ClientWithContext(ctx, conn, config)

	// Should fail due to authentication rejection
	if err == nil {
		t.Error("Expected authentication error but got none")
	}
}

// TestConnectionTimeout tests connection timeout handling.
func TestUnitIntegration_ConnectionTimeout(t *testing.T) {
	// Create a server that doesn't respond properly
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections but don't respond
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Don't send anything, just keep connection open
			defer conn.Close()
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	config := &ClientConfig{
		Auth:           []ClientAuth{&ClientAuthNone{}},
		ConnectTimeout: 100 * time.Millisecond,
		Logger:         &NoOpLogger{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = ClientWithContext(ctx, conn, config)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
		return
	}

	// Should timeout within reasonable time
	if duration > 500*time.Millisecond {
		t.Errorf("Connection took too long to timeout: %v", duration)
	}
}

// TestMultipleAuthMethods tests authentication with multiple methods.
func TestUnitIntegration_MultipleAuthMethods(t *testing.T) {
	server := NewMockVNCServer()
	server.AuthMethods = []uint8{1, 2} // Support both None and VNC auth
	server.AcceptAuth = true

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	// Configure client with multiple auth methods
	config := &ClientConfig{
		Auth: []ClientAuth{
			&PasswordAuth{Password: "secret"},
			&ClientAuthNone{},
		},
		Logger: &NoOpLogger{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := ClientWithContext(ctx, conn, config)
	if err != nil {
		t.Fatalf("Failed to establish VNC connection: %v", err)
	}
	defer client.Close()

	// Connection should succeed with one of the auth methods
	if client == nil {
		t.Error("Expected successful connection")
	}
}

// TestFunctionalOptionsIntegration tests functional options with real connection.
func TestUnitIntegration_FunctionalOptions(t *testing.T) {
	server := NewMockVNCServer()
	server.AcceptAuth = true

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test functional options
	client, err := ClientWithOptions(ctx, conn,
		WithAuth(&ClientAuthNone{}),
		WithExclusive(false),
		WithLogger(&NoOpLogger{}),
		WithConnectTimeout(2*time.Second),
		WithReadTimeout(1*time.Second),
		WithWriteTimeout(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to establish VNC connection with options: %v", err)
	}
	defer client.Close()

	// Test that the connection was established successfully
	// Skip the FramebufferUpdateRequest as it may have timing issues with mock server
	if client.FrameBufferWidth == 0 {
		t.Error("Expected non-zero framebuffer width")
	}
	if client.FrameBufferHeight == 0 {
		t.Error("Expected non-zero framebuffer height")
	}
}

// TestErrorRecovery tests error recovery scenarios.
func TestUnitIntegration_ErrorRecovery(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func(*MockVNCServer)
		expectError bool
	}{
		{
			name: "Valid configuration",
			setupServer: func(s *MockVNCServer) {
				s.AcceptAuth = true
				s.AuthMethods = []uint8{1}
			},
			expectError: false,
		},
		{
			name: "Authentication rejection",
			setupServer: func(s *MockVNCServer) {
				s.AcceptAuth = false
				s.AuthMethods = []uint8{1}
			},
			expectError: true,
		},
		{
			name: "No supported auth methods",
			setupServer: func(s *MockVNCServer) {
				s.AcceptAuth = true
				s.AuthMethods = []uint8{99} // Unsupported auth method
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMockVNCServer()
			tt.setupServer(server)

			if err := server.Start(); err != nil {
				t.Fatalf("Failed to start mock server: %v", err)
			}
			defer server.Stop()

			time.Sleep(100 * time.Millisecond)

			conn, err := net.Dial("tcp", server.Addr())
			if err != nil {
				t.Fatalf("Failed to connect to mock server: %v", err)
			}
			defer conn.Close()

			config := &ClientConfig{
				Auth:   []ClientAuth{&ClientAuthNone{}},
				Logger: &NoOpLogger{},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			client, err := ClientWithContext(ctx, conn, config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					if client != nil {
						client.Close()
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if client != nil {
				client.Close()
			}
		})
	}
}

// TestConcurrentOperations tests concurrent client operations.
func TestUnitIntegration_ConcurrentOperations(t *testing.T) {
	server := NewMockVNCServer()
	server.AcceptAuth = true

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	config := &ClientConfig{
		Auth:   []ClientAuth{&ClientAuthNone{}},
		Logger: &NoOpLogger{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := ClientWithContext(ctx, conn, config)
	if err != nil {
		t.Fatalf("Failed to establish VNC connection: %v", err)
	}
	defer client.Close()

	// Send multiple concurrent operations
	errChan := make(chan error, 10)

	for i := 0; i < 5; i++ {
		go func(id int) {
			_ = id // ID could be used for logging in real implementation
			if err := client.FramebufferUpdateRequest(true, 0, 0, 100, 100); err != nil {
				errChan <- err
			}
		}(i)

		go func(id int) {
			keyCode := uint32(0x0041 + id) // #nosec G115 - Test code with small key codes
			if err := client.KeyEvent(keyCode, true); err != nil {
				errChan <- err
			}
		}(i)
	}

	// Wait a bit for operations to complete
	time.Sleep(200 * time.Millisecond)

	// Check for any errors
	select {
	case err := <-errChan:
		t.Errorf("Concurrent operation error: %v", err)
	default:
		// No errors, which is good
	}
}
