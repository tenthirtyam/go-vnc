// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"strings"
	"testing"
)

func TestValidation_ProtocolVersion(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{
			name:    "valid version 3.8",
			version: "RFB 003.008\n",
			wantErr: false,
		},
		{
			name:    "valid version 3.3",
			version: "RFB 003.003\n",
			wantErr: false,
		},
		{
			name:    "invalid length",
			version: "RFB 003.008",
			wantErr: true,
		},
		{
			name:    "invalid prefix",
			version: "VNC 003.008\n",
			wantErr: true,
		},
		{
			name:    "missing newline",
			version: "RFB 003.008 ",
			wantErr: true,
		},
		{
			name:    "invalid format",
			version: "RFB 003008\n",
			wantErr: true,
		},
		{
			name:    "non-digit version",
			version: "RFB abc.def\n",
			wantErr: true,
		},
		{
			name:    "empty string",
			version: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateProtocolVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProtocolVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_SecurityTypes(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name          string
		securityTypes []uint8
		wantErr       bool
	}{
		{
			name:          "valid types",
			securityTypes: []uint8{1, 2},
			wantErr:       false,
		},
		{
			name:          "single type",
			securityTypes: []uint8{1},
			wantErr:       false,
		},
		{
			name:          "custom type",
			securityTypes: []uint8{16},
			wantErr:       false,
		},
		{
			name:          "empty array",
			securityTypes: []uint8{},
			wantErr:       true,
		},
		{
			name:          "failure type",
			securityTypes: []uint8{0},
			wantErr:       true,
		},
		{
			name:          "too many types",
			securityTypes: make([]uint8, 256),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateSecurityTypes(tt.securityTypes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecurityTypes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_SecurityType(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name         string
		securityType uint8
		wantErr      bool
	}{
		{
			name:         "none auth",
			securityType: 1,
			wantErr:      false,
		},
		{
			name:         "vnc auth",
			securityType: 2,
			wantErr:      false,
		},
		{
			name:         "reserved type",
			securityType: 5,
			wantErr:      false,
		},
		{
			name:         "custom type",
			securityType: 16,
			wantErr:      false,
		},
		{
			name:         "failure type",
			securityType: 0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateSecurityType(tt.securityType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecurityType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_FramebufferDimensions(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name    string
		width   uint16
		height  uint16
		wantErr bool
	}{
		{
			name:    "valid dimensions",
			width:   1920,
			height:  1080,
			wantErr: false,
		},
		{
			name:    "minimum dimensions",
			width:   1,
			height:  1,
			wantErr: false,
		},
		{
			name:    "zero width",
			width:   0,
			height:  1080,
			wantErr: true,
		},
		{
			name:    "zero height",
			width:   1920,
			height:  0,
			wantErr: true,
		},
		{
			name:    "maximum valid dimensions",
			width:   32768,
			height:  32768,
			wantErr: false,
		},
		{
			name:    "width too large",
			width:   32769,
			height:  1080,
			wantErr: true,
		},
		{
			name:    "height too large",
			width:   1920,
			height:  32769,
			wantErr: true,
		},
		{
			name:    "area too large",
			width:   32768,
			height:  32769, // This would exceed max area
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateFramebufferDimensions(tt.width, tt.height)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFramebufferDimensions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_Rectangle(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name     string
		x, y     uint16
		w, h     uint16
		fbW, fbH uint16
		wantErr  bool
	}{
		{
			name: "valid rectangle",
			x:    0, y: 0, w: 100, h: 100,
			fbW: 1920, fbH: 1080,
			wantErr: false,
		},
		{
			name: "rectangle at edge",
			x:    1820, y: 980, w: 100, h: 100,
			fbW: 1920, fbH: 1080,
			wantErr: false,
		},
		{
			name: "zero width",
			x:    0, y: 0, w: 0, h: 100,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
		{
			name: "zero height",
			x:    0, y: 0, w: 100, h: 0,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
		{
			name: "exceeds width",
			x:    1900, y: 0, w: 100, h: 100,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
		{
			name: "exceeds height",
			x:    0, y: 1000, w: 100, h: 100,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
		{
			name: "overflow x coordinate",
			x:    65535, y: 0, w: 2, h: 100,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
		{
			name: "overflow y coordinate",
			x:    0, y: 65535, w: 100, h: 2,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateRectangle(tt.x, tt.y, tt.w, tt.h, tt.fbW, tt.fbH)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRectangle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_PixelFormat(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name    string
		pf      *PixelFormat
		wantErr bool
	}{
		{
			name: "valid 32-bit true color",
			pf: &PixelFormat{
				BPP: 32, Depth: 24, BigEndian: false, TrueColor: true,
				RedMax: 255, GreenMax: 255, BlueMax: 255,
				RedShift: 16, GreenShift: 8, BlueShift: 0,
			},
			wantErr: false,
		},
		{
			name: "valid 16-bit true color",
			pf: &PixelFormat{
				BPP: 16, Depth: 16, BigEndian: false, TrueColor: true,
				RedMax: 31, GreenMax: 63, BlueMax: 31,
				RedShift: 11, GreenShift: 5, BlueShift: 0,
			},
			wantErr: false,
		},
		{
			name: "valid 8-bit indexed color",
			pf: &PixelFormat{
				BPP: 8, Depth: 8, BigEndian: false, TrueColor: false,
			},
			wantErr: false,
		},
		{
			name:    "nil pixel format",
			pf:      nil,
			wantErr: true,
		},
		{
			name: "invalid BPP",
			pf: &PixelFormat{
				BPP: 24, Depth: 24, BigEndian: false, TrueColor: true,
			},
			wantErr: true,
		},
		{
			name: "zero depth",
			pf: &PixelFormat{
				BPP: 32, Depth: 0, BigEndian: false, TrueColor: true,
			},
			wantErr: true,
		},
		{
			name: "depth exceeds BPP",
			pf: &PixelFormat{
				BPP: 16, Depth: 32, BigEndian: false, TrueColor: true,
			},
			wantErr: true,
		},
		{
			name: "zero color max in true color",
			pf: &PixelFormat{
				BPP: 32, Depth: 24, BigEndian: false, TrueColor: true,
				RedMax: 0, GreenMax: 255, BlueMax: 255,
			},
			wantErr: true,
		},
		{
			name: "shift too large",
			pf: &PixelFormat{
				BPP: 16, Depth: 16, BigEndian: false, TrueColor: true,
				RedMax: 31, GreenMax: 63, BlueMax: 31,
				RedShift: 16, GreenShift: 5, BlueShift: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidatePixelFormat(tt.pf)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePixelFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_EncodingType(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name         string
		encodingType int32
		wantErr      bool
	}{
		{
			name:         "raw encoding",
			encodingType: 0,
			wantErr:      false,
		},
		{
			name:         "copyrect encoding",
			encodingType: 1,
			wantErr:      false,
		},
		{
			name:         "rre encoding",
			encodingType: 2,
			wantErr:      false,
		},
		{
			name:         "hextile encoding",
			encodingType: 5,
			wantErr:      false,
		},
		{
			name:         "cursor pseudo-encoding",
			encodingType: -1,
			wantErr:      false,
		},
		{
			name:         "desktop size pseudo-encoding",
			encodingType: -2,
			wantErr:      false,
		},
		{
			name:         "unknown positive encoding",
			encodingType: 100,
			wantErr:      false,
		},
		{
			name:         "unknown pseudo-encoding",
			encodingType: -100,
			wantErr:      false,
		},
		{
			name:         "encoding too large",
			encodingType: 1000001,
			wantErr:      true,
		},
		{
			name:         "pseudo-encoding too negative",
			encodingType: -1000001,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateEncodingType(tt.encodingType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEncodingType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_TextData(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name      string
		text      string
		maxLength int
		wantErr   bool
	}{
		{
			name:      "valid text",
			text:      "Hello, World!",
			maxLength: 100,
			wantErr:   false,
		},
		{
			name:      "text with newlines",
			text:      "Line 1\nLine 2\r\nLine 3",
			maxLength: 100,
			wantErr:   false,
		},
		{
			name:      "text with tabs",
			text:      "Column1\tColumn2\tColumn3",
			maxLength: 100,
			wantErr:   false,
		},
		{
			name:      "empty text",
			text:      "",
			maxLength: 100,
			wantErr:   false,
		},
		{
			name:      "text too long",
			text:      strings.Repeat("a", 101),
			maxLength: 100,
			wantErr:   true,
		},
		{
			name:      "text with control characters",
			text:      "Hello\x01World",
			maxLength: 100,
			wantErr:   true,
		},
		{
			name:      "invalid UTF-8",
			text:      "Hello\xff\xfeWorld",
			maxLength: 100,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateTextData(tt.text, tt.maxLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTextData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_MessageLength(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name      string
		length    uint32
		maxLength uint32
		wantErr   bool
	}{
		{
			name:      "valid length",
			length:    100,
			maxLength: 1000,
			wantErr:   false,
		},
		{
			name:      "maximum length",
			length:    1000,
			maxLength: 1000,
			wantErr:   false,
		},
		{
			name:      "zero length",
			length:    0,
			maxLength: 1000,
			wantErr:   true,
		},
		{
			name:      "length too large",
			length:    1001,
			maxLength: 1000,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateMessageLength(tt.length, tt.maxLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMessageLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_ColorMapEntries(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name       string
		firstColor uint16
		numColors  uint16
		maxColors  uint16
		wantErr    bool
	}{
		{
			name:       "valid range",
			firstColor: 0,
			numColors:  256,
			maxColors:  256,
			wantErr:    false,
		},
		{
			name:       "partial range",
			firstColor: 100,
			numColors:  50,
			maxColors:  256,
			wantErr:    false,
		},
		{
			name:       "zero colors",
			firstColor: 0,
			numColors:  0,
			maxColors:  256,
			wantErr:    true,
		},
		{
			name:       "range exceeds maximum",
			firstColor: 200,
			numColors:  100,
			maxColors:  256,
			wantErr:    true,
		},
		{
			name:       "overflow",
			firstColor: 65535,
			numColors:  2,
			maxColors:  256,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateColorMapEntries(tt.firstColor, tt.numColors, tt.maxColors)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateColorMapEntries() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_KeySymbol(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name    string
		keysym  uint32
		wantErr bool
	}{
		{
			name:    "valid ascii key",
			keysym:  0x0041, // 'A'
			wantErr: false,
		},
		{
			name:    "valid function key",
			keysym:  0xFF0D, // Return
			wantErr: false,
		},
		{
			name:    "valid unicode key",
			keysym:  0x1000041, // Unicode 'A'
			wantErr: false,
		},
		{
			name:    "zero keysym",
			keysym:  0,
			wantErr: true,
		},
		{
			name:    "keysym too large",
			keysym:  0x2000000,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateKeySymbol(tt.keysym)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKeySymbol() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_PointerPosition(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name     string
		x, y     uint16
		fbW, fbH uint16
		wantErr  bool
	}{
		{
			name: "valid position",
			x:    100, y: 100,
			fbW: 1920, fbH: 1080,
			wantErr: false,
		},
		{
			name: "position at edge",
			x:    1919, y: 1079,
			fbW: 1920, fbH: 1080,
			wantErr: false,
		},
		{
			name: "x exceeds width",
			x:    1920, y: 100,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
		{
			name: "y exceeds height",
			x:    100, y: 1080,
			fbW: 1920, fbH: 1080,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidatePointerPosition(tt.x, tt.y, tt.fbW, tt.fbH)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePointerPosition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidation_SanitizeText(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean text",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "text with allowed whitespace",
			input:    "Line 1\nLine 2\r\nTab\there",
			expected: "Line 1\nLine 2\r\nTab\there",
		},
		{
			name:     "text with control characters",
			input:    "Hello\x01\x02World",
			expected: "Hello  World",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "text with non-printable characters",
			input:    "Hello\x7FWorld",
			expected: "Hello�World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := iv.SanitizeText(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidation_BinaryData(t *testing.T) {
	iv := newInputValidator()

	tests := []struct {
		name           string
		data           []byte
		expectedLength int
		maxLength      int
		wantErr        bool
	}{
		{
			name:           "valid data",
			data:           []byte{1, 2, 3, 4},
			expectedLength: 4,
			maxLength:      10,
			wantErr:        false,
		},
		{
			name:           "no length check",
			data:           []byte{1, 2, 3, 4},
			expectedLength: 0,
			maxLength:      10,
			wantErr:        false,
		},
		{
			name:           "nil data",
			data:           nil,
			expectedLength: 0,
			maxLength:      10,
			wantErr:        true,
		},
		{
			name:           "wrong length",
			data:           []byte{1, 2, 3},
			expectedLength: 4,
			maxLength:      10,
			wantErr:        true,
		},
		{
			name:           "data too large",
			data:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			expectedLength: 0,
			maxLength:      10,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := iv.ValidateBinaryData(tt.data, tt.expectedLength, tt.maxLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBinaryData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
