// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"context"
	"testing"
)

// TestAuthRegistry_NewAuthRegistry tests the creation of a new authentication registry.
func TestAuthRegistry_New(t *testing.T) {
	registry := NewAuthRegistry()

	if registry == nil {
		t.Fatal("NewAuthRegistry returned nil")
	}

	// Check that default authentication methods are registered
	if !registry.IsSupported(1) {
		t.Error("None authentication should be supported by default")
	}

	if !registry.IsSupported(2) {
		t.Error("VNC password authentication should be supported by default")
	}

	supportedTypes := registry.GetSupportedTypes()
	if len(supportedTypes) < 2 {
		t.Errorf("Expected at least 2 supported types, got %d", len(supportedTypes))
	}
}

// TestAuthRegistry_Register tests registering custom authentication methods.
func TestAuthRegistry_Register(t *testing.T) {
	registry := NewAuthRegistry()

	// Register a custom authentication method
	customSecurityType := uint8(16)
	registry.Register(customSecurityType, func() ClientAuth {
		return &ClientAuthNone{} // Use None as a placeholder for custom auth
	})

	if !registry.IsSupported(customSecurityType) {
		t.Error("Custom authentication method should be supported after registration")
	}

	// Test creating the custom authentication method
	auth, err := registry.CreateAuth(customSecurityType)
	if err != nil {
		t.Fatalf("Failed to create custom authentication method: %v", err)
	}

	if auth == nil {
		t.Error("Created authentication method should not be nil")
	}

	if auth.SecurityType() != 1 { // ClientAuthNone returns 1
		t.Errorf("Expected security type 1, got %d", auth.SecurityType())
	}
}

// TestAuthRegistry_Unregister tests unregistering authentication methods.
func TestAuthRegistry_Unregister(t *testing.T) {
	registry := NewAuthRegistry()

	// Register a custom method first
	customSecurityType := uint8(16)
	registry.Register(customSecurityType, func() ClientAuth {
		return &ClientAuthNone{}
	})

	// Verify it's registered
	if !registry.IsSupported(customSecurityType) {
		t.Error("Custom authentication method should be supported after registration")
	}

	// Unregister it
	removed := registry.Unregister(customSecurityType)
	if !removed {
		t.Error("Unregister should return true when removing existing method")
	}

	// Verify it's no longer supported
	if registry.IsSupported(customSecurityType) {
		t.Error("Custom authentication method should not be supported after unregistration")
	}

	// Try to unregister non-existent method
	removed = registry.Unregister(99)
	if removed {
		t.Error("Unregister should return false when removing non-existent method")
	}
}

// TestAuthRegistry_CreateAuth tests creating authentication method instances.
func TestAuthRegistry_CreateAuth(t *testing.T) {
	registry := NewAuthRegistry()

	// Test creating None authentication
	auth, err := registry.CreateAuth(1)
	if err != nil {
		t.Fatalf("Failed to create None authentication: %v", err)
	}

	if auth.SecurityType() != 1 {
		t.Errorf("Expected security type 1, got %d", auth.SecurityType())
	}

	// Test creating VNC password authentication
	auth, err = registry.CreateAuth(2)
	if err != nil {
		t.Fatalf("Failed to create VNC password authentication: %v", err)
	}

	if auth.SecurityType() != 2 {
		t.Errorf("Expected security type 2, got %d", auth.SecurityType())
	}

	// Test creating unsupported authentication method
	_, err = registry.CreateAuth(99)
	if err == nil {
		t.Error("Expected error when creating unsupported authentication method")
	}

	if !IsVNCError(err, ErrUnsupported) {
		t.Errorf("Expected UnsupportedError, got %T", err)
	}
}

// TestAuthRegistry_NegotiateAuth tests authentication method negotiation.
func TestAuthRegistry_NegotiateAuth(t *testing.T) {
	registry := NewAuthRegistry()
	ctx := context.Background()

	// Test successful negotiation with mutual support
	serverTypes := []uint8{1, 2, 16}
	auth, secType, err := registry.NegotiateAuth(ctx, serverTypes, nil)
	if err != nil {
		t.Fatalf("Negotiation should succeed with mutual support: %v", err)
	}

	if auth == nil {
		t.Error("Negotiated authentication method should not be nil")
	}

	// Should select the first mutually supported type (1 - None)
	if secType != 1 {
		t.Errorf("Expected security type 1, got %d", secType)
	}

	// Test negotiation with preferred order
	preferredOrder := []uint8{2, 1} // Prefer password over none
	auth2, secType2, err2 := registry.NegotiateAuth(ctx, serverTypes, preferredOrder)
	if err2 != nil {
		t.Fatalf("Negotiation should succeed with preferred order: %v", err2)
	}

	if auth2 == nil {
		t.Error("Negotiated authentication method should not be nil")
	}

	// Should select password authentication (type 2) due to preference
	if secType2 != 2 {
		t.Errorf("Expected security type 2 due to preference, got %d", secType2)
	}

	// Test negotiation failure with no mutual support
	unsupportedServerTypes := []uint8{99, 100}
	_, _, err = registry.NegotiateAuth(ctx, unsupportedServerTypes, nil)
	if err == nil {
		t.Error("Expected error when no mutual authentication methods exist")
	}

	if !IsVNCError(err, ErrUnsupported) {
		t.Errorf("Expected UnsupportedError, got %T", err)
	}
}

