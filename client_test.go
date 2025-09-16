// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

// newMockServer creates a mock VNC server for testing with the specified protocol version.
// Returns the server address that clients can connect to.
func newMockServer(t *testing.T, version string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("error listening: %s", err)
	}

	go func() {
		defer ln.Close()
		c, err := ln.Accept()
		if err != nil {
			t.Logf("error accepting conn: %s", err)
			return
		}
		defer c.Close()

		_, err = fmt.Fprintf(c, "RFB %s\n", version)
		if err != nil {
			t.Logf("failed writing version: %s", err)
			return
		}
	}()

	return ln.Addr().String()
}

func TestClient_LowMajorVersion(t *testing.T) {
	nc, err := net.Dial("tcp", newMockServer(t, "002.009"))
	if err != nil {
		t.Fatalf("error connecting to mock server: %s", err)
	}

	_, err = Client(nc, &ClientConfig{})
	if err == nil {
		t.Fatal("error expected")
	}

	expectedMsg := "vnc unsupported: handshake: unsupported major version, less than 3: 2"
	if err.Error() != expectedMsg {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestClient_LowMinorVersion(t *testing.T) {
	nc, err := net.Dial("tcp", newMockServer(t, "003.007"))
	if err != nil {
		t.Fatalf("error connecting to mock server: %s", err)
	}

	_, err = Client(nc, &ClientConfig{})
	if err == nil {
		t.Fatal("error expected")
	}

	expectedMsg := "vnc unsupported: handshake: unsupported minor version, less than 8: 7"
	if err.Error() != expectedMsg {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestClient_WithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	cancel()

	config := &ClientConfig{
		Auth: []ClientAuth{&ClientAuthNone{}},
	}

	_, err := ClientWithContext(ctx, client, config)

	if err == nil {
		t.Error("Expected error due to context cancellation, but got nil")
	}

	if err != context.Canceled {
		t.Logf("Got error: %v (type: %T)", err, err)
	}
}

func TestClient_WithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	config := &ClientConfig{
		Auth: []ClientAuth{&ClientAuthNone{}},
	}

	start := time.Now()
	_, err := ClientWithContext(ctx, client, config)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected error due to context timeout, but got nil")
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Operation took too long: %v, expected around 1ms", duration)
	}
}

// TestClientWithOptions_ConfigurationApplication tests that functional options
// are properly applied to the client configuration.
func TestClient_WithOptionsConfiguration(t *testing.T) {
	ctx := context.Background()

	// Create a mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	logger := &StandardLogger{}
	metrics := &NoOpMetrics{}

	// Use a short timeout context to avoid long waits
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := ClientWithOptions(shortCtx, client,
		WithAuth(&ClientAuthNone{}),
		WithExclusive(true),
		WithLogger(logger),
		WithConnectTimeout(50*time.Millisecond),
		WithReadTimeout(10*time.Millisecond),
		WithWriteTimeout(10*time.Millisecond),
		WithMetrics(metrics),
	)

	// The handshake will fail with a mock connection, but that's expected
	// The important thing is that the options were processed without error
	if err == nil {
		t.Error("Expected handshake to fail with mock connection")
	}

	// Test that individual options work
	config := &ClientConfig{}

	WithAuth(&ClientAuthNone{})(config)
	if len(config.Auth) != 1 {
		t.Errorf("Expected 1 auth method, got %d", len(config.Auth))
	}

	WithExclusive(true)(config)
	if !config.Exclusive {
		t.Error("Expected Exclusive to be true")
	}

	WithLogger(logger)(config)
	if config.Logger != logger {
		t.Error("Expected logger to be set")
	}

	WithConnectTimeout(5 * time.Second)(config)
	if config.ConnectTimeout != 5*time.Second {
		t.Errorf("Expected ConnectTimeout to be 5s, got %v", config.ConnectTimeout)
	}

	WithReadTimeout(2 * time.Second)(config)
	if config.ReadTimeout != 2*time.Second {
		t.Errorf("Expected ReadTimeout to be 2s, got %v", config.ReadTimeout)
	}

	WithWriteTimeout(3 * time.Second)(config)
	if config.WriteTimeout != 3*time.Second {
		t.Errorf("Expected WriteTimeout to be 3s, got %v", config.WriteTimeout)
	}

	WithMetrics(metrics)(config)
	if config.Metrics != metrics {
		t.Error("Expected metrics to be set")
	}
}

// TestClientWithOptions_BackwardCompatibility tests that the new functional options
// approach produces equivalent results to the traditional ClientConfig approach.
func TestClient_WithOptionsBackwardCompatibility(t *testing.T) {
	ctx := context.Background()

	// Create mock connections
	server1, client1 := net.Pipe()
	defer server1.Close()
	defer client1.Close()

	server2, client2 := net.Pipe()
	defer server2.Close()
	defer client2.Close()

	logger := &StandardLogger{}
	auth := &ClientAuthNone{}

	// Use short timeout contexts to avoid long waits
	shortCtx1, cancel1 := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel1()
	shortCtx2, cancel2 := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel2()

	// Traditional approach
	config := &ClientConfig{
		Auth:           []ClientAuth{auth},
		Exclusive:      true,
		Logger:         logger,
		ConnectTimeout: 10 * time.Millisecond,
		ReadTimeout:    5 * time.Millisecond,
		WriteTimeout:   5 * time.Millisecond,
	}

	_, err1 := ClientWithContext(shortCtx1, client1, config)

	// Functional options approach
	_, err2 := ClientWithOptions(shortCtx2, client2,
		WithAuth(auth),
		WithExclusive(true),
		WithLogger(logger),
		WithConnectTimeout(10*time.Millisecond),
		WithReadTimeout(5*time.Millisecond),
		WithWriteTimeout(5*time.Millisecond),
	)

	// Both should fail in the same way (mock connection)
	if (err1 == nil) != (err2 == nil) {
		t.Errorf("Expected both approaches to fail similarly, got err1=%v, err2=%v", err1, err2)
	}
}

// TestFunctionalOptions_Composition tests that functional options can be composed
// and reused effectively.
func TestClient_FunctionalOptionsComposition(t *testing.T) {
	// Define reusable option sets
	basicAuth := WithAuth(&ClientAuthNone{})
	standardLogging := WithLogger(&StandardLogger{})
	fastTimeouts := WithTimeout(1 * time.Second)

	// Compose options
	baseOptions := []ClientOption{basicAuth, standardLogging}
	fastOptions := append(baseOptions, fastTimeouts)
	exclusiveOptions := append(fastOptions, WithExclusive(true))

	// Test that options can be applied in sequence
	config := &ClientConfig{}

	for _, option := range exclusiveOptions {
		option(config)
	}

	// Verify all options were applied
	if len(config.Auth) != 1 {
		t.Errorf("Expected 1 auth method, got %d", len(config.Auth))
	}

	if config.Logger == nil {
		t.Error("Expected logger to be set")
	}

	if config.ReadTimeout != 1*time.Second {
		t.Errorf("Expected ReadTimeout to be 1s, got %v", config.ReadTimeout)
	}

	if config.WriteTimeout != 1*time.Second {
		t.Errorf("Expected WriteTimeout to be 1s, got %v", config.WriteTimeout)
	}

	if !config.Exclusive {
		t.Error("Expected Exclusive to be true")
	}
}
