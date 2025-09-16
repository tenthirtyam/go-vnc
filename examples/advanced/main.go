// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

// Package main demonstrates advanced VNC client usage patterns with context
// support.
//
// This example shows scenarios including connection pooling, performance
// optimization, error handling, and real-world automation tasks.

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tenthirtyam/go-vnc"
)

func main() {
	// Create a context for the entire application
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Example 1: Connection pool management
	fmt.Println("=== Connection Pool Management Example ===")
	if err := connectionPoolExample(ctx); err != nil {
		log.Printf("Connection pool example failed: %v", err)
	}

	// Example 2: Performance optimization
	fmt.Println("\n=== Performance Optimization Example ===")
	performanceOptimizationExample(ctx)

	// Example 3: Advanced error handling
	fmt.Println("\n=== Advanced Error Handling Example ===")
	if err := advancedErrorHandlingExample(ctx); err != nil {
		log.Printf("Advanced error handling example failed: %v", err)
	}

	// Example 4: Automated workflow
	fmt.Println("\n=== Automated Workflow Example ===")
	if err := automatedWorkflowExample(ctx); err != nil {
		log.Printf("Automated workflow example failed: %v", err)
	}

	fmt.Println("\n=== All Advanced Examples Completed ===")
}

// VNCPool manages multiple VNC connections with context support.
type VNCPool struct {
	connections map[string]*vnc.ClientConn
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *log.Logger
}

// NewVNCPool creates a new connection pool with context support.
func NewVNCPool(ctx context.Context, logger *log.Logger) *VNCPool {
	poolCtx, cancel := context.WithCancel(ctx)
	return &VNCPool{
		connections: make(map[string]*vnc.ClientConn),
		ctx:         poolCtx,
		cancel:      cancel,
		logger:      logger,
	}
}

// Connect establishes a new VNC connection and adds it to the pool.
func (p *VNCPool) Connect(address, password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if connection already exists
	if _, exists := p.connections[address]; exists {
		return fmt.Errorf("connection to %s already exists", address)
	}

	p.logger.Printf("Connecting to %s...", address)

	// In a real application, establish network connection:
	// conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	// if err != nil {
	//     return fmt.Errorf("failed to connect to %s: %w", address, err)
	// }

	config := &vnc.ClientConfig{
		Auth: []vnc.ClientAuth{
			&vnc.PasswordAuth{Password: password},
			&vnc.ClientAuthNone{},
		},
	}

	// In a real application:
	// client, err := vnc.ClientWithContext(p.ctx, conn, config)
	_ = config // Suppress unused variable warning for example
	// if err != nil {
	//     conn.Close()
	//     return fmt.Errorf("VNC handshake failed for %s: %w", address, err)
	// }

	// p.connections[address] = client
	p.logger.Printf("Successfully connected to %s", address)
	return nil
}

// GetConnection retrieves a connection from the pool.
func (p *VNCPool) GetConnection(address string) (*vnc.ClientConn, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	conn, exists := p.connections[address]
	return conn, exists
}

// Close closes all connections and cleans up the pool.
func (p *VNCPool) Close() {
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()

	for address, client := range p.connections {
		p.logger.Printf("Closing connection to %s", address)
		client.Close()
	}
	p.connections = make(map[string]*vnc.ClientConn)
}

// connectionPoolExample demonstrates managing multiple VNC connections.
func connectionPoolExample(ctx context.Context) error {
	fmt.Println("Creating VNC connection pool...")

	logger := log.New(log.Writer(), "[VNC-POOL] ", log.LstdFlags)
	pool := NewVNCPool(ctx, logger)
	defer pool.Close()

	// Simulate connecting to multiple servers
	servers := []struct {
		address  string
		password string
	}{
		{"server1:5900", "password1"},
		{"server2:5900", "password2"},
		{"server3:5900", "password3"},
	}

	fmt.Printf("✓ Pool created, connecting to %d servers...\n", len(servers))

	for _, server := range servers {
		if err := pool.Connect(server.address, server.password); err != nil {
			fmt.Printf("✗ Failed to connect to %s: %v\n", server.address, err)
		} else {
			fmt.Printf("✓ Connected to %s\n", server.address)
		}
	}

	// Demonstrate using connections from the pool
	for _, server := range servers {
		if conn, exists := pool.GetConnection(server.address); exists {
			fmt.Printf("✓ Retrieved connection for %s: %v\n", server.address, conn != nil)
		} else {
			fmt.Printf("✗ Connection not found for %s\n", server.address)
		}
	}

	fmt.Println("✓ Connection pool example completed")
	return nil
}

