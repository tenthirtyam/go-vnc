// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

// Package main demonstrates basic VNC client usage with context support.
//
// This example shows common scenarios for connecting to and interacting with
// VNC servers using the library.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tenthirtyam/go-vnc"
)

func main() {
	// Create a context with timeout for the entire VNC session
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Example 1: Basic connection with password authentication
	fmt.Println("=== Basic VNC Connection Example ===")
	basicConnectionExample(ctx)

	// Example 2: Connection with multiple authentication methods
	fmt.Println("\n=== Multiple Authentication Methods Example ===")
	multiAuthExample(ctx)

	// Example 3: Message processing example
	fmt.Println("\n=== Message Processing Example ===")
	if err := messageProcessingExample(ctx); err != nil {
		log.Printf("Message processing example failed: %v", err)
	}

	// Example 4: Input automation example
	fmt.Println("\n=== Input Automation Example ===")
	inputAutomationExample(ctx)

	fmt.Println("\n=== All Examples Completed ===")
}

// basicConnectionExample demonstrates the simplest way to connect to a VNC server
// with context support and proper error handling.
func basicConnectionExample(ctx context.Context) {
	_ = ctx // Context would be used for cancellation in real implementation
	fmt.Println("Attempting to connect to VNC server...")

	// In a real application, replace with actual server address
	// conn, err := net.DialTimeout("tcp", "localhost:5900", 10*time.Second)
	// if err != nil {
	//     return fmt.Errorf("failed to connect: %w", err)
	// }
	// defer conn.Close()

	// For this example, we'll simulate the connection process
	fmt.Println("✓ Network connection established")

	// Configure authentication
	config := &vnc.ClientConfig{
		Auth: []vnc.ClientAuth{
			&vnc.PasswordAuth{Password: "your-password"},
		},
		Exclusive: true, // Disconnect other clients
	}

	fmt.Printf("✓ Authentication configured with %d method(s)\n", len(config.Auth))

	// In a real application:
	// client, err := vnc.ClientWithContext(ctx, conn, config)
	// if err != nil {
	//     return fmt.Errorf("VNC handshake failed: %w", err)
	// }
	// defer client.Close()

	fmt.Println("✓ VNC handshake completed")
	fmt.Println("✓ Ready to send/receive VNC messages")
}

// multiAuthExample demonstrates using multiple authentication methods
// with fallback support and proper context handling.
func multiAuthExample(ctx context.Context) {
	_ = ctx // Context would be used for cancellation in real implementation
	fmt.Println("Configuring multiple authentication methods...")

	// Configure multiple authentication methods in order of preference
	config := &vnc.ClientConfig{
		Auth: []vnc.ClientAuth{
			&vnc.PasswordAuth{Password: "primary-password"},
			&vnc.PasswordAuth{Password: "backup-password"},
			&vnc.ClientAuthNone{}, // Fallback for servers without auth
		},
		Exclusive: false, // Allow other clients
	}

	fmt.Printf("✓ Configured %d authentication methods:\n", len(config.Auth))
	for i, auth := range config.Auth {
		fmt.Printf("  %d. %s (Security Type: %d)\n", i+1, auth.String(), auth.SecurityType())
	}

	// Simulate server authentication negotiation
	serverSupportedTypes := []uint8{1, 2} // None and VNC Password
	fmt.Printf("✓ Server supports security types: %v\n", serverSupportedTypes)

	// Find compatible authentication method
	var selectedAuth vnc.ClientAuth
	for _, auth := range config.Auth {
		for _, serverType := range serverSupportedTypes {
			if auth.SecurityType() == serverType {
				selectedAuth = auth
				break
			}
		}
		if selectedAuth != nil {
			break
		}
	}

	if selectedAuth != nil {
		fmt.Printf("✓ Selected authentication method: %s\n", selectedAuth.String())
	} else {
		fmt.Println("✗ No compatible authentication method found")
	}
}

// messageProcessingExample demonstrates how to handle server messages
// with context support and proper error handling.
func messageProcessingExample(ctx context.Context) error {
	fmt.Println("Setting up server message processing...")

	// Create a buffered channel for server messages
	msgCh := make(chan vnc.ServerMessage, 50)
	defer close(msgCh)

	fmt.Printf("✓ Message channel created (capacity: %d)\n", cap(msgCh))

	// Start message processing goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("✓ Message processing stopped (context cancelled)")
				return
			case msg := <-msgCh:
				handleServerMessage(msg)
			}
		}
	}()

	// Simulate receiving different types of server messages
	simulateServerMessages(msgCh)

	// Wait a moment for message processing
	time.Sleep(100 * time.Millisecond)

	fmt.Println("✓ Message processing example completed")
	return nil
}

