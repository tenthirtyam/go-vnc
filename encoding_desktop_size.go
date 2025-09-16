// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"io"
)

// DesktopSizePseudoEncoding represents the DesktopSize pseudo-encoding.
// Allows the server to notify the client when the framebuffer size changes dynamically.
type DesktopSizePseudoEncoding struct {
	// Width is the new framebuffer width in pixels.
	Width uint16

	// Height is the new framebuffer height in pixels.
	Height uint16
}

// Type returns the encoding type identifier for DesktopSize pseudo-encoding.
func (*DesktopSizePseudoEncoding) Type() int32 {
	return -223
}

// IsPseudo returns true indicating this is a pseudo-encoding.
func (*DesktopSizePseudoEncoding) IsPseudo() bool {
	return true
}

// Read decodes DesktopSize pseudo-encoding data from the server.
func (*DesktopSizePseudoEncoding) Read(c *ClientConn, rect *Rectangle, r io.Reader) (Encoding, error) {
	if rect.Width == 0 || rect.Height == 0 {
		return nil, validationError("DesktopSizePseudoEncoding.Read", "desktop dimensions cannot be zero", nil)
	}

	if rect.Width > 32767 || rect.Height > 32767 {
		return nil, validationError("DesktopSizePseudoEncoding.Read", "desktop dimensions too large", nil)
	}

	const maxPixels = 100 * 1024 * 1024
	if uint64(rect.Width)*uint64(rect.Height) > maxPixels {
		return nil, validationError("DesktopSizePseudoEncoding.Read", "desktop size would require too much memory", nil)
	}

	return &DesktopSizePseudoEncoding{
		Width:  rect.Width,
		Height: rect.Height,
	}, nil
}

// Handle processes the desktop size pseudo-encoding by updating the client's framebuffer dimensions.
// This method implements the PseudoEncoding interface and automatically updates the client
// connection's framebuffer size when a desktop resize occurs.
//
// The method updates the client connection's FrameBufferWidth and FrameBufferHeight fields
// to reflect the new desktop dimensions. Applications can then respond to the size change
// by resizing their display windows, updating viewports, or requesting new framebuffer data.
//
// Parameters:
//   - c: The client connection to update with new framebuffer dimensions
//   - rect: The rectangle containing position information (unused for desktop size)
//
// Returns:
//   - error: Always returns nil for desktop size pseudo-encoding (no processing errors expected)
//
// Example usage:
//
//	// This method is typically called automatically by the VNC client
//	desktopSizeEnc := &DesktopSizePseudoEncoding{Width: 1920, Height: 1080}
//	err := desktopSizeEnc.Handle(clientConn, rectangle)
//	if err != nil {
//		log.Printf("Failed to handle desktop size update: %v", err)
//	}
//
//	// After handling, the client connection will have updated dimensions:
//	fmt.Printf("New framebuffer size: %dx%d\n",
//		clientConn.FrameBufferWidth, clientConn.FrameBufferHeight)
//
// Integration with application:
//
//	// Applications can monitor for desktop size changes:
//	func monitorDesktopSize(client *ClientConn) {
//		oldWidth, oldHeight := client.FrameBufferWidth, client.FrameBufferHeight
//
//		// Check periodically or in message handler
//		if client.FrameBufferWidth != oldWidth || client.FrameBufferHeight != oldHeight {
//			// Handle desktop size change
//			handleDesktopResize(client.FrameBufferWidth, client.FrameBufferHeight)
//			oldWidth, oldHeight = client.FrameBufferWidth, client.FrameBufferHeight
//		}
//	}
//
// Note: Applications should typically request a full framebuffer update after
// a desktop size change to refresh the display content for the new dimensions.
func (desktop *DesktopSizePseudoEncoding) Handle(c *ClientConn, rect *Rectangle) error {
	oldWidth, oldHeight := c.GetFrameBufferSize()

	c.mu.Lock()
	c.FrameBufferWidth = desktop.Width
	c.FrameBufferHeight = desktop.Height
	c.mu.Unlock()

	c.logger.Info("Desktop size changed",
		Field{Key: "old_width", Value: oldWidth},
		Field{Key: "old_height", Value: oldHeight},
		Field{Key: "new_width", Value: desktop.Width},
		Field{Key: "new_height", Value: desktop.Height})

	return nil
}
