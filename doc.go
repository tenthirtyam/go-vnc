// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

// Package vnc implements a VNC (Virtual Network Computing) client library for Go.
//
// This library provides a complete implementation of the VNC protocol as defined
// in RFC 6143, enabling Go applications to connect to and interact with VNC servers.
//
// # Basic Usage
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	conn, err := net.Dial("tcp", "localhost:5900")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer conn.Close()
//
//	config := &vnc.ClientConfig{
//		Auth: []vnc.ClientAuth{&vnc.PasswordAuth{Password: "secret"}},
//	}
//
//	client, err := vnc.ClientWithContext(ctx, conn, config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
// # Message Handling
//
//	msgCh := make(chan vnc.ServerMessage, 100)
//	config.ServerMessageCh = msgCh
//
//	go func() {
//		for msg := range msgCh {
//			switch m := msg.(type) {
//			case *vnc.FramebufferUpdateMessage:
//				// Handle framebuffer updates
//			case *vnc.BellMessage:
//				// Handle bell notifications
//			}
//		}
//	}()
//
// # Input Events
//
//	// Send keyboard input
//	client.KeyEvent(0x0061, true)  // 'a' key down
//	client.KeyEvent(0x0061, false) // 'a' key up
//
//	// Send mouse input
//	client.PointerEvent(vnc.ButtonLeft, 100, 100) // Click
//	client.PointerEvent(0, 100, 100)              // Release
//
// # Error Handling
//
//	if vnc.IsVNCError(err, vnc.ErrAuthentication) {
//		log.Printf("Authentication failed: %v", err)
//	}
//
// This library maintains API compatibility with github.com/mitchellh/go-vnc
// and can be used as a drop-in replacement.

package vnc
