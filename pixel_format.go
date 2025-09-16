// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

// PixelFormat describes how pixel color data is encoded and interpreted in a VNC connection.
type PixelFormat struct {
	// BPP (bits-per-pixel) specifies how many bits are used to represent each pixel.
	BPP uint8

	// Depth specifies the number of useful bits within each pixel value.
	Depth uint8

	// BigEndian determines the byte order for multi-byte pixel values.
	BigEndian bool

	// TrueColor determines whether pixels represent direct RGB values (true)
	// or indices into a color map (false).
	TrueColor bool

	// RedMax specifies the maximum value for the red color component.
	RedMax uint16

	// GreenMax specifies the maximum value for the green color component.
	GreenMax uint16

	// BlueMax specifies the maximum value for the blue color component.
	BlueMax uint16

	// RedShift specifies how many bits to right-shift a pixel value
	// to position the red color component at the least significant bits.
	RedShift uint8

	// GreenShift specifies how many bits to right-shift a pixel value
	// to position the green color component at the least significant bits.
	GreenShift uint8

	// BlueShift specifies how many bits to right-shift a pixel value
	// to position the blue color component at the least significant bits.
	BlueShift uint8
}

// readPixelFormat reads a VNC pixel format from the wire format.
// Parses the 16-byte pixel format structure as defined in RFC 6143.
func readPixelFormat(r io.Reader, result *PixelFormat) error {
	var rawPixelFormat [16]byte
	if _, err := io.ReadFull(r, rawPixelFormat[:]); err != nil {
		return networkError("readPixelFormat", "failed to read pixel format data", err)
	}

	var pfBoolByte uint8
	brPF := bytes.NewReader(rawPixelFormat[:])
	if err := binary.Read(brPF, binary.BigEndian, &result.BPP); err != nil {
		return protocolError("readPixelFormat", "failed to read BPP field", err)
	}

	if err := binary.Read(brPF, binary.BigEndian, &result.Depth); err != nil {
		return protocolError("readPixelFormat", "failed to read depth field", err)
	}

	if err := binary.Read(brPF, binary.BigEndian, &pfBoolByte); err != nil {
		return protocolError("readPixelFormat", "failed to read big endian flag", err)
	}

	if pfBoolByte != 0 {
		// Big endian is true
		result.BigEndian = true
	}

	if err := binary.Read(brPF, binary.BigEndian, &pfBoolByte); err != nil {
		return protocolError("readPixelFormat", "failed to read true color flag", err)
	}

	if pfBoolByte != 0 {
		// True Color is true. So we also have to read all the color max & shifts.
		result.TrueColor = true

		if err := binary.Read(brPF, binary.BigEndian, &result.RedMax); err != nil {
			return protocolError("readPixelFormat", "failed to read red max value", err)
		}

		if err := binary.Read(brPF, binary.BigEndian, &result.GreenMax); err != nil {
			return protocolError("readPixelFormat", "failed to read green max value", err)
		}

		if err := binary.Read(brPF, binary.BigEndian, &result.BlueMax); err != nil {
			return protocolError("readPixelFormat", "failed to read blue max value", err)
		}

		if err := binary.Read(brPF, binary.BigEndian, &result.RedShift); err != nil {
			return protocolError("readPixelFormat", "failed to read red shift value", err)
		}

		if err := binary.Read(brPF, binary.BigEndian, &result.GreenShift); err != nil {
			return protocolError("readPixelFormat", "failed to read green shift value", err)
		}

		if err := binary.Read(brPF, binary.BigEndian, &result.BlueShift); err != nil {
			return protocolError("readPixelFormat", "failed to read blue shift value", err)
		}
	}

	return nil
}

