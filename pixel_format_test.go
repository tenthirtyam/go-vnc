// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestPixelFormat_PresetsValidate(t *testing.T) {
	presets := []*PixelFormat{
		PixelFormat32BitRGBA,
		PixelFormat16BitRGB565,
		PixelFormat16BitRGB555,
		PixelFormat8BitIndexed,
	}

	for _, pf := range presets {
		if err := pf.Validate(); err != nil {
			t.Errorf("preset %+v failed validation: %v", pf, err)
		}
	}
}

func TestPixelFormat_ConvertTrueColor(t *testing.T) {
	srcConverter, err := NewPixelFormatConverter(PixelFormat32BitRGBA)
	if err != nil {
		t.Fatalf("NewPixelFormatConverter(src): %v", err)
	}

	var srcBuf bytes.Buffer
	pixels := []struct{ r, g, b uint8 }{
		{255, 0, 0},
		{0, 255, 0},
		{0, 0, 255},
		{255, 255, 255},
	}
	for _, p := range pixels {
		if err := srcConverter.WritePixel(&srcBuf, srcConverter.CreatePixel(p.r, p.g, p.b)); err != nil {
			t.Fatalf("WritePixel: %v", err)
		}
	}

	dstData, err := ConvertPixelFormat(context.Background(), srcBuf.Bytes(), PixelFormat32BitRGBA, PixelFormat16BitRGB565)
	if err != nil {
		t.Fatalf("ConvertPixelFormat: %v", err)
	}

	expectedLen := len(pixels) * 2 // 16-bit destination
	if len(dstData) != expectedLen {
		t.Fatalf("expected destination length %d, got %d", expectedLen, len(dstData))
	}

	dstConverter, err := NewPixelFormatConverter(PixelFormat16BitRGB565)
	if err != nil {
		t.Fatalf("NewPixelFormatConverter(dst): %v", err)
	}

	dstReader := bytes.NewReader(dstData)
	for i, want := range pixels {
		pixel, err := dstConverter.ReadPixel(dstReader)
		if err != nil {
			t.Fatalf("ReadPixel %d: %v", i, err)
		}
		r, g, b := dstConverter.ExtractRGB(pixel)

		// 565 conversion loses precision; allow a small channel delta.
		if absDiff(r, want.r) > 8 || absDiff(g, want.g) > 4 || absDiff(b, want.b) > 8 {
			t.Errorf("pixel %d RGB = (%d,%d,%d), want near (%d,%d,%d)", i, r, g, b, want.r, want.g, want.b)
		}
	}
}

func TestPixelFormat_ConvertInvalidLength(t *testing.T) {
	// 3 bytes is not a multiple of 4-byte RGBA pixels.
	_, err := ConvertPixelFormat(context.Background(), []byte{1, 2, 3}, PixelFormat32BitRGBA, PixelFormat16BitRGB565)
	if err == nil {
		t.Fatal("expected error for misaligned source data")
	}
}

func TestPixelFormat_ConvertInvalidFormat(t *testing.T) {
	invalid := &PixelFormat{} // BPP 0 fails validation
	_, err := ConvertPixelFormat(context.Background(), []byte{0, 0, 0, 0}, invalid, PixelFormat32BitRGBA)
	if err == nil {
		t.Fatal("expected error for invalid source format")
	}

	_, err = ConvertPixelFormat(context.Background(), []byte{0, 0, 0, 0}, PixelFormat32BitRGBA, invalid)
	if err == nil {
		t.Fatal("expected error for invalid destination format")
	}
}

func TestPixelFormat_ConvertContextCanceled(t *testing.T) {
	srcConverter, err := NewPixelFormatConverter(PixelFormat32BitRGBA)
	if err != nil {
		t.Fatalf("NewPixelFormatConverter: %v", err)
	}

	var srcBuf bytes.Buffer
	for range 64 {
		if err := srcConverter.WritePixel(&srcBuf, srcConverter.CreatePixel(128, 64, 32)); err != nil {
			t.Fatalf("WritePixel: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = ConvertPixelFormat(ctx, srcBuf.Bytes(), PixelFormat32BitRGBA, PixelFormat16BitRGB555)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestPixelFormat_ConvertIndexedPassthrough(t *testing.T) {
	// Indexed formats cannot remap without a color map; values are copied through.
	src := []byte{0, 1, 2, 255}
	dst, err := ConvertPixelFormat(context.Background(), src, PixelFormat8BitIndexed, PixelFormat8BitIndexed)
	if err != nil {
		t.Fatalf("ConvertPixelFormat: %v", err)
	}
	if !bytes.Equal(src, dst) {
		t.Errorf("expected passthrough %v, got %v", src, dst)
	}
}

func TestPixelFormat_ConvertWithTimeout(t *testing.T) {
	srcConverter, err := NewPixelFormatConverter(PixelFormat32BitRGBA)
	if err != nil {
		t.Fatalf("NewPixelFormatConverter: %v", err)
	}

	var srcBuf bytes.Buffer
	if err := srcConverter.WritePixel(&srcBuf, srcConverter.CreatePixel(10, 20, 30)); err != nil {
		t.Fatalf("WritePixel: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	dst, err := ConvertPixelFormat(ctx, srcBuf.Bytes(), PixelFormat32BitRGBA, PixelFormat16BitRGB555)
	if err != nil {
		t.Fatalf("ConvertPixelFormat: %v", err)
	}
	if len(dst) != 2 {
		t.Fatalf("expected 2 destination bytes, got %d", len(dst))
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
