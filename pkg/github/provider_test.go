// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package github

import (
	"testing"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider("test-token")

	if provider.Name() != "github" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "github")
	}

	if provider.token != "test-token" {
		t.Errorf("token = %q, want %q", provider.token, "test-token")
	}

	if provider.client == nil {
		t.Error("client should not be nil")
	}
}

func TestNewProvider_EmptyToken(t *testing.T) {
	provider := NewProvider("")

	if provider.Name() != "github" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "github")
	}

	// Client should still be created (for unauthenticated access)
	if provider.client == nil {
		t.Error("client should not be nil even with empty token")
	}
}

func TestProvider_SetToken(t *testing.T) {
	provider := NewProvider("initial-token")

	err := provider.SetToken("new-token")
	if err != nil {
		t.Errorf("SetToken failed: %v", err)
	}

	if provider.token != "new-token" {
		t.Errorf("token = %q, want %q", provider.token, "new-token")
	}
}

func TestProvider_ValidateToken_EmptyToken(t *testing.T) {
	provider := NewProvider("")

	valid, err := provider.ValidateToken(nil)
	if err != nil {
		t.Errorf("ValidateToken returned error: %v", err)
	}
	if valid {
		t.Error("ValidateToken should return false for empty token")
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider("token")

	if provider.Name() != "github" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "github")
	}
}
