// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"sync"
	"testing"
)

func TestColor_Map(t *testing.T) {
	cm := NewColorMap()

	// Test default initialization
	black := cm.Get(0)
	white := cm.Get(255)

	if black.R != 0 || black.G != 0 || black.B != 0 {
		t.Errorf("Expected black at index 0, got (%d,%d,%d)", black.R, black.G, black.B)
	}

	if white.R != 65535 || white.G != 65535 || white.B != 65535 {
		t.Errorf("Expected white at index 255, got (%d,%d,%d)", white.R, white.G, white.B)
	}

	// Test setting individual colors
	red := Color{R: 65535, G: 0, B: 0}
	cm.Set(1, red)

	retrieved := cm.Get(1)
	if retrieved != red {
		t.Errorf("Color set/get mismatch: set %+v, got %+v", red, retrieved)
	}
}

func TestColor_MapSetRange(t *testing.T) {
	cm := NewColorMap()

	// Test valid range setting
	colors := []Color{
		{R: 65535, G: 0, B: 0}, // Red
		{R: 0, G: 65535, B: 0}, // Green
		{R: 0, G: 0, B: 65535}, // Blue
	}

	err := cm.SetRange(10, colors)
	if err != nil {
		t.Fatalf("SetRange failed: %v", err)
	}

	// Verify colors were set correctly
	for i, expectedColor := range colors {
		actualColor := cm.Get(uint8(10 + i)) // #nosec G115 - Test code with small bounded values
		if actualColor != expectedColor {
			t.Errorf("Color at index %d: expected %+v, got %+v", 10+i, expectedColor, actualColor)
		}
	}
}

