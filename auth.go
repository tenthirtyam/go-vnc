// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

// ClientAuth defines the interface for VNC authentication methods.
type ClientAuth interface {
	SecurityType() uint8
	Handshake(ctx context.Context, conn net.Conn) error
	String() string
}

// ClientAuthNone implements the "None" authentication method (security type 1).
type ClientAuthNone struct {
	logger Logger
}

// SecurityType returns the security type identifier for None authentication.
func (c *ClientAuthNone) SecurityType() uint8 {
	return 1
}

// Handshake performs the None authentication handshake.
func (c *ClientAuthNone) Handshake(ctx context.Context, conn net.Conn) error {
	select {
	case <-ctx.Done():
		if c.logger != nil {
			c.logger.Warn("None authentication cancelled by context")
		}
		return timeoutError("ClientAuthNone.Handshake", "authentication cancelled", ctx.Err())
	default:
	}

	if c.logger != nil {
		c.logger.Debug("Performing None authentication handshake")
		c.logger.Info("None authentication completed successfully")
	}

	return nil
}

// String returns a human-readable description of the authentication method.
func (c *ClientAuthNone) String() string {
	return "None"
}

// SetLogger sets the logger for the authentication method.
func (c *ClientAuthNone) SetLogger(logger Logger) {
	c.logger = logger
}

// PasswordAuth implements VNC Authentication (security type 2).
type PasswordAuth struct {
	Password     string
	logger       Logger
	secureMemory *SecureMemory
}

// NewPasswordAuth creates a new PasswordAuth instance with enhanced security features.
func NewPasswordAuth(password string) *PasswordAuth {
	return &PasswordAuth{
		Password:     password,
		secureMemory: &SecureMemory{},
	}
}

// SecurityType returns the security type identifier for VNC Password authentication.
func (p *PasswordAuth) SecurityType() uint8 {
	return 2
}

// Handshake performs the VNC Authentication handshake with the server.
func (p *PasswordAuth) Handshake(ctx context.Context, c net.Conn) error {
	select {
	case <-ctx.Done():
		if p.logger != nil {
			p.logger.Warn("VNC authentication cancelled by context")
		}
		return timeoutError("PasswordAuth.Handshake", "authentication cancelled", ctx.Err())
	default:
	}

	if p.logger != nil {
		p.logger.Debug("Starting VNC password authentication handshake")

		if len(p.Password) > VNCMaxPasswordLength {
			p.logger.Warn("Password exceeds VNC maximum length, will be truncated for DES encryption",
				Field{Key: "password_length", Value: len(p.Password)})
		}

		if len(p.Password) == 0 {
			p.logger.Warn("Empty password provided for VNC authentication")
		}
	}

	if p.secureMemory == nil {
		p.secureMemory = &SecureMemory{}
	}

	memProtection := newMemoryProtection()
	challengeBuffer := memProtection.NewProtectedBytes(VNCChallengeSize)
	defer challengeBuffer.Clear()

	if err := binary.Read(c, binary.BigEndian, challengeBuffer.Data()); err != nil {
		if p.logger != nil {
			p.logger.Error("Failed to read authentication challenge from server",
				Field{Key: "error", Value: err})
		}
		return networkError("PasswordAuth.Handshake", "failed to read authentication challenge", err)
	}

	if p.logger != nil {
		p.logger.Debug("Received authentication challenge from server",
			Field{Key: "challenge_length", Value: challengeBuffer.Size()})
	}

	select {
	case <-ctx.Done():
		if p.logger != nil {
			p.logger.Warn("VNC authentication cancelled during encryption")
		}
		return timeoutError("PasswordAuth.Handshake", "authentication cancelled during encryption", ctx.Err())
	default:
	}

	crypted, err := p.encrypt(p.Password, challengeBuffer.Data())
	if err != nil {
		if p.logger != nil {
			p.logger.Error("Failed to encrypt password challenge",
				Field{Key: "error", Value: err})
		}
		return authenticationError("PasswordAuth.Handshake", "failed to encrypt password", err)
	}

	responseBuffer := memProtection.NewProtectedBytes(len(crypted))
	defer responseBuffer.Clear()

	if err := responseBuffer.Copy(crypted); err != nil {
		if p.logger != nil {
			p.logger.Error("Failed to copy encrypted response to protected buffer",
				Field{Key: "error", Value: err})
		}
		return authenticationError("PasswordAuth.Handshake", "failed to prepare encrypted response", err)
	}

	if p.secureMemory != nil {
		p.secureMemory.ClearBytes(crypted)
	}

	if p.logger != nil {
		p.logger.Debug("Successfully encrypted authentication challenge")
	}

	if err := binary.Write(c, binary.BigEndian, responseBuffer.Data()); err != nil {
		if p.logger != nil {
			p.logger.Error("Failed to send encrypted password response",
				Field{Key: "error", Value: err})
		}
		return networkError("PasswordAuth.Handshake", "failed to send encrypted password", err)
	}

	if p.logger != nil {
		p.logger.Debug("VNC password authentication handshake completed")
	}

	return nil
}

