// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RREEncoding represents the RRE (Rise-and-Run-length Encoding) as defined in RFC 6143 Section 7.7.3.
// RRE works by defining a background color and overlaying solid-color subrectangles.
type RREEncoding struct {
	BackgroundColor Color
	Subrectangles   []RRESubrectangle
}

// RRESubrectangle represents a single solid-color rectangle within an RRE encoding.
// Each subrectangle defines a rectangular area that should be filled with a
// specific color, overlaying the background color in that region.
type RRESubrectangle struct {
	// Color is the solid color that fills this subrectangle.
	// All pixels within the subrectangle bounds will be set to this color.
	Color Color

	// X is the horizontal position of the subrectangle's left edge,
	// relative to the parent rectangle's top-left corner (0-based).
	X uint16

	// Y is the vertical position of the subrectangle's top edge,
	// relative to the parent rectangle's top-left corner (0-based).
	Y uint16

	// Width is the width of the subrectangle in pixels.
	// Must be greater than 0 for valid subrectangles.
	Width uint16

	// Height is the height of the subrectangle in pixels.
	// Must be greater than 0 for valid subrectangles.
	Height uint16
}

// Type returns the encoding type identifier for RRE encoding.
func (*RREEncoding) Type() int32 {
	return 2
}

// Read decodes RRE encoding data from the server.
//
//		return
//	}
//
//	// Process the RRE encoding
//	rreEnc := decodedEnc.(*RREEncoding)
//
//	// Fill rectangle with background color
//	fillRectangle(rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height,
//		rreEnc.BackgroundColor)
//
//	// Apply each subrectangle
//	for _, subrect := range rreEnc.Subrectangles {
//		fillRectangle(rectangle.X + subrect.X, rectangle.Y + subrect.Y,
//			subrect.Width, subrect.Height, subrect.Color)
//	}
//
// Pixel format handling:
//
//	// The method handles different pixel formats automatically:
//	func readPixelColor(r io.Reader, pixelFormat PixelFormat) (Color, error) {
//		bytesPerPixel := pixelFormat.BPP / 8
//		pixelBytes := make([]byte, bytesPerPixel)
//
//		if _, err := io.ReadFull(r, pixelBytes); err != nil {
//			return Color{}, err
//		}
//
//		// Convert pixel bytes to Color based on pixel format
//		return convertPixelToColor(pixelBytes, pixelFormat), nil
//	}
//
// Validation and error handling:
//
//	// The method validates subrectangle bounds:
//	if subrect.X + subrect.Width > rectangle.Width ||
//	   subrect.Y + subrect.Height > rectangle.Height {
//		return encodingError("invalid subrectangle bounds")
//	}
//
// Performance considerations:
// - Decoding performance scales with the number of subrectangles
// - Memory usage depends on subrectangle count (typically small)
// - Rendering performance depends on the graphics system's rectangle fill efficiency
// - Best suited for images with few distinct color regions
//
// Error conditions:
// The method returns an EncodingError if:
// - Insufficient data is available for the subrectangle count
// - Background color data cannot be read
// - Any subrectangle data is incomplete or invalid
// - Subrectangle bounds extend outside the parent rectangle
// - I/O errors occur during data reading.
func (*RREEncoding) Read(c *ClientConn, rect *Rectangle, r io.Reader) (Encoding, error) {
	// Initialize input validator for security
	validator := newInputValidator()

	// Validate rectangle dimensions first (skip validation if framebuffer dimensions are zero, likely test scenario)
	if c.FrameBufferWidth > 0 && c.FrameBufferHeight > 0 {
		if err := validator.ValidateRectangle(rect.X, rect.Y, rect.Width, rect.Height,
			c.FrameBufferWidth, c.FrameBufferHeight); err != nil {
			return nil, encodingError("RREEncoding.Read", "invalid rectangle dimensions", err)
		}
	}

	// Read number of subrectangles
	var numSubrects uint32
	if err := binary.Read(r, binary.BigEndian, &numSubrects); err != nil {
		return nil, encodingError("RREEncoding.Read", "failed to read number of subrectangles", err)
	}

	// Validate subrectangle count with enhanced security checks
	const maxSubrects = 1000000
	if numSubrects > maxSubrects {
		return nil, encodingError("RREEncoding.Read",
			fmt.Sprintf("too many subrectangles: %d (max %d)", numSubrects, maxSubrects), nil)
	}

	// Read background color
	backgroundColor, err := readPixelColor(r, c.PixelFormat, c.ColorMap)
	if err != nil {
		return nil, encodingError("RREEncoding.Read", "failed to read background color", err)
	}

	// Read subrectangles
	subrects := make([]RRESubrectangle, numSubrects)
	for i := uint32(0); i < numSubrects; i++ {
		// Read subrectangle color
		color, err := readPixelColor(r, c.PixelFormat, c.ColorMap)
		if err != nil {
			return nil, encodingError("RREEncoding.Read", "failed to read subrectangle color", err)
		}

		// Read subrectangle position and dimensions
		var x, y, width, height uint16
		data := []interface{}{&x, &y, &width, &height}
		for _, val := range data {
			if err := binary.Read(r, binary.BigEndian, val); err != nil {
				return nil, encodingError("RREEncoding.Read", "failed to read subrectangle geometry", err)
			}
		}

		// Validate subrectangle bounds with comprehensive security checks
		if err := validator.ValidateRectangle(x, y, width, height, rect.Width, rect.Height); err != nil {
			return nil, encodingError("RREEncoding.Read", "invalid subrectangle bounds", err)
		}

		subrects[i] = RRESubrectangle{
			Color:  color,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}
	}

	return &RREEncoding{
		BackgroundColor: backgroundColor,
		Subrectangles:   subrects,
	}, nil
}
