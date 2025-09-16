// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ButtonMask represents the state of pointer buttons in a VNC pointer event.
type ButtonMask uint8

// Button mask constants for standard mouse buttons and scroll wheel events.
const (
	ButtonLeft ButtonMask = 1 << iota
	ButtonMiddle
	ButtonRight
	Button4
	Button5
	Button6
	Button7
	Button8
)

// VNC protocol constants.
const (
	ColorMapSize             = 256
	MaxClipboardLength       = 1024 * 1024
	Latin1MaxCodePoint       = 255
	MaxRectanglesPerUpdate   = 10000
	MaxServerClipboardLength = 10 * 1024 * 1024
)

// MetricsCollector defines the interface for collecting metrics and observability data.
type MetricsCollector interface {
	Counter(name string, tags ...interface{}) interface{}
	Gauge(name string, tags ...interface{}) interface{}
	Histogram(name string, tags ...interface{}) interface{}
}

// NoOpMetrics is a MetricsCollector implementation that discards all metrics.
type NoOpMetrics struct{}

// Counter returns a no-op counter metric.
func (m *NoOpMetrics) Counter(name string, tags ...interface{}) interface{} { return nil }

// Gauge returns a no-op gauge metric.
func (m *NoOpMetrics) Gauge(name string, tags ...interface{}) interface{} { return nil }

// Histogram returns a no-op histogram metric.
func (m *NoOpMetrics) Histogram(name string, tags ...interface{}) interface{} { return nil }

// ClientConn represents an active VNC client connection.
// Safe for concurrent use for sending client messages.
type ClientConn struct {
	c      net.Conn
	config *ClientConfig
	logger Logger

	// Context and cancellation support
	ctx    context.Context
	cancel context.CancelFunc

	// Mutex for protecting concurrent access to connection state
	mu sync.RWMutex

	// ColorMap contains the color map for indexed color modes.
	ColorMap [ColorMapSize]Color

	// Encs contains the list of encodings supported by this client.
	Encs []Encoding

	// FrameBufferWidth is the width of the remote framebuffer in pixels.
	FrameBufferWidth uint16

	// FrameBufferHeight is the height of the remote framebuffer in pixels.
	FrameBufferHeight uint16

	// DesktopName is the human-readable name of the desktop.
	DesktopName string

	// PixelFormat describes the format of pixel data used in this connection.
	PixelFormat PixelFormat
}

// ClientConfig configures VNC client connection behavior.
type ClientConfig struct {
	// Auth specifies the authentication methods supported by the client.
	Auth []ClientAuth

	// Exclusive determines whether this client requests exclusive access.
	Exclusive bool

	// ServerMessageCh is the channel where server messages will be delivered.
	ServerMessageCh chan<- ServerMessage

	// ServerMessages specifies additional custom server message types.
	ServerMessages []ServerMessage

	// Logger specifies the logger instance to use for connection logging.
	Logger Logger

	// AuthRegistry specifies the authentication registry to use.
	AuthRegistry *AuthRegistry

	// ConnectTimeout specifies the timeout for the initial connection handshake.
	ConnectTimeout time.Duration

	// ReadTimeout specifies the timeout for individual read operations.
	ReadTimeout time.Duration

	// WriteTimeout specifies the timeout for individual write operations.
	WriteTimeout time.Duration

	// Metrics specifies the metrics collector to use for connection monitoring.
	Metrics MetricsCollector
}

// ClientOption represents a functional option for configuring a VNC client connection.
type ClientOption func(*ClientConfig)

// WithAuth sets the authentication methods for the client connection.
// The methods are tried in the order provided during server negotiation.
func WithAuth(auth ...ClientAuth) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.Auth = auth
	}
}

// WithAuthRegistry sets a custom authentication registry for the client.
// This allows registration of custom authentication methods beyond the defaults.
func WithAuthRegistry(registry *AuthRegistry) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.AuthRegistry = registry
	}
}

// WithExclusive sets whether the client should request exclusive access to the server.
// When true, other clients will be disconnected when this client connects.
func WithExclusive(exclusive bool) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.Exclusive = exclusive
	}
}

// WithLogger sets the logger for the client connection.
// Use NoOpLogger to disable logging or provide a custom implementation.
func WithLogger(logger Logger) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.Logger = logger
	}
}

// WithServerMessageChannel sets the channel where server messages will be delivered.
// The channel should be buffered to prevent blocking the message processing loop.
func WithServerMessageChannel(ch chan<- ServerMessage) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.ServerMessageCh = ch
	}
}

// WithServerMessages sets additional custom server message types.
// These will be registered alongside the standard VNC message types.
func WithServerMessages(messages ...ServerMessage) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.ServerMessages = messages
	}
}

// WithConnectTimeout sets the timeout for the initial connection handshake.
// This includes protocol negotiation, security handshake, and initialization.
func WithConnectTimeout(timeout time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.ConnectTimeout = timeout
	}
}

// WithReadTimeout sets the timeout for individual read operations.
// This applies to reading server messages and framebuffer data.
func WithReadTimeout(timeout time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.ReadTimeout = timeout
	}
}

// WithWriteTimeout sets the timeout for individual write operations.
// This applies to sending client messages like key events and pointer events.
func WithWriteTimeout(timeout time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.WriteTimeout = timeout
	}
}

// WithTimeout sets both read and write timeouts to the same value.
// This is a convenience function for setting both timeouts at once.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.ReadTimeout = timeout
		cfg.WriteTimeout = timeout
	}
}

// WithMetrics sets the metrics collector for connection monitoring.
// Use NoOpMetrics to disable metrics collection or provide a custom implementation.
func WithMetrics(metrics MetricsCollector) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.Metrics = metrics
	}
}

// Client establishes a VNC client connection with the provided configuration.
// Performs complete handshake and starts background message processing.
//
// Deprecated: Use ClientWithContext for better cancellation support.
func Client(c net.Conn, cfg *ClientConfig) (*ClientConn, error) {
	return ClientWithContext(context.Background(), c, cfg)
}

