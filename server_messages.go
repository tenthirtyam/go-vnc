// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ServerMessage defines the interface for messages sent from a VNC server to the client.
type ServerMessage interface {
	Type() uint8
	Read(conn *ClientConn, r io.Reader) (ServerMessage, error)
}

// FramebufferUpdateMessage represents a framebuffer update from the server (message type 0).
type FramebufferUpdateMessage struct {
	// Rectangles contains the list of screen rectangles being updated.
	Rectangles []Rectangle
}

// Rectangle represents a rectangular region of the framebuffer with associated pixel data.
// Rectangles are the fundamental unit of framebuffer updates in the VNC protocol,
// defining both the screen coordinates and the encoded pixel data for a specific area.
//
// Each rectangle specifies:
// - The position and dimensions of the screen area being updated
// - The encoding method used for the pixel data
// - The actual encoded pixel data (contained within the Encoding)
//
// Rectangles are processed sequentially, and some encoding types (like CopyRect)
// may reference pixel data from previously processed rectangles or other areas
// of the framebuffer.
type Rectangle struct {
	// X is the horizontal position of the rectangle's left edge in pixels,
	// measured from the left edge of the framebuffer (0-based coordinate system).
	X uint16

	// Y is the vertical position of the rectangle's top edge in pixels,
	// measured from the top edge of the framebuffer (0-based coordinate system).
	Y uint16

	// Width is the width of the rectangle in pixels.
	// Must be greater than 0 for valid rectangles.
	Width uint16

	// Height is the height of the rectangle in pixels.
	// Must be greater than 0 for valid rectangles.
	Height uint16

	// Enc contains the encoding implementation and decoded pixel data for this rectangle.
	// The encoding type determines how the pixel data was compressed and transmitted,
	// and the encoding instance contains the actual decoded pixel information.
	Enc Encoding
}

// Type returns the message type identifier for framebuffer update messages.
func (*FramebufferUpdateMessage) Type() uint8 {
	return 0
}

