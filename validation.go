// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"fmt"
	"math"
	"unicode"
	"unicode/utf8"
)

// InputValidator validates network input data and prevents protocol vulnerabilities.
type InputValidator struct{}

// newInputValidator creates a new input validator for network input data.
// Used to validate and sanitize data to prevent protocol vulnerabilities.
func newInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateProtocolVersion validates VNC protocol version strings.
func (iv *InputValidator) ValidateProtocolVersion(version string) error {
	if len(version) != 12 {
		return validationError("InputValidator.ValidateProtocolVersion",
			fmt.Sprintf("protocol version must be exactly 12 characters, got %d", len(version)), nil)
	}

	if version[:4] != "RFB " {
		return validationError("InputValidator.ValidateProtocolVersion",
			"protocol version must start with 'RFB '", nil)
	}

	if version[11] != '\n' {
		return validationError("InputValidator.ValidateProtocolVersion",
			"protocol version must end with newline", nil)
	}

	versionPart := version[4:11]
	if len(versionPart) != 7 || versionPart[3] != '.' {
		return validationError("InputValidator.ValidateProtocolVersion",
			"protocol version format must be XXX.YYY", nil)
	}

	for i, char := range versionPart {
		if i == 3 {
			continue
		}
		if !unicode.IsDigit(char) {
			return validationError("InputValidator.ValidateProtocolVersion",
				"protocol version must contain only digits and dot", nil)
		}
	}

	return nil
}

// ValidateSecurityTypes validates an array of VNC security types.
func (iv *InputValidator) ValidateSecurityTypes(securityTypes []uint8) error {
	if len(securityTypes) == 0 {
		return validationError("InputValidator.ValidateSecurityTypes",
			"security types array cannot be empty", nil)
	}

	if len(securityTypes) > 255 {
		return validationError("InputValidator.ValidateSecurityTypes",
			"security types array too large", nil)
	}

	for i, secType := range securityTypes {
		if err := iv.ValidateSecurityType(secType); err != nil {
			return validationError("InputValidator.ValidateSecurityTypes",
				fmt.Sprintf("invalid security type at index %d", i), err)
		}
	}

	return nil
}

// ValidateSecurityType validates a VNC security type identifier.
func (iv *InputValidator) ValidateSecurityType(securityType uint8) error {
	switch securityType {
	case 0:
		return validationError("InputValidator.ValidateSecurityType",
			"security type 0 indicates connection failure", nil)
	case 1, 2:
		return nil
	case 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15:
		return nil
	default:
		return nil
	}
}

// ValidateFramebufferDimensions validates framebuffer dimensions.
func (iv *InputValidator) ValidateFramebufferDimensions(width, height uint16) error {
	if width == 0 || height == 0 {
		return validationError("InputValidator.ValidateFramebufferDimensions",
			"framebuffer dimensions cannot be zero", nil)
	}

	const maxDimension = 32768
	if width > maxDimension || height > maxDimension {
		return validationError("InputValidator.ValidateFramebufferDimensions",
			fmt.Sprintf("framebuffer dimensions too large: %dx%d (max %d)",
				width, height, maxDimension), nil)
	}

	area := uint64(width) * uint64(height)
	const maxArea = 1024 * 1024 * 1024
	if area > maxArea {
		return validationError("InputValidator.ValidateFramebufferDimensions",
			fmt.Sprintf("framebuffer area too large: %d pixels (max %d)",
				area, maxArea), nil)
	}

	return nil
}

// ValidateRectangle validates rectangle bounds against framebuffer dimensions.
func (iv *InputValidator) ValidateRectangle(x, y, width, height, fbWidth, fbHeight uint16) error {
	if width == 0 || height == 0 {
		return validationError("InputValidator.ValidateRectangle",
			"rectangle dimensions cannot be zero", nil)
	}

	if x > math.MaxUint16-width || y > math.MaxUint16-height {
		return validationError("InputValidator.ValidateRectangle",
			"rectangle coordinates would cause integer overflow", nil)
	}

	if x+width > fbWidth || y+height > fbHeight {
		return validationError("InputValidator.ValidateRectangle",
			fmt.Sprintf("rectangle (%d,%d,%d,%d) exceeds framebuffer bounds (%d,%d)",
				x, y, width, height, fbWidth, fbHeight), nil)
	}

	return nil
}

// ValidatePixelFormat validates a VNC pixel format structure.
func (iv *InputValidator) ValidatePixelFormat(pf *PixelFormat) error {
	if pf == nil {
		return validationError("InputValidator.ValidatePixelFormat",
			"pixel format cannot be nil", nil)
	}

	validBPP := []uint8{8, 16, 32}
	bppValid := false
	for _, valid := range validBPP {
		if pf.BPP == valid {
			bppValid = true
			break
		}
	}
	if !bppValid {
		return validationError("InputValidator.ValidatePixelFormat",
			fmt.Sprintf("invalid bits per pixel: %d (must be 8, 16, or 32)", pf.BPP), nil)
	}

	if pf.Depth == 0 || pf.Depth > pf.BPP {
		return validationError("InputValidator.ValidatePixelFormat",
			fmt.Sprintf("invalid depth: %d (must be 1-%d for %d BPP)",
				pf.Depth, pf.BPP, pf.BPP), nil)
	}

	if pf.TrueColor {
		if pf.RedMax == 0 || pf.GreenMax == 0 || pf.BlueMax == 0 {
			return validationError("InputValidator.ValidatePixelFormat",
				"color component maximums cannot be zero in true color format", nil)
		}

		maxShift := pf.BPP - 1
		if pf.RedShift >= maxShift || pf.GreenShift >= maxShift || pf.BlueShift >= maxShift {
			return validationError("InputValidator.ValidatePixelFormat",
				fmt.Sprintf("color shifts too large for %d BPP format", pf.BPP), nil)
		}

		redBits := iv.countBits(uint32(pf.RedMax))
		greenBits := iv.countBits(uint32(pf.GreenMax))
		blueBits := iv.countBits(uint32(pf.BlueMax))

		if redBits+greenBits+blueBits > int(pf.Depth) {
			return validationError("InputValidator.ValidatePixelFormat",
				"color component bits exceed pixel depth", nil)
		}
	}

	return nil
}

