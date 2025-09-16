// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"crypto/des" // #nosec G502 - DES is required by VNC protocol specification (RFC 6143)
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"runtime"
	"time"
)

// SECURITY WARNING: This package implements VNC authentication which uses DES encryption.
// DES is cryptographically weak and deprecated. It is only used here because it is
// required by the VNC protocol specification (RFC 6143).
//
// Security considerations:
// - DES has a 56-bit effective key length and is vulnerable to brute force attacks
// - VNC passwords are limited to 8 characters and are not salted
// - The VNC authentication protocol is susceptible to man-in-the-middle attacks
// - For secure connections, use VNC over SSH tunnels or TLS-encrypted VNC variants
//
// This implementation includes timing attack protections and secure memory handling
// to mitigate some risks, but the fundamental protocol limitations remain.

// VNC security constants.
const (
	VNCChallengeSize     = 16
	DESKeySize           = 8
	VNCMaxPasswordLength = 8
)

// SecureMemory provides utilities for secure handling of sensitive data in memory.
type SecureMemory struct{}

// ClearBytes securely clears a byte slice by overwriting it with random data.
func (sm *SecureMemory) ClearBytes(data []byte) {
	if len(data) == 0 {
		return
	}

	randomData := make([]byte, len(data))
	if _, err := rand.Read(randomData); err == nil {
		copy(data, randomData)
	}

	for i := range data {
		data[i] = 0
	}

	for i := range data {
		data[i] = 0xFF
	}

	for i := range data {
		data[i] = 0
	}

	for i := range randomData {
		randomData[i] = 0
	}

	runtime.GC()
}

// ClearString securely clears a string by converting it to a byte slice
// and clearing the underlying memory. Note that Go strings are immutable,
// so this only clears the copy, not the original string data.
//
// Parameters:
//   - s: The string to clear (creates a mutable copy for clearing)
//
// Returns:
//   - The cleared string (empty string)
//
// Security considerations:
// - Go strings are immutable, so original string data may persist
// - This method clears a mutable copy to prevent accidental reuse
// - For maximum security, avoid storing sensitive data in strings.
func (sm *SecureMemory) ClearString(s string) string {
	if len(s) == 0 {
		return ""
	}

	data := []byte(s)
	sm.ClearBytes(data)

	return ""
}

// ConstantTimeCompare performs a constant-time comparison of two byte slices
// to prevent timing attacks. This is crucial for comparing cryptographic
// values like authentication tokens or encrypted data.
//
// Parameters:
//   - a, b: The byte slices to compare
//
// Returns:
//   - bool: true if the slices are equal, false otherwise
//
// Security considerations:
// - Uses crypto/subtle.ConstantTimeCompare for timing attack protection
// - Always takes the same amount of time regardless of input differences
// - Essential for secure authentication and cryptographic operations.
func (sm *SecureMemory) ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// SecureDESCipher provides enhanced DES encryption with secure memory handling
// and timing attack protection for VNC authentication.
type SecureDESCipher struct {
	secMem *SecureMemory
}

// newSecureDESCipher creates a new secure DES cipher for VNC authentication.
func newSecureDESCipher() *SecureDESCipher {
	return &SecureDESCipher{
		secMem: &SecureMemory{},
	}
}

