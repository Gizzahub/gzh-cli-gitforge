// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"testing"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider("test-token", "")

	if provider.Name() != "github" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "github")
	}

	if provider.token != "test-token" {
		t.Errorf("token = %q, want %q", provider.token, "test-token")
	}

	if provider.baseURL != "" {
		t.Errorf("baseURL = %q, want empty", provider.baseURL)
	}

	if provider.client == nil {
		t.Error("client should not be nil")
	}
}

func TestNewProvider_EmptyToken(t *testing.T) {
	provider := NewProvider("", "")

	if provider.Name() != "github" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "github")
	}

	// Client should still be created (for unauthenticated access)
	if provider.client == nil {
		t.Error("client should not be nil even with empty token")
	}
}

func TestNewProvider_EnterpriseBaseURL(t *testing.T) {
	provider := NewProvider("token", "https://github.example.com")

	if provider.baseURL != "https://github.example.com" {
		t.Errorf("baseURL = %q, want %q", provider.baseURL, "https://github.example.com")
	}
	if provider.client == nil {
		t.Error("client should not be nil for enterprise URL")
	}
}

func TestNewProvider_NormalizesBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://github.example.com/":    "https://github.example.com", // trailing slash stripped
		"  https://github.example.com  ": "https://github.example.com", // whitespace trimmed
		"https://github.example.com//":   "https://github.example.com", // repeated trailing slashes stripped
		"ftp://github.example.com":       "",                           // non-http(s) scheme rejected
		"github.example.com":             "",                           // missing scheme rejected
	}
	for input, want := range cases {
		p := NewProvider("token", input)
		if p.baseURL != want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", input, p.baseURL, want)
		}
		// client must never be nil, even when baseURL collapses to ""
		if p.client == nil {
			t.Errorf("client is nil for baseURL %q", input)
		}
	}
}

func TestNewProviderWithOptions(t *testing.T) {
	p := NewProviderWithOptions(ProviderOptions{
		Token:   "tok",
		BaseURL: "https://ghe.acme.io",
	})
	if p.token != "tok" {
		t.Errorf("token = %q, want %q", p.token, "tok")
	}
	if p.baseURL != "https://ghe.acme.io" {
		t.Errorf("baseURL = %q, want %q", p.baseURL, "https://ghe.acme.io")
	}
	if p.client == nil {
		t.Error("client should not be nil")
	}
}

func TestProvider_SetToken(t *testing.T) {
	provider := NewProvider("initial-token", "https://github.example.com")

	err := provider.SetToken("new-token")
	if err != nil {
		t.Errorf("SetToken failed: %v", err)
	}

	if provider.token != "new-token" {
		t.Errorf("token = %q, want %q", provider.token, "new-token")
	}

	// SetToken must preserve the configured base URL
	if provider.baseURL != "https://github.example.com" {
		t.Errorf("baseURL = %q, want %q", provider.baseURL, "https://github.example.com")
	}
	if provider.client == nil {
		t.Error("client should not be nil after SetToken")
	}
}

func TestProvider_ValidateToken_EmptyToken(t *testing.T) {
	provider := NewProvider("", "")

	valid, err := provider.ValidateToken(context.TODO())
	if err != nil {
		t.Errorf("ValidateToken returned error: %v", err)
	}
	if valid {
		t.Error("ValidateToken should return false for empty token")
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider("token", "")

	if provider.Name() != "github" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "github")
	}
}

func TestBaseURLAccessor(t *testing.T) {
	if got := NewProvider("token", "").BaseURL(); got != "" {
		t.Errorf("BaseURL() = %q, want empty", got)
	}
	want := "https://github.example.com"
	if got := NewProvider("token", want).BaseURL(); got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

func TestEnterpriseUploadURL(t *testing.T) {
	cases := map[string]string{
		"https://github.example.com":  "https://github.example.com/api/uploads",
		"https://github.example.com/": "https://github.example.com/api/uploads",
		"https://ghe.io:8443":         "https://ghe.io:8443/api/uploads",
	}
	for base, want := range cases {
		if got := enterpriseUploadURL(base); got != want {
			t.Errorf("enterpriseUploadURL(%q) = %q, want %q", base, got, want)
		}
	}
}