// ClientWithContext establishes a VNC client connection with context support.
// Performs complete handshake including protocol negotiation, security, and initialization.
func ClientWithContext(ctx context.Context, c net.Conn, cfg *ClientConfig) (*ClientConn, error) {
	// Initialize logger from config or use NoOpLogger as default
	var logger Logger = &NoOpLogger{}
	if cfg != nil && cfg.Logger != nil {
		logger = cfg.Logger
	}

	// Create a cancellable context for this connection
	connCtx, cancel := context.WithCancel(ctx)

	conn := &ClientConn{
		c:      c,
		config: cfg,
		logger: logger,
		ctx:    connCtx,
		cancel: cancel,
	}

	if err := conn.handshakeWithContext(connCtx); err != nil {
		conn.Close()
		return nil, err
	}

	go conn.mainLoop()

	return conn, nil
}

// ClientWithOptions establishes a VNC client connection using functional options for configuration.
// This provides a modern, flexible way to configure client connections while maintaining
// backward compatibility. Options are applied in the order they are provided.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - c: An established network connection to a VNC server (typically TCP)
//   - options: Functional options for configuring the client behavior
//
// Returns:
//   - *ClientConn: A configured VNC client connection ready for use
//   - error: Any error that occurred during the handshake process
//
// Example usage:
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
//	client, err := ClientWithOptions(ctx, conn,
//		WithAuth(&PasswordAuth{Password: "secret"}),
//		WithExclusive(true),
//		WithLogger(&StandardLogger{}),
//		WithTimeout(10*time.Second),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
// Advanced configuration example:
//
//	msgCh := make(chan ServerMessage, 100)
//	registry := NewAuthRegistry()
//	registry.Register(16, func() ClientAuth { return &CustomAuth{} })
//
//	client, err := ClientWithOptions(ctx, conn,
//		WithAuthRegistry(registry),
//		WithServerMessageChannel(msgCh),
//		WithConnectTimeout(30*time.Second),
//		WithReadTimeout(5*time.Second),
//		WithWriteTimeout(5*time.Second),
//		WithMetrics(&PrometheusMetrics{}),
//	)
//
// The functional options approach provides several benefits:
// - Type-safe configuration with compile-time validation
// - Extensible without breaking existing code
// - Self-documenting through option names
// - Composable and reusable option sets
// - Optional parameters with sensible defaults.
func ClientWithOptions(ctx context.Context, c net.Conn, options ...ClientOption) (*ClientConn, error) {
	// Create default configuration
	cfg := &ClientConfig{}

	// Apply all functional options
	for _, option := range options {
		option(cfg)
	}

	// Apply connect timeout to context if specified
	if cfg.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
		defer cancel()
	}

	// Use the existing ClientWithContext function with the configured options
	return ClientWithContext(ctx, c, cfg)
}

// Close terminates the VNC connection and releases associated resources.
// This method closes the underlying network connection, cancels the connection context,
// and will cause the message processing goroutine to exit and close the server message channel.
//
// It is safe to call Close multiple times; subsequent calls will have no effect.
// After calling Close, the ClientConn should not be used for any other operations.
//
// Returns:
//   - error: Any error that occurred while closing the network connection
//
// Example usage:
//
//	client, err := Client(conn, config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close() // Ensure cleanup
//
//	// Use the client...
//
//	// Explicit close when done
//	if err := client.Close(); err != nil {
//		log.Printf("Error closing VNC connection: %v", err)
//	}
func (c *ClientConn) Close() error {
	// Cancel the context to signal all operations to stop
	if c.cancel != nil {
		c.cancel()
	}

	// Close the network connection
	return c.c.Close()
}

// CutText sends clipboard text from the client to the VNC server.
// This method implements the ClientCutText message as defined in RFC 6143 Section 7.5.6,
// allowing the client to share clipboard content with the remote desktop.
//
// The text must contain only Latin-1 characters (Unicode code points 0-255).
// Characters outside this range will cause a validation error. This restriction
// is imposed by the VNC protocol specification for compatibility across different
// systems and character encodings.
//
// Parameters:
//   - text: The clipboard text to send to the server (Latin-1 characters only)
//
// Returns:
//   - error: ValidationError if text contains invalid characters, NetworkError for transmission issues
//
// Example usage:
//
//	// Send simple ASCII text
//	err := client.CutText("Hello, World!")
//	if err != nil {
//		log.Printf("Failed to send clipboard text: %v", err)
//	}
//
//	// Handle clipboard synchronization
//	clipboardText := getLocalClipboard()
//	if isValidLatin1(clipboardText) {
//		client.CutText(clipboardText)
//	}
//
// Character validation:
// The method validates each character to ensure it falls within the Latin-1
// character set (0-255). Characters beyond this range will result in an error:
//
//	// This will fail - contains Unicode characters outside Latin-1
//	err := client.CutText("Hello 世界") // Contains Chinese characters
//	if err != nil {
//		// Handle validation error
//	}
//
// Security considerations:
// Clipboard sharing can potentially expose sensitive information. Applications
// should consider whether clipboard synchronization is appropriate for their
// security requirements and may want to filter or sanitize clipboard content.
func (c *ClientConn) CutText(text string) error {
	// Validate and sanitize clipboard text for security
	validator := newInputValidator()

	if err := validator.ValidateTextData(text, MaxClipboardLength); err != nil {
		c.logger.Error("Invalid clipboard text",
			Field{Key: "text_length", Value: len(text)},
			Field{Key: "error", Value: err})
		return validationError("CutText", "invalid clipboard text", err)
	}

	// Sanitize the text to remove potentially dangerous characters
	sanitizedText := validator.SanitizeText(text)
	if sanitizedText != text {
		c.logger.Warn("Clipboard text was sanitized",
			Field{Key: "original_length", Value: len(text)},
			Field{Key: "sanitized_length", Value: len(sanitizedText)})
		text = sanitizedText
	}

	var buf bytes.Buffer

	// This is the fixed size data we'll send
	fixedData := []interface{}{
		uint8(6),
		uint8(0),
		uint8(0),
		uint8(0),
		uint32(len(text)), // #nosec G115 - len(text) was already validated by ValidateTextData
	}

	for _, val := range fixedData {
		if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
			return networkError("CutText", "failed to write fixed data to buffer", err)
		}
	}

	for _, char := range text {
		if char > Latin1MaxCodePoint {
			return validationError("CutText", fmt.Sprintf("character '%c' is not valid Latin-1", char), nil)
		}

		if err := binary.Write(&buf, binary.BigEndian, uint8(char)); err != nil {
			return networkError("CutText", "failed to write character to buffer", err)
		}
	}

	dataLength := 8 + len(text)
	if err := c.writeWithContext(c.ctx, buf.Bytes()[0:dataLength]); err != nil {
		return networkError("CutText", "failed to send cut text message", err)
	}

	return nil
}