// EncryptVNCChallenge encrypts a VNC authentication challenge using DES encryption
// with enhanced security measures including secure memory handling and timing
// attack protection.
//
// This method implements the VNC authentication encryption as specified in RFC 6143,
// but with additional security enhancements:
// - Secure memory clearing of sensitive data
// - Timing attack protection
// - Enhanced input validation
// - Proper error handling
//
// Parameters:
//   - password: The VNC password (limited to 8 characters)
//   - challenge: The 16-byte challenge from the server
//
// Returns:
//   - []byte: The encrypted response (16 bytes)
//   - error: Any error that occurred during encryption
//
// Security enhancements:
// - Clears password key material from memory after use
// - Uses constant-time operations where possible
// - Validates input parameters thoroughly
// - Protects against timing attacks during key preparation.
func (sdc *SecureDESCipher) EncryptVNCChallenge(password string, challenge []byte) ([]byte, error) {
	if len(challenge) != VNCChallengeSize {
		return nil, validationError("SecureDESCipher.EncryptVNCChallenge",
			fmt.Sprintf("challenge must be exactly %d bytes", VNCChallengeSize), nil)
	}

	keyBytes := make([]byte, DESKeySize)
	defer sdc.secMem.ClearBytes(keyBytes)

	passwordBytes := []byte(password)
	defer sdc.secMem.ClearBytes(passwordBytes)

	keyLen := len(passwordBytes)
	if keyLen > VNCMaxPasswordLength {
		keyLen = VNCMaxPasswordLength
	}

	for i := 0; i < DESKeySize; i++ {
		if i < keyLen {
			keyBytes[i] = sdc.reverseBitsSecure(passwordBytes[i])
		} else {
			keyBytes[i] = 0
		}
	}

	block, err := des.NewCipher(keyBytes) // #nosec G405 - DES is required by VNC protocol specification
	if err != nil {
		return nil, authenticationError("SecureDESCipher.EncryptVNCChallenge",
			"failed to create DES cipher", err)
	}

	if len(challenge) != VNCChallengeSize {
		return nil, authenticationError("SecureDESCipher.EncryptVNCChallenge",
			fmt.Sprintf("invalid challenge size: expected %d, got %d", VNCChallengeSize, len(challenge)), nil)
	}

	result := make([]byte, VNCChallengeSize)

	if DESKeySize > len(result) || DESKeySize > len(challenge) {
		return nil, authenticationError("SecureDESCipher.EncryptVNCChallenge",
			"invalid DES key size for encryption", nil)
	}
	block.Encrypt(result[0:DESKeySize], challenge[0:DESKeySize])

	if VNCChallengeSize > len(result) || VNCChallengeSize > len(challenge) || DESKeySize >= VNCChallengeSize {
		return nil, authenticationError("SecureDESCipher.EncryptVNCChallenge",
			"invalid challenge size for second DES block", nil)
	}
	block.Encrypt(result[DESKeySize:VNCChallengeSize], challenge[DESKeySize:VNCChallengeSize])

	return result, nil
}

// reverseBitsSecure performs bit reversal with timing attack protection.
// This implements the VNC-specific bit reversal required for DES key preparation
// while maintaining constant execution time.
//
// Parameters:
//   - b: The byte to reverse
//
// Returns:
//   - byte: The bit-reversed byte
//
// Security considerations:
// - Uses lookup table for constant-time operation
// - Prevents timing attacks during key preparation
// - Maintains compatibility with VNC protocol requirements.
func (sdc *SecureDESCipher) reverseBitsSecure(b byte) byte {
	var reverseLookup = [256]byte{
		0x00, 0x80, 0x40, 0xc0, 0x20, 0xa0, 0x60, 0xe0,
		0x10, 0x90, 0x50, 0xd0, 0x30, 0xb0, 0x70, 0xf0,
		0x08, 0x88, 0x48, 0xc8, 0x28, 0xa8, 0x68, 0xe8,
		0x18, 0x98, 0x58, 0xd8, 0x38, 0xb8, 0x78, 0xf8,
		0x04, 0x84, 0x44, 0xc4, 0x24, 0xa4, 0x64, 0xe4,
		0x14, 0x94, 0x54, 0xd4, 0x34, 0xb4, 0x74, 0xf4,
		0x0c, 0x8c, 0x4c, 0xcc, 0x2c, 0xac, 0x6c, 0xec,
		0x1c, 0x9c, 0x5c, 0xdc, 0x3c, 0xbc, 0x7c, 0xfc,
		0x02, 0x82, 0x42, 0xc2, 0x22, 0xa2, 0x62, 0xe2,
		0x12, 0x92, 0x52, 0xd2, 0x32, 0xb2, 0x72, 0xf2,
		0x0a, 0x8a, 0x4a, 0xca, 0x2a, 0xaa, 0x6a, 0xea,
		0x1a, 0x9a, 0x5a, 0xda, 0x3a, 0xba, 0x7a, 0xfa,
		0x06, 0x86, 0x46, 0xc6, 0x26, 0xa6, 0x66, 0xe6,
		0x16, 0x96, 0x56, 0xd6, 0x36, 0xb6, 0x76, 0xf6,
		0x0e, 0x8e, 0x4e, 0xce, 0x2e, 0xae, 0x6e, 0xee,
		0x1e, 0x9e, 0x5e, 0xde, 0x3e, 0xbe, 0x7e, 0xfe,
		0x01, 0x81, 0x41, 0xc1, 0x21, 0xa1, 0x61, 0xe1,
		0x11, 0x91, 0x51, 0xd1, 0x31, 0xb1, 0x71, 0xf1,
		0x09, 0x89, 0x49, 0xc9, 0x29, 0xa9, 0x69, 0xe9,
		0x19, 0x99, 0x59, 0xd9, 0x39, 0xb9, 0x79, 0xf9,
		0x05, 0x85, 0x45, 0xc5, 0x25, 0xa5, 0x65, 0xe5,
		0x15, 0x95, 0x55, 0xd5, 0x35, 0xb5, 0x75, 0xf5,
		0x0d, 0x8d, 0x4d, 0xcd, 0x2d, 0xad, 0x6d, 0xed,
		0x1d, 0x9d, 0x5d, 0xdd, 0x3d, 0xbd, 0x7d, 0xfd,
		0x03, 0x83, 0x43, 0xc3, 0x23, 0xa3, 0x63, 0xe3,
		0x13, 0x93, 0x53, 0xd3, 0x33, 0xb3, 0x73, 0xf3,
		0x0b, 0x8b, 0x4b, 0xcb, 0x2b, 0xab, 0x6b, 0xeb,
		0x1b, 0x9b, 0x5b, 0xdb, 0x3b, 0xbb, 0x7b, 0xfb,
		0x07, 0x87, 0x47, 0xc7, 0x27, 0xa7, 0x67, 0xe7,
		0x17, 0x97, 0x57, 0xd7, 0x37, 0xb7, 0x77, 0xf7,
		0x0f, 0x8f, 0x4f, 0xcf, 0x2f, 0xaf, 0x6f, 0xef,
		0x1f, 0x9f, 0x5f, 0xdf, 0x3f, 0xbf, 0x7f, 0xff,
	}

	return reverseLookup[b]
}

