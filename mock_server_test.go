// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

// MockVNCServer provides a simple mock VNC server for testing.
type MockVNCServer struct {
	listener net.Listener
	addr     string
	wg       sync.WaitGroup
	stop     chan struct{}

	// Configuration
	AuthMethods []uint8
	Password    string
	FrameWidth  uint16
	FrameHeight uint16
	DesktopName string
	AcceptAuth  bool
	SendUpdates bool
}

// NewMockVNCServer creates a new mock VNC server.
func NewMockVNCServer() *MockVNCServer {
	return &MockVNCServer{
		AuthMethods: []uint8{1}, // No auth by default
		FrameWidth:  800,
		FrameHeight: 600,
		DesktopName: "Mock VNC Server",
		AcceptAuth:  true,
		SendUpdates: false,
		stop:        make(chan struct{}),
	}
}

// Start starts the mock server on a random available port.
func (m *MockVNCServer) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	m.listener = listener
	m.addr = listener.Addr().String()

	m.wg.Add(1)
	go m.serve()

	return nil
}

// Stop stops the mock server.
func (m *MockVNCServer) Stop() {
	close(m.stop)
	if m.listener != nil {
		m.listener.Close()
	}
	m.wg.Wait()
}

// Addr returns the server address.
func (m *MockVNCServer) Addr() string {
	return m.addr
}

func (m *MockVNCServer) serve() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stop:
			return
		default:
		}

		conn, err := m.listener.Accept()
		if err != nil {
			select {
			case <-m.stop:
				return
			default:
				continue
			}
		}

		go m.handleConnection(conn)
	}
}

func (m *MockVNCServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set a reasonable timeout
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return
	}

	// Protocol version handshake
	if err := m.handleProtocolVersion(conn); err != nil {
		return
	}

	// Security handshake
	if err := m.handleSecurity(conn); err != nil {
		return
	}

	// Client init
	if err := m.handleClientInit(conn); err != nil {
		return
	}

	// Server init
	if err := m.handleServerInit(conn); err != nil {
		return
	}

	// Message loop (simplified)
	m.handleMessages(conn)
}

func (m *MockVNCServer) handleProtocolVersion(conn net.Conn) error {
	// Send protocol version
	_, err := conn.Write([]byte("RFB 003.008\n"))
	if err != nil {
		return err
	}

	// Read client protocol version
	buf := make([]byte, 12)
	_, err = io.ReadFull(conn, buf)
	return err
}

func (m *MockVNCServer) handleSecurity(conn net.Conn) error {
	// Send number of security types
	authMethodsLen := uint8(len(m.AuthMethods)) // #nosec G115 - Test code with small arrays
	if err := binary.Write(conn, binary.BigEndian, authMethodsLen); err != nil {
		return err
	}

	// Send security types
	for _, authType := range m.AuthMethods {
		if err := binary.Write(conn, binary.BigEndian, authType); err != nil {
			return err
		}
	}

	// Read client's chosen security type
	var chosenType uint8
	if err := binary.Read(conn, binary.BigEndian, &chosenType); err != nil {
		return err
	}

	// Handle authentication based on type
	switch chosenType {
	case 1: // No auth
		// Send security result based on AcceptAuth flag
		if m.AcceptAuth {
			return binary.Write(conn, binary.BigEndian, uint32(0)) // OK
		} else {
			return binary.Write(conn, binary.BigEndian, uint32(1)) // Failed
		}
	case 2: // VNC auth
		return m.handleVNCAuth(conn)
	default:
		// Send security result (1 = failed)
		return binary.Write(conn, binary.BigEndian, uint32(1))
	}
}

func (m *MockVNCServer) handleVNCAuth(conn net.Conn) error {
	// Send challenge (VNC challenge size)
	challenge := make([]byte, VNCChallengeSize)
	for i := range challenge {
		challenge[i] = byte(i) // Simple pattern for testing
	}

	if _, err := conn.Write(challenge); err != nil {
		return err
	}

	// Read response
	response := make([]byte, VNCChallengeSize)
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}

	// Send security result
	if m.AcceptAuth {
		return binary.Write(conn, binary.BigEndian, uint32(0)) // OK
	} else {
		return binary.Write(conn, binary.BigEndian, uint32(1)) // Failed
	}
}

func (m *MockVNCServer) handleClientInit(conn net.Conn) error {
	// Read shared flag
	var shared uint8
	return binary.Read(conn, binary.BigEndian, &shared)
}

func (m *MockVNCServer) handleServerInit(conn net.Conn) error {
	// Send framebuffer width and height
	if err := binary.Write(conn, binary.BigEndian, m.FrameWidth); err != nil {
		return err
	}
	if err := binary.Write(conn, binary.BigEndian, m.FrameHeight); err != nil {
		return err
	}

	// Send pixel format (simplified - 32-bit RGBA)
	pixelFormat := []byte{
		32, 24, 0, 1, // BPP, Depth, BigEndian, TrueColor
		0, 255, 0, 255, 0, 255, // RedMax, GreenMax, BlueMax
		16, 8, 0, // RedShift, GreenShift, BlueShift
		0, 0, 0, // Padding
	}
	if _, err := conn.Write(pixelFormat); err != nil {
		return err
	}

	// Send desktop name
	nameBytes := []byte(m.DesktopName)
	nameBytesLen := uint32(len(nameBytes)) // #nosec G115 - Test code with short names
	if err := binary.Write(conn, binary.BigEndian, nameBytesLen); err != nil {
		return err
	}
	_, err := conn.Write(nameBytes)
	return err
}

func (m *MockVNCServer) handleMessages(conn net.Conn) {
	buf := make([]byte, 1024)

	for {
		select {
		case <-m.stop:
			return
		default:
		}

		// Set a short read timeout
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return
		}

		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		if n > 0 {
			// Simple message handling - just echo back a basic response
			msgType := buf[0]
			switch msgType {
			case 3: // FramebufferUpdateRequest
				if m.SendUpdates {
					m.sendFramebufferUpdate(conn)
				}
			case 4: // KeyEvent
				// Ignore key events
			case 5: // PointerEvent
				// Ignore pointer events
			case 6: // ClientCutText
				// Ignore cut text
			}
		}
	}
}

func (m *MockVNCServer) sendFramebufferUpdate(conn net.Conn) {
	// Send a simple framebuffer update with one rectangle
	update := []byte{
		0,    // Message type (FramebufferUpdate)
		0,    // Padding
		0, 1, // Number of rectangles
		0, 0, // X
		0, 0, // Y
		0, 10, // Width
		0, 10, // Height
		0, 0, 0, 0, // Encoding (Raw)
	}

	if _, err := conn.Write(update); err != nil {
		return
	}

	// Send 10x10 pixels of red color (simplified)
	pixelData := make([]byte, 10*10*4) // 4 bytes per pixel
	for i := 0; i < len(pixelData); i += 4 {
		pixelData[i] = 0     // Blue
		pixelData[i+1] = 0   // Green
		pixelData[i+2] = 255 // Red
		pixelData[i+3] = 255 // Alpha
	}
	if _, err := conn.Write(pixelData); err != nil {
		return
	}
}

// StartMockServer is a helper function to start a mock server for testing.
func StartMockServer() (*MockVNCServer, error) {
	server := NewMockVNCServer()
	if err := server.Start(); err != nil {
		return nil, err
	}

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	return server, nil
}