// FramebufferUpdateRequest requests a framebuffer update from the VNC server.
// This method implements the FramebufferUpdateRequest message as defined in RFC 6143 Section 7.5.3,
// asking the server to send pixel data for a specified rectangular region of the desktop.
//
// The server will respond asynchronously with a FramebufferUpdateMessage containing
// the requested pixel data. There is no guarantee about response timing, and the
// server may combine multiple requests or send partial updates.
//
// Parameters:
//   - incremental: If true, only send pixels that have changed since the last update.
//     If false, send all pixels in the specified rectangle regardless of changes.
//   - x, y: The top-left corner coordinates of the requested rectangle (0-based)
//   - width, height: The dimensions of the requested rectangle in pixels
//
// Returns:
//   - error: NetworkError if the request cannot be sent to the server
//
// Example usage:
//
//	// Request full screen update (non-incremental)
//	err := client.FramebufferUpdateRequest(false, 0, 0,
//		client.FrameBufferWidth, client.FrameBufferHeight)
//	if err != nil {
//		log.Printf("Failed to request framebuffer update: %v", err)
//	}
//
//	// Request incremental update for a specific region
//	err = client.FramebufferUpdateRequest(true, 100, 100, 200, 150)
//	if err != nil {
//		log.Printf("Failed to request incremental update: %v", err)
//	}
//
// Update strategies:
//
//	// Initial full screen capture
//	client.FramebufferUpdateRequest(false, 0, 0, width, height)
//
//	// Continuous incremental updates for live viewing
//	ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
//	go func() {
//		for range ticker.C {
//			client.FramebufferUpdateRequest(true, 0, 0, width, height)
//		}
//	}()
//
// Performance considerations:
// - Incremental updates are more bandwidth-efficient for live viewing
// - Non-incremental updates ensure complete accuracy but use more bandwidth
// - Request frequency should balance responsiveness with network/CPU usage
// - Large rectangles may be split by the server into multiple smaller updates.
func (c *ClientConn) FramebufferUpdateRequest(incremental bool, x, y, width, height uint16) error {
	c.logger.Debug("Sending framebuffer update request",
		Field{Key: "incremental", Value: incremental},
		Field{Key: "x", Value: x},
		Field{Key: "y", Value: y},
		Field{Key: "width", Value: width},
		Field{Key: "height", Value: height})

	var buf bytes.Buffer
	var incrementalByte uint8 = 0

	if incremental {
		incrementalByte = 1
	}

	data := []interface{}{
		uint8(3),
		incrementalByte,
		x, y, width, height,
	}

	for _, val := range data {
		if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
			c.logger.Error("Failed to write framebuffer request data to buffer", Field{Key: "error", Value: err})
			return networkError("FramebufferUpdateRequest", "failed to write request data to buffer", err)
		}
	}

	if err := c.writeWithContext(c.ctx, buf.Bytes()[0:10]); err != nil {
		c.logger.Error("Failed to send framebuffer update request", Field{Key: "error", Value: err})
		return networkError("FramebufferUpdateRequest", "failed to send framebuffer update request", err)
	}

	return nil
}

// KeyEvent sends a keyboard key press or release event to the VNC server.
// This method implements the KeyEvent message as defined in RFC 6143 Section 7.5.4,
// allowing the client to send keyboard input to the remote desktop.
//
// Keys are identified using X Window System keysym values, which provide a
// standardized way to represent keyboard keys across different platforms and
// keyboard layouts. To simulate a complete key press, you must send both a
// key down event (down=true) followed by a key up event (down=false).
//
// Parameters:
//   - keysym: The X11 keysym value identifying the key (see X11/keysymdef.h)
//   - down: true for key press, false for key release
//
// Returns:
//   - error: NetworkError if the event cannot be sent to the server
//
// Example usage:
//
//	// Send the letter 'A' (complete key press and release)
//	const XK_A = 0x0041
//	client.KeyEvent(XK_A, true)  // Key down
//	client.KeyEvent(XK_A, false) // Key up
//
//	// Send Enter key
//	const XK_Return = 0xff0d
//	client.KeyEvent(XK_Return, true)
//	client.KeyEvent(XK_Return, false)
//
//	// Send Ctrl+C (hold Ctrl, press C, release C, release Ctrl)
//	const XK_Control_L = 0xffe3
//	const XK_c = 0x0063
//	client.KeyEvent(XK_Control_L, true)  // Ctrl down
//	client.KeyEvent(XK_c, true)          // C down
//	client.KeyEvent(XK_c, false)         // C up
//	client.KeyEvent(XK_Control_L, false) // Ctrl up
//
// Common keysym values:
//
//	// Letters (uppercase when Shift is held)
//	XK_a = 0x0061, XK_b = 0x0062, ..., XK_z = 0x007a
//	XK_A = 0x0041, XK_B = 0x0042, ..., XK_Z = 0x005a
//
//	// Numbers
//	XK_0 = 0x0030, XK_1 = 0x0031, ..., XK_9 = 0x0039
//
//	// Special keys
//	XK_Return = 0xff0d     // Enter
//	XK_Escape = 0xff1b     // Escape
//	XK_BackSpace = 0xff08  // Backspace
//	XK_Tab = 0xff09        // Tab
//	XK_space = 0x0020      // Space
//
//	// Modifier keys
//	XK_Shift_L = 0xffe1    // Left Shift
//	XK_Control_L = 0xffe3  // Left Ctrl
//	XK_Alt_L = 0xffe9      // Left Alt
//
// Key sequence helper:
//
//	func (c *ClientConn) SendKey(keysym uint32) error {
//		if err := c.KeyEvent(keysym, true); err != nil {
//			return err
//		}
//		return c.KeyEvent(keysym, false)
//	}
//
// For a complete reference of keysym values, consult the X11 keysym definitions
// or online keysym references. The values are standardized and consistent across
// VNC implementations.
func (c *ClientConn) KeyEvent(keysym uint32, down bool) error {
	// Validate keysym for security
	validator := newInputValidator()
	if err := validator.ValidateKeySymbol(keysym); err != nil {
		c.logger.Error("Invalid keysym value",
			Field{Key: "keysym", Value: keysym},
			Field{Key: "error", Value: err})
		return validationError("KeyEvent", "invalid keysym value", err)
	}

	c.logger.Debug("Sending key event",
		Field{Key: "keysym", Value: keysym},
		Field{Key: "down", Value: down})

	var downFlag uint8 = 0
	if down {
		downFlag = 1
	}

	data := []interface{}{
		uint8(4),
		downFlag,
		uint8(0),
		uint8(0),
		keysym,
	}

	var buf bytes.Buffer
	for _, val := range data {
		if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
			c.logger.Error("Failed to write key event data to buffer", Field{Key: "error", Value: err})
			return networkError("KeyEvent", "failed to write key event data to buffer", err)
		}
	}

	if err := c.writeWithContext(c.ctx, buf.Bytes()); err != nil {
		c.logger.Error("Failed to send key event", Field{Key: "error", Value: err})
		return networkError("KeyEvent", "failed to send key event", err)
	}

	return nil
}

