// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"fmt"
	"sync"
)

// Color represents an RGB color value used in VNC color maps and pixel data.
type Color struct {
	// R is the red color component value (0-65535).
	R uint16

	// G is the green color component value (0-65535).
	G uint16

	// B is the blue color component value (0-65535).
	B uint16
}

// ColorMapValidationError represents a color map validation error with detailed context.
type ColorMapValidationError struct {
	Index   uint16
	Value   interface{}
	Rule    string
	Message string
}

// Error returns the formatted error message for color map validation errors.
func (e *ColorMapValidationError) Error() string {
	return fmt.Sprintf("color map validation failed at index %d: %s (value: %v)",
		e.Index, e.Message, e.Value)
}

// ColorMap represents a thread-safe color map for indexed color modes.
// It provides efficient access and updates with proper synchronization
// for concurrent operations.
type ColorMap struct {
	colors [ColorMapSize]Color
	mu     sync.RWMutex
}

// NewColorMap creates a new color map initialized with default grayscale colors.
// The default color map provides a reasonable fallback for indexed color modes.
func NewColorMap() *ColorMap {
	cm := &ColorMap{}
	cm.initializeDefault()
	return cm
}

// initializeDefault sets up the color map with default grayscale values.
func (cm *ColorMap) initializeDefault() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i := 0; i < ColorMapSize; i++ {
		value := uint16(i * 257) // #nosec G115 - i is bounded by ColorMapSize (256)
		cm.colors[i] = Color{R: value, G: value, B: value}
	}
}

// Get retrieves the color at the specified index from the color map.
func (cm *ColorMap) Get(index uint8) Color {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.colors[index]
}

// Set updates the color at the specified index in the color map.
func (cm *ColorMap) Set(index uint8, color Color) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.colors[index] = color
}