// writePixelFormat converts a PixelFormat to its wire format representation.
// Returns the 16-byte pixel format structure as defined in RFC 6143.
func writePixelFormat(format *PixelFormat) ([]byte, error) {
	var buf bytes.Buffer

	// Byte 1
	if err := binary.Write(&buf, binary.BigEndian, format.BPP); err != nil {
		return nil, encodingError("writePixelFormat", "failed to write BPP field", err)
	}

	// Byte 2
	if err := binary.Write(&buf, binary.BigEndian, format.Depth); err != nil {
		return nil, encodingError("writePixelFormat", "failed to write depth field", err)
	}

	var boolByte byte
	if format.BigEndian {
		boolByte = 1
	} else {
		boolByte = 0
	}

	// Byte 3 (BigEndian)
	if err := binary.Write(&buf, binary.BigEndian, boolByte); err != nil {
		return nil, encodingError("writePixelFormat", "failed to write big endian flag", err)
	}

	if format.TrueColor {
		boolByte = 1
	} else {
		boolByte = 0
	}

	// Byte 4 (TrueColor)
	if err := binary.Write(&buf, binary.BigEndian, boolByte); err != nil {
		return nil, encodingError("writePixelFormat", "failed to write true color flag", err)
	}

	// If we have true color enabled then we have to fill in the rest of the
	// structure with the color values.
	if format.TrueColor {
		if err := binary.Write(&buf, binary.BigEndian, format.RedMax); err != nil {
			return nil, encodingError("writePixelFormat", "failed to write red max value", err)
		}

		if err := binary.Write(&buf, binary.BigEndian, format.GreenMax); err != nil {
			return nil, encodingError("writePixelFormat", "failed to write green max value", err)
		}

		if err := binary.Write(&buf, binary.BigEndian, format.BlueMax); err != nil {
			return nil, encodingError("writePixelFormat", "failed to write blue max value", err)
		}

		if err := binary.Write(&buf, binary.BigEndian, format.RedShift); err != nil {
			return nil, encodingError("writePixelFormat", "failed to write red shift value", err)
		}

		if err := binary.Write(&buf, binary.BigEndian, format.GreenShift); err != nil {
			return nil, encodingError("writePixelFormat", "failed to write green shift value", err)
		}

		if err := binary.Write(&buf, binary.BigEndian, format.BlueShift); err != nil {
			return nil, encodingError("writePixelFormat", "failed to write blue shift value", err)
		}
	}

	return buf.Bytes()[0:16], nil
}

// PixelFormatValidationError represents a pixel format validation error with detailed context.
type PixelFormatValidationError struct {
	Field   string
	Value   interface{}
	Rule    string
	Message string
}

// Error returns the formatted error message for pixel format validation errors.
func (e *PixelFormatValidationError) Error() string {
	return fmt.Sprintf("pixel format validation failed for field %s: %s (value: %v)",
		e.Field, e.Message, e.Value)
}