// Read parses a FramebufferUpdate message from the server.
// This method implements the ServerMessage interface and processes framebuffer update
// data as defined in RFC 6143 Section 7.6.1. It reads one or more rectangles of
// pixel data, each potentially using different encoding methods.
//
// The method handles the complete message parsing including:
//  1. Reading the message padding and rectangle count
//  2. For each rectangle: position, dimensions, and encoding type
//  3. Delegating encoding-specific decoding to the appropriate Encoding implementation
//  4. Building a complete FramebufferUpdateMessage with all decoded rectangles
//
// Parameters:
//   - c: The client connection providing encoding support and state information
//   - r: Reader containing the message data (excluding the message type byte)
//
// Returns:
//   - ServerMessage: A new FramebufferUpdateMessage containing all decoded rectangles
//   - error: NetworkError for I/O issues, UnsupportedError for unknown encodings, EncodingError for decoding failures
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	msg := &FramebufferUpdateMessage{}
//	parsedMsg, err := msg.Read(clientConn, messageReader)
//	if err != nil {
//		log.Printf("Failed to parse framebuffer update: %v", err)
//		return
//	}
//
//	// Process the parsed message
//	fbUpdate := parsedMsg.(*FramebufferUpdateMessage)
//	for _, rect := range fbUpdate.Rectangles {
//		fmt.Printf("Rectangle at (%d,%d) size %dx%d using encoding %d\n",
//			rect.X, rect.Y, rect.Width, rect.Height, rect.Enc.Type())
//
//		// Apply rectangle to local framebuffer
//		switch enc := rect.Enc.(type) {
//		case *RawEncoding:
//			// Handle raw pixel data
//			applyRawPixels(rect, enc.Colors)
//		case *CopyRectEncoding:
//			// Handle copy rectangle operation
//			applyCopyRect(rect, enc)
//		}
//	}
//
// Encoding support:
//
//	// The method uses the client's configured encodings plus mandatory Raw encoding
//	// Supported encodings are determined by previous SetEncodings calls:
//	clientConn.SetEncodings([]Encoding{
//		&HextileEncoding{},
//		&CopyRectEncoding{},
//		&RawEncoding{}, // Always supported as fallback
//	})
//
// Message structure:
//
//	// Wire format (after message type byte):
//	// [1 byte]  - Padding (ignored)
//	// [2 bytes] - Number of rectangles (big-endian uint16)
//	// For each rectangle:
//	//   [2 bytes] - X position (big-endian uint16)
//	//   [2 bytes] - Y position (big-endian uint16)
//	//   [2 bytes] - Width (big-endian uint16)
//	//   [2 bytes] - Height (big-endian uint16)
//	//   [4 bytes] - Encoding type (big-endian int32)
//	//   [variable] - Encoding-specific pixel data
//
// Error handling:
// The method may return various error types:
//   - NetworkError: I/O failures reading message data
//   - UnsupportedError: Unknown or unsupported encoding type encountered
//   - EncodingError: Failure decoding rectangle pixel data
//   - All errors include context about which rectangle or operation failed
//
// Performance considerations:
// - Large updates with many rectangles may consume significant memory
// - Different encodings have varying decode performance characteristics
// - Rectangle processing is sequential and may benefit from parallel decoding
// - Memory usage scales with total pixel count across all rectangles.
func (*FramebufferUpdateMessage) Read(c *ClientConn, r io.Reader) (ServerMessage, error) {
	validator := newInputValidator()

	var padding [1]byte
	if _, err := io.ReadFull(r, padding[:]); err != nil {
		return nil, networkError("FramebufferUpdateMessage.Read", "failed to read padding", err)
	}

	var numRects uint16
	if err := binary.Read(r, binary.BigEndian, &numRects); err != nil {
		return nil, networkError("FramebufferUpdateMessage.Read", "failed to read number of rectangles", err)
	}

	if numRects > MaxRectanglesPerUpdate {
		return nil, protocolError("FramebufferUpdateMessage.Read",
			fmt.Sprintf("too many rectangles in update: %d (max %d)", numRects, MaxRectanglesPerUpdate), nil)
	}

	encMap := make(map[int32]Encoding)
	for _, enc := range c.Encs {
		encMap[enc.Type()] = enc
	}

	rawEnc := new(RawEncoding)
	encMap[rawEnc.Type()] = rawEnc

	cursorPseudo := new(CursorPseudoEncoding)
	encMap[cursorPseudo.Type()] = cursorPseudo

	desktopSizePseudo := new(DesktopSizePseudoEncoding)
	encMap[desktopSizePseudo.Type()] = desktopSizePseudo

	rects := make([]Rectangle, numRects)
	for i := uint16(0); i < numRects; i++ {
		var encodingType int32

		rect := &rects[i]
		data := []interface{}{
			&rect.X,
			&rect.Y,
			&rect.Width,
			&rect.Height,
			&encodingType,
		}

		for _, val := range data {
			if err := binary.Read(r, binary.BigEndian, val); err != nil {
				return nil, networkError("FramebufferUpdateMessage.Read", "failed to read rectangle header", err)
			}
		}

		if err := validator.ValidateEncodingType(encodingType); err != nil {
			return nil, protocolError("FramebufferUpdateMessage.Read",
				fmt.Sprintf("invalid encoding type for rectangle %d", i), err)
		}

		isPseudoEncoding := encodingType < 0
		if !isPseudoEncoding {
			if err := validator.ValidateRectangle(rect.X, rect.Y, rect.Width, rect.Height,
				c.FrameBufferWidth, c.FrameBufferHeight); err != nil {
				return nil, protocolError("FramebufferUpdateMessage.Read",
					fmt.Sprintf("invalid rectangle %d", i), err)
			}
		}

		enc, ok := encMap[encodingType]
		if !ok {
			return nil, unsupportedError("FramebufferUpdateMessage.Read", fmt.Sprintf("unsupported encoding type: %d", encodingType), nil)
		}

		var err error
		rect.Enc, err = enc.Read(c, rect, r)
		if err != nil {
			return nil, encodingError("FramebufferUpdateMessage.Read", "failed to read rectangle encoding data", err)
		}

		if pseudoEnc, isPseudo := rect.Enc.(PseudoEncoding); isPseudo {
			if err := pseudoEnc.Handle(c, rect); err != nil {
				c.logger.Error("Failed to handle pseudo-encoding",
					Field{Key: "encoding_type", Value: encodingType},
					Field{Key: "error", Value: err})
			}
		}
	}

	return &FramebufferUpdateMessage{rects}, nil
}