// String returns a human-readable description of the authentication method.
func (p *PasswordAuth) String() string {
	return "VNC Password"
}

// SetLogger sets the logger for the authentication method.
func (p *PasswordAuth) SetLogger(logger Logger) {
	p.logger = logger
}

// ClearPassword securely clears the password from memory.
func (p *PasswordAuth) ClearPassword() {
	if p.secureMemory != nil && p.Password != "" {
		p.Password = p.secureMemory.ClearString(p.Password)
	}
}

// encrypt performs DES encryption of the challenge using the provided password.
func (p *PasswordAuth) encrypt(key string, bytes []byte) ([]byte, error) {
	secureCipher := newSecureDESCipher()
	timingProtection := newTimingProtection()

	var result []byte
	var encryptErr error

	err := timingProtection.ConstantTimeAuthentication(func() error {
		var err error
		result, err = secureCipher.EncryptVNCChallenge(key, bytes)
		encryptErr = err
		return err
	}, 50*time.Millisecond)

	if err != nil {
		return nil, err
	}

	if encryptErr != nil {
		return nil, encryptErr
	}

	return result, nil
}

// AuthFactory is a function type that creates new instances of authentication methods.
type AuthFactory func() ClientAuth

// AuthRegistry manages available authentication methods.
type AuthRegistry struct {
	factories map[uint8]AuthFactory
	mu        sync.RWMutex
	logger    Logger
}

// NewAuthRegistry creates a new authentication registry with default authentication methods.
func NewAuthRegistry() *AuthRegistry {
	registry := &AuthRegistry{
		factories: make(map[uint8]AuthFactory),
		logger:    &NoOpLogger{},
	}

	registry.Register(1, func() ClientAuth {
		return &ClientAuthNone{}
	})

	registry.Register(2, func() ClientAuth {
		return &PasswordAuth{}
	})

	return registry
}

// Register adds an authentication method factory to the registry.
func (r *AuthRegistry) Register(securityType uint8, factory AuthFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.logger != nil {
		r.logger.Debug("Registering authentication method",
			Field{Key: "security_type", Value: securityType})
	}

	r.factories[securityType] = factory
}

// Unregister removes an authentication method from the registry.
func (r *AuthRegistry) Unregister(securityType uint8) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[securityType]; exists {
		delete(r.factories, securityType)

		if r.logger != nil {
			r.logger.Debug("Unregistered authentication method",
				Field{Key: "security_type", Value: securityType})
		}

		return true
	}

	return false
}

// CreateAuth creates a new instance of the authentication method for the given security type.
func (r *AuthRegistry) CreateAuth(securityType uint8) (ClientAuth, error) {
	r.mu.RLock()
	factory, exists := r.factories[securityType]
	r.mu.RUnlock()

	if !exists {
		if r.logger != nil {
			r.logger.Warn("Unsupported authentication method requested",
				Field{Key: "security_type", Value: securityType})
		}
		return nil, unsupportedError("AuthRegistry.CreateAuth",
			fmt.Sprintf("unsupported security type: %d", securityType), nil)
	}

	auth := factory()

	if r.logger != nil {
		r.logger.Debug("Created authentication method instance",
			Field{Key: "security_type", Value: securityType},
			Field{Key: "method", Value: auth.String()})
	}

	return auth, nil
}