// Validate performs comprehensive validation of a pixel format according to RFC 6143.
// It checks all fields for consistency and validity, returning detailed error information
// if any validation rules are violated.
func (pf *PixelFormat) Validate() error {
	// Validate BPP (bits per pixel)
	if pf.BPP == 0 {
		return &PixelFormatValidationError{
			Field:   "BPP",
			Value:   pf.BPP,
			Rule:    "BPP must be greater than 0",
			Message: "bits per pixel cannot be zero",
		}
	}

	if pf.BPP != 8 && pf.BPP != 16 && pf.BPP != 32 {
		return &PixelFormatValidationError{
			Field:   "BPP",
			Value:   pf.BPP,
			Rule:    "BPP must be 8, 16, or 32",
			Message: "bits per pixel must be 8, 16, or 32",
		}
	}

	// Validate Depth
	if pf.Depth == 0 {
		return &PixelFormatValidationError{
			Field:   "Depth",
			Value:   pf.Depth,
			Rule:    "Depth must be greater than 0",
			Message: "color depth cannot be zero",
		}
	}

	if pf.Depth > pf.BPP {
		return &PixelFormatValidationError{
			Field:   "Depth",
			Value:   pf.Depth,
			Rule:    "Depth cannot exceed BPP",
			Message: fmt.Sprintf("color depth (%d) cannot exceed bits per pixel (%d)", pf.Depth, pf.BPP),
		}
	}

	// Validate TrueColor mode specific fields
	if pf.TrueColor {
		// Validate color maximums
		if pf.RedMax == 0 && pf.GreenMax == 0 && pf.BlueMax == 0 {
			return &PixelFormatValidationError{
				Field:   "ColorMax",
				Value:   fmt.Sprintf("R:%d G:%d B:%d", pf.RedMax, pf.GreenMax, pf.BlueMax),
				Rule:    "At least one color component must have non-zero maximum in TrueColor mode",
				Message: "all color maximums cannot be zero in true color mode",
			}
		}

		// Validate shifts don't exceed BPP
		maxShift := pf.BPP - 1
		if pf.RedShift > maxShift {
			return &PixelFormatValidationError{
				Field:   "RedShift",
				Value:   pf.RedShift,
				Rule:    fmt.Sprintf("RedShift cannot exceed %d for %d-bit pixels", maxShift, pf.BPP),
				Message: fmt.Sprintf("red shift (%d) exceeds maximum for %d-bit pixels", pf.RedShift, pf.BPP),
			}
		}
		if pf.GreenShift > maxShift {
			return &PixelFormatValidationError{
				Field:   "GreenShift",
				Value:   pf.GreenShift,
				Rule:    fmt.Sprintf("GreenShift cannot exceed %d for %d-bit pixels", maxShift, pf.BPP),
				Message: fmt.Sprintf("green shift (%d) exceeds maximum for %d-bit pixels", pf.GreenShift, pf.BPP),
			}
		}
		if pf.BlueShift > maxShift {
			return &PixelFormatValidationError{
				Field:   "BlueShift",
				Value:   pf.BlueShift,
				Rule:    fmt.Sprintf("BlueShift cannot exceed %d for %d-bit pixels", maxShift, pf.BPP),
				Message: fmt.Sprintf("blue shift (%d) exceeds maximum for %d-bit pixels", pf.BlueShift, pf.BPP),
			}
		}

		// Validate color component bit ranges don't overlap
		redBits := countBits(pf.RedMax)
		greenBits := countBits(pf.GreenMax)
		blueBits := countBits(pf.BlueMax)

		if redBits+greenBits+blueBits > pf.Depth {
			return &PixelFormatValidationError{
				Field:   "ColorBits",
				Value:   fmt.Sprintf("R:%d G:%d B:%d (total:%d)", redBits, greenBits, blueBits, redBits+greenBits+blueBits),
				Rule:    fmt.Sprintf("Total color bits cannot exceed depth (%d)", pf.Depth),
				Message: fmt.Sprintf("total color component bits (%d) exceed color depth (%d)", redBits+greenBits+blueBits, pf.Depth),
			}
		}
	}

	return nil
}

// countBits counts the number of bits needed to represent the given maximum value.
// Returns 0 for input 0, otherwise returns the position of the highest set bit + 1.
func countBits(maxVal uint16) uint8 {
	if maxVal == 0 {
		return 0
	}
	bits := uint8(0)
	for maxVal > 0 {
		maxVal >>= 1
		bits++
	}
	return bits
}

// Common pixel format presets for easy configuration.
var (
	// PixelFormat32BitRGBA represents high-quality 32-bit RGBA true color format.
	// This format provides the best color fidelity but uses the most bandwidth.
	PixelFormat32BitRGBA = &PixelFormat{
		BPP:        32,
		Depth:      24,
		BigEndian:  false,
		TrueColor:  true,
		RedMax:     255,
		GreenMax:   255,
		BlueMax:    255,
		RedShift:   16,
		GreenShift: 8,
		BlueShift:  0,
	}

	// PixelFormat16BitRGB565 represents balanced 16-bit RGB565 true color format.
	// This format provides good color quality with moderate bandwidth usage.
	PixelFormat16BitRGB565 = &PixelFormat{
		BPP:        16,
		Depth:      16,
		BigEndian:  false,
		TrueColor:  true,
		RedMax:     31,
		GreenMax:   63,
		BlueMax:    31,
		RedShift:   11,
		GreenShift: 5,
		BlueShift:  0,
	}

	// PixelFormat16BitRGB555 represents 16-bit RGB555 true color format.
	// This format provides balanced color with equal bits per color component.
	PixelFormat16BitRGB555 = &PixelFormat{
		BPP:        16,
		Depth:      15,
		BigEndian:  false,
		TrueColor:  true,
		RedMax:     31,
		GreenMax:   31,
		BlueMax:    31,
		RedShift:   10,
		GreenShift: 5,
		BlueShift:  0,
	}

	// PixelFormat8BitIndexed represents bandwidth-efficient 8-bit indexed color format.
	// This format uses the least bandwidth but is limited to 256 simultaneous colors.
	PixelFormat8BitIndexed = &PixelFormat{
		BPP:       8,
		Depth:     8,
		BigEndian: false,
		TrueColor: false,
	}
)

