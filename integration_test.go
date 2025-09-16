// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// testVNCServerCompatibility tests compatibility with real VNC server implementations.
// These tests require actual VNC servers to be running and are typically run in CI/CD
// environments or during manual testing.
func TestIntegration_RealVNCServers(t *testing.T) {
	// Skip these tests unless explicitly enabled.
	if os.Getenv("VNC_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping real VNC server tests. Set VNC_INTEGRATION_TESTS=1 to enable.")
	}

	// Test configuration for different VNC server implementations.
	testServers := []struct {
		name     string
		address  string
		password string
		authType uint8
		timeout  time.Duration
	}{
		{
			name:     "TightVNC",
			address:  getEnvOrDefault("TIGHTVNC_ADDRESS", "localhost:5901"),
			password: getEnvOrDefault("TIGHTVNC_PASSWORD", ""),
			authType: 2, // VNC Authentication
			timeout:  30 * time.Second,
		},
		{
			name:     "RealVNC",
			address:  getEnvOrDefault("REALVNC_ADDRESS", "localhost:5902"),
			password: getEnvOrDefault("REALVNC_PASSWORD", ""),
			authType: 2, // VNC Authentication
			timeout:  30 * time.Second,
		},
		{
			name:     "TigerVNC",
			address:  getEnvOrDefault("TIGERVNC_ADDRESS", "localhost:5903"),
			password: getEnvOrDefault("TIGERVNC_PASSWORD", ""),
			authType: 2, // VNC Authentication
			timeout:  30 * time.Second,
		},
	}

	for _, server := range testServers {
		t.Run(server.name, func(t *testing.T) {
			testVNCServer(t, server.name, server.address, server.password, server.authType, server.timeout)
		})
	}
}