// SetColorMapEntriesMessage represents a color map update from the server (message type 1).
// This message is sent when the server needs to update entries in the client's color map,
// which is used when the pixel format uses indexed color mode rather than true color.
//
// As defined in RFC 6143 Section 7.6.2, this message allows the server to dynamically
// change the color palette used for interpreting pixel values. This is particularly
// important for 8-bit color modes where pixel values are indices into a 256-entry
// color table rather than direct RGB values.
//
// The message automatically updates the connection's ColorMap field when processed,
// ensuring that subsequent framebuffer updates use the correct color interpretations.
// Applications can also access the color change data directly for custom processing.
//
// Example usage:
//
//	switch msg := serverMsg.(type) {
//	case *SetColorMapEntriesMessage:
//		fmt.Printf("Color map updated: %d colors starting at index %d\n",
//			len(msg.Colors), msg.FirstColor)
//		// The connection's ColorMap is automatically updated
//	}
type SetColorMapEntriesMessage struct {
	// FirstColor is the index of the first color map entry being updated.
	// Color map indices range from 0 to 255, and this value indicates
	// where in the color map the new color values should be placed.
	FirstColor uint16

	// Colors contains the new color values to be installed in the color map.
	// These colors will be placed in the color map starting at the FirstColor
	// index. The length of this slice determines how many consecutive color
	// map entries will be updated.
	Colors []Color
}

// Type returns the message type identifier for color map update messages.
func (*SetColorMapEntriesMessage) Type() uint8 {
	return 1
}

// Read parses a SetColorMapEntries message from the server.
// This method implements the ServerMessage interface and processes color map updates
// as defined in RFC 6143 Section 7.6.2. It reads new color values and automatically
// updates the connection's color map for use with indexed color pixel formats.
//
// The method handles the complete message parsing including:
//  1. Reading message padding and the starting color index
//  2. Reading the number of colors being updated
//  3. Reading RGB values for each color entry
//  4. Automatically updating the connection's ColorMap field
//
// Parameters:
//   - c: The client connection whose color map will be updated
//   - r: Reader containing the message data (excluding the message type byte)
//
// Returns:
//   - ServerMessage: A new SetColorMapEntriesMessage containing the color update data
//   - error: NetworkError for I/O issues during message parsing
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	msg := &SetColorMapEntriesMessage{}
//	parsedMsg, err := msg.Read(clientConn, messageReader)
//	if err != nil {
//		log.Printf("Failed to parse color map update: %v", err)
//		return
//	}
//
//	// Process the parsed message
//	colorUpdate := parsedMsg.(*SetColorMapEntriesMessage)
//	fmt.Printf("Updated %d colors starting at index %d\n",
//		len(colorUpdate.Colors), colorUpdate.FirstColor)
//
//	// The connection's ColorMap is automatically updated
//	// Access updated colors if needed:
//	for i, color := range colorUpdate.Colors {
//		index := colorUpdate.FirstColor + uint16(i)
//		fmt.Printf("Color[%d] = RGB(%d, %d, %d)\n",
//			index, color.R, color.G, color.B)
//	}
//
// Color map usage:
//
//	// After this message is processed, indexed pixel values will use the new colors:
//	// For 8-bit indexed pixel format:
//	pixelValue := uint8(42) // Example pixel value from framebuffer update
//	actualColor := clientConn.ColorMap[pixelValue] // Uses updated color map
//
// Message structure:
//
//	// Wire format (after message type byte):
//	// [1 byte]  - Padding (ignored)
//	// [2 bytes] - First color index (big-endian uint16)
//	// [2 bytes] - Number of colors (big-endian uint16)
//	// For each color:
//	//   [2 bytes] - Red component (big-endian uint16)
//	//   [2 bytes] - Green component (big-endian uint16)
//	//   [2 bytes] - Blue component (big-endian uint16)
//
// Automatic color map update:
//
//	// The method automatically updates the connection's color map:
//	for i, color := range newColors {
//		clientConn.ColorMap[firstColor + i] = color
//	}
//
//	// This ensures that subsequent framebuffer updates with indexed pixels
//	// will use the correct color interpretations
//
// Color value format:
// Color components are 16-bit values (0-65535) providing high precision color
// representation. When displaying on 8-bit systems, applications should scale
// appropriately (e.g., divide by 257 to convert to 8-bit values).
//
// Error handling:
// The method may return NetworkError for:
//   - I/O failures reading message padding, indices, or color data
//   - Incomplete color data (fewer bytes than expected)
//   - Network connection issues during message parsing
//
// Performance considerations:
// - Color map updates are typically small (few colors changed at once)
// - The automatic color map update is performed synchronously
// - Large color map changes (updating many colors) are rare but possible
// - Color map updates may trigger visual changes in subsequent framebuffer updates.
func (*SetColorMapEntriesMessage) Read(c *ClientConn, r io.Reader) (ServerMessage, error) {
	validator := newInputValidator()

	var padding [1]byte
	if _, err := io.ReadFull(r, padding[:]); err != nil {
		return nil, networkError("SetColorMapEntriesMessage.Read", "failed to read padding", err)
	}

	var result SetColorMapEntriesMessage
	if err := binary.Read(r, binary.BigEndian, &result.FirstColor); err != nil {
		return nil, networkError("SetColorMapEntriesMessage.Read", "failed to read first color index", err)
	}

	var numColors uint16
	if err := binary.Read(r, binary.BigEndian, &numColors); err != nil {
		return nil, networkError("SetColorMapEntriesMessage.Read", "failed to read number of colors", err)
	}

	if err := validator.ValidateColorMapEntries(result.FirstColor, numColors, ColorMapSize); err != nil {
		return nil, protocolError("SetColorMapEntriesMessage.Read", "invalid color map entries", err)
	}

	result.Colors = make([]Color, numColors)
	for i := uint16(0); i < numColors; i++ {

		color := &result.Colors[i]
		data := []interface{}{
			&color.R,
			&color.G,
			&color.B,
		}

		for _, val := range data {
			if err := binary.Read(r, binary.BigEndian, val); err != nil {
				return nil, networkError("SetColorMapEntriesMessage.Read", "failed to read color data", err)
			}
		}

		c.ColorMap[result.FirstColor+i] = *color
	}

	return &result, nil
}