// PointerEvent sends mouse movement and button state to the VNC server.
// This method implements the PointerEvent message as defined in RFC 6143 Section 7.5.5,
// allowing the client to send mouse input including movement, clicks, and scroll events
// to the remote desktop.
//
// The button mask represents the current state of all mouse buttons simultaneously.
// When a bit is set (1), the corresponding button is pressed; when clear (0), the
// button is released. This allows for complex interactions like drag operations
// where multiple buttons may be held simultaneously.
//
// Parameters:
//   - mask: Bitmask indicating which buttons are currently pressed (see ButtonMask constants)
//   - x, y: Mouse cursor coordinates in pixels (0-based, relative to framebuffer)
//
// Returns:
//   - error: NetworkError if the event cannot be sent to the server
//
// Example usage:
//
//	// Simple mouse movement (no buttons pressed)
//	err := client.PointerEvent(0, 100, 200)
//
//	// Left mouse button click at coordinates (150, 300)
//	client.PointerEvent(ButtonLeft, 150, 300)      // Button down
//	client.PointerEvent(0, 150, 300)               // Button up
//
//	// Right mouse button click
//	client.PointerEvent(ButtonRight, 200, 100)     // Right button down
//	client.PointerEvent(0, 200, 100)               // Button up
//
//	// Drag operation (left button held while moving)
//	client.PointerEvent(ButtonLeft, 100, 100)      // Start drag
//	client.PointerEvent(ButtonLeft, 120, 120)      // Drag to new position
//	client.PointerEvent(ButtonLeft, 140, 140)      // Continue dragging
//	client.PointerEvent(0, 140, 140)               // End drag (release button)
//
// Scroll wheel events:
//
//	// Scroll up (wheel away from user)
//	client.PointerEvent(Button4, x, y)
//	client.PointerEvent(0, x, y)
//
//	// Scroll down (wheel toward user)
//	client.PointerEvent(Button5, x, y)
//	client.PointerEvent(0, x, y)
//
// Multiple buttons simultaneously:
//
//	// Left and right buttons pressed together
//	mask := ButtonLeft | ButtonRight
//	client.PointerEvent(mask, x, y)
//	client.PointerEvent(0, x, y) // Release both buttons
//
// Helper functions for common operations:
//
//	func (c *ClientConn) MouseMove(x, y uint16) error {
//		return c.PointerEvent(0, x, y)
//	}
//
//	func (c *ClientConn) LeftClick(x, y uint16) error {
//		if err := c.PointerEvent(ButtonLeft, x, y); err != nil {
//			return err
//		}
//		return c.PointerEvent(0, x, y)
//	}
//
//	func (c *ClientConn) ScrollUp(x, y uint16) error {
//		if err := c.PointerEvent(Button4, x, y); err != nil {
//			return err
//		}
//		return c.PointerEvent(0, x, y)
//	}
//
// Coordinate system:
// Mouse coordinates are relative to the framebuffer origin (0,0) at the top-left corner.
// Valid coordinates range from (0,0) to (FrameBufferWidth-1, FrameBufferHeight-1).
// Coordinates outside this range may be clamped or ignored by the server.
func (c *ClientConn) PointerEvent(mask ButtonMask, x, y uint16) error {
	// Validate pointer coordinates for security
	validator := newInputValidator()
	width, height := c.GetFrameBufferSize()
	if err := validator.ValidatePointerPosition(x, y, width, height); err != nil {
		c.logger.Error("Invalid pointer coordinates",
			Field{Key: "x", Value: x},
			Field{Key: "y", Value: y},
			Field{Key: "framebuffer_width", Value: c.FrameBufferWidth},
			Field{Key: "framebuffer_height", Value: c.FrameBufferHeight},
			Field{Key: "error", Value: err})
		return validationError("PointerEvent", "invalid pointer coordinates", err)
	}

	c.logger.Debug("Sending pointer event",
		Field{Key: "mask", Value: mask},
		Field{Key: "x", Value: x},
		Field{Key: "y", Value: y})

	var buf bytes.Buffer

	data := []interface{}{
		uint8(5),
		uint8(mask),
		x,
		y,
	}

	for _, val := range data {
		if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
			c.logger.Error("Failed to write pointer event data to buffer", Field{Key: "error", Value: err})
			return networkError("PointerEvent", "failed to write pointer event data to buffer", err)
		}
	}

	if err := c.writeWithContext(c.ctx, buf.Bytes()[0:6]); err != nil {
		c.logger.Error("Failed to send pointer event", Field{Key: "error", Value: err})
		return networkError("PointerEvent", "failed to send pointer event", err)
	}

	return nil
}