// testVNCServer performs comprehensive testing against a real VNC server.
func testVNCServer(t *testing.T, serverName, address, password string, authType uint8, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Logf("Testing %s server at %s", serverName, address)

	// Test basic connection establishment
	t.Run("Connection establishment", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to %s server at %s: %v", serverName, address, err)
		}
		defer conn.Close()

		// Create appropriate authentication
		var auth ClientAuth
		switch authType {
		case 1:
			auth = &ClientAuthNone{}
		case 2:
			if password == "" {
				t.Skipf("Password required for %s server but not provided", serverName)
			}
			auth = NewPasswordAuth(password)
		default:
			t.Fatalf("Unsupported auth type %d for %s", authType, serverName)
		}

		config := &ClientConfig{
			Auth: []ClientAuth{auth},
		}

		client, err := ClientWithContext(ctx, conn, config)
		if err != nil {
			t.Fatalf("Failed to establish VNC connection to %s: %v", serverName, err)
		}
		defer client.Close()

		// Verify basic connection properties
		if client.FrameBufferWidth == 0 || client.FrameBufferHeight == 0 {
			t.Errorf("%s: Invalid framebuffer dimensions: %dx%d",
				serverName, client.FrameBufferWidth, client.FrameBufferHeight)
		}

		t.Logf("%s: Connected successfully, framebuffer: %dx%d, desktop: %s",
			serverName, client.FrameBufferWidth, client.FrameBufferHeight, client.DesktopName)
	})

	// Test encoding support
	t.Run("Encoding support", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to %s server: %v", serverName, err)
		}
		defer conn.Close()

		var auth ClientAuth
		if authType == 1 {
			auth = &ClientAuthNone{}
		} else {
			auth = NewPasswordAuth(password)
		}

		config := &ClientConfig{Auth: []ClientAuth{auth}}
		client, err := ClientWithContext(ctx, conn, config)
		if err != nil {
			t.Skipf("Failed to connect to %s: %v", serverName, err)
		}
		defer client.Close()

		// Test different encoding combinations
		encodingTests := []struct {
			name      string
			encodings []Encoding
		}{
			{
				name:      "Raw only",
				encodings: []Encoding{&RawEncoding{}},
			},
			{
				name: "Multiple encodings",
				encodings: []Encoding{
					&HextileEncoding{},
					&CopyRectEncoding{},
					&RREEncoding{},
					&RawEncoding{},
				},
			},
			{
				name: "With pseudo-encodings",
				encodings: []Encoding{
					&RawEncoding{},
					&CursorPseudoEncoding{},
					&DesktopSizePseudoEncoding{},
				},
			},
		}

		for _, test := range encodingTests {
			t.Run(test.name, func(t *testing.T) {
				err := client.SetEncodings(test.encodings)
				if err != nil {
					t.Errorf("%s: Failed to set encodings %s: %v", serverName, test.name, err)
				} else {
					t.Logf("%s: Successfully set encodings: %s", serverName, test.name)
				}
			})
		}
	})

	// Test framebuffer updates
	t.Run("Framebuffer updates", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to %s server: %v", serverName, err)
		}
		defer conn.Close()

		var auth ClientAuth
		if authType == 1 {
			auth = &ClientAuthNone{}
		} else {
			auth = NewPasswordAuth(password)
		}

		// Set up message channel to receive updates
		msgCh := make(chan ServerMessage, 10)
		config := &ClientConfig{
			Auth:            []ClientAuth{auth},
			ServerMessageCh: msgCh,
		}

		client, err := ClientWithContext(ctx, conn, config)
		if err != nil {
			t.Skipf("Failed to connect to %s: %v", serverName, err)
		}
		defer client.Close()

		// Set encodings
		encodings := []Encoding{&RawEncoding{}}
		err = client.SetEncodings(encodings)
		if err != nil {
			t.Fatalf("%s: Failed to set encodings: %v", serverName, err)
		}

		// Request full framebuffer update
		err = client.FramebufferUpdateRequest(false, 0, 0, client.FrameBufferWidth, client.FrameBufferHeight)
		if err != nil {
			t.Fatalf("%s: Failed to request framebuffer update: %v", serverName, err)
		}

		// Wait for framebuffer update message
		select {
		case msg := <-msgCh:
			if fbUpdate, ok := msg.(*FramebufferUpdateMessage); ok {
				t.Logf("%s: Received framebuffer update with %d rectangles", serverName, len(fbUpdate.Rectangles))

				// Verify we got some rectangles
				if len(fbUpdate.Rectangles) == 0 {
					t.Errorf("%s: Framebuffer update contained no rectangles", serverName)
				}
			} else {
				t.Logf("%s: Received non-framebuffer message: %T", serverName, msg)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("%s: Timeout waiting for framebuffer update", serverName)
		}

		// Test incremental update
		err = client.FramebufferUpdateRequest(true, 0, 0, client.FrameBufferWidth, client.FrameBufferHeight)
		if err != nil {
			t.Errorf("%s: Failed to request incremental update: %v", serverName, err)
		}
	})

	// Test input events
	t.Run("Input events", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to %s server: %v", serverName, err)
		}
		defer conn.Close()

		var auth ClientAuth
		if authType == 1 {
			auth = &ClientAuthNone{}
		} else {
			auth = NewPasswordAuth(password)
		}

		config := &ClientConfig{Auth: []ClientAuth{auth}}
		client, err := ClientWithContext(ctx, conn, config)
		if err != nil {
			t.Skipf("Failed to connect to %s: %v", serverName, err)
		}
		defer client.Close()

		// Test key events
		testKeys := []struct {
			name   string
			keysym uint32
		}{
			{"Letter A", 0x0041},
			{"Enter", 0xff0d},
			{"Escape", 0xff1b},
			{"Space", 0x0020},
		}

		for _, key := range testKeys {
			t.Run("Key "+key.name, func(t *testing.T) {
				// Key down
				err := client.KeyEvent(key.keysym, true)
				if err != nil {
					t.Errorf("%s: Failed to send key down for %s: %v", serverName, key.name, err)
				}

				// Key up
				err = client.KeyEvent(key.keysym, false)
				if err != nil {
					t.Errorf("%s: Failed to send key up for %s: %v", serverName, key.name, err)
				}
			})
		}

		// Test pointer events
		t.Run("Pointer events", func(t *testing.T) {
			// Mouse movement
			err := client.PointerEvent(0, 100, 100)
			if err != nil {
				t.Errorf("%s: Failed to send mouse movement: %v", serverName, err)
			}

			// Left click
			err = client.PointerEvent(ButtonLeft, 100, 100)
			if err != nil {
				t.Errorf("%s: Failed to send left button down: %v", serverName, err)
			}

			err = client.PointerEvent(0, 100, 100)
			if err != nil {
				t.Errorf("%s: Failed to send left button up: %v", serverName, err)
			}

			// Right click
			err = client.PointerEvent(ButtonRight, 150, 150)
			if err != nil {
				t.Errorf("%s: Failed to send right button down: %v", serverName, err)
			}

			err = client.PointerEvent(0, 150, 150)
			if err != nil {
				t.Errorf("%s: Failed to send right button up: %v", serverName, err)
			}
		})

		// Test clipboard
		t.Run("Clipboard", func(t *testing.T) {
			testText := "Hello VNC Server!"
			err := client.CutText(testText)
			if err != nil {
				t.Errorf("%s: Failed to send clipboard text: %v", serverName, err)
			} else {
				t.Logf("%s: Successfully sent clipboard text", serverName)
			}
		})
	})

	// Test error handling and recovery
	t.Run("Error handling", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to %s server: %v", serverName, err)
		}
		defer conn.Close()

		var auth ClientAuth
		if authType == 1 {
			auth = &ClientAuthNone{}
		} else {
			auth = NewPasswordAuth(password)
		}

		config := &ClientConfig{Auth: []ClientAuth{auth}}
		client, err := ClientWithContext(ctx, conn, config)
		if err != nil {
			t.Skipf("Failed to connect to %s: %v", serverName, err)
		}
		defer client.Close()

		// Test invalid coordinates (should be handled gracefully)
		err = client.PointerEvent(0, 65535, 65535)
		if err != nil {
			t.Logf("%s: Invalid coordinates properly rejected: %v", serverName, err)
		}

		// Test invalid keysym (should be handled gracefully)
		err = client.KeyEvent(0xFFFFFFFF, true)
		if err != nil {
			t.Logf("%s: Invalid keysym properly rejected: %v", serverName, err)
		}

		// Test very long clipboard text (should be handled gracefully)
		longText := strings.Repeat("A", 10000)
		err = client.CutText(longText)
		if err != nil {
			t.Logf("%s: Long clipboard text properly handled: %v", serverName, err)
		}
	})
}