// SetRange updates multiple consecutive color map entries starting at the specified index.
// This method is thread-safe and provides efficient bulk updates.
func (cm *ColorMap) SetRange(startIndex uint16, colors []Color) error {
	if startIndex > ColorMapSize-1 {
		return &ColorMapValidationError{
			Index:   startIndex,
			Value:   startIndex,
			Rule:    fmt.Sprintf("Color map index must be 0-%d", ColorMapSize-1),
			Message: "start index exceeds color map bounds",
		}
	}

	if int(startIndex)+len(colors) > ColorMapSize {
		return &ColorMapValidationError{
			Index: startIndex,
			Value: len(colors),
			Rule:  "Color range must not exceed color map bounds",
			Message: fmt.Sprintf("color range [%d:%d] exceeds color map bounds",
				startIndex, int(startIndex)+len(colors)-1),
		}
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, color := range colors {
		index := startIndex + uint16(i) // #nosec G115 - i is bounded by len(colors) which was validated
		cm.colors[index] = color
	}

	return nil
}

// GetRange retrieves multiple consecutive color map entries starting at the specified index.
// This method is thread-safe and provides efficient bulk access.
func (cm *ColorMap) GetRange(startIndex uint16, count uint16) ([]Color, error) {
	if startIndex > ColorMapSize-1 {
		return nil, &ColorMapValidationError{
			Index:   startIndex,
			Value:   startIndex,
			Rule:    fmt.Sprintf("Color map index must be 0-%d", ColorMapSize-1),
			Message: "start index exceeds color map bounds",
		}
	}

	if int(startIndex)+int(count) > ColorMapSize {
		return nil, &ColorMapValidationError{
			Index: startIndex,
			Value: count,
			Rule:  "Color range must not exceed color map bounds",
			Message: fmt.Sprintf("color range [%d:%d] exceeds color map bounds",
				startIndex, int(startIndex)+int(count)-1),
		}
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	colors := make([]Color, count)
	for i := uint16(0); i < count; i++ {
		colors[i] = cm.colors[startIndex+i]
	}

	return colors, nil
}

// Copy creates a deep copy of the color map.
func (cm *ColorMap) Copy() *ColorMap {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	newCM := &ColorMap{}
	newCM.colors = cm.colors
	return newCM
}

// ToArray returns the color map as a fixed-size array.
func (cm *ColorMap) ToArray() [ColorMapSize]Color {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.colors
}

// FromArray updates the color map from a fixed-size array.
func (cm *ColorMap) FromArray(colors [ColorMapSize]Color) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.colors = colors
}

// ColorFormatConverter provides utilities for converting between different color formats
// and performing color space transformations.
type ColorFormatConverter struct{}

// NewColorFormatConverter creates a new color format converter.
// Used for converting between different color formats and performing color space transformations.
func NewColorFormatConverter() *ColorFormatConverter {
	return &ColorFormatConverter{}
}

// RGB8ToColor converts 8-bit RGB values to a 16-bit Color.
func (c *ColorFormatConverter) RGB8ToColor(r, g, b uint8) Color {
	return Color{
		R: uint16(r) * 257,
		G: uint16(g) * 257,
		B: uint16(b) * 257,
	}
}

// ColorToRGB8 converts a 16-bit Color to 8-bit RGB values.
func (c *ColorFormatConverter) ColorToRGB8(color Color) (r, g, b uint8) {
	r = uint8(color.R / 257) // #nosec G115 - Safe conversion: dividing uint16 by 257 always fits in uint8
	g = uint8(color.G / 257) // #nosec G115 - Safe conversion: dividing uint16 by 257 always fits in uint8
	b = uint8(color.B / 257) // #nosec G115 - Safe conversion: dividing uint16 by 257 always fits in uint8
	return r, g, b
}

// RGB16ToColor converts 16-bit RGB values to a Color.
func (c *ColorFormatConverter) RGB16ToColor(r, g, b uint16) Color {
	return Color{R: r, G: g, B: b}
}

// ColorToRGB16 extracts 16-bit RGB values from a Color.
func (c *ColorFormatConverter) ColorToRGB16(color Color) (r, g, b uint16) {
	return color.R, color.G, color.B
}

// HSVToColor converts HSV (Hue, Saturation, Value) to RGB Color.
// H is in degrees (0-360), S and V are percentages (0-100).
func (c *ColorFormatConverter) HSVToColor(h, s, v float64) Color {
	// Normalize inputs
	h = h / 60.0
	s = s / 100.0
	v = v / 100.0

	chroma := v * s
	x := chroma * (1.0 - abs(mod(h, 2.0)-1.0))
	m := v - chroma

	var r, g, b float64

	switch int(h) {
	case 0:
		r, g, b = chroma, x, 0
	case 1:
		r, g, b = x, chroma, 0
	case 2:
		r, g, b = 0, chroma, x
	case 3:
		r, g, b = 0, x, chroma
	case 4:
		r, g, b = x, 0, chroma
	case 5:
		r, g, b = chroma, 0, x
	default:
		r, g, b = 0, 0, 0
	}

	// Convert to 16-bit values
	return Color{
		R: uint16((r + m) * 65535),
		G: uint16((g + m) * 65535),
		B: uint16((b + m) * 65535),
	}
}

// ColorToHSV converts RGB Color to HSV (Hue, Saturation, Value).
// Returns H in degrees (0-360), S and V as percentages (0-100).
func (c *ColorFormatConverter) ColorToHSV(color Color) (h, s, v float64) {
	// Convert to 0-1 range
	r := float64(color.R) / 65535.0
	g := float64(color.G) / 65535.0
	b := float64(color.B) / 65535.0

	maxVal := max3(r, g, b)
	minVal := min3(r, g, b)
	delta := maxVal - minVal

	// Value
	v = maxVal * 100.0

	// Saturation
	if maxVal == 0 {
		s = 0
	} else {
		s = (delta / maxVal) * 100.0
	}

	// Hue
	if delta == 0 {
		h = 0
	} else if maxVal == r {
		h = 60.0 * mod((g-b)/delta, 6.0)
	} else if maxVal == g {
		h = 60.0 * ((b-r)/delta + 2.0)
	} else {
		h = 60.0 * ((r-g)/delta + 4.0)
	}

	if h < 0 {
		h += 360.0
	}

	return h, s, v
}

// Helper functions for color calculations
// abs returns the absolute value of x.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// mod returns the floating-point remainder of x/y.
func mod(x, y float64) float64 {
	return x - y*float64(int(x/y))
}

// max3 returns the maximum of three float64 values.
func max3(a, b, c float64) float64 {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

// min3 returns the minimum of three float64 values.
func min3(a, b, c float64) float64 {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// Common color constants for convenience.
var (
	// ColorBlack represents pure black (0, 0, 0).
	ColorBlack = Color{R: 0, G: 0, B: 0}

	// ColorWhite represents pure white (65535, 65535, 65535).
	ColorWhite = Color{R: 65535, G: 65535, B: 65535}

	// ColorRed represents pure red (65535, 0, 0).
	ColorRed = Color{R: 65535, G: 0, B: 0}

	// ColorGreen represents pure green (0, 65535, 0).
	ColorGreen = Color{R: 0, G: 65535, B: 0}

	// ColorBlue represents pure blue (0, 0, 65535).
	ColorBlue = Color{R: 0, G: 0, B: 65535}

	// ColorYellow represents pure yellow (65535, 65535, 0).
	ColorYellow = Color{R: 65535, G: 65535, B: 0}

	// ColorMagenta represents pure magenta (65535, 0, 65535).
	ColorMagenta = Color{R: 65535, G: 0, B: 65535}

	// ColorCyan represents pure cyan (0, 65535, 65535).
	ColorCyan = Color{R: 0, G: 65535, B: 65535}
)