// ValidateEncodingType validates encoding type values.
func (iv *InputValidator) ValidateEncodingType(encodingType int32) error {
	if encodingType >= 0 {
		switch encodingType {
		case 0, 1, 2, 4, 5, 15, 16:
			return nil
		default:
			if encodingType > 1000000 {
				return validationError("InputValidator.ValidateEncodingType",
					fmt.Sprintf("encoding type too large: %d", encodingType), nil)
			}
		}
		return nil
	}

	switch encodingType {
	case -1, -2, -223, -224, -232, -239, -240, -247, -314:
		return nil
	default:
		if encodingType < -1000000 {
			return validationError("InputValidator.ValidateEncodingType",
				fmt.Sprintf("pseudo-encoding type too negative: %d", encodingType), nil)
		}
	}

	return nil
}

// ValidateTextData validates text data for clipboard operations.
func (iv *InputValidator) ValidateTextData(text string, maxLength int) error {
	if len(text) > maxLength {
		return validationError("InputValidator.ValidateTextData",
			fmt.Sprintf("text length %d exceeds maximum %d", len(text), maxLength), nil)
	}

	if !utf8.ValidString(text) {
		return validationError("InputValidator.ValidateTextData",
			"text contains invalid UTF-8 sequences", nil)
	}

	for i, char := range text {
		if char < 32 && char != '\t' && char != '\n' && char != '\r' {
			return validationError("InputValidator.ValidateTextData",
				fmt.Sprintf("text contains invalid control character at position %d", i), nil)
		}
	}

	return nil
}

// ValidateMessageLength validates message length fields to prevent overflow.
func (iv *InputValidator) ValidateMessageLength(length uint32, maxLength uint32) error {
	if length == 0 {
		return validationError("InputValidator.ValidateMessageLength",
			"message length cannot be zero", nil)
	}

	if length > maxLength {
		return validationError("InputValidator.ValidateMessageLength",
			fmt.Sprintf("message length %d exceeds maximum %d", length, maxLength), nil)
	}

	return nil
}

// ValidateColorMapEntries validates color map entry data.
func (iv *InputValidator) ValidateColorMapEntries(firstColor, numColors, maxColors uint16) error {
	if numColors == 0 {
		return validationError("InputValidator.ValidateColorMapEntries",
			"number of colors cannot be zero", nil)
	}

	if firstColor > math.MaxUint16-numColors {
		return validationError("InputValidator.ValidateColorMapEntries",
			"color map range would cause integer overflow", nil)
	}

	if firstColor+numColors > maxColors {
		return validationError("InputValidator.ValidateColorMapEntries",
			fmt.Sprintf("color map range (%d-%d) exceeds maximum colors %d",
				firstColor, firstColor+numColors-1, maxColors), nil)
	}

	return nil
}

// ValidateKeySymbol validates X11 keysym values for key events.
func (iv *InputValidator) ValidateKeySymbol(keysym uint32) error {
	if keysym == 0 {
		return validationError("InputValidator.ValidateKeySymbol",
			"keysym cannot be zero", nil)
	}

	if keysym > 0x1FFFFFF {
		return validationError("InputValidator.ValidateKeySymbol",
			fmt.Sprintf("keysym value too large: 0x%X", keysym), nil)
	}

	return nil
}

// ValidatePointerPosition validates pointer coordinates against framebuffer bounds.
func (iv *InputValidator) ValidatePointerPosition(x, y, fbWidth, fbHeight uint16) error {
	if x >= fbWidth || y >= fbHeight {
		return validationError("InputValidator.ValidatePointerPosition",
			fmt.Sprintf("pointer position (%d,%d) exceeds framebuffer bounds (%d,%d)",
				x, y, fbWidth, fbHeight), nil)
	}

	return nil
}

// countBits counts the number of set bits in a uint32 value.
func (iv *InputValidator) countBits(value uint32) int {
	count := 0
	for value != 0 {
		count++
		value &= value - 1
	}
	return count
}

// SanitizeText sanitizes text data by removing or replacing potentially dangerous characters.
func (iv *InputValidator) SanitizeText(text string) string {
	if text == "" {
		return text
	}

	runes := []rune(text)
	sanitized := make([]rune, 0, len(runes))

	for _, r := range runes {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			sanitized = append(sanitized, r)
		case r < 32:
			sanitized = append(sanitized, ' ')
		case unicode.IsPrint(r):
			sanitized = append(sanitized, r)
		default:
			sanitized = append(sanitized, '\uFFFD')
		}
	}

	return string(sanitized)
}

// ValidateBinaryData validates binary data for protocol messages.
func (iv *InputValidator) ValidateBinaryData(data []byte, expectedLength, maxLength int) error {
	if data == nil {
		return validationError("InputValidator.ValidateBinaryData",
			"binary data cannot be nil", nil)
	}

	if expectedLength > 0 && len(data) != expectedLength {
		return validationError("InputValidator.ValidateBinaryData",
			fmt.Sprintf("binary data length %d does not match expected %d",
				len(data), expectedLength), nil)
	}

	if len(data) > maxLength {
		return validationError("InputValidator.ValidateBinaryData",
			fmt.Sprintf("binary data length %d exceeds maximum %d",
				len(data), maxLength), nil)
	}

	return nil
}
