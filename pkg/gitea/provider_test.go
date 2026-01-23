// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package gitea

import (
	"testing"
)

func TestNewProvider_RequiresBaseURL(t *testing.T) {
	_, err := NewProvider("token", "")
	if err == nil {
		t.Error("Expected error when baseURL is empty")
	}
}

func TestProviderOptions(t *testing.T) {
	opts := ProviderOptions{
		Token:   "test-token",
		BaseURL: "https://gitea.example.com",
	}

	if opts.Token != "test-token" {
		t.Errorf("Token = %q, want %q", opts.Token, "test-token")
	}
	if opts.BaseURL != "https://gitea.example.com" {
		t.Errorf("BaseURL = %q, want %q", opts.BaseURL, "https://gitea.example.com")
	}
}

func TestProvider_Name(t *testing.T) {
	// Test Name() method directly on a minimal provider struct
	// without creating a real client
	p := &Provider{}
	if p.Name() != "gitea" {
		t.Errorf("Name() = %q, want %q", p.Name(), "gitea")
	}
}

func TestProvider_ValidateToken_EmptyToken(t *testing.T) {
	// Test with empty token without network call
	p := &Provider{
		token: "",
	}

	valid, err := p.ValidateToken(nil)
	if err != nil {
		t.Errorf("ValidateToken returned error: %v", err)
	}
	if valid {
		t.Error("ValidateToken should return false for empty token")
	}
}

// Note: Integration tests that require network access should be in a separate
// file with build tag: //go:build integration