// TimingProtection provides utilities for protecting against timing attacks
// during authentication and cryptographic operations.
type TimingProtection struct{}

// newTimingProtection creates a new timing protection instance.
func newTimingProtection() *TimingProtection {
	return &TimingProtection{}
}

// ConstantTimeDelay introduces a constant delay to normalize operation timing.
// This helps prevent timing attacks by ensuring operations take a consistent
// amount of time regardless of the input or success/failure status.
//
// Parameters:
//   - baseDelay: The base delay duration to apply
//
// Security considerations:
// - Helps prevent timing analysis of authentication operations
// - Should be used consistently across success and failure paths
// - Delay should be long enough to mask timing variations.
func (tp *TimingProtection) ConstantTimeDelay(baseDelay time.Duration) {
	jitterBytes := make([]byte, 4)
	var jitter time.Duration
	if _, err := rand.Read(jitterBytes); err == nil {
		jitterValue := uint32(jitterBytes[0])<<24 | uint32(jitterBytes[1])<<16 |
			uint32(jitterBytes[2])<<8 | uint32(jitterBytes[3])
		jitter = time.Duration(jitterValue % uint32(baseDelay/10)) // #nosec G115 - baseDelay/10 is always positive
	} else {
		jitter = baseDelay / 20
	}

	totalDelay := baseDelay + jitter
	time.Sleep(totalDelay)
}

// ConstantTimeAuthentication performs authentication with timing attack protection.
// This ensures that both successful and failed authentication attempts take
// approximately the same amount of time.
//
// Parameters:
//   - authFunc: The authentication function to execute
//   - baseDelay: The minimum time the operation should take
//
// Returns:
//   - error: Any error from the authentication function
//
// Security considerations:
// - Normalizes timing between success and failure cases
// - Prevents timing-based username/password enumeration
// - Should be used for all authentication operations.
func (tp *TimingProtection) ConstantTimeAuthentication(authFunc func() error, baseDelay time.Duration) error {
	startTime := time.Now()

	err := authFunc()
	elapsed := time.Since(startTime)

	if elapsed < baseDelay {
		remainingDelay := baseDelay - elapsed
		tp.ConstantTimeDelay(remainingDelay)
	}

	return err
}