// testVNCServerStress performs stress testing against real VNC servers.
func TestIntegration_Stress(t *testing.T) {
	if os.Getenv("VNC_STRESS_TESTS") != "1" {
		t.Skip("Skipping VNC stress tests. Set VNC_STRESS_TESTS=1 to enable.")
	}

	address := getEnvOrDefault("VNC_STRESS_ADDRESS", "localhost:5900")
	password := getEnvOrDefault("VNC_STRESS_PASSWORD", "")

	t.Run("Multiple concurrent connections", func(t *testing.T) {
		const numConnections = 5
		const testDuration = 30 * time.Second

		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()

		for i := 0; i < numConnections; i++ {
			go func(connID int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Connection %d panicked: %v", connID, r)
					}
				}()

				conn, err := net.DialTimeout("tcp", address, 10*time.Second)
				if err != nil {
					t.Logf("Connection %d failed to dial: %v", connID, err)
					return
				}
				defer conn.Close()

				var auth ClientAuth
				if password == "" {
					auth = &ClientAuthNone{}
				} else {
					auth = NewPasswordAuth(password)
				}

				config := &ClientConfig{Auth: []ClientAuth{auth}}
				client, err := ClientWithContext(ctx, conn, config)
				if err != nil {
					t.Logf("Connection %d failed to establish VNC: %v", connID, err)
					return
				}
				defer client.Close()

				// Perform continuous operations
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						// Send random input events
						err := client.PointerEvent(0, uint16(connID*10), uint16(connID*10)) // #nosec G115 - Test code with small values
						if err != nil {
							t.Logf("Connection %d pointer event failed: %v", connID, err)
							return
						}

						err = client.FramebufferUpdateRequest(true, 0, 0,
							client.FrameBufferWidth, client.FrameBufferHeight)
						if err != nil {
							t.Logf("Connection %d framebuffer request failed: %v", connID, err)
							return
						}
					}
				}
			}(i)
		}

		// Wait for test duration
		<-ctx.Done()
		t.Logf("Stress test completed after %v", testDuration)
	})

	t.Run("High frequency updates", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to server: %v", err)
		}
		defer conn.Close()

		var auth ClientAuth
		if password == "" {
			auth = &ClientAuthNone{}
		} else {
			auth = NewPasswordAuth(password)
		}

		config := &ClientConfig{Auth: []ClientAuth{auth}}
		client, err := Client(conn, config)
		if err != nil {
			t.Skipf("Failed to establish VNC connection: %v", err)
		}
		defer client.Close()

		// Send high frequency framebuffer update requests
		const requestCount = 1000
		const requestInterval = 10 * time.Millisecond

		start := time.Now()
		for i := 0; i < requestCount; i++ {
			err := client.FramebufferUpdateRequest(true, 0, 0,
				client.FrameBufferWidth, client.FrameBufferHeight)
			if err != nil {
				t.Errorf("Request %d failed: %v", i, err)
				break
			}

			time.Sleep(requestInterval)
		}
		duration := time.Since(start)

		t.Logf("Sent %d framebuffer requests in %v (avg: %v per request)",
			requestCount, duration, duration/requestCount)
	})
}