// handleServerMessage processes different types of VNC server messages.
func handleServerMessage(msg vnc.ServerMessage) {
	switch m := msg.(type) {
	case *vnc.FramebufferUpdateMessage:
		fmt.Printf("  📺 Framebuffer update: %d rectangle(s)\n", len(m.Rectangles))
		for i, rect := range m.Rectangles {
			fmt.Printf("    Rectangle %d: (%d,%d) %dx%d\n",
				i+1, rect.X, rect.Y, rect.Width, rect.Height)
		}

	case *vnc.BellMessage:
		fmt.Println("  🔔 Bell notification received")

	case *vnc.ServerCutTextMessage:
		fmt.Printf("  📋 Clipboard update: %q\n", m.Text)

	case *vnc.SetColorMapEntriesMessage:
		fmt.Printf("  🎨 Color map update: %d entries\n", len(m.Colors))

	default:
		fmt.Printf("  ❓ Unknown message type: %T\n", msg)
	}
}

// simulateServerMessages creates sample server messages for demonstration.
func simulateServerMessages(msgCh chan<- vnc.ServerMessage) {
	// Simulate framebuffer update
	fbUpdate := &vnc.FramebufferUpdateMessage{
		Rectangles: []vnc.Rectangle{
			{
				X: 0, Y: 0, Width: 800, Height: 600,
				Enc: &vnc.RawEncoding{},
			},
		},
	}
	msgCh <- fbUpdate

	// Simulate bell notification
	var bellMsg vnc.BellMessage
	msgCh <- &bellMsg

	// Simulate clipboard update
	msgCh <- &vnc.ServerCutTextMessage{
		Text: "Hello from VNC server!",
	}

	// Simulate color map update
	msgCh <- &vnc.SetColorMapEntriesMessage{
		FirstColor: 0,
		Colors: []vnc.Color{
			vnc.ColorRed,
			vnc.ColorGreen,
			vnc.ColorBlue,
		},
	}
}

// inputAutomationExample demonstrates sending keyboard and mouse input
// to the VNC server with context support.
func inputAutomationExample(ctx context.Context) {
	_ = ctx // Context would be used for cancellation in real implementation
	fmt.Println("Demonstrating input automation...")

	// Simulate VNC client connection
	// In a real application, you would have an actual client connection:
	// client, err := vnc.ClientWithContext(ctx, conn, config)

	fmt.Println("✓ VNC client ready for input")

	// Example 1: Mouse operations
	fmt.Println("  🖱️  Mouse operations:")
	fmt.Println("    - Moving mouse to (100, 100)")
	fmt.Println("    - Left click down")
	fmt.Println("    - Left click up")

	// In a real application:
	// client.PointerEvent(0, 100, 100)                    // Move mouse
	// client.PointerEvent(vnc.ButtonLeft, 100, 100)      // Click down
	// client.PointerEvent(0, 100, 100)                    // Click up

	// Example 2: Keyboard operations
	fmt.Println("  ⌨️  Keyboard operations:")
	fmt.Println("    - Typing: 'Hello VNC!'")

	text := "Hello VNC!"
	for _, char := range text {
		fmt.Printf("    - Key: '%c'\n", char)
		// In a real application:
		// client.KeyEvent(uint32(char), true)  // Key down
		// client.KeyEvent(uint32(char), false) // Key up
	}

	// Example 3: Special keys
	fmt.Println("  🔑 Special keys:")
	fmt.Println("    - Pressing Enter")
	fmt.Println("    - Pressing Tab")
	fmt.Println("    - Pressing Escape")

	// In a real application:
	// const XK_Return = 0xff0d
	// const XK_Tab = 0xff09
	// const XK_Escape = 0xff1b
	// client.KeyEvent(XK_Return, true)
	// client.KeyEvent(XK_Return, false)

	// Example 4: Drag and drop
	fmt.Println("  🖱️  Drag and drop operation:")
	fmt.Println("    - Mouse down at (50, 50)")
	fmt.Println("    - Drag to (200, 200)")
	fmt.Println("    - Mouse up")

	// In a real application:
	// client.PointerEvent(vnc.ButtonLeft, 50, 50)    // Start drag
	// client.PointerEvent(vnc.ButtonLeft, 200, 200)  // Drag to position
	// client.PointerEvent(0, 200, 200)               // End drag

	fmt.Println("✓ Input automation example completed")
}