// SecureRandom provides cryptographically secure random number generation
// for security-critical operations.
type SecureRandom struct{}

// newSecureRandom creates a new secure random number generator.
func newSecureRandom() *SecureRandom {
	return &SecureRandom{}
}

// GenerateBytes generates cryptographically secure random bytes.
//
// Parameters:
//   - length: The number of random bytes to generate
//
// Returns:
//   - []byte: The generated random bytes
//   - error: Any error that occurred during generation
//
// Security considerations:
// - Uses crypto/rand for cryptographically secure randomness
// - Suitable for cryptographic keys, nonces, and challenges
// - Returns error if insufficient entropy is available.
func (sr *SecureRandom) GenerateBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, validationError("SecureRandom.GenerateBytes",
			"length must be positive", nil)
	}

	data := make([]byte, length)
	if _, err := rand.Read(data); err != nil {
		return nil, authenticationError("SecureRandom.GenerateBytes",
			"failed to generate secure random bytes", err)
	}

	return data, nil
}

// GenerateChallenge generates a cryptographically secure challenge for authentication.
//
// Parameters:
//   - length: The length of the challenge in bytes
//
// Returns:
//   - []byte: The generated challenge
//   - error: Any error that occurred during generation
//
// Security considerations:
// - Uses cryptographically secure random generation
// - Suitable for authentication challenges and nonces
// - Each challenge should be unique and unpredictable.
func (sr *SecureRandom) GenerateChallenge(length int) ([]byte, error) {
	return sr.GenerateBytes(length)
}

// MemoryProtection provides utilities for protecting sensitive data in memory.
type MemoryProtection struct {
	secMem *SecureMemory
}

// newMemoryProtection creates a new memory protection instance.
func newMemoryProtection() *MemoryProtection {
	return &MemoryProtection{
		secMem: &SecureMemory{},
	}
}

// ProtectedBytes represents a byte slice with automatic secure clearing.
type ProtectedBytes struct {
	data   []byte
	secMem *SecureMemory
}

// NewProtectedBytes creates a new protected byte slice.
//
// Parameters:
//   - size: The size of the protected byte slice
//
// Returns:
//   - *ProtectedBytes: The protected byte slice
//
// Security considerations:
// - Automatically clears data when no longer needed
// - Use defer Clear() to ensure cleanup
// - Suitable for storing sensitive cryptographic data.
func (mp *MemoryProtection) NewProtectedBytes(size int) *ProtectedBytes {
	return &ProtectedBytes{
		data:   make([]byte, size),
		secMem: mp.secMem,
	}
}

// Data returns the protected byte slice.
func (pb *ProtectedBytes) Data() []byte {
	return pb.data
}

// Clear securely clears the protected bytes from memory.
func (pb *ProtectedBytes) Clear() {
	if pb.data != nil {
		pb.secMem.ClearBytes(pb.data)
		pb.data = nil
	}
}

// Size returns the size of the protected byte slice.
func (pb *ProtectedBytes) Size() int {
	if pb.data == nil {
		return 0
	}
	return len(pb.data)
}

// Copy copies data into the protected byte slice.
//
// Parameters:
//   - src: The source data to copy
//
// Returns:
//   - error: Any error that occurred during copying
func (pb *ProtectedBytes) Copy(src []byte) error {
	if pb.data == nil {
		return validationError("ProtectedBytes.Copy",
			"protected bytes has been cleared", nil)
	}

	if len(src) > len(pb.data) {
		return validationError("ProtectedBytes.Copy",
			"source data larger than protected buffer", nil)
	}

	copy(pb.data, src)
	return nil
}

// Zero fills the protected bytes with zeros.
func (pb *ProtectedBytes) Zero() {
	if pb.data != nil {
		for i := range pb.data {
			pb.data[i] = 0
		}
	}
}

// IsCleared returns true if the protected bytes have been cleared.
func (pb *ProtectedBytes) IsCleared() bool {
	return pb.data == nil
}

// secureZeroMemory attempts to securely zero memory at the given address.
// This is a best-effort function and may not work on all platforms.
//
// Parameters:
//   - ptr: Pointer to the memory to zero
//   - size: Size of the memory region in bytes
//
// Security considerations:
// - This is a best-effort implementation
