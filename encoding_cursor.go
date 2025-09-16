// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"io"
)

// CursorPseudoEncoding represents the Cursor pseudo-encoding as defined in RFC 6143.
// Allows the server to send cursor shape and hotspot information to the client.
type CursorPseudoEncoding struct {
	// Width is the width of the cursor in pixels.
	// A width of 0 indicates that the cursor should be hidden.
	Width uint16

	// Height is the height of the cursor in pixels.
	// A height of 0 indicates that the cursor should be hidden.
	Height uint16

	// HotspotX is the horizontal offset from the cursor's left edge to the hotspot.
	// This represents the active point of the cursor (e.g., the tip of an arrow).
	HotspotX uint16

	// HotspotY is the vertical offset from the cursor's top edge to the hotspot.
	// This represents the active point of the cursor (e.g., the tip of an arrow).
	HotspotY uint16

	// PixelData contains the cursor image data in the current pixel format.
	// The data is organized in row-major order (left-to-right, top-to-bottom).
	// The length is width × height × bytes-per-pixel.
	PixelData []uint8

	// MaskData contains the transparency mask for the cursor.
	// Each bit represents one pixel: 1 = opaque, 0 = transparent.
	// The data is organized in row-major order with bits packed into bytes.
	// The length is ceil(width/8) × height bytes.
	MaskData []uint8
}

// Type returns the encoding type identifier for Cursor pseudo-encoding.
func (*CursorPseudoEncoding) Type() int32 {
	return -239
}

// IsPseudo returns true indicating this is a pseudo-encoding.
func (*CursorPseudoEncoding) IsPseudo() bool {
	return true
}

// Read decodes Cursor pseudo-encoding data from the server for the specified rectangle.
// This method implements the Encoding interface and processes cursor shape data
// including pixel data, transparency mask, and hotspot coordinates.
//
// The rectangle's X and Y coordinates represent the cursor hotspot offset,
// while Width and Height represent the cursor dimensions. A cursor with
// width=0 and height=0 indicates that the cursor should be hidden.
//
// Parameters:
//   - c: The client connection providing pixel format information
//   - rect: The rectangle containing cursor dimensions and hotspot coordinates
//   - r: Reader containing the cursor pixel data and mask
//
// Returns:
//   - Encoding: A new CursorPseudoEncoding instance containing the cursor data
//   - error: EncodingError if the cursor data cannot be read or is invalid
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	enc := &CursorPseudoEncoding{}
//	decodedEnc, err := enc.Read(clientConn, rectangle, dataReader)
//	if err != nil {
//		log.Printf("Failed to decode cursor pseudo-encoding: %v", err)
//		return
//	}
//
//	// Process the cursor update
//	cursorEnc := decodedEnc.(*CursorPseudoEncoding)
//	if cursorEnc.Width == 0 && cursorEnc.Height == 0 {
//		// Hide cursor
//		hideCursor()
//	} else {
//		// Update cursor shape
//		updateCursor(cursorEnc.PixelData, cursorEnc.MaskData,
//			cursorEnc.Width, cursorEnc.Height,
//			cursorEnc.HotspotX, cursorEnc.HotspotY)
//	}
//
// Cursor rendering example:
//
//	func updateCursor(pixelData, maskData []uint8, width, height, hotspotX, hotspotY uint16) {
//		// Create cursor bitmap from pixel data and mask
//		cursor := createCursorBitmap(pixelData, maskData, width, height)
//
//		// Set cursor hotspot
//		cursor.SetHotspot(int(hotspotX), int(hotspotY))
//
//		// Apply cursor to window/display
//		window.SetCursor(cursor)
//	}
//
// Mask processing example:
//
//	func processCursorMask(maskData []uint8, width, height uint16) []bool {
//		mask := make([]bool, width*height)
//		bytesPerRow := (width + 7) / 8
//
//		for y := uint16(0); y < height; y++ {
//			for x := uint16(0); x < width; x++ {
//				byteIndex := y*bytesPerRow + x/8
//				bitIndex := 7 - (x % 8)
//				mask[y*width+x] = (maskData[byteIndex] & (1 << bitIndex)) != 0
//			}
//		}
//		return mask
//	}
//
// Error conditions:
// The method returns an EncodingError if:
// - Insufficient pixel data is available (less than width × height × bytes-per-pixel)
// - Insufficient mask data is available (less than ceil(width/8) × height)
// - I/O errors occur while reading cursor data
// - Invalid cursor dimensions (width or height too large).
func (*CursorPseudoEncoding) Read(c *ClientConn, rect *Rectangle, r io.Reader) (Encoding, error) {
	cursor := &CursorPseudoEncoding{
		Width:    rect.Width,
		Height:   rect.Height,
		HotspotX: rect.X,
		HotspotY: rect.Y,
	}

	if rect.Width == 0 && rect.Height == 0 {
		return cursor, nil
	}

	if rect.Width > 256 || rect.Height > 256 {
		return nil, encodingError("CursorPseudoEncoding.Read", "cursor dimensions too large", nil)
	}

	pixelDataSize := calculatePixelDataSize(rect.Width, rect.Height, c.PixelFormat)
	maskDataSize := calculateMaskDataSize(rect.Width, rect.Height)

	pixelReader := NewPixelReader(c.PixelFormat, c.ColorMap)

	var err error
	cursor.PixelData, err = pixelReader.ReadPixelData(r, pixelDataSize)
	if err != nil {
		return nil, encodingError("CursorPseudoEncoding.Read", "failed to read cursor pixel data", err)
	}

	cursor.MaskData, err = pixelReader.ReadPixelData(r, maskDataSize)
	if err != nil {
		return nil, encodingError("CursorPseudoEncoding.Read", "failed to read cursor mask data", err)
	}

	return cursor, nil
}

// Handle processes the cursor pseudo-encoding by updating the client's cursor state.
// This method implements the PseudoEncoding interface and provides a way to handle
// cursor updates without requiring the application to manually process the encoding data.
//
// The method updates the client connection's cursor state and can trigger cursor
// visibility changes or cursor shape updates based on the encoding data.
//
// Parameters:
//   - c: The client connection to update
//   - rect: The rectangle containing cursor position and dimensions (unused for cursor)
//
// Returns:
//   - error: Always returns nil for cursor pseudo-encoding (no processing errors expected)
//
// Example usage:
//
//	// This method is typically called automatically by the VNC client
//	cursorEnc := &CursorPseudoEncoding{...}
//	err := cursorEnc.Handle(clientConn, rectangle)
//	if err != nil {
//		log.Printf("Failed to handle cursor update: %v", err)
//	}
//
// Note: This is a basic implementation that logs the cursor update.
// Applications should extend this to integrate with their cursor management system.
func (cursor *CursorPseudoEncoding) Handle(c *ClientConn, rect *Rectangle) error {
	if cursor.Width == 0 && cursor.Height == 0 {
		c.logger.Debug("Cursor hidden")
	} else {
		c.logger.Debug("Cursor updated",
			Field{Key: "width", Value: cursor.Width},
			Field{Key: "height", Value: cursor.Height},
			Field{Key: "hotspot_x", Value: cursor.HotspotX},
			Field{Key: "hotspot_y", Value: cursor.HotspotY})
	}

	// Applications should override this method or handle cursor updates
	// in their message processing loop to integrate with their cursor system
	return nil
}