// SetEncodings configures which encoding types the client supports for framebuffer updates.
// This method implements the SetEncodings message as defined in RFC 6143 Section 7.5.2,
// informing the server about the client's encoding capabilities and preferences.
//
// The server will use this information to select appropriate encodings when sending
// framebuffer updates, potentially choosing different encodings for different rectangles
// based on content characteristics and bandwidth considerations.
//
// The encodings are specified in preference order - the server will prefer encodings
// that appear earlier in the slice when multiple options are suitable. The Raw encoding
// is always supported as a fallback and does not need to be explicitly included.
//
// Parameters:
//   - encs: Slice of supported encodings in preference order (most preferred first)
//
// Returns:
//   - error: NetworkError if the encoding list cannot be sent to the server
//
// Example usage:
//
//	// Basic encoding support (Raw is always supported)
//	encodings := []Encoding{
//		&RawEncoding{},
//	}
//	err := client.SetEncodings(encodings)
//
//	// Multiple encodings in preference order
//	encodings := []Encoding{
//		&HextileEncoding{},    // Preferred for mixed content
//		&CopyRectEncoding{},   // Efficient for window movement
//		&RREEncoding{},        // Good for simple graphics
//		&RawEncoding{},        // Fallback for complex content
//	}
//	err := client.SetEncodings(encodings)
//
// Encoding selection strategy:
//
//	// Optimize for bandwidth (slower connections)
//	encodings := []Encoding{
//		&ZRLEEncoding{},       // Best compression
//		&HextileEncoding{},    // Good compression
//		&RREEncoding{},        // Moderate compression
//		&RawEncoding{},        // No compression (fallback)
//	}
//
//	// Optimize for speed (fast connections)
//	encodings := []Encoding{
//		&CopyRectEncoding{},   // Fastest for window operations
//		&RawEncoding{},        // Fast for complex content
//		&HextileEncoding{},    // Moderate speed/compression balance
//	}
//
// Pseudo-encodings for additional features:
//
//	encodings := []Encoding{
//		&HextileEncoding{},
//		&RawEncoding{},
//		&CursorPseudoEncoding{},     // Client-side cursor rendering
//		&DesktopSizePseudoEncoding{}, // Dynamic desktop resizing
//	}
//
// Important considerations:
// - The provided slice should not be modified after calling this method
// - Raw encoding support is mandatory and always available as fallback
// - Pseudo-encodings provide additional features beyond pixel data
// - Encoding preferences affect bandwidth usage and rendering performance
// - Some servers may not support all encoding types
//
// The method updates the connection's Encs field to reflect the configured encodings,
// which can be inspected to verify the current encoding configuration.
func (c *ClientConn) SetEncodings(encs []Encoding) error {
	// Initialize input validator for security
	validator := newInputValidator()

	// Validate encoding count to prevent excessive memory usage
	const maxEncodings = 100
	if len(encs) > maxEncodings {
		c.logger.Error("Too many encodings specified",
			Field{Key: "count", Value: len(encs)},
			Field{Key: "max", Value: maxEncodings})
		return validationError("SetEncodings", fmt.Sprintf("too many encodings: %d (max %d)", len(encs), maxEncodings), nil)
	}

	encodingTypes := make([]int32, len(encs))
	for i, enc := range encs {
		encodingType := enc.Type()

		// Validate each encoding type for security
		if err := validator.ValidateEncodingType(encodingType); err != nil {
			c.logger.Error("Invalid encoding type",
				Field{Key: "index", Value: i},
				Field{Key: "type", Value: encodingType},
				Field{Key: "error", Value: err})
			return validationError("SetEncodings", fmt.Sprintf("invalid encoding type at index %d", i), err)
		}

		encodingTypes[i] = encodingType
	}

	c.logger.Info("Setting supported encodings",
		Field{Key: "count", Value: len(encs)},
		Field{Key: "types", Value: encodingTypes})

	data := make([]interface{}, 3+len(encs))
	data[0] = uint8(2)
	data[1] = uint8(0)
	data[2] = uint16(len(encs)) // #nosec G115 - len(encs) was already validated to be <= maxEncodings (100)

	for i, enc := range encs {
		data[3+i] = enc.Type()
	}

	var buf bytes.Buffer
	for _, val := range data {
		if err := binary.Write(&buf, binary.BigEndian, val); err != nil {
			c.logger.Error("Failed to write encoding data to buffer", Field{Key: "error", Value: err})
			return networkError("SetEncodings", "failed to write encoding data to buffer", err)
		}
	}

	dataLength := 4 + (4 * len(encs))
	if err := c.writeWithContext(c.ctx, buf.Bytes()[0:dataLength]); err != nil {
		c.logger.Error("Failed to send set encodings message", Field{Key: "error", Value: err})
		return networkError("SetEncodings", "failed to send set encodings message", err)
	}

	c.Encs = encs

	return nil
}

// SetPixelFormat configures the pixel format used for framebuffer updates from the server.
// This method implements the SetPixelFormat message as defined in RFC 6143 Section 7.5.1,
// allowing the client to specify how pixel color data should be encoded in subsequent
// framebuffer updates.
//
// Changing the pixel format affects all future framebuffer updates and can be used to
// optimize for different display characteristics, color depths, or bandwidth requirements.
// The server will convert its internal pixel representation to match the requested format.
//
// When the pixel format is changed to indexed color mode (TrueColor=false), the
// connection's color map is automatically reset, and the server may send
// SetColorMapEntries messages to populate the new color map.
//
// Parameters:
//   - format: The desired pixel format specification
//
// Returns:
//   - error: EncodingError if the format cannot be encoded, NetworkError for transmission issues
//
// Example usage:
//
//	// 32-bit true color RGBA (high quality, more bandwidth)
//	format := &PixelFormat{
//		BPP: 32, Depth: 24, BigEndian: false, TrueColor: true,
//		RedMax: 255, GreenMax: 255, BlueMax: 255,
//		RedShift: 16, GreenShift: 8, BlueShift: 0,
//	}
//	err := client.SetPixelFormat(format)
//
//	// 16-bit true color RGB565 (balanced quality/bandwidth)
//	format := &PixelFormat{
//		BPP: 16, Depth: 16, BigEndian: false, TrueColor: true,
//		RedMax: 31, GreenMax: 63, BlueMax: 31,
//		RedShift: 11, GreenShift: 5, BlueShift: 0,
//	}
//	err := client.SetPixelFormat(format)
//
//	// 8-bit indexed color (low bandwidth, limited colors)
//	format := &PixelFormat{
//		BPP: 8, Depth: 8, BigEndian: false, TrueColor: false,
//	}
//	err := client.SetPixelFormat(format)
//
// Bandwidth optimization:
//
//	// For slow connections - use 8-bit indexed color
//	lowBandwidthFormat := &PixelFormat{
//		BPP: 8, Depth: 8, TrueColor: false,
//	}
//
//	// For fast connections - use 32-bit true color
//	highQualityFormat := &PixelFormat{
//		BPP: 32, Depth: 24, TrueColor: true,
//		RedMax: 255, GreenMax: 255, BlueMax: 255,
//		RedShift: 16, GreenShift: 8, BlueShift: 0,
//	}
//
// Color depth considerations:
// - 32-bit: Best quality, highest bandwidth usage
// - 16-bit: Good quality, moderate bandwidth usage
// - 8-bit: Limited colors (256), lowest bandwidth usage
// - True color: Direct RGB values, more colors available
// - Indexed color: Uses color map, limited to 256 simultaneous colors
//
// Performance impact:
// - Higher bit depths provide better color accuracy but use more bandwidth
// - Indexed color modes require color map synchronization
// - Format changes may cause temporary visual artifacts during transition
// - Some servers may perform better with specific pixel formats
//
// The method automatically resets the color map when switching to indexed color mode,
// as the previous color map may not be compatible with the new pixel format.
func (c *ClientConn) SetPixelFormat(format *PixelFormat) error {
	// Initialize input validator for security
	validator := newInputValidator()

	// Validate pixel format before sending to server
	if err := validator.ValidatePixelFormat(format); err != nil {
		c.logger.Error("Invalid pixel format specified",
			Field{Key: "pixel_format", Value: format},
			Field{Key: "error", Value: err})
		return validationError("SetPixelFormat", "invalid pixel format", err)
	}

	c.logger.Info("Setting pixel format",
		Field{Key: "bpp", Value: format.BPP},
		Field{Key: "depth", Value: format.Depth},
		Field{Key: "true_color", Value: format.TrueColor})

	var keyEvent [20]byte
	keyEvent[0] = 0

	pfBytes, err := writePixelFormat(format)
	if err != nil {
		return encodingError("SetPixelFormat", "failed to encode pixel format", err)
	}

	// Copy the pixel format bytes into the proper slice location
	copy(keyEvent[4:], pfBytes)

	// Send the data down the connection
	if err := c.writeWithContext(c.ctx, keyEvent[:]); err != nil {
		return networkError("SetPixelFormat", "failed to send pixel format message", err)
	}

	// Reset the color map as according to RFC.
	var newColorMap [256]Color
	c.ColorMap = newColorMap

	return nil
}

