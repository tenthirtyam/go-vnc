// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"io"
)

// RawEncoding represents uncompressed pixel data as defined in RFC 6143 Section 7.7.1.
type RawEncoding struct {
	// Colors contains the decoded pixel data for the rectangle.
	Colors []Color
}

// Type returns the encoding type identifier for Raw encoding.
func (*RawEncoding) Type() int32 {
	return 0
}

// Read decodes raw pixel data from the server for the specified rectangle.
// This method implements the Encoding interface and processes uncompressed pixel data
// as defined in RFC 6143 Section 7.7.1. Each pixel is transmitted in the format
// specified by the connection's PixelFormat without any compression or transformation.
//
// The method reads pixel data in left-to-right, top-to-bottom order and converts
// each pixel from the wire format to the standard Color representation. For true color
// formats, it extracts RGB components using the pixel format's shift and mask values.
// For indexed color formats, it looks up colors in the connection's color map.
//
// Parameters:
//   - c: The client connection providing pixel format and color map information
//   - rect: The rectangle being decoded, specifying dimensions and position
//   - r: Reader containing the raw pixel data from the server
//
// Returns:
//   - Encoding: A new RawEncoding instance containing the decoded pixel colors
//   - error: EncodingError if pixel data cannot be read or decoded
//
// Example usage:
//
//	// This method is typically called by the VNC client's message processing loop
//	enc := &RawEncoding{}
//	decodedEnc, err := enc.Read(clientConn, rectangle, dataReader)
//	if err != nil {
//		log.Printf("Failed to decode raw encoding: %v", err)
//		return
//	}
//
//	// Access the decoded pixel data
//	rawEnc := decodedEnc.(*RawEncoding)
//	for i, color := range rawEnc.Colors {
//		// Process each pixel color
//		x := uint16(i % int(rectangle.Width))
//		y := uint16(i / int(rectangle.Width))
//		// Apply color to framebuffer at (rect.X + x, rect.Y + y)
//	}
//
// Pixel format handling:
//
//	// The method automatically handles different pixel formats:
//	// - 8-bit: Single byte per pixel (indexed or true color)
//	// - 16-bit: Two bytes per pixel (typically RGB565 true color)
//	// - 32-bit: Four bytes per pixel (typically RGBA true color)
//
//	// For true color formats, RGB components are extracted:
//	// red = (pixel >> RedShift) & RedMax
//	// green = (pixel >> GreenShift) & GreenMax
//	// blue = (pixel >> BlueShift) & BlueMax
//
//	// For indexed color formats, the pixel value is used as a color map index:
//	// color = colorMap[pixelValue]
//
// Performance characteristics:
// - No compression overhead (fastest decoding)
// - Highest bandwidth usage (largest data size)
// - Predictable memory usage (width × height × bytes-per-pixel)
// - Suitable for complex images with high color variation
//
// Error conditions:
// The method returns an EncodingError if:
// - Insufficient pixel data is available in the reader
// - I/O errors occur while reading pixel data
// - Invalid pixel format parameters are encountered.
func (*RawEncoding) Read(c *ClientConn, rect *Rectangle, r io.Reader) (Encoding, error) {
	pixelReader := NewPixelReader(c.PixelFormat, c.ColorMap)
	colors := make([]Color, int(rect.Height)*int(rect.Width))

	for y := uint16(0); y < rect.Height; y++ {
		for x := uint16(0); x < rect.Width; x++ {
			color, err := pixelReader.ReadPixelColor(r)
			if err != nil {
				return nil, encodingError("RawEncoding.Read", "failed to read pixel data", err)
			}
			colors[int(y)*int(rect.Width)+int(x)] = color
		}
	}

	return &RawEncoding{colors}, nil
}