// TestAuthRegistry_ValidateAuthMethod tests authentication method validation.
func TestAuthRegistry_ValidateAuthMethod(t *testing.T) {
	registry := NewAuthRegistry()

	// Test validating nil authentication method
	err := registry.ValidateAuthMethod(nil)
	if err == nil {
		t.Error("Expected error when validating nil authentication method")
	}

	if !IsVNCError(err, ErrValidation) {
		t.Errorf("Expected ValidationError, got %T", err)
	}

	// Test validating None authentication (should always pass)
	noneAuth := &ClientAuthNone{}
	err = registry.ValidateAuthMethod(noneAuth)
	if err != nil {
		t.Errorf("None authentication validation should pass: %v", err)
	}

	// Test validating password authentication with valid password
	passwordAuth := &PasswordAuth{Password: "secret"}
	err = registry.ValidateAuthMethod(passwordAuth)
	if err != nil {
		t.Errorf("Password authentication with valid password should pass: %v", err)
	}

	// Test validating password authentication with empty password
	emptyPasswordAuth := &PasswordAuth{Password: ""}
	err = registry.ValidateAuthMethod(emptyPasswordAuth)
	if err == nil {
		t.Error("Expected error when validating password authentication with empty password")
	}

	if !IsVNCError(err, ErrValidation) {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

// TestAuthRegistry_ConcurrentAccess tests concurrent access to the registry.
func TestAuthRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewAuthRegistry()

	// Test concurrent registration and access
	done := make(chan bool, 10)

	// Start multiple goroutines that register and access authentication methods
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			securityType := uint8(16 + id) // #nosec G115 - Test code with bounded values

			// Register a method
			registry.Register(securityType, func() ClientAuth {
				return &ClientAuthNone{}
			})

			// Check if it's supported
			if !registry.IsSupported(securityType) {
				t.Errorf("Security type %d should be supported after registration", securityType)
				return
			}

			// Create an instance
			auth, err := registry.CreateAuth(securityType)
			if err != nil {
				t.Errorf("Failed to create auth for type %d: %v", securityType, err)
				return
			}

			if auth == nil {
				t.Errorf("Created auth should not be nil for type %d", securityType)
				return
			}

			// Unregister it
			registry.Unregister(securityType)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestAuthRegistry_SetLogger tests setting a logger for the registry.
func TestAuthRegistry_SetLogger(t *testing.T) {
	registry := NewAuthRegistry()
	logger := &NoOpLogger{}

	registry.SetLogger(logger)

	// The logger is set internally, so we can't directly test it,
	// but we can verify that operations still work correctly
	auth, err := registry.CreateAuth(1)
	if err != nil {
		t.Fatalf("Failed to create auth after setting logger: %v", err)
	}

	if auth == nil {
		t.Error("Created auth should not be nil")
	}
}

// TestAuthRegistryIntegration tests the integration of AuthRegistry with ClientConfig.
func TestAuthRegistry_Integration(t *testing.T) {
	// Create a custom authentication registry
	registry := NewAuthRegistry()

	// Register a custom authentication method (using None as placeholder)
	customSecurityType := uint8(16)
	registry.Register(customSecurityType, func() ClientAuth {
		return &ClientAuthNone{}
	})

	// Create a client config with the registry
	config := &ClientConfig{
		AuthRegistry: registry,
		Logger:       &NoOpLogger{},
	}

	// Test that the registry is properly set
	if config.AuthRegistry == nil {
		t.Fatal("AuthRegistry should not be nil")
	}

	// Test that the registry has the expected methods
	if !config.AuthRegistry.IsSupported(1) {
		t.Error("Registry should support None authentication")
	}

	if !config.AuthRegistry.IsSupported(2) {
		t.Error("Registry should support VNC password authentication")
	}

	if !config.AuthRegistry.IsSupported(customSecurityType) {
		t.Error("Registry should support custom authentication method")
	}
}

// TestAuthRegistryNegotiation tests authentication negotiation using the registry.
func TestAuthRegistry_Negotiation(t *testing.T) {
	registry := NewAuthRegistry()

	// Test negotiation with server types
	serverTypes := []uint8{1, 2}
	ctx := context.Background()

	auth, secType, err := registry.NegotiateAuth(ctx, serverTypes, nil)
	if err != nil {
		t.Fatalf("Negotiation should succeed: %v", err)
	}

	if auth == nil {
		t.Error("Negotiated auth should not be nil")
	}

	if secType != 1 && secType != 2 {
		t.Errorf("Security type should be 1 or 2, got %d", secType)
	}

	// Test with preferred order
	preferredOrder := []uint8{2, 1} // Prefer password over none
	auth2, secType2, err2 := registry.NegotiateAuth(ctx, serverTypes, preferredOrder)
	if err2 != nil {
		t.Fatalf("Negotiation with preference should succeed: %v", err2)
	}

	if auth2 == nil {
		t.Error("Negotiated authentication method should not be nil")
	}

	// Should select password authentication (type 2) due to preference
	if secType2 != 2 {
		t.Errorf("Expected security type 2 due to preference, got %d", secType2)
	}
}

// TestAuthRegistryValidation tests authentication method validation.
func TestAuthRegistry_Validation(t *testing.T) {
	registry := NewAuthRegistry()

	// Test validation of valid authentication methods
	noneAuth := &ClientAuthNone{}
	err := registry.ValidateAuthMethod(noneAuth)
	if err != nil {
		t.Errorf("None auth validation should pass: %v", err)
	}

	passwordAuth := &PasswordAuth{Password: "secret"}
	err = registry.ValidateAuthMethod(passwordAuth)
	if err != nil {
		t.Errorf("Password auth validation should pass: %v", err)
	}

	// Test validation of invalid authentication methods
	emptyPasswordAuth := &PasswordAuth{Password: ""}
	err = registry.ValidateAuthMethod(emptyPasswordAuth)
	if err == nil {
		t.Error("Empty password auth validation should fail")
	}

	if !IsVNCError(err, ErrValidation) {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

// TestAuthRegistryWithLogger tests that the registry properly uses a logger.
func TestAuthRegistry_WithLogger(t *testing.T) {
	registry := NewAuthRegistry()
	logger := &NoOpLogger{} // Use NoOpLogger for testing

	registry.SetLogger(logger)

	// Test that operations work with logger set
	customSecurityType := uint8(16)
	registry.Register(customSecurityType, func() ClientAuth {
		return &ClientAuthNone{}
	})

	auth, err := registry.CreateAuth(customSecurityType)
	if err != nil {
		t.Fatalf("Failed to create auth with logger: %v", err)
	}

	if auth == nil {
		t.Error("Created auth should not be nil")
	}

	// Test negotiation with logger
	serverTypes := []uint8{1, customSecurityType}
	ctx := context.Background()

	auth, secType, err := registry.NegotiateAuth(ctx, serverTypes, nil)
	if err != nil {
		t.Fatalf("Negotiation with logger should succeed: %v", err)
	}

	if auth == nil {
		t.Error("Negotiated auth should not be nil")
	}

	if secType != 1 && secType != customSecurityType {
		t.Errorf("Security type should be 1 or %d, got %d", customSecurityType, secType)
	}
}

// TestBackwardCompatibility tests that the old Auth slice still works.
func TestAuthRegistry_BackwardCompatibility(t *testing.T) {
	// Test that ClientConfig with Auth slice still works (without AuthRegistry)
	config := &ClientConfig{
		Auth: []ClientAuth{
			&ClientAuthNone{},
			&PasswordAuth{Password: "secret"},
		},
		Logger: &NoOpLogger{},
	}

	// Verify that Auth slice is properly set
	if len(config.Auth) != 2 {
		t.Errorf("Expected 2 auth methods, got %d", len(config.Auth))
	}

	// Verify that AuthRegistry is nil (backward compatibility)
	if config.AuthRegistry != nil {
		t.Error("AuthRegistry should be nil for backward compatibility")
	}

	// Test that auth methods have correct security types
	if config.Auth[0].SecurityType() != 1 {
		t.Errorf("First auth method should be type 1, got %d", config.Auth[0].SecurityType())
	}

	if config.Auth[1].SecurityType() != 2 {
		t.Errorf("Second auth method should be type 2, got %d", config.Auth[1].SecurityType())
	}
}

// TestAuthRegistryPrecedence tests that AuthRegistry takes precedence over Auth slice.
func TestAuthRegistry_Precedence(t *testing.T) {
	// Create a registry with only None authentication
	registry := NewAuthRegistry()
	registry.Unregister(2) // Remove password auth to test precedence

	config := &ClientConfig{
		Auth: []ClientAuth{
			&PasswordAuth{Password: "secret"}, // This should be ignored
		},
		AuthRegistry: registry, // This should take precedence
		Logger:       &NoOpLogger{},
	}

	// Verify that both Auth and AuthRegistry are set
	if len(config.Auth) != 1 {
		t.Errorf("Expected 1 auth method in Auth slice, got %d", len(config.Auth))
	}

	if config.AuthRegistry == nil {
		t.Error("AuthRegistry should not be nil")
	}

	// Verify that AuthRegistry doesn't support password auth (type 2)
	if config.AuthRegistry.IsSupported(2) {
		t.Error("AuthRegistry should not support password auth after unregistering")
	}

	// Verify that AuthRegistry still supports None auth (type 1)
	if !config.AuthRegistry.IsSupported(1) {
		t.Error("AuthRegistry should support None auth")
	}
}