// GetSupportedTypes returns a list of all supported security types.
func (r *AuthRegistry) GetSupportedTypes() []uint8 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]uint8, 0, len(r.factories))
	for securityType := range r.factories {
		types = append(types, securityType)
	}

	return types
}

// IsSupported checks if a security type is supported by the registry.
func (r *AuthRegistry) IsSupported(securityType uint8) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[securityType]
	return exists
}

// SetLogger sets the logger for the authentication registry.
func (r *AuthRegistry) SetLogger(logger Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger = logger
}

// NegotiateAuth performs authentication method negotiation between client and server.
func (r *AuthRegistry) NegotiateAuth(ctx context.Context, serverTypes []uint8, preferredOrder []uint8) (ClientAuth, uint8, error) {
	select {
	case <-ctx.Done():
		if r.logger != nil {
			r.logger.Warn("Authentication negotiation cancelled by context")
		}
		return nil, 0, timeoutError("AuthRegistry.NegotiateAuth", "negotiation cancelled", ctx.Err())
	default:
	}

	if r.logger != nil {
		r.logger.Debug("Starting authentication negotiation",
			Field{Key: "server_types", Value: serverTypes},
			Field{Key: "preferred_order", Value: preferredOrder})
	}

	if preferredOrder == nil {
		preferredOrder = serverTypes
	}

	for _, preferredType := range preferredOrder {
		for _, serverType := range serverTypes {
			if preferredType == serverType && r.IsSupported(preferredType) {
				auth, err := r.CreateAuth(preferredType)
				if err != nil {
					if r.logger != nil {
						r.logger.Error("Failed to create authentication method during negotiation",
							Field{Key: "security_type", Value: preferredType},
							Field{Key: "error", Value: err})
					}
					continue
				}

				if r.logger != nil {
					r.logger.Info("Authentication method negotiated successfully",
						Field{Key: "security_type", Value: preferredType},
						Field{Key: "method", Value: auth.String()})
				}

				return auth, preferredType, nil
			}
		}
	}

	supportedTypes := r.GetSupportedTypes()
	if r.logger != nil {
		r.logger.Error("No mutual authentication method found",
			Field{Key: "server_types", Value: serverTypes},
			Field{Key: "client_types", Value: supportedTypes})
	}

	return nil, 0, unsupportedError("AuthRegistry.NegotiateAuth",
		fmt.Sprintf("no mutual authentication method found. server: %v, client: %v", serverTypes, supportedTypes), nil)
}

// ValidateAuthMethod performs validation on an authentication method instance.
func (r *AuthRegistry) ValidateAuthMethod(auth ClientAuth) error {
	if auth == nil {
		return validationError("AuthRegistry.ValidateAuthMethod", "authentication method is nil", nil)
	}

	securityType := auth.SecurityType()
	if securityType == 0 {
		return validationError("AuthRegistry.ValidateAuthMethod", "invalid security type 0", nil)
	}

	switch a := auth.(type) {
	case *PasswordAuth:
		if a.Password == "" {
			if r.logger != nil {
				r.logger.Warn("Password authentication method has empty password")
			}
			return validationError("AuthRegistry.ValidateAuthMethod", "password authentication requires non-empty password", nil)
		}
		if len(a.Password) > VNCMaxPasswordLength {
			if r.logger != nil {
				r.logger.Warn("Password exceeds VNC maximum length",
					Field{Key: "length", Value: len(a.Password)})
			}
		}
	case *ClientAuthNone:
		// No validation required.
	default:
		if r.logger != nil {
			r.logger.Debug("Validating custom authentication method",
				Field{Key: "method", Value: auth.String()},
				Field{Key: "security_type", Value: securityType})
		}
	}

	if r.logger != nil {
		r.logger.Debug("Authentication method validation successful",
			Field{Key: "method", Value: auth.String()},
			Field{Key: "security_type", Value: securityType})
	}

	return nil
}
