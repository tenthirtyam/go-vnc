// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"encoding/binary"
	"io"
)

// PixelReader provides utilities for reading pixel data from VNC streams.
type PixelReader struct {
	pixelFormat PixelFormat
	colorMap    [ColorMapSize]Color
	byteOrder   binary.ByteOrder
}

// NewPixelReader creates a new pixel reader for the given pixel format and color map.
func NewPixelReader(pixelFormat PixelFormat, colorMap [ColorMapSize]Color) *PixelReader {
	var byteOrder binary.ByteOrder = binary.LittleEndian
	if pixelFormat.BigEndian {
		byteOrder = binary.BigEndian
	}

	return &PixelReader{
		pixelFormat: pixelFormat,
		colorMap:    colorMap,
		byteOrder:   byteOrder,
	}
}

// BytesPerPixel returns the number of bytes per pixel for the current pixel format.
func (pr *PixelReader) BytesPerPixel() int {
	return int(pr.pixelFormat.BPP / 8)
}

// ReadPixelColor reads a single pixel from the reader and converts it to a Color.
// This consolidates the pixel reading logic that was duplicated across encoding files.
func (pr *PixelReader) ReadPixelColor(r io.Reader) (Color, error) {
	bytesPerPixel := pr.BytesPerPixel()
	pixelBytes := make([]uint8, bytesPerPixel)

	if _, err := io.ReadFull(r, pixelBytes); err != nil {
		return Color{}, err
	}

	rawPixel := pr.bytesToPixel(pixelBytes)
	return pr.pixelToColor(rawPixel), nil
}

// ReadPixelData reads raw pixel data without color conversion.
// Used by encodings that need the raw pixel bytes (like cursor encoding).
func (pr *PixelReader) ReadPixelData(r io.Reader, size int) ([]uint8, error) {
	data := make([]uint8, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// bytesToPixel converts pixel bytes to a raw pixel value based on the pixel format.
func (pr *PixelReader) bytesToPixel(pixelBytes []uint8) uint32 {
	switch pr.pixelFormat.BPP {
	case 8:
		return uint32(pixelBytes[0])
	case 16:
		return uint32(pr.byteOrder.Uint16(pixelBytes))
	case 32:
		return pr.byteOrder.Uint32(pixelBytes)
	default:
		return 0
	}
}

// pixelToColor converts a raw pixel value to a Color based on the pixel format.
func (pr *PixelReader) pixelToColor(rawPixel uint32) Color {
	if pr.pixelFormat.TrueColor {
		return Color{
			R: uint16((rawPixel >> pr.pixelFormat.RedShift) & uint32(pr.pixelFormat.RedMax)),     // #nosec G115 - Masked by RedMax
			G: uint16((rawPixel >> pr.pixelFormat.GreenShift) & uint32(pr.pixelFormat.GreenMax)), // #nosec G115 - Masked by GreenMax
			B: uint16((rawPixel >> pr.pixelFormat.BlueShift) & uint32(pr.pixelFormat.BlueMax)),   // #nosec G115 - Masked by BlueMax
		}
	} else {
		return pr.colorMap[rawPixel]
	}
}

// Convenience functions for backward compatibility and ease of use

// readPixelColor is a convenience function that maintains the existing API.
// It creates a temporary PixelReader and reads a single pixel color.
func readPixelColor(r io.Reader, pixelFormat PixelFormat, colorMap [ColorMapSize]Color) (Color, error) {
	reader := NewPixelReader(pixelFormat, colorMap)
	return reader.ReadPixelColor(r)
}

// calculatePixelDataSize calculates the size needed for pixel data.
func calculatePixelDataSize(width, height uint16, pixelFormat PixelFormat) int {
	bytesPerPixel := int(pixelFormat.BPP / 8)
	return int(width) * int(height) * bytesPerPixel
}

// calculateMaskDataSize calculates the size needed for cursor mask data.
func calculateMaskDataSize(width, height uint16) int {
	bytesPerRow := (width + 7) / 8
	return int(bytesPerRow) * int(height)
}