func TestColor_MapSetRangeValidation(t *testing.T) {
	cm := NewColorMap()

	tests := []struct {
		name        string
		startIndex  uint16
		colors      []Color
		expectError bool
	}{
		{
			name:        "Valid range",
			startIndex:  0,
			colors:      make([]Color, 10),
			expectError: false,
		},
		{
			name:        "Start index too high",
			startIndex:  256,
			colors:      make([]Color, 1),
			expectError: true,
		},
		{
			name:        "Range exceeds bounds",
			startIndex:  250,
			colors:      make([]Color, 10),
			expectError: true,
		},
		{
			name:        "Exact fit at end",
			startIndex:  255,
			colors:      make([]Color, 1),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.SetRange(tt.startIndex, tt.colors)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestColor_MapGetRange(t *testing.T) {
	cm := NewColorMap()

	// Set some test colors
	testColors := []Color{
		{R: 65535, G: 0, B: 0},
		{R: 0, G: 65535, B: 0},
		{R: 0, G: 0, B: 65535},
	}

	err := cm.SetRange(5, testColors)
	if err != nil {
		t.Fatalf("SetRange failed: %v", err)
	}

	// Get the range back
	retrievedColors, err := cm.GetRange(5, 3)
	if err != nil {
		t.Fatalf("GetRange failed: %v", err)
	}

	if len(retrievedColors) != len(testColors) {
		t.Errorf("Retrieved color count: expected %d, got %d", len(testColors), len(retrievedColors))
	}

	for i, expectedColor := range testColors {
		if retrievedColors[i] != expectedColor {
			t.Errorf("Color at index %d: expected %+v, got %+v", i, expectedColor, retrievedColors[i])
		}
	}
}

func TestColor_MapGetRangeValidation(t *testing.T) {
	cm := NewColorMap()

	tests := []struct {
		name        string
		startIndex  uint16
		count       uint16
		expectError bool
	}{
		{
			name:        "Valid range",
			startIndex:  0,
			count:       10,
			expectError: false,
		},
		{
			name:        "Start index too high",
			startIndex:  256,
			count:       1,
			expectError: true,
		},
		{
			name:        "Range exceeds bounds",
			startIndex:  250,
			count:       10,
			expectError: true,
		},
		{
			name:        "Zero count",
			startIndex:  0,
			count:       0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cm.GetRange(tt.startIndex, tt.count)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestColor_MapCopy(t *testing.T) {
	cm := NewColorMap()

	// Modify original
	red := Color{R: 65535, G: 0, B: 0}
	cm.Set(10, red)

	// Create copy
	cmCopy := cm.Copy()

	// Verify copy has the same data
	if cmCopy.Get(10) != red {
		t.Error("Copy does not contain modified color")
	}

	// Modify original after copy
	blue := Color{R: 0, G: 0, B: 65535}
	cm.Set(10, blue)

	// Verify copy is independent
	if cmCopy.Get(10) != red {
		t.Error("Copy was affected by modification to original")
	}
}

func TestColor_MapArrayConversion(t *testing.T) {
	cm := NewColorMap()

	// Modify color map
	red := Color{R: 65535, G: 0, B: 0}
	cm.Set(10, red)

	// Convert to array
	array := cm.ToArray()
	if array[10] != red {
		t.Error("ToArray does not contain modified color")
	}

	// Create new color map from array
	newCM := NewColorMap()
	newCM.FromArray(array)

	if newCM.Get(10) != red {
		t.Error("FromArray did not restore color correctly")
	}
}

func TestColor_MapConcurrency(t *testing.T) {
	cm := NewColorMap()

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				color := Color{R: uint16(id), G: uint16(j), B: 0} // #nosec G115 - Test code with bounded values
				cm.Set(uint8(id), color)                          // #nosec G115 - Test code with bounded values
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = cm.Get(uint8(id)) // #nosec G115 - Test code with bounded values
			}
		}(i)
	}

	wg.Wait()
	// If we get here without deadlock or race conditions, the test passes
}

func TestColor_FormatConverter(t *testing.T) {
	converter := NewColorFormatConverter()

	// Test 8-bit RGB conversion
	r8, g8, b8 := uint8(255), uint8(128), uint8(64)
	color := converter.RGB8ToColor(r8, g8, b8)

	expectedColor := Color{R: 65535, G: 32896, B: 16448}
	if color != expectedColor {
		t.Errorf("RGB8ToColor failed: got %+v, expected %+v", color, expectedColor)
	}

	// Test reverse conversion
	rBack, gBack, bBack := converter.ColorToRGB8(color)
	if rBack != r8 || gBack != g8 || bBack != b8 {
		t.Errorf("ColorToRGB8 failed: got (%d,%d,%d), expected (%d,%d,%d)",
			rBack, gBack, bBack, r8, g8, b8)
	}
}

func TestColor_FormatConverterHSV(t *testing.T) {
	converter := NewColorFormatConverter()

	// Test HSV conversion for red color
	h, s, v := 0.0, 100.0, 100.0 // Pure red
	color := converter.HSVToColor(h, s, v)

	// Should be close to pure red
	if color.R < 65000 || color.G > 1000 || color.B > 1000 {
		t.Errorf("HSVToColor red failed: got (%d,%d,%d)", color.R, color.G, color.B)
	}

	// Test reverse conversion
	hBack, sBack, vBack := converter.ColorToHSV(ColorRed)

	// Allow some tolerance for floating point precision
	if hBack < -1 || hBack > 1 { // Red should be around 0 degrees
		t.Errorf("ColorToHSV hue failed: got %f, expected ~0", hBack)
	}
	if sBack < 99 || sBack > 101 { // Should be 100% saturation
		t.Errorf("ColorToHSV saturation failed: got %f, expected ~100", sBack)
	}
	if vBack < 99 || vBack > 101 { // Should be 100% value
		t.Errorf("ColorToHSV value failed: got %f, expected ~100", vBack)
	}
}

func TestColor_Constants(t *testing.T) {
	// Test color constants
	if ColorBlack != (Color{R: 0, G: 0, B: 0}) {
		t.Errorf("ColorBlack incorrect: %+v", ColorBlack)
	}

	if ColorWhite != (Color{R: 65535, G: 65535, B: 65535}) {
		t.Errorf("ColorWhite incorrect: %+v", ColorWhite)
	}

	if ColorRed != (Color{R: 65535, G: 0, B: 0}) {
		t.Errorf("ColorRed incorrect: %+v", ColorRed)
	}
}

func TestColor_MapValidationError(t *testing.T) {
	err := &ColorMapValidationError{
		Index:   256,
		Value:   256,
		Rule:    "Color map index must be 0-255",
		Message: "start index exceeds color map bounds",
	}

	expectedMsg := "color map validation failed at index 256: start index exceeds color map bounds (value: 256)"
	if err.Error() != expectedMsg {
		t.Errorf("Error message mismatch:\nGot: %s\nExpected: %s", err.Error(), expectedMsg)
	}
}

func BenchmarkColorMapGet(b *testing.B) {
	cm := NewColorMap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cm.Get(uint8(i % 256)) // #nosec G115 - Modulo ensures value fits in uint8
	}
}

func BenchmarkColorMapSet(b *testing.B) {
	cm := NewColorMap()
	color := Color{R: 65535, G: 32768, B: 16384}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.Set(uint8(i%256), color) // #nosec G115 - Modulo ensures value fits in uint8
	}
}

func BenchmarkColorFormatConverter(b *testing.B) {
	converter := NewColorFormatConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		color := converter.RGB8ToColor(255, 128, 64)
		converter.ColorToRGB8(color)
	}
}