// PerformanceConfig holds configuration for different performance scenarios.
type PerformanceConfig struct {
	Name        string
	PixelFormat string
	Encodings   []string
	UpdateRate  int
	Compression string
	Description string
}

// performanceOptimizationExample demonstrates optimizing VNC performance for different scenarios.
func performanceOptimizationExample(ctx context.Context) {
	_ = ctx // Context would be used for cancellation in real implementation
	fmt.Println("Demonstrating performance optimization strategies...")

	configs := []PerformanceConfig{
		{
			Name:        "High Quality (LAN)",
			PixelFormat: "32 BPP, 24-bit depth",
			Encodings:   []string{"Raw", "CopyRect"},
			UpdateRate:  60,
			Compression: "None",
			Description: "Optimal for local network with high bandwidth",
		},
		{
			Name:        "Balanced (Broadband)",
			PixelFormat: "16 BPP, 16-bit depth",
			Encodings:   []string{"Hextile", "CopyRect", "RRE", "Raw"},
			UpdateRate:  30,
			Compression: "Medium",
			Description: "Good balance of quality and bandwidth usage",
		},
		{
			Name:        "Low Bandwidth (Mobile)",
			PixelFormat: "8 BPP, indexed color",
			Encodings:   []string{"Hextile", "RRE", "Raw"},
			UpdateRate:  15,
			Compression: "High",
			Description: "Optimized for slow connections",
		},
	}

	for i, config := range configs {
		fmt.Printf("\n%d. %s:\n", i+1, config.Name)
		fmt.Printf("   Description: %s\n", config.Description)
		fmt.Printf("   Pixel Format: %s\n", config.PixelFormat)
		fmt.Printf("   Encodings: %v\n", config.Encodings)
		fmt.Printf("   Update Rate: %d FPS\n", config.UpdateRate)
		fmt.Printf("   Compression: %s\n", config.Compression)

		// In a real application, you would apply these settings:
		// err := client.SetPixelFormat(pixelFormat)
		// err = client.SetEncodings(encodings)
		// err = client.SetUpdateRate(updateRate)

		fmt.Printf("   ✓ Configuration applied\n")
	}

	// Demonstrate adaptive configuration based on network conditions
	fmt.Println("\nAdaptive Configuration:")
	networkConditions := []string{"excellent", "good", "poor", "very poor"}

	for _, condition := range networkConditions {
		var selectedConfig PerformanceConfig
		switch condition {
		case "excellent":
			selectedConfig = configs[0]
		case "good":
			selectedConfig = configs[1]
		case "poor", "very poor":
			selectedConfig = configs[2]
		}

		fmt.Printf("  Network: %-10s -> %s\n", condition, selectedConfig.Name)
	}

	fmt.Println("✓ Performance optimization example completed")
}

// VNCError represents a structured VNC error with context.
type VNCError struct {
	Operation string
	Code      string
	Message   string
	Cause     error
}

func (e *VNCError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s failed (%s): %s - %v", e.Operation, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s failed (%s): %s", e.Operation, e.Code, e.Message)
}

func (e *VNCError) Unwrap() error {
	return e.Cause
}

// advancedErrorHandlingExample demonstrates comprehensive error handling patterns.
func advancedErrorHandlingExample(ctx context.Context) error {
	fmt.Println("Demonstrating advanced error handling patterns...")

	// Simulate various error scenarios
	errorScenarios := []struct {
		name        string
		operation   string
		errorCode   string
		message     string
		shouldRetry bool
	}{
		{
			name:        "Network Timeout",
			operation:   "connect",
			errorCode:   "TIMEOUT",
			message:     "connection timed out after 30 seconds",
			shouldRetry: true,
		},
		{
			name:        "Authentication Failed",
			operation:   "authenticate",
			errorCode:   "AUTH_FAILED",
			message:     "invalid password provided",
			shouldRetry: false,
		},
		{
			name:        "Protocol Error",
			operation:   "handshake",
			errorCode:   "PROTOCOL_ERROR",
			message:     "unsupported protocol version",
			shouldRetry: false,
		},
		{
			name:        "Encoding Error",
			operation:   "decode_framebuffer",
			errorCode:   "ENCODING_ERROR",
			message:     "corrupted encoding data received",
			shouldRetry: true,
		},
	}

	for i, scenario := range errorScenarios {
		fmt.Printf("\n%d. %s:\n", i+1, scenario.name)

		// Create structured error
		vncErr := &VNCError{
			Operation: scenario.operation,
			Code:      scenario.errorCode,
			Message:   scenario.message,
		}

		fmt.Printf("   Error: %v\n", vncErr)
		fmt.Printf("   Retryable: %v\n", scenario.shouldRetry)

		// Demonstrate error handling strategy
		if scenario.shouldRetry {
			fmt.Printf("   Strategy: Retry with exponential backoff\n")
			for attempt := 1; attempt <= 3; attempt++ {
				fmt.Printf("     Attempt %d: Retrying after %ds delay\n", attempt, attempt*2)
				// In a real application: time.Sleep(time.Duration(attempt*2) * time.Second)
			}
		} else {
			fmt.Printf("   Strategy: Fail fast, report to user\n")
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			fmt.Printf("   Context cancelled: %v\n", ctx.Err())
			return ctx.Err()
		default:
			fmt.Printf("   ✓ Error handled successfully\n")
		}
	}

	fmt.Println("✓ Advanced error handling example completed")
	return nil
}

