// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"
)

func TestSecurity_ClearBytes(t *testing.T) {
	sm := &SecureMemory{}

	// Test clearing normal data
	data := []byte("sensitive password data")
	originalData := make([]byte, len(data))
	copy(originalData, data)

	sm.ClearBytes(data)

	// Verify data has been cleared
	for i, b := range data {
		if b != 0 {
			t.Errorf("Byte at index %d not cleared: got %d, want 0", i, b)
		}
	}

	// Test clearing empty slice
	emptyData := []byte{}
	sm.ClearBytes(emptyData) // Should not panic

	// Test clearing nil slice
	sm.ClearBytes(nil) // Should not panic
}

func TestSecurity_ClearString(t *testing.T) {
	sm := &SecureMemory{}

	// Test clearing string
	sensitiveString := "password123"
	result := sm.ClearString(sensitiveString)

	if result != "" {
		t.Errorf("ClearString should return empty string, got %q", result)
	}

	// Test clearing empty string
	result = sm.ClearString("")
	if result != "" {
		t.Errorf("ClearString with empty string should return empty string, got %q", result)
	}
}

func TestSecurity_ConstantTimeCompare(t *testing.T) {
	sm := &SecureMemory{}

	tests := []struct {
		name     string
		a, b     []byte
		expected bool
	}{
		{
			name:     "equal slices",
			a:        []byte("hello"),
			b:        []byte("hello"),
			expected: true,
		},
		{
			name:     "different slices same length",
			a:        []byte("hello"),
			b:        []byte("world"),
			expected: false,
		},
		{
			name:     "different lengths",
			a:        []byte("hello"),
			b:        []byte("hi"),
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []byte{},
			b:        []byte{},
			expected: true,
		},
		{
			name:     "nil slices",
			a:        nil,
			b:        nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.ConstantTimeCompare(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ConstantTimeCompare() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSecurity_EncryptVNCChallenge(t *testing.T) {
	cipher := newSecureDESCipher()

	// Test with valid challenge
	challenge := make([]byte, VNCChallengeSize)
	if _, err := rand.Read(challenge); err != nil {
		t.Fatalf("Failed to generate test challenge: %v", err)
	}

	password := "testpass"
	result, err := cipher.EncryptVNCChallenge(password, challenge)
	if err != nil {
		t.Fatalf("EncryptVNCChallenge failed: %v", err)
	}

	if len(result) != VNCChallengeSize {
		t.Errorf("Expected result length %d, got %d", VNCChallengeSize, len(result))
	}

	// Test with invalid challenge length
	invalidChallenge := make([]byte, DESKeySize)
	_, err = cipher.EncryptVNCChallenge(password, invalidChallenge)
	if err == nil {
		t.Error("Expected error for invalid challenge length")
	}

	// Test with empty password
	_, err = cipher.EncryptVNCChallenge("", challenge)
	if err != nil {
		t.Errorf("Empty password should not cause error: %v", err)
	}

	// Test with long password (should be truncated)
	longPassword := "verylongpasswordthatexceeds8characters"
	result1, err := cipher.EncryptVNCChallenge(longPassword, challenge)
	if err != nil {
		t.Fatalf("Long password encryption failed: %v", err)
	}

	// Should produce same result as truncated password
	truncatedPassword := longPassword[:VNCMaxPasswordLength]
	result2, err := cipher.EncryptVNCChallenge(truncatedPassword, challenge)
	if err != nil {
		t.Fatalf("Truncated password encryption failed: %v", err)
	}

	if !bytes.Equal(result1, result2) {
		t.Error("Long password and truncated password should produce same result")
	}
}

func TestSecurity_ReverseBitsSecure(t *testing.T) {
	cipher := newSecureDESCipher()

	tests := []struct {
		input    byte
		expected byte
	}{
		{0x00, 0x00},
		{0xFF, 0xFF},
		{0x01, 0x80},
		{0x80, 0x01},
		{0x0F, 0xF0},
		{0xF0, 0x0F},
		{0xAA, 0x55},
		{0x55, 0xAA},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := cipher.reverseBitsSecure(tt.input)
			if result != tt.expected {
				t.Errorf("reverseBitsSecure(0x%02X) = 0x%02X, want 0x%02X",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestSecurity_ConstantTimeDelay(t *testing.T) {
	tp := newTimingProtection()

	baseDelay := 10 * time.Millisecond
	start := time.Now()
	tp.ConstantTimeDelay(baseDelay)
	elapsed := time.Since(start)

	// Should take at least the base delay
	if elapsed < baseDelay {
		t.Errorf("Delay too short: %v, expected at least %v", elapsed, baseDelay)
	}

	// Should not take more than 2x the base delay (accounting for jitter)
	maxDelay := baseDelay * 2
	if elapsed > maxDelay {
		t.Errorf("Delay too long: %v, expected at most %v", elapsed, maxDelay)
	}
}

func TestSecurity_ConstantTimeAuthentication(t *testing.T) {
	tp := newTimingProtection()

	baseDelay := 50 * time.Millisecond

	// Test successful authentication
	successFunc := func() error {
		time.Sleep(10 * time.Millisecond) // Simulate quick success
		return nil
	}

	start := time.Now()
	err := tp.ConstantTimeAuthentication(successFunc, baseDelay)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if elapsed < baseDelay {
		t.Errorf("Authentication too fast: %v, expected at least %v", elapsed, baseDelay)
	}

	// Test failed authentication
	failFunc := func() error {
		time.Sleep(5 * time.Millisecond) // Simulate quick failure
		return NewVNCError("test", ErrAuthentication, "authentication failed", nil)
	}

	start = time.Now()
	err = tp.ConstantTimeAuthentication(failFunc, baseDelay)
	elapsed = time.Since(start)

	if err == nil {
		t.Error("Expected authentication error")
	}

	if elapsed < baseDelay {
		t.Errorf("Failed authentication too fast: %v, expected at least %v", elapsed, baseDelay)
	}
}

func TestSecurity_GenerateBytes(t *testing.T) {
	sr := newSecureRandom()

	// Test valid length
	data, err := sr.GenerateBytes(16)
	if err != nil {
		t.Fatalf("GenerateBytes failed: %v", err)
	}

	if len(data) != 16 {
		t.Errorf("Expected 16 bytes, got %d", len(data))
	}

	// Test zero length
	_, err = sr.GenerateBytes(0)
	if err == nil {
		t.Error("Expected error for zero length")
	}

	// Test negative length
	_, err = sr.GenerateBytes(-1)
	if err == nil {
		t.Error("Expected error for negative length")
	}

	// Test randomness (two calls should produce different results)
	data1, err := sr.GenerateBytes(32)
	if err != nil {
		t.Fatalf("First GenerateBytes failed: %v", err)
	}

	data2, err := sr.GenerateBytes(32)
	if err != nil {
		t.Fatalf("Second GenerateBytes failed: %v", err)
	}

	if bytes.Equal(data1, data2) {
		t.Error("Two random generations produced identical results (very unlikely)")
	}
}

func TestSecurity_GenerateChallenge(t *testing.T) {
	sr := newSecureRandom()

	// Test standard VNC challenge length
	challenge, err := sr.GenerateChallenge(VNCChallengeSize)
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	if len(challenge) != VNCChallengeSize {
		t.Errorf("Expected %d bytes, got %d", VNCChallengeSize, len(challenge))
	}

	// Test different lengths
	for _, length := range []int{8, 16, 32, 64} {
		challenge, err := sr.GenerateChallenge(length)
		if err != nil {
			t.Errorf("GenerateChallenge(%d) failed: %v", length, err)
		}
		if len(challenge) != length {
			t.Errorf("Expected %d bytes, got %d", length, len(challenge))
		}
	}
}

func TestSecurity_ProtectedBytes(t *testing.T) {
	mp := newMemoryProtection()

	// Test creation
	pb := mp.NewProtectedBytes(32)
	if pb.Size() != 32 {
		t.Errorf("Expected size 32, got %d", pb.Size())
	}

	if pb.IsCleared() {
		t.Error("New protected bytes should not be cleared")
	}

	// Test data access
	data := pb.Data()
	if len(data) != 32 {
		t.Errorf("Expected data length 32, got %d", len(data))
	}

	// Test copy
	testData := []byte("test data for protected bytes")
	err := pb.Copy(testData)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Verify data was copied
	if !bytes.Equal(pb.Data()[:len(testData)], testData) {
		t.Error("Data was not copied correctly")
	}

	// Test copy with oversized data
	oversizedData := make([]byte, 64)
	err = pb.Copy(oversizedData)
	if err == nil {
		t.Error("Expected error for oversized data")
	}

	// Test zero
	pb.Zero()
	for i, b := range pb.Data() {
		if b != 0 {
			t.Errorf("Byte at index %d not zeroed: got %d", i, b)
		}
	}

	// Test clear
	pb.Clear()
	if !pb.IsCleared() {
		t.Error("Protected bytes should be cleared")
	}

	if pb.Size() != 0 {
		t.Errorf("Cleared protected bytes should have size 0, got %d", pb.Size())
	}

	// Test operations on cleared bytes
	err = pb.Copy(testData)
	if err == nil {
		t.Error("Expected error when copying to cleared protected bytes")
	}
}

func TestSecurity_NewPasswordAuth(t *testing.T) {
	auth := NewPasswordAuth("testpassword")

	if auth.Password != "testpassword" {
		t.Errorf("Expected password 'testpassword', got %q", auth.Password)
	}

	if auth.secureMemory == nil {
		t.Error("SecureMemory should be initialized")
	}

	// Test clear password
	auth.ClearPassword()
	if auth.Password != "" {
		t.Errorf("Password should be cleared, got %q", auth.Password)
	}
}

func BenchmarkSecureDESCipher_EncryptVNCChallenge(b *testing.B) {
	cipher := newSecureDESCipher()
	challenge := make([]byte, VNCChallengeSize)
	if _, err := rand.Read(challenge); err != nil {
		b.Fatalf("Failed to generate random challenge: %v", err)
	}
	password := "testpass"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cipher.EncryptVNCChallenge(password, challenge)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}
}

func BenchmarkSecureMemory_ClearBytes(b *testing.B) {
	sm := &SecureMemory{}
	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Refill data for each iteration
		for j := range data {
			data[j] = byte(j)
		}
		sm.ClearBytes(data)
	}
}

func BenchmarkTimingProtection_ConstantTimeAuthentication(b *testing.B) {
	tp := newTimingProtection()
	authFunc := func() error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tp.ConstantTimeAuthentication(authFunc, 5*time.Millisecond)
		if err != nil {
			b.Fatalf("Authentication failed: %v", err)
		}
	}
}