// PixelFormatConverter provides utilities for converting between different pixel formats
// and extracting color components from pixel data.
type PixelFormatConverter struct {
	format *PixelFormat
}

// NewPixelFormatConverter creates a new pixel format converter for the given format.
func NewPixelFormatConverter(format *PixelFormat) (*PixelFormatConverter, error) {
	if err := format.Validate(); err != nil {
		return nil, validationError("NewPixelFormatConverter", "invalid pixel format", err)
	}

	return &PixelFormatConverter{
		format: format,
	}, nil
}

// ExtractRGB extracts RGB color components from a pixel value according to the pixel format.
// Returns 8-bit RGB values (0-255) regardless of the source pixel format.
func (c *PixelFormatConverter) ExtractRGB(pixel uint32) (r, g, b uint8) {
	if !c.format.TrueColor {
		// For indexed color, the pixel value is the color map index
		// RGB extraction requires the color map, which is not available here
		return 0, 0, 0
	}

	// Extract color components using shifts and masks
	redValue := (pixel >> c.format.RedShift) & uint32(c.format.RedMax)
	greenValue := (pixel >> c.format.GreenShift) & uint32(c.format.GreenMax)
	blueValue := (pixel >> c.format.BlueShift) & uint32(c.format.BlueMax)

	// Convert to 8-bit values with overflow protection
	if c.format.RedMax > 0 {
		r = uint8((redValue * 255) / uint32(c.format.RedMax)) // #nosec G115 - Result is always <= 255
	}
	if c.format.GreenMax > 0 {
		g = uint8((greenValue * 255) / uint32(c.format.GreenMax)) // #nosec G115 - Result is always <= 255
	}
	if c.format.BlueMax > 0 {
		b = uint8((blueValue * 255) / uint32(c.format.BlueMax)) // #nosec G115 - Result is always <= 255
	}

	return r, g, b
}

// CreatePixel creates a pixel value from 8-bit RGB components according to the pixel format.
// The RGB values are scaled to match the pixel format's color depth.
func (c *PixelFormatConverter) CreatePixel(r, g, b uint8) uint32 {
	if !c.format.TrueColor {
		// For indexed color, pixel creation requires color map lookup
		// This is not supported by this method
		return 0
	}

	// Scale 8-bit values to the pixel format's color depth
	redValue := (uint32(r) * uint32(c.format.RedMax)) / 255
	greenValue := (uint32(g) * uint32(c.format.GreenMax)) / 255
	blueValue := (uint32(b) * uint32(c.format.BlueMax)) / 255

	// Combine components using shifts
	pixel := (redValue << c.format.RedShift) |
		(greenValue << c.format.GreenShift) |
		(blueValue << c.format.BlueShift)

	return pixel
}

// BytesPerPixel returns the number of bytes per pixel for this format.
func (c *PixelFormatConverter) BytesPerPixel() int {
	return int(c.format.BPP) / 8
}

// ReadPixel reads a single pixel from the reader according to the pixel format's byte order.
func (c *PixelFormatConverter) ReadPixel(r io.Reader) (uint32, error) {
	bytesPerPixel := c.BytesPerPixel()
	pixelBytes := make([]byte, bytesPerPixel)

	if _, err := io.ReadFull(r, pixelBytes); err != nil {
		return 0, networkError("PixelFormatConverter.ReadPixel", "failed to read pixel data", err)
	}

	var pixel uint32
	if c.format.BigEndian {
		switch bytesPerPixel {
		case 1:
			pixel = uint32(pixelBytes[0])
		case 2:
			pixel = uint32(binary.BigEndian.Uint16(pixelBytes))
		case 4:
			pixel = binary.BigEndian.Uint32(pixelBytes)
		}
	} else {
		switch bytesPerPixel {
		case 1:
			pixel = uint32(pixelBytes[0])
		case 2:
			pixel = uint32(binary.LittleEndian.Uint16(pixelBytes))
		case 4:
			pixel = binary.LittleEndian.Uint32(pixelBytes)
		}
	}

	return pixel, nil
}

