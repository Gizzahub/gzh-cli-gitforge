// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// TestValidateHostNameCatalogGrammar pins host fields to the device catalog
// grammar ^[a-z0-9]+(?:-[a-z0-9]+)*$ and nothing looser.
func TestValidateHostNameCatalogGrammar(t *testing.T) {
	t.Parallel()
	for _, valid := range []string{"mbp", "mst", "a1-b2", "0", "lab-2-x", strings.Repeat("a", 128)} {
		if err := validateHostName(valid, "host"); err != nil {
			t.Errorf("validateHostName(%q) = %v, want nil", valid, err)
		}
	}
	for _, invalid := range []string{"", "MBP", "Mbp", "m;x", "-a", "a-", "a--b", "mbé", "ホスト", "m bp", " mbp", "a_b", "a.b", "a/b", "mbp\n", strings.Repeat("a", 129)} {
		err := validateHostName(invalid, "owner-host")
		if err == nil {
			t.Errorf("validateHostName(%q) = nil, want rejection", invalid)
			continue
		}
		if !errors.Is(err, ErrInvalidExecutionContext) || !strings.Contains(err.Error(), "owner-host") {
			t.Errorf("validateHostName(%q) = %v, want ErrInvalidExecutionContext naming the field", invalid, err)
		}
	}
}

// TestHostFieldsUseCatalogGrammar covers each host field: the run owner, the
// legacy cleanup plan owner, and only the host. Actor stays an opaque token.
func TestHostFieldsUseCatalogGrammar(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("a", 40)
	ctx := ExecutionContext{ExecutionID: "Run_42", Generation: "G.7", Owner: OwnerIdentity{Actor: "Claude_Agent", Host: "mbp", Kind: "agent"}, ExpectSource: sha}
	if err := ctx.Validate(); err != nil {
		t.Fatalf("opaque execution-id/generation/actor with a catalog host: %v", err)
	}
	ctx.Owner.Host = "m;x"
	if err := ctx.Validate(); !errors.Is(err, ErrInvalidExecutionContext) || !strings.Contains(err.Error(), "owner-host") {
		t.Fatalf("ExecutionContext.Validate with host m;x = %v, want owner-host rejection", err)
	}
	if err := validateOwner(OwnerIdentity{Actor: "Claude_Agent", Host: "mst", Kind: "human"}); err != nil {
		t.Fatalf("validateOwner with a catalog host: %v", err)
	}
	if err := validateOwner(OwnerIdentity{Actor: "codex", Host: "MST", Kind: "agent"}); err == nil || !strings.Contains(err.Error(), "owner host") {
		t.Fatalf("validateOwner with host MST = %v, want owner host rejection", err)
	}
}

func TestDeviceIdentityLabel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		device DeviceIdentity
		want   string
	}{
		{DeviceIdentity{Host: "mst"}, "mst"},
		{DeviceIdentity{}, DeviceUndeclared},
		{DeviceIdentity{Rejected: true}, DeviceInvalid},
	} {
		if got := tc.device.Label(); got != tc.want {
			t.Errorf("%+v.Label() = %q, want %q", tc.device, got, tc.want)
		}
	}
}

func TestFormatRunProvenance(t *testing.T) {
	const hint = "integrate: declare the device with GIT_WORK_HOST=<name> or git config --global ce.workHost <name> (lowercase letters, digits, single hyphens)\n"
	const invalidHint = "integrate: the declared device name is invalid; fix GIT_WORK_HOST or git config --global ce.workHost to lowercase letters, digits, single hyphens\n"
	for _, tc := range []struct {
		name    string
		device  DeviceIdentity
		version string
		want    string
	}{
		{"declared", DeviceIdentity{Host: "mst"}, "0.10.6", "integrate: run device=mst gz-git=0.10.6\n"},
		{"undeclared", DeviceIdentity{}, "0.10.6", "integrate: run device=undeclared gz-git=0.10.6\n" + hint},
		{"invalid", DeviceIdentity{Rejected: true}, "0.10.6", "integrate: run device=invalid gz-git=0.10.6\n" + invalidHint},
		{"unstamped build", DeviceIdentity{Host: "mbp"}, "", "integrate: run device=mbp gz-git=unknown\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteRunProvenance(&out, tc.device, tc.version); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("provenance = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestResolveCallerOwnerHostGrammar separates a missing device from a
// malformed one, so the caller learns which to fix.
func TestResolveCallerOwnerHostGrammar(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	// An empty declared value; a missing key is a Git read error instead.
	if err := os.WriteFile(global, []byte("[ce]\n\tworkHost = \"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GZ_GIT_TASK_ACTOR", "claude")
	exec := gitcmd.NewExecutor()

	t.Setenv("GIT_WORK_HOST", "mbp")
	if owner, err := ResolveCallerOwner(context.Background(), exec); err != nil || owner.Host != "mbp" {
		t.Fatalf("declared host: owner = %+v err = %v", owner, err)
	}
	for _, bad := range []string{"m;x", "MBP", strings.Repeat("a", 129)} {
		t.Setenv("GIT_WORK_HOST", bad)
		_, err := ResolveCallerOwner(context.Background(), exec)
		if err == nil || !strings.HasPrefix(err.Error(), "device identity is invalid: host ") || !strings.Contains(err.Error(), "GIT_WORK_HOST") {
			t.Fatalf("malformed host %q: err = %v, want invalid device identity", bad, err)
		}
		// The caller's own device is not an execution-context field.
		if errors.Is(err, ErrInvalidExecutionContext) {
			t.Fatalf("malformed host %q: err = %v, must not wrap ErrInvalidExecutionContext", bad, err)
		}
	}
	t.Setenv("GIT_WORK_HOST", "")
	if _, err := ResolveCallerOwner(context.Background(), exec); err == nil || !strings.Contains(err.Error(), "device identity is not configured") {
		t.Fatalf("missing host: err = %v, want unconfigured device identity", err)
	}
}

func TestResolveDeclaredDevice(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[ce]\n\tworkHost = lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	g := newGitRepo(gitcmd.NewExecutor(), t.TempDir())
	ctx := context.Background()

	for _, tc := range []struct {
		env, want string
		rejected  bool
	}{
		{"mst", "mst", false},     // environment wins over global Git config
		{"", "lab", false},        // global ce.workHost
		{"bad host", "", true},    // malformed declaration is not repaired or guessed
		{"-mbp", "", true},        // option-like value is refused
		{"  mbp  ", "mbp", false}, // surrounding space is not part of the name
		{"MBP", "", true},         // outside the device catalog grammar
		{"m;x", "", true},         // a separator would split the notice field
	} {
		t.Setenv("GIT_WORK_HOST", tc.env)
		if got := resolveDeclaredDevice(ctx, g); got.Host != tc.want || got.Rejected != tc.rejected {
			t.Errorf("GIT_WORK_HOST=%q: host = %q rejected = %v, want %q rejected = %v", tc.env, got.Host, got.Rejected, tc.want, tc.rejected)
		}
	}

	t.Setenv("GIT_WORK_HOST", "")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "missing"))
	if got := resolveDeclaredDevice(ctx, g); got.Host != "" || got.Rejected {
		t.Errorf("undeclared device: host = %q rejected = %v, want empty and not rejected (no hostname fallback)", got.Host, got.Rejected)
	}
}