// BellMessage represents an audible bell notification from the server (message type 2).
// This message indicates that the server wants the client to produce an audible
// alert, typically corresponding to a system bell or notification sound on the
// remote desktop.
//
// As defined in RFC 6143 Section 7.6.3, this is a simple notification message
// with no additional data. The client should respond by producing an appropriate
// audible alert using the local system's notification mechanisms.
//
// The message contains no payload data beyond the message type identifier.
// Applications can handle this message to provide audio feedback, visual
// notifications, or other appropriate user alerts.
//
// Example usage:
//
//	switch msg := serverMsg.(type) {
//	case *BellMessage:
//		// Play system bell sound or show notification
//		fmt.Println("Bell notification received")
//		// Could trigger: system beep, notification popup, etc.
//	}
type BellMessage byte

// Type returns the message type identifier for bell messages.
func (*BellMessage) Type() uint8 {
	return 2
}

// Read processes a bell message from the server.
func (*BellMessage) Read(*ClientConn, io.Reader) (ServerMessage, error) {
	return new(BellMessage), nil
}

// ServerCutTextMessage represents clipboard data from the server (message type 3).
// This message is sent when the server's clipboard (cut buffer) contents change
// and should be synchronized with the client's clipboard.
//
// As defined in RFC 6143 Section 7.6.4, this message enables clipboard sharing
// between the client and server systems. When the server's clipboard is updated
// (typically by a user copying text on the remote desktop), this message
// communicates the new clipboard contents to the client.
//
// The text data is transmitted as Latin-1 encoded bytes, which is compatible
// with ASCII and the first 256 Unicode code points. Applications should handle
// character encoding appropriately when integrating with local clipboard systems.
//
// Example usage:
//
//	switch msg := serverMsg.(type) {
//	case *ServerCutTextMessage:
//		// Update local clipboard with server's clipboard content
//		clipboard.WriteAll(msg.Text)
//		fmt.Printf("Clipboard updated: %q\n", msg.Text)
//	}
type ServerCutTextMessage struct {
	// Text contains the clipboard text from the server.
	// The text is encoded using Latin-1 character encoding, which includes
	// all ASCII characters plus additional characters in the 128-255 range.
	// Applications should handle character encoding conversion if needed
	// for integration with local clipboard systems that use different encodings.
	Text string
}

// Type returns the message type identifier for server cut text messages.
func (*ServerCutTextMessage) Type() uint8 {
	return 3
}

