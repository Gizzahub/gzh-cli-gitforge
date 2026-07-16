// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab"
)

// TestNewForgeProviderWithAuth_SSHPortWiring locks the drift fix from task 12:
// the GitLab SSH port must survive provider construction identically regardless
// of which former call site (forge from / forge setup / doctor) requested the
// provider. All of them now funnel through NewForgeProviderWithAuth, so this
// asserts (a) the single owner honors the port and (b) the sync entry point
// CreateForgeProviderRaw preserves it — exactly the wiring doctor used to omit.
// doctor calls NewForgeProviderWithAuth directly, so covering the owner covers
// the doctor path too.
func TestNewForgeProviderWithAuth_SSHPortWiring(t *testing.T) {
	const baseURL = "https://gitlab.example.com"

	for _, tc := range []struct {
		name    string
		sshPort int
	}{
		{"custom port", 2224},
		{"default port", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Owner / doctor path.
			p, err := NewForgeProviderWithAuth("gitlab", "tok", baseURL, tc.sshPort)
			if err != nil {
				t.Fatalf("NewForgeProviderWithAuth(gitlab): %v", err)
			}
			gl, ok := p.(*gitlab.Provider)
			if !ok {
				t.Fatalf("expected *gitlab.Provider, got %T", p)
			}
			if gl.SSHPort() != tc.sshPort {
				t.Errorf("owner: gitlab SSHPort = %d, want %d", gl.SSHPort(), tc.sshPort)
			}

			// Sync entry point (forge from / forge setup). Unwrap the narrow
			// ForgeProvider adapter to reach the concrete provider and confirm it
			// carries the same port — the divergence this task removes.
			fp, err := CreateForgeProviderRaw("gitlab", "tok", baseURL, tc.sshPort)
			if err != nil {
				t.Fatalf("CreateForgeProviderRaw(gitlab): %v", err)
			}
			adapter, ok := fp.(forgeProviderAdapter)
			if !ok {
				t.Fatalf("expected forgeProviderAdapter, got %T", fp)
			}
			glSync, ok := adapter.Provider.(*gitlab.Provider)
			if !ok {
				t.Fatalf("expected *gitlab.Provider inside adapter, got %T", adapter.Provider)
			}
			if glSync.SSHPort() != tc.sshPort {
				t.Errorf("sync entry: gitlab SSHPort = %d, want %d", glSync.SSHPort(), tc.sshPort)
			}
		})
	}
}

// TestNewForgeProviderWithAuth_ProviderMatrix verifies the single constructor
// builds each supported provider uniformly and rejects unknown ones. GitHub
// ignores the SSH port (it has no such field) and constructs offline; Gitea is
// not constructed here because its client performs a server-version probe over
// the network, so only its network-free error path (missing base URL) is
// exercised.
func TestNewForgeProviderWithAuth_ProviderMatrix(t *testing.T) {
	gh, err := NewForgeProviderWithAuth("github", "tok", "", 2224)
	if err != nil {
		t.Fatalf("github: unexpected error: %v", err)
	}
	if gh.Name() != "github" {
		t.Errorf("github Name = %q, want github", gh.Name())
	}

	if _, err := NewForgeProviderWithAuth("gitea", "tok", "", 0); err == nil {
		t.Error("gitea without base URL: expected error, got nil")
	}

	if _, err := NewForgeProviderWithAuth("bitbucket", "tok", "", 0); err == nil {
		t.Error("unsupported provider: expected error, got nil")
	}
}
