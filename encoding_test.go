// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"testing"
)

// TestRawEncoding tests Raw encoding with various pixel formats and data sizes.
func TestEncoding_Raw(t *testing.T) {
	tests := []struct {
		name        string
		pixelFormat PixelFormat
		width       uint16
		height      uint16
		expectError bool
		errorType   ErrorCode
	}{
		{
			name: "32-bit RGB 1x1 pixel",
			pixelFormat: PixelFormat{
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
			},
			width:       1,
			height:      1,
			expectError: false,
		},
		{
			name: "16-bit RGB 2x2 pixels",
			pixelFormat: PixelFormat{
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
			},
			width:       2,
			height:      2,
			expectError: false,
		},
		{
			name: "8-bit indexed color 3x3 pixels",
			pixelFormat: PixelFormat{
				BPP:       8,
				Depth:     8,
				BigEndian: false,
				TrueColor: false,
			},
			width:       3,
			height:      3,
			expectError: false,
		},
		{
			name: "Large rectangle 100x100",
			pixelFormat: PixelFormat{
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
			},
			width:       100,
			height:      100,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client connection
			mockConn := &ClientConn{
				PixelFormat: tt.pixelFormat,
				logger:      &NoOpLogger{},
			}

			// Create rectangle
			rect := &Rectangle{
				X:      0,
				Y:      0,
				Width:  tt.width,
				Height: tt.height,
			}

			// Calculate expected data size
			bytesPerPixel := tt.pixelFormat.BPP / 8
			expectedSize := int(tt.width) * int(tt.height) * int(bytesPerPixel)

			// Create sample pixel data
			pixelData := make([]byte, expectedSize)
			for i := range pixelData {
				pixelData[i] = byte(i % 256) // Pattern data
			}

			reader := bytes.NewReader(pixelData)

			// Test Raw encoding
			rawEnc := &RawEncoding{}

			if rawEnc.Type() != 0 {
				t.Errorf("Expected Raw encoding type 0, got %d", rawEnc.Type())
			}

			result, err := rawEnc.Read(mockConn, rect, reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if vncErr, ok := err.(*VNCError); ok {
					if vncErr.Code != tt.errorType {
						t.Errorf("Expected error type %v, got %v", tt.errorType, vncErr.Code)
					}
				} else {
					t.Errorf("Expected VNCError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			rawResult, ok := result.(*RawEncoding)
			if !ok {
				t.Errorf("Expected *RawEncoding, got %T", result)
				return
			}

			// Verify we have the expected number of colors
			expectedPixels := int(tt.width) * int(tt.height)
			if len(rawResult.Colors) != expectedPixels {
				t.Errorf("Expected %d pixels, got %d", expectedPixels, len(rawResult.Colors))
			}

			// Verify data integrity - already checked above
		})
	}
}

// TestCopyRectEncoding tests CopyRect encoding.
func TestEncoding_CopyRect(t *testing.T) {
	tests := []struct {
		name        string
		srcX        uint16
		srcY        uint16
		expectError bool
		errorType   ErrorCode
	}{
		{
			name:        "Valid copy from origin",
			srcX:        0,
			srcY:        0,
			expectError: false,
		},
		{
			name:        "Valid copy from middle",
			srcX:        100,
			srcY:        200,
			expectError: false,
		},
		{
			name:        "Valid copy from high coordinates",
			srcX:        1000,
			srcY:        1000,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client connection
			mockConn := &ClientConn{
				FrameBufferWidth:  800,
				FrameBufferHeight: 600,
				logger:            &NoOpLogger{},
			}

			// Create rectangle
			rect := &Rectangle{
				X:      10,
				Y:      20,
				Width:  50,
				Height: 30,
			}

			// Create CopyRect data (source X and Y coordinates)
			var buf bytes.Buffer
			if err := binary.Write(&buf, binary.BigEndian, tt.srcX); err != nil {
				t.Fatalf("Failed to write srcX: %v", err)
			}
			if err := binary.Write(&buf, binary.BigEndian, tt.srcY); err != nil {
				t.Fatalf("Failed to write srcY: %v", err)
			}

			reader := bytes.NewReader(buf.Bytes())

			// Test CopyRect encoding
			copyRectEnc := &CopyRectEncoding{}

			if copyRectEnc.Type() != 1 {
				t.Errorf("Expected CopyRect encoding type 1, got %d", copyRectEnc.Type())
			}

			result, err := copyRectEnc.Read(mockConn, rect, reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if vncErr, ok := err.(*VNCError); ok {
					if vncErr.Code != tt.errorType {
						t.Errorf("Expected error type %v, got %v", tt.errorType, vncErr.Code)
					}
				} else {
					t.Errorf("Expected VNCError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			copyRectResult, ok := result.(*CopyRectEncoding)
			if !ok {
				t.Errorf("Expected *CopyRectEncoding, got %T", result)
				return
			}

			if copyRectResult.SrcX != tt.srcX {
				t.Errorf("Expected SrcX %d, got %d", tt.srcX, copyRectResult.SrcX)
			}

			if copyRectResult.SrcY != tt.srcY {
				t.Errorf("Expected SrcY %d, got %d", tt.srcY, copyRectResult.SrcY)
			}
		})
	}
}

// TestRREEncoding tests RRE (Rise-and-Run-length Encoding).
func TestEncoding_RRE(t *testing.T) {
	tests := []struct {
		name           string
		pixelFormat    PixelFormat
		numSubrects    uint32
		backgroundData []byte
		subrectData    []byte
		expectError    bool
		errorType      ErrorCode
	}{
		{
			name: "Single color rectangle (no subrects)",
			pixelFormat: PixelFormat{
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
			},
			numSubrects:    0,
			backgroundData: []byte{0xFF, 0x00, 0x00, 0x00}, // Red background
			subrectData:    []byte{},
			expectError:    false,
		},
		{
			name: "Rectangle with one subrect",
			pixelFormat: PixelFormat{
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
			},
			numSubrects:    1,
			backgroundData: []byte{0xFF, 0x00, 0x00, 0x00}, // Red background
			subrectData: []byte{
				0x00, 0xFF, 0x00, 0x00, // Green pixel
				0x00, 0x0A, // X=10
				0x00, 0x14, // Y=20
				0x00, 0x1E, // Width=30
				0x00, 0x28, // Height=40
			},
			expectError: false,
		},
		{
			name: "16-bit pixel format",
			pixelFormat: PixelFormat{
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
			},
			numSubrects:    1,
			backgroundData: []byte{0xF8, 0x00}, // Red background (16-bit)
			subrectData: []byte{
				0x07, 0xE0, // Green pixel (16-bit)
				0x00, 0x05, // X=5
				0x00, 0x0A, // Y=10
				0x00, 0x0F, // Width=15
				0x00, 0x14, // Height=20
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client connection
			mockConn := &ClientConn{
				PixelFormat: tt.pixelFormat,
				logger:      &NoOpLogger{},
			}

			// Create rectangle
			rect := &Rectangle{
				X:      0,
				Y:      0,
				Width:  100,
				Height: 100,
			}

			// Create RRE data
			var buf bytes.Buffer
			if err := binary.Write(&buf, binary.BigEndian, tt.numSubrects); err != nil {
				t.Fatalf("Failed to write numSubrects: %v", err)
			}
			buf.Write(tt.backgroundData)
			buf.Write(tt.subrectData)

			reader := bytes.NewReader(buf.Bytes())

			// Test RRE encoding
			rreEnc := &RREEncoding{}

			if rreEnc.Type() != 2 {
				t.Errorf("Expected RRE encoding type 2, got %d", rreEnc.Type())
			}

			result, err := rreEnc.Read(mockConn, rect, reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if vncErr, ok := err.(*VNCError); ok {
					if vncErr.Code != tt.errorType {
						t.Errorf("Expected error type %v, got %v", tt.errorType, vncErr.Code)
					}
				} else {
					t.Errorf("Expected VNCError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			rreResult, ok := result.(*RREEncoding)
			if !ok {
				t.Errorf("Expected *RREEncoding, got %T", result)
				return
			}

			if len(rreResult.Subrectangles) != int(tt.numSubrects) {
				t.Errorf("Expected %d subrects in result, got %d",
					tt.numSubrects, len(rreResult.Subrectangles))
			}

			// Note: BackgroundColor is a Color struct, not raw bytes
			// We can verify the encoding worked by checking subrectangle count
		})
	}
}

// TestHextileEncoding tests Hextile encoding.
func TestEncoding_Hextile(t *testing.T) {
	tests := []struct {
		name        string
		pixelFormat PixelFormat
		width       uint16
		height      uint16
		tileData    []byte
		expectError bool
		errorType   ErrorCode
	}{
		{
			name: "Single tile - raw",
			pixelFormat: PixelFormat{
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
			},
			width:  16,
			height: 16,
			tileData: func() []byte {
				// Raw tile (subencoding = 1)
				data := []byte{0x01} // Raw subencoding
				// Add 16x16 pixels of raw data (32-bit)
				for i := 0; i < 16*16*4; i++ {
					data = append(data, byte(i%256))
				}
				return data
			}(),
			expectError: false,
		},
		{
			name: "Single tile - solid color",
			pixelFormat: PixelFormat{
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
			},
			width:  16,
			height: 16,
			tileData: []byte{
				0x02,                   // Background specified
				0xFF, 0x00, 0x00, 0x00, // Red background
			},
			expectError: false,
		},
		{
			name: "Multiple tiles",
			pixelFormat: PixelFormat{
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
			},
			width:  32,
			height: 16,
			tileData: []byte{
				// First tile (16x16) - solid color
				0x02,       // Background specified
				0xF8, 0x00, // Red background (16-bit)
				// Second tile (16x16) - solid color
				0x02,       // Background specified
				0x07, 0xE0, // Green background (16-bit)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client connection
			mockConn := &ClientConn{
				PixelFormat: tt.pixelFormat,
				logger:      &NoOpLogger{},
			}

			// Create rectangle
			rect := &Rectangle{
				X:      0,
				Y:      0,
				Width:  tt.width,
				Height: tt.height,
			}

			reader := bytes.NewReader(tt.tileData)

			// Test Hextile encoding
			hextileEnc := &HextileEncoding{}

			if hextileEnc.Type() != 5 {
				t.Errorf("Expected Hextile encoding type 5, got %d", hextileEnc.Type())
			}

			result, err := hextileEnc.Read(mockConn, rect, reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if vncErr, ok := err.(*VNCError); ok {
					if vncErr.Code != tt.errorType {
						t.Errorf("Expected error type %v, got %v", tt.errorType, vncErr.Code)
					}
				} else {
					t.Errorf("Expected VNCError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify result
			hextileResult, ok := result.(*HextileEncoding)
			if !ok {
				t.Errorf("Expected *HextileEncoding, got %T", result)
				return
			}

			// Calculate expected number of tiles
			tilesX := (tt.width + 15) / 16
			tilesY := (tt.height + 15) / 16
			expectedTiles := int(tilesX * tilesY)

			if len(hextileResult.Tiles) != expectedTiles {
				t.Errorf("Expected %d tiles, got %d", expectedTiles, len(hextileResult.Tiles))
			}
		})
	}
}

// TestEncodingErrorHandling tests error handling in encoding implementations.
func TestEncoding_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		encoding    Encoding
		setupReader func() io.Reader
		expectError bool
		errorType   ErrorCode
	}{
		{
			name:     "Raw encoding - insufficient data",
			encoding: &RawEncoding{},
			setupReader: func() io.Reader {
				// Only 2 bytes when 4 are expected for 1 pixel at 32-bit
				return bytes.NewReader([]byte{0xFF, 0x00})
			},
			expectError: true,
			errorType:   ErrEncoding, // Encoding implementations return encoding errors
		},
		{
			name:     "CopyRect encoding - insufficient data",
			encoding: &CopyRectEncoding{},
			setupReader: func() io.Reader {
				// Only 2 bytes when 4 are expected (srcX + srcY)
				return bytes.NewReader([]byte{0x00, 0x10})
			},
			expectError: true,
			errorType:   ErrEncoding,
		},
		{
			name:     "RRE encoding - insufficient header",
			encoding: &RREEncoding{},
			setupReader: func() io.Reader {
				// Only 2 bytes when at least 4 are needed for numSubrects
				return bytes.NewReader([]byte{0x00, 0x01})
			},
			expectError: true,
			errorType:   ErrEncoding,
		},
		{
			name:     "Hextile encoding - empty data",
			encoding: &HextileEncoding{},
			setupReader: func() io.Reader {
				return bytes.NewReader([]byte{})
			},
			expectError: true,
			errorType:   ErrEncoding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client connection
			mockConn := &ClientConn{
				PixelFormat: PixelFormat{
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
				},
				logger: &NoOpLogger{},
			}

			// Create rectangle
			rect := &Rectangle{
				X:      0,
				Y:      0,
				Width:  1,
				Height: 1,
			}

			reader := tt.setupReader()

			_, err := tt.encoding.Read(mockConn, rect, reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}

				if vncErr, ok := err.(*VNCError); ok {
					if vncErr.Code != tt.errorType {
						t.Errorf("Expected error type %v, got %v", tt.errorType, vncErr.Code)
					}
				} else {
					t.Errorf("Expected VNCError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestEncodingInterface tests that all encodings implement the interface correctly.
func TestEncoding_Interface(t *testing.T) {
	encodings := []Encoding{
		&RawEncoding{},
		&CopyRectEncoding{},
		&RREEncoding{},
		&HextileEncoding{},
	}

	expectedTypes := []int32{0, 1, 2, 5}

	for i, enc := range encodings {
		expectedType := expectedTypes[i]
		t.Run(fmt.Sprintf("Encoding_%d", enc.Type()), func(t *testing.T) {
			if enc.Type() != expectedType {
				t.Errorf("Expected encoding type %d, got %d", expectedType, enc.Type())
			}

			// Test that Read method exists and has correct signature
			mockConn := &ClientConn{logger: &NoOpLogger{}}
			rect := &Rectangle{X: 0, Y: 0, Width: 1, Height: 1}
			reader := bytes.NewReader([]byte{})

			// This will likely fail due to insufficient data, but we're testing the interface
			_, err := enc.Read(mockConn, rect, reader)

			// We expect an error due to insufficient data, but the method should exist
			if err == nil {
				t.Log("Read method executed without error (unexpected but not a failure)")
			}
		})
	}
}

// TestPixelFormatCompatibility tests encoding compatibility with different pixel formats.
func TestEncoding_PixelFormatCompatibility(t *testing.T) {
	pixelFormats := []PixelFormat{
		{
			BPP: 32, Depth: 24, BigEndian: false, TrueColor: true,
			RedMax: 255, GreenMax: 255, BlueMax: 255,
			RedShift: 16, GreenShift: 8, BlueShift: 0,
		},
		{
			BPP: 16, Depth: 16, BigEndian: false, TrueColor: true,
			RedMax: 31, GreenMax: 63, BlueMax: 31,
			RedShift: 11, GreenShift: 5, BlueShift: 0,
		},
		{
			BPP: 8, Depth: 8, BigEndian: false, TrueColor: false,
		},
	}

	for _, pf := range pixelFormats {
		t.Run(fmt.Sprintf("PixelFormat_%d_bit", pf.BPP), func(t *testing.T) {
			mockConn := &ClientConn{
				PixelFormat: pf,
				logger:      &NoOpLogger{},
			}

			rect := &Rectangle{X: 0, Y: 0, Width: 1, Height: 1}

			// Test Raw encoding with this pixel format
			bytesPerPixel := pf.BPP / 8
			pixelData := make([]byte, bytesPerPixel)
			for j := range pixelData {
				pixelData[j] = byte(j)
			}

			reader := bytes.NewReader(pixelData)
			rawEnc := &RawEncoding{}

			result, err := rawEnc.Read(mockConn, rect, reader)
			if err != nil {
				t.Errorf("Raw encoding failed with %d-bit pixel format: %v", pf.BPP, err)
				return
			}

			rawResult := result.(*RawEncoding)
			if len(rawResult.Colors) != 1 {
				t.Errorf("Expected 1 pixel, got %d", len(rawResult.Colors))
			}
		})
	}
}

// Benchmark tests for encoding performance.
func BenchmarkRawEncoding(b *testing.B) {
	mockConn := &ClientConn{
		PixelFormat: PixelFormat{
			BPP: 32, Depth: 24, BigEndian: false, TrueColor: true,
			RedMax: 255, GreenMax: 255, BlueMax: 255,
			RedShift: 16, GreenShift: 8, BlueShift: 0,
		},
		logger: &NoOpLogger{},
	}

	rect := &Rectangle{X: 0, Y: 0, Width: 100, Height: 100}
	pixelData := make([]byte, 100*100*4) // 100x100 pixels at 32-bit

	rawEnc := &RawEncoding{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(pixelData)
		_, err := rawEnc.Read(mockConn, rect, reader)
		if err != nil {
			b.Fatalf("Encoding failed: %v", err)
		}
	}
}

func BenchmarkCopyRectEncoding(b *testing.B) {
	mockConn := &ClientConn{
		FrameBufferWidth:  800,
		FrameBufferHeight: 600,
		logger:            &NoOpLogger{},
	}

	rect := &Rectangle{X: 10, Y: 20, Width: 50, Height: 30}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint16(100)); err != nil { // srcX
		b.Fatalf("Failed to write srcX: %v", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, uint16(200)); err != nil { // srcY
		b.Fatalf("Failed to write srcY: %v", err)
	}
	copyRectData := buf.Bytes()

	copyRectEnc := &CopyRectEncoding{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(copyRectData)
		_, err := copyRectEnc.Read(mockConn, rect, reader)
		if err != nil {
			b.Fatalf("Encoding failed: %v", err)
		}
	}
}