// WritePixel writes a single pixel to the writer according to the pixel format's byte order.
func (c *PixelFormatConverter) WritePixel(w io.Writer, pixel uint32) error {
	bytesPerPixel := c.BytesPerPixel()
	pixelBytes := make([]byte, bytesPerPixel)

	if c.format.BigEndian {
		switch bytesPerPixel {
		case 1:
			pixelBytes[0] = uint8(pixel & 0xFF) // #nosec G115 - Masked to 8 bits
		case 2:
			binary.BigEndian.PutUint16(pixelBytes, uint16(pixel&0xFFFF)) // #nosec G115 - Masked to 16 bits
		case 4:
			binary.BigEndian.PutUint32(pixelBytes, pixel)
		}
	} else {
		switch bytesPerPixel {
		case 1:
			pixelBytes[0] = uint8(pixel & 0xFF) // #nosec G115 - Masked to 8 bits
		case 2:
			binary.LittleEndian.PutUint16(pixelBytes, uint16(pixel&0xFFFF)) // #nosec G115 - Masked to 16 bits
		case 4:
			binary.LittleEndian.PutUint32(pixelBytes, pixel)
		}
	}

	if _, err := w.Write(pixelBytes); err != nil {
		return networkError("PixelFormatConverter.WritePixel", "failed to write pixel data", err)
	}

	return nil
}

// ConvertPixelFormat converts pixel data from one format to another.
// This is useful for format conversion during encoding/decoding operations.
func ConvertPixelFormat(ctx context.Context, srcData []byte, srcFormat, dstFormat *PixelFormat) ([]byte, error) {
	if err := srcFormat.Validate(); err != nil {
		return nil, validationError("ConvertPixelFormat", "invalid source pixel format", err)
	}
	if err := dstFormat.Validate(); err != nil {
		return nil, validationError("ConvertPixelFormat", "invalid destination pixel format", err)
	}

	srcConverter, err := NewPixelFormatConverter(srcFormat)
	if err != nil {
		return nil, configurationError("ConvertPixelFormat", "failed to create source converter", err)
	}

	dstConverter, err := NewPixelFormatConverter(dstFormat)
	if err != nil {
		return nil, configurationError("ConvertPixelFormat", "failed to create destination converter", err)
	}

	srcBytesPerPixel := srcConverter.BytesPerPixel()
	dstBytesPerPixel := dstConverter.BytesPerPixel()

	if len(srcData)%srcBytesPerPixel != 0 {
		return nil, validationError("ConvertPixelFormat",
			fmt.Sprintf("source data length (%d) is not a multiple of source bytes per pixel (%d)",
				len(srcData), srcBytesPerPixel), nil)
	}

	pixelCount := len(srcData) / srcBytesPerPixel
	dstData := make([]byte, pixelCount*dstBytesPerPixel)

	srcReader := bytes.NewReader(srcData)
	dstWriter := bytes.NewBuffer(dstData[:0])

	for i := 0; i < pixelCount; i++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Read pixel from source format
		srcPixel, err := srcConverter.ReadPixel(srcReader)
		if err != nil {
			return nil, encodingError("ConvertPixelFormat", fmt.Sprintf("failed to read source pixel %d", i), err)
		}

		// Convert pixel if both formats are true color
		var dstPixel uint32
		if srcFormat.TrueColor && dstFormat.TrueColor {
			r, g, b := srcConverter.ExtractRGB(srcPixel)
			dstPixel = dstConverter.CreatePixel(r, g, b)
		} else {
			// For indexed color formats, direct conversion is not possible
			// without color map information
			dstPixel = srcPixel
		}

		// Write pixel in destination format
		if err := dstConverter.WritePixel(dstWriter, dstPixel); err != nil {
			return nil, encodingError("ConvertPixelFormat", fmt.Sprintf("failed to write destination pixel %d", i), err)
		}
	}

	return dstWriter.Bytes(), nil
}