const pvLen = 12

// parseProtocolVersion parses a VNC protocol version string.
func parseProtocolVersion(pv []byte) (uint, uint, error) {
	var major, minor uint

	if len(pv) < pvLen {
		return 0, 0, protocolError("parseProtocolVersion",
			fmt.Sprintf("protocol version message too short (%v < %v)", len(pv), pvLen), nil)
	}

	l, err := fmt.Sscanf(string(pv), "RFB %d.%d\n", &major, &minor)
	if l != 2 {
		return 0, 0, protocolError("parseProtocolVersion", "invalid protocol version format", nil)
	}
	if err != nil {
		return 0, 0, protocolError("parseProtocolVersion", "failed to parse protocol version", err)
	}

	return major, minor, nil
}

// handshakeWithContext performs the VNC handshake with context support for cancellation.
// This method handles protocol version negotiation, security handshake, authentication,
// and initialization while respecting context cancellation and timeouts.
func (c *ClientConn) handshakeWithContext(ctx context.Context) error {
	c.logger.Info("Starting VNC handshake")

	// Initialize input validator for security enhancements
	validator := newInputValidator()

	var protocolVersion [pvLen]byte

	// 7.1.1, read the ProtocolVersion message sent by the server.
	if err := c.readWithContext(ctx, protocolVersion[:]); err != nil {
		c.logger.Error("Failed to read protocol version from server", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to read protocol version from server", err)
	}

	// Validate protocol version format for security
	if err := validator.ValidateProtocolVersion(string(protocolVersion[:])); err != nil {
		c.logger.Error("Invalid protocol version format received from server",
			Field{Key: "version", Value: string(protocolVersion[:])},
			Field{Key: "error", Value: err})
		return protocolError("handshake", "server sent invalid protocol version format", err)
	}

	maxMajor, maxMinor, err := parseProtocolVersion(protocolVersion[:])
	if err != nil {
		c.logger.Error("Failed to parse protocol version", Field{Key: "error", Value: err})
		return err
	}

	c.logger.Info("Received protocol version",
		Field{Key: "major", Value: maxMajor},
		Field{Key: "minor", Value: maxMinor})

	if maxMajor < 3 {
		c.logger.Error("Unsupported major version", Field{Key: "version", Value: maxMajor})
		return unsupportedError("handshake", fmt.Sprintf("unsupported major version, less than 3: %d", maxMajor), nil)
	}
	if maxMinor < 8 {
		c.logger.Error("Unsupported minor version", Field{Key: "version", Value: maxMinor})
		return unsupportedError("handshake", fmt.Sprintf("unsupported minor version, less than 8: %d", maxMinor), nil)
	}

	// Respond with the version we will support
	c.logger.Debug("Sending protocol version response: RFB 003.008")
	if err = c.writeWithContext(ctx, []byte("RFB 003.008\n")); err != nil {
		c.logger.Error("Failed to send protocol version response", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to send protocol version response", err)
	}

	// 7.1.2 Security Handshake from server
	c.logger.Debug("Reading security types from server")
	var numSecurityTypes uint8
	if err = c.readBinaryWithContext(ctx, &numSecurityTypes); err != nil {
		c.logger.Error("Failed to read number of security types", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to read number of security types", err)
	}

	if numSecurityTypes == 0 {
		reason := c.readErrorReason()
		c.logger.Error("No security types available", Field{Key: "reason", Value: reason})
		return authenticationError("handshake", fmt.Sprintf("no security types available: %s", reason), nil)
	}

	// numSecurityTypes is uint8, so it's already bounded to 0-255

	securityTypes := make([]uint8, numSecurityTypes)
	if err = c.readBinaryWithContext(ctx, &securityTypes); err != nil {
		c.logger.Error("Failed to read security types", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to read security types", err)
	}

	// Validate security types for security
	if err := validator.ValidateSecurityTypes(securityTypes); err != nil {
		c.logger.Error("Invalid security types received from server",
			Field{Key: "types", Value: securityTypes},
			Field{Key: "error", Value: err})
		return protocolError("handshake", "server sent invalid security types", err)
	}

	c.logger.Info("Received security types from server",
		Field{Key: "count", Value: numSecurityTypes},
		Field{Key: "types", Value: securityTypes})

	// Use AuthRegistry for authentication negotiation if available
	var auth ClientAuth
	var selectedSecurityType uint8

	if c.config.AuthRegistry != nil {
		// Use the authentication registry for advanced negotiation
		c.logger.Debug("Using authentication registry for negotiation")

		// Create preferred order from Auth slice if provided
		var preferredOrder []uint8
		if c.config.Auth != nil {
			preferredOrder = make([]uint8, len(c.config.Auth))
			for i, authMethod := range c.config.Auth {
				preferredOrder[i] = authMethod.SecurityType()
			}
		}

		var err error
		auth, selectedSecurityType, err = c.config.AuthRegistry.NegotiateAuth(ctx, securityTypes, preferredOrder)
		if err != nil {
			c.logger.Error("Authentication registry negotiation failed",
				Field{Key: "server_types", Value: securityTypes},
				Field{Key: "error", Value: err})
			return authenticationError("handshake", "authentication negotiation failed", err)
		}
	} else {
		// Fall back to legacy authentication method selection
		c.logger.Debug("Using legacy authentication method selection")

		clientSecurityTypes := c.config.Auth
		if clientSecurityTypes == nil {
			clientSecurityTypes = []ClientAuth{new(ClientAuthNone)}
		}

	FindAuth:
		for _, curAuth := range clientSecurityTypes {
			for _, securityType := range securityTypes {
				if curAuth.SecurityType() == securityType {
					// We use the first matching supported authentication
					auth = curAuth
					selectedSecurityType = securityType
					break FindAuth
				}
			}
		}

		if auth == nil {
			c.logger.Error("No suitable authentication method found",
				Field{Key: "server_types", Value: securityTypes})
			return authenticationError("handshake", fmt.Sprintf("no suitable auth schemes found. server supported: %#v", securityTypes), nil)
		}
	}

	c.logger.Info("Selected authentication method",
		Field{Key: "type", Value: selectedSecurityType},
		Field{Key: "method", Value: auth.String()})

	// Respond back with the security type we'll use
	if err = c.writeBinaryWithContext(ctx, selectedSecurityType); err != nil {
		c.logger.Error("Failed to send selected security type", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to send selected security type", err)
	}

	// Validate the authentication method before using it
	if c.config.AuthRegistry != nil {
		if err = c.config.AuthRegistry.ValidateAuthMethod(auth); err != nil {
			c.logger.Error("Authentication method validation failed",
				Field{Key: "type", Value: selectedSecurityType},
				Field{Key: "method", Value: auth.String()},
				Field{Key: "error", Value: err})
			return authenticationError("handshake", "authentication method validation failed", err)
		}
	}

	c.logger.Debug("Starting authentication handshake")

	// Set logger for authentication method if it supports it
	if authWithLogger, ok := auth.(interface{ SetLogger(Logger) }); ok {
		authWithLogger.SetLogger(c.logger)
	}

	if err = auth.Handshake(ctx, c.c); err != nil {
		c.logger.Error("Authentication handshake failed",
			Field{Key: "type", Value: selectedSecurityType},
			Field{Key: "method", Value: auth.String()},
			Field{Key: "error", Value: err})
		return authenticationError("handshake", "authentication handshake failed", err)
	}

	// 7.1.3 SecurityResult Handshake
	c.logger.Debug("Reading security result")
	var securityResult uint32
	if err = c.readBinaryWithContext(ctx, &securityResult); err != nil {
		c.logger.Error("Failed to read security result", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to read security result", err)
	}

	if securityResult == 1 {
		reason := c.readErrorReason()
		c.logger.Error("Authentication failed", Field{Key: "reason", Value: reason})
		return authenticationError("handshake", fmt.Sprintf("security handshake failed: %s", reason), nil)
	}

	c.logger.Info("Authentication successful")

	// 7.3.1 ClientInit
	var sharedFlag uint8 = 1
	if c.config.Exclusive {
		sharedFlag = 0
	}

	c.logger.Debug("Sending client init message",
		Field{Key: "shared", Value: sharedFlag == 1})
	if err = c.writeBinaryWithContext(ctx, sharedFlag); err != nil {
		c.logger.Error("Failed to send client init message", Field{Key: "error", Value: err})
		return networkError("handshake", "failed to send client init message", err)
	}

	// 7.3.2 ServerInit
	var width, height uint16
	if err = c.readBinaryWithContext(ctx, &width); err != nil {
		return networkError("handshake", "failed to read framebuffer width", err)
	}

	if err = c.readBinaryWithContext(ctx, &height); err != nil {
		return networkError("handshake", "failed to read framebuffer height", err)
	}

	// Validate framebuffer dimensions for security
	if err := validator.ValidateFramebufferDimensions(width, height); err != nil {
		c.logger.Error("Invalid framebuffer dimensions received from server",
			Field{Key: "width", Value: width},
			Field{Key: "height", Value: height},
			Field{Key: "error", Value: err})
		return protocolError("handshake", "server sent invalid framebuffer dimensions", err)
	}

	// Read the pixel format
	var pixelFormat PixelFormat
	if err = c.readPixelFormatWithContext(ctx, &pixelFormat); err != nil {
		return protocolError("handshake", "failed to read pixel format", err)
	}

	// Update connection state with mutex protection
	c.mu.Lock()
	c.FrameBufferWidth = width
	c.FrameBufferHeight = height
	c.PixelFormat = pixelFormat
	c.mu.Unlock()

	// Validate pixel format for security
	if err := validator.ValidatePixelFormat(&c.PixelFormat); err != nil {
		c.logger.Error("Invalid pixel format received from server",
			Field{Key: "pixel_format", Value: c.PixelFormat},
			Field{Key: "error", Value: err})
		return protocolError("handshake", "server sent invalid pixel format", err)
	}

	var nameLength uint32
	if err = c.readBinaryWithContext(ctx, &nameLength); err != nil {
		return networkError("handshake", "failed to read desktop name length", err)
	}

	// Validate desktop name length to prevent buffer overflow
	const maxDesktopNameLength = 1024 * 1024
	if err := validator.ValidateMessageLength(nameLength, maxDesktopNameLength); err != nil {
		c.logger.Error("Invalid desktop name length received from server",
			Field{Key: "length", Value: nameLength},
			Field{Key: "error", Value: err})
		return protocolError("handshake", "server sent invalid desktop name length", err)
	}

	nameBytes := make([]uint8, nameLength)
	if err = c.readBinaryWithContext(ctx, &nameBytes); err != nil {
		return networkError("handshake", "failed to read desktop name", err)
	}

	// Validate and sanitize desktop name
	desktopNameStr := string(nameBytes)
	if err := validator.ValidateTextData(desktopNameStr, int(maxDesktopNameLength)); err != nil {
		c.logger.Warn("Invalid desktop name received from server, sanitizing",
			Field{Key: "original_name", Value: desktopNameStr},
			Field{Key: "error", Value: err})
		desktopNameStr = validator.SanitizeText(desktopNameStr)
	}

	// Update desktop name with mutex protection
	c.mu.Lock()
	c.DesktopName = desktopNameStr
	c.mu.Unlock()

	// Get current values for logging (thread-safe)
	logWidth, logHeight := c.GetFrameBufferSize()
	logDesktopName := c.GetDesktopName()
	logPixelFormat := c.GetPixelFormat()

	c.logger.Info("VNC handshake completed successfully",
		Field{Key: "desktop_name", Value: logDesktopName},
		Field{Key: "framebuffer_width", Value: logWidth},
		Field{Key: "framebuffer_height", Value: logHeight},
		Field{Key: "pixel_format_bpp", Value: logPixelFormat.BPP})

	return nil
}

// mainLoop reads messages sent from the server and routes them to the
// proper channels for users of the client to read.
func (c *ClientConn) mainLoop() {
	defer c.Close()

	c.logger.Info("Starting message processing loop")

	// Build the map of available server messages
	typeMap := make(map[uint8]ServerMessage)

	defaultMessages := []ServerMessage{
		new(FramebufferUpdateMessage),
		new(SetColorMapEntriesMessage),
		new(BellMessage),
		new(ServerCutTextMessage),
	}

	for _, msg := range defaultMessages {
		typeMap[msg.Type()] = msg
	}

	if c.config.ServerMessages != nil {
		for _, msg := range c.config.ServerMessages {
			typeMap[msg.Type()] = msg
		}
	}

	for {
		// Check if context is cancelled before reading
		select {
		case <-c.ctx.Done():
			c.logger.Info("Message processing loop cancelled by context")
			return
		default:
		}

		var messageType uint8
		if err := c.readBinaryWithContext(c.ctx, &messageType); err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				c.logger.Info("Message processing loop cancelled", Field{Key: "error", Value: err})
			} else {
				c.logger.Debug("Connection closed or error reading message type", Field{Key: "error", Value: err})
			}
			break
		}

		c.logger.Debug("Received server message", Field{Key: "type", Value: messageType})

		msg, ok := typeMap[messageType]
		if !ok {
			c.logger.Error("Unsupported message type received", Field{Key: "type", Value: messageType})
			break
		}

		parsedMsg, err := msg.Read(c, c.c)
		if err != nil {
			c.logger.Error("Failed to parse server message",
				Field{Key: "type", Value: messageType},
				Field{Key: "error", Value: err})
			break
		}

		c.logger.Debug("Successfully parsed server message",
			Field{Key: "type", Value: messageType},
			Field{Key: "message_type", Value: fmt.Sprintf("%T", parsedMsg)})

		if c.config.ServerMessageCh == nil {
			c.logger.Debug("No server message channel configured, discarding message")
			continue
		}

		// Try to send message to channel with context cancellation support
		select {
		case c.config.ServerMessageCh <- parsedMsg:
			// Message sent successfully
		case <-c.ctx.Done():
			c.logger.Info("Message processing loop cancelled while sending message")
			return
		}
	}

	c.logger.Info("Message processing loop ended")
}

// readErrorReason reads an error reason string from the server.
func (c *ClientConn) readErrorReason() string {
	// Initialize input validator for security
	validator := newInputValidator()

	var reasonLen uint32
	if err := binary.Read(c.c, binary.BigEndian, &reasonLen); err != nil {
		return "<failed to read error reason length>"
	}

	// Validate error reason length to prevent buffer overflow
	const maxErrorReasonLength = 64 * 1024
	if err := validator.ValidateMessageLength(reasonLen, maxErrorReasonLength); err != nil {
		c.logger.Warn("Invalid error reason length received from server",
			Field{Key: "length", Value: reasonLen},
			Field{Key: "error", Value: err})
		return "<invalid error reason length>"
	}

	reason := make([]uint8, reasonLen)
	if err := binary.Read(c.c, binary.BigEndian, &reason); err != nil {
		return "<failed to read error reason>"
	}

	// Validate and sanitize error reason text
	reasonText := string(reason)
	if err := validator.ValidateTextData(reasonText, int(maxErrorReasonLength)); err != nil {
		c.logger.Warn("Invalid error reason text received from server, sanitizing",
			Field{Key: "original_text", Value: reasonText},
			Field{Key: "error", Value: err})
		reasonText = validator.SanitizeText(reasonText)
	}

	return reasonText
}

// Context-aware network operation helpers

// readWithContext reads data from the connection with context cancellation support.
func (c *ClientConn) readWithContext(ctx context.Context, buf []byte) error {
	done := make(chan error, 1)

	go func() {
		_, err := io.ReadFull(c.c, buf)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// writeWithContext writes data to the connection with context cancellation support.
func (c *ClientConn) writeWithContext(ctx context.Context, data []byte) error {
	done := make(chan error, 1)

	go func() {
		_, err := c.c.Write(data)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// readBinaryWithContext reads binary data with context cancellation support.
func (c *ClientConn) readBinaryWithContext(ctx context.Context, data interface{}) error {
	done := make(chan error, 1)

	go func() {
		done <- binary.Read(c.c, binary.BigEndian, data)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// writeBinaryWithContext writes binary data with context cancellation support.
func (c *ClientConn) writeBinaryWithContext(ctx context.Context, data interface{}) error {
	done := make(chan error, 1)

	go func() {
		done <- binary.Write(c.c, binary.BigEndian, data)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// readPixelFormatWithContext reads pixel format data with context cancellation support.
func (c *ClientConn) readPixelFormatWithContext(ctx context.Context, pf *PixelFormat) error {
	done := make(chan error, 1)

	go func() {
		done <- readPixelFormat(c.c, pf)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetFrameBufferSize returns the current framebuffer dimensions in a thread-safe manner.
func (c *ClientConn) GetFrameBufferSize() (width, height uint16) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.FrameBufferWidth, c.FrameBufferHeight
}

// GetDesktopName returns the desktop name in a thread-safe manner.
func (c *ClientConn) GetDesktopName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DesktopName
}

// GetPixelFormat returns a copy of the current pixel format in a thread-safe manner.
func (c *ClientConn) GetPixelFormat() PixelFormat {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PixelFormat
}