// testVNCServerProtocolCompliance tests protocol compliance with real servers.
func TestIntegration_ProtocolCompliance(t *testing.T) {
	if os.Getenv("VNC_PROTOCOL_TESTS") != "1" {
		t.Skip("Skipping VNC protocol compliance tests. Set VNC_PROTOCOL_TESTS=1 to enable.")
	}

	address := getEnvOrDefault("VNC_PROTOCOL_ADDRESS", "localhost:5900")
	_ = getEnvOrDefault("VNC_PROTOCOL_PASSWORD", "") // Password for future use

	t.Run("Protocol version negotiation", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to server: %v", err)
		}
		defer conn.Close()

		// Read protocol version
		version := make([]byte, 12)
		_, err = conn.Read(version)
		if err != nil {
			t.Fatalf("Failed to read protocol version: %v", err)
		}

		versionStr := string(version)
		t.Logf("Server protocol version: %s", versionStr)

		// Verify it's a valid RFB protocol version
		if !strings.HasPrefix(versionStr, "RFB ") {
			t.Errorf("Invalid protocol version format: %s", versionStr)
		}

		// Send our version
		_, err = conn.Write([]byte("RFB 003.008\n"))
		if err != nil {
			t.Fatalf("Failed to send protocol version: %v", err)
		}
	})

	t.Run("Security type negotiation", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", address, 10*time.Second)
		if err != nil {
			t.Skipf("Cannot connect to server: %v", err)
		}
		defer conn.Close()

		// Complete protocol handshake manually to test security negotiation
		version := make([]byte, 12)
		if _, err := conn.Read(version); err != nil {
			t.Fatalf("Failed to read version: %v", err)
		}
		if _, err := conn.Write([]byte("RFB 003.008\n")); err != nil {
			t.Fatalf("Failed to write version: %v", err)
		}

		// Read security types
		securityCount := make([]byte, 1)
		_, err = conn.Read(securityCount)
		if err != nil {
			t.Fatalf("Failed to read security count: %v", err)
		}

		if securityCount[0] == 0 {
			// Server rejected connection
			reasonLength := make([]byte, 4)
			if _, err := conn.Read(reasonLength); err != nil {
				t.Fatalf("Failed to read reason length: %v", err)
			}
			t.Skipf("Server rejected connection")
		}

		securityTypes := make([]byte, securityCount[0])
		_, err = conn.Read(securityTypes)
		if err != nil {
			t.Fatalf("Failed to read security types: %v", err)
		}

		t.Logf("Server supports security types: %v", securityTypes)

		// Verify we support at least one of the offered types
		supportedFound := false
		for _, secType := range securityTypes {
			if secType == 1 || secType == 2 { // None or VNC auth
				supportedFound = true
				break
			}
		}

		if !supportedFound {
			t.Errorf("Server offers no supported security types: %v", securityTypes)
		}
	})
}

// getEnvOrDefault returns the value of an environment variable or a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
