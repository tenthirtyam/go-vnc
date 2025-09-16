// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"encoding/binary"
	"io"
)

// CopyRectEncoding represents the CopyRect encoding as defined in RFC 6143 Section 7.7.2.
// This encoding is used to efficiently update screen regions by copying pixel data
// from another location within the same framebuffer, rather than transmitting new pixel data.
//
// CopyRect is particularly efficient for operations like window movement, scrolling,
// or any scenario where screen content is moved from one location to another without
// modification. Instead of sending the actual pixel data, the server only needs to
// send the source coordinates from which to copy the pixels.
//
// The encoding provides significant bandwidth savings when large areas of the screen
// are moved, as only 4 bytes of coordinate data are transmitted regardless of the
// rectangle size, compared to width × height × bytes-per-pixel for raw encoding.
//
// Example usage scenarios:
// - Moving windows across the desktop
// - Scrolling content within applications
// - Drag and drop operations
// - Any operation that relocates existing screen content
//
// Wire format:
// The CopyRect encoding data consists of exactly 4 bytes:
//
//	[2 bytes] - Source X coordinate (big-endian uint16)
//	[2 bytes] - Source Y coordinate (big-endian uint16)
//
// The source coordinates specify the top-left corner of the source rectangle
// from which pixels should be copied. The dimensions of the source rectangle
// are identical to the destination rectangle specified in the Rectangle header.
type CopyRectEncoding struct {
	// SrcX is the X coordinate of the source rectangle's top-left corner.
	// This specifies the horizontal position within the framebuffer from
	// which pixels should be copied to the destination rectangle.
	SrcX uint16

	// SrcY is the Y coordinate of the source rectangle's top-left corner.
	// This specifies the vertical position within the framebuffer from
	// which pixels should be copied to the destination rectangle.
	SrcY uint16
}

// Type returns the encoding type identifier for CopyRect encoding.
func (*CopyRectEncoding) Type() int32 {
	return 1
}

// Read decodes CopyRect encoding data from the server for the specified rectangle.
// This method implements the Encoding interface and processes CopyRect encoding
// as defined in RFC 6143 Section 7.7.2. It reads the source coordinates from
// which pixels should be copied within the framebuffer.
//
// The CopyRect encoding is unique among VNC encodings because it doesn't contain
// actual pixel data. Instead, it specifies coordinates within the existing
// framebuffer from which to copy pixels to the destination rectangle.
//
// Parameters:
//   - c: The client connection (unused for CopyRect but required by interface)
//   - rect: The destination rectangle specifying where the copied pixels should be placed
//   - r: Reader containing the CopyRect encoding data (4 bytes: source X and Y coordinates)
//
// Returns:
//   - Encoding: A new CopyRectEncoding instance containing the source coordinates
//   - error: EncodingError if the coordinate data cannot be read
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	enc := &CopyRectEncoding{}
//	decodedEnc, err := enc.Read(clientConn, rectangle, dataReader)
//	if err != nil {
//		log.Printf("Failed to decode CopyRect encoding: %v", err)
//		return
//	}
//
//	// Process the CopyRect operation
//	copyRectEnc := decodedEnc.(*CopyRectEncoding)
//	fmt.Printf("Copy from (%d,%d) to (%d,%d) size %dx%d\n",
//		copyRectEnc.SrcX, copyRectEnc.SrcY,
//		rectangle.X, rectangle.Y,
//		rectangle.Width, rectangle.Height)
//
//	// Perform the copy operation on the local framebuffer
//	copyFramebufferRegion(
//		copyRectEnc.SrcX, copyRectEnc.SrcY,    // Source
//		rectangle.X, rectangle.Y,              // Destination
//		rectangle.Width, rectangle.Height)     // Dimensions
//
// Implementation considerations:
//
//	// The copy operation should handle overlapping regions correctly:
//	func copyFramebufferRegion(srcX, srcY, dstX, dstY, width, height uint16) {
//		// For overlapping regions, copy direction matters to avoid corruption
//		if srcY < dstY || (srcY == dstY && srcX < dstX) {
//			// Copy from bottom-right to top-left to avoid overwriting source data
//			copyBackward(srcX, srcY, dstX, dstY, width, height)
//		} else {
//			// Copy from top-left to bottom-right (normal case)
//			copyForward(srcX, srcY, dstX, dstY, width, height)
//		}
//	}
//
// Validation considerations:
//
//	// Ensure source rectangle is within framebuffer bounds:
//	if copyRectEnc.SrcX + rectangle.Width > framebufferWidth ||
//	   copyRectEnc.SrcY + rectangle.Height > framebufferHeight {
//		// Handle out-of-bounds source rectangle
//		return fmt.Errorf("source rectangle extends beyond framebuffer")
//	}
//
// Performance characteristics:
// - Extremely efficient: only 4 bytes transmitted regardless of rectangle size
// - Copy operation performance depends on local framebuffer implementation
// - No network bandwidth used for pixel data transmission
// - Ideal for large rectangle moves (windows, scrolling)
//
// Error conditions:
// The method returns an EncodingError if:
// - Insufficient data is available in the reader (less than 4 bytes)
// - I/O errors occur while reading coordinate data
// - Network connection issues during data reading
//
// Wire format details:
//
//	// Byte layout in network stream:
//	// Offset 0-1: Source X coordinate (big-endian uint16)
//	// Offset 2-3: Source Y coordinate (big-endian uint16)
//	// Total size: 4 bytes
//
//	// Example: Copy from (100, 200) to destination rectangle
//	// Wire bytes: [0x00, 0x64, 0x00, 0xC8]
//	//             |  100   |  200   |
func (*CopyRectEncoding) Read(c *ClientConn, rect *Rectangle, r io.Reader) (Encoding, error) {
	var srcX, srcY uint16

	if err := binary.Read(r, binary.BigEndian, &srcX); err != nil {
		return nil, encodingError("CopyRectEncoding.Read", "failed to read source X coordinate", err)
	}

	if err := binary.Read(r, binary.BigEndian, &srcY); err != nil {
		return nil, encodingError("CopyRectEncoding.Read", "failed to read source Y coordinate", err)
	}

	// Basic validation - full bounds checking should be done by application.
	if srcX > 32767 || srcY > 32767 {
		return nil, encodingError("CopyRectEncoding.Read", "source coordinates appear invalid (too large)", nil)
	}

	return &CopyRectEncoding{
		SrcX: srcX,
		SrcY: srcY,
	}, nil
}