// Read parses a ServerCutText message from the server.
// This method implements the ServerMessage interface and processes clipboard data
// as defined in RFC 6143 Section 7.6.4. It reads text content from the server's
// clipboard and makes it available for integration with the client's clipboard system.
//
// The method handles the complete message parsing including:
//  1. Reading message padding
//  2. Reading the text length field
//  3. Reading the clipboard text data
//  4. Converting the text to a Go string
//
// Parameters:
//   - c: The client connection (unused for ServerCutText messages)
//   - r: Reader containing the message data (excluding the message type byte)
//
// Returns:
//   - ServerMessage: A new ServerCutTextMessage containing the clipboard text
//   - error: NetworkError for I/O issues during message parsing
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	msg := &ServerCutTextMessage{}
//	parsedMsg, err := msg.Read(clientConn, messageReader)
//	if err != nil {
//		log.Printf("Failed to parse server cut text: %v", err)
//		return
//	}
//
//	// Process the parsed message
//	cutTextMsg := parsedMsg.(*ServerCutTextMessage)
//	fmt.Printf("Server clipboard updated: %q\n", cutTextMsg.Text)
//
//	// Update local clipboard
//	updateLocalClipboard(cutTextMsg.Text)
//
// Clipboard integration examples:
//
//	func updateLocalClipboard(text string) {
//		// Cross-platform clipboard integration
//		switch runtime.GOOS {
//		case "windows":
//			// Use Windows clipboard API
//			clipboard.WriteAll(text)
//		case "darwin":
//			// Use macOS pasteboard
//			clipboard.WriteAll(text)
//		case "linux":
//			// Use X11 clipboard
//			clipboard.WriteAll(text)
//		default:
//			// Log or store for manual handling
//			log.Printf("Clipboard text: %s", text)
//		}
//	}
//
// Message structure:
//
//	// Wire format (after message type byte):
//	// [3 bytes] - Padding (ignored)
//	// [4 bytes] - Text length (big-endian uint32)
//	// [N bytes] - Text data (Latin-1 encoded)
//
// Character encoding:
//
//	// The text is transmitted as Latin-1 encoded bytes
//	// Latin-1 includes ASCII (0-127) plus extended characters (128-255)
//	// Go strings handle this encoding naturally for most use cases
//
//	// For applications requiring specific encoding handling:
//	func convertFromLatin1(text string) string {
//		// Convert Latin-1 to UTF-8 if needed
//		bytes := []byte(text)
//		runes := make([]rune, len(bytes))
//		for i, b := range bytes {
//			runes[i] = rune(b) // Latin-1 maps directly to Unicode
//		}
//		return string(runes)
//	}
//
// Bidirectional clipboard synchronization:
//
//	// Handle server clipboard updates
//	func handleServerCutText(msg *ServerCutTextMessage) {
//		// Update local clipboard
//		clipboard.WriteAll(msg.Text)
//
//		// Prevent feedback loops by tracking clipboard source
//		lastServerClipboard = msg.Text
//	}
//
//	// Monitor local clipboard changes
//	func monitorLocalClipboard(client *ClientConn) {
//		for {
//			text, _ := clipboard.ReadAll()
//			if text != lastServerClipboard && text != "" {
//				client.CutText(text) // Send to server
//			}
//			time.Sleep(100 * time.Millisecond)
//		}
//	}
//
// Error handling:
// The method may return NetworkError for:
//   - I/O failures reading message padding or text length
//   - I/O failures reading text data
//   - Incomplete text data (fewer bytes than specified length)
//   - Network connection issues during message parsing
//
// Security considerations:
// - Clipboard data may contain sensitive information
// - Applications should consider whether automatic clipboard sync is appropriate
// - Large clipboard content may consume significant memory
// - Consider sanitizing or filtering clipboard content based on security requirements
//
// Performance considerations:
// - Text length is limited by uint32 maximum (4GB theoretical limit)
// - Large clipboard content is rare but possible
// - Memory usage scales linearly with text length
// - Consider implementing size limits for clipboard content in security-sensitive applications.
func (*ServerCutTextMessage) Read(c *ClientConn, r io.Reader) (ServerMessage, error) {
	validator := newInputValidator()

	var padding [1]byte
	if _, err := io.ReadFull(r, padding[:]); err != nil {
		return nil, networkError("ServerCutTextMessage.Read", "failed to read padding", err)
	}

	var textLength uint32
	if err := binary.Read(r, binary.BigEndian, &textLength); err != nil {
		return nil, networkError("ServerCutTextMessage.Read", "failed to read text length", err)
	}

	if err := validator.ValidateMessageLength(textLength, MaxServerClipboardLength); err != nil {
		return nil, protocolError("ServerCutTextMessage.Read", "invalid clipboard text length", err)
	}

	textBytes := make([]uint8, textLength)
	if err := binary.Read(r, binary.BigEndian, &textBytes); err != nil {
		return nil, networkError("ServerCutTextMessage.Read", "failed to read text data", err)
	}

	clipboardText := string(textBytes)
	if err := validator.ValidateTextData(clipboardText, int(MaxServerClipboardLength)); err != nil {
		c.logger.Warn("Invalid clipboard text received from server, sanitizing",
			Field{Key: "original_length", Value: len(clipboardText)},
			Field{Key: "error", Value: err})
		clipboardText = validator.SanitizeText(clipboardText)
	}

	return &ServerCutTextMessage{clipboardText}, nil
}