// WorkflowStep represents a single step in an automated workflow.
type WorkflowStep struct {
	Name        string
	Description string
	Action      func(ctx context.Context) error
	Timeout     time.Duration
	Required    bool
}

// automatedWorkflowExample demonstrates complex automation workflows with context support.
func automatedWorkflowExample(ctx context.Context) error {
	fmt.Println("Executing automated VNC workflow...")

	// Define workflow steps
	workflow := []WorkflowStep{
		{
			Name:        "Initialize",
			Description: "Connect to VNC server and establish session",
			Timeout:     30 * time.Second,
			Required:    true,
			Action: func(ctx context.Context) error {
				fmt.Println("    - Establishing VNC connection")
				fmt.Println("    - Performing authentication")
				fmt.Println("    - Requesting initial framebuffer")
				return nil
			},
		},
		{
			Name:        "Login",
			Description: "Perform automated login to remote system",
			Timeout:     15 * time.Second,
			Required:    true,
			Action: func(ctx context.Context) error {
				fmt.Println("    - Clicking on username field")
				fmt.Println("    - Typing username")
				fmt.Println("    - Clicking on password field")
				fmt.Println("    - Typing password")
				fmt.Println("    - Clicking login button")
				return nil
			},
		},
		{
			Name:        "Navigate",
			Description: "Navigate to target application",
			Timeout:     20 * time.Second,
			Required:    true,
			Action: func(ctx context.Context) error {
				fmt.Println("    - Opening start menu")
				fmt.Println("    - Searching for application")
				fmt.Println("    - Launching target application")
				fmt.Println("    - Waiting for application to load")
				return nil
			},
		},
		{
			Name:        "Execute Task",
			Description: "Perform the main automation task",
			Timeout:     60 * time.Second,
			Required:    true,
			Action: func(ctx context.Context) error {
				fmt.Println("    - Opening file menu")
				fmt.Println("    - Creating new document")
				fmt.Println("    - Entering data")
				fmt.Println("    - Saving document")
				fmt.Println("    - Generating report")
				return nil
			},
		},
		{
			Name:        "Cleanup",
			Description: "Clean up and logout",
			Timeout:     10 * time.Second,
			Required:    false,
			Action: func(ctx context.Context) error {
				fmt.Println("    - Closing applications")
				fmt.Println("    - Logging out")
				fmt.Println("    - Disconnecting VNC session")
				return nil
			},
		},
	}

	// Execute workflow steps
	for i, step := range workflow {
		fmt.Printf("\n%d. %s:\n", i+1, step.Name)
		fmt.Printf("   %s\n", step.Description)

		// Create step context with timeout
		stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
		defer cancel()

		// Execute step
		if err := step.Action(stepCtx); err != nil {
			fmt.Printf("   ✗ Step failed: %v\n", err)
			if step.Required {
				return fmt.Errorf("required step '%s' failed: %w", step.Name, err)
			}
			fmt.Printf("   ⚠ Optional step failed, continuing...\n")
		} else {
			fmt.Printf("   ✓ Step completed successfully\n")
		}

		// Check if main context was cancelled
		select {
		case <-ctx.Done():
			fmt.Printf("   ⚠ Workflow cancelled: %v\n", ctx.Err())
			return ctx.Err()
		default:
		}
	}

	fmt.Println("✓ Automated workflow completed successfully")
	return nil
}
