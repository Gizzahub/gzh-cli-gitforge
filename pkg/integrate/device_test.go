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
	for _, valid := range []string{"mbp", "mst", "a1-b2", "0", "lab-2-x"} {
		if err := validateHostName(valid, "host"); err != nil {
			t.Errorf("validateHostName(%q) = %v, want nil", valid, err)
		}
	}
	for _, invalid := range []string{"", "MBP", "Mbp", "m;x", "-a", "a-", "a--b", "mbé", "ホスト", "m bp", " mbp", "a_b", "a.b", "a/b", "mbp\n"} {
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
		if got := hostLockDeviceLabel(hostLockHolder{Host: tc.device.Host, hostRejected: tc.device.Rejected}); got != "host "+tc.want {
			t.Errorf("host-lock label for %+v = %q, want %q", tc.device, got, "host "+tc.want)
		}
	}
}

func TestFormatRunProvenance(t *testing.T) {
	const hint = "integrate: declare the device with GIT_WORK_HOST=<name> or git config --global ce.workHost <name> (lowercase letters, digits, single hyphens)\n"
	for _, tc := range []struct {
		name    string
		device  DeviceIdentity
		version string
		want    string
	}{
		{"declared", DeviceIdentity{Host: "mst"}, "0.10.6", "integrate: run device=mst gz-git=0.10.6\n"},
		{"undeclared", DeviceIdentity{}, "0.10.6", "integrate: run device=undeclared gz-git=0.10.6\n" + hint},
		{"invalid", DeviceIdentity{Rejected: true}, "0.10.6", "integrate: run device=invalid gz-git=0.10.6\n" + hint},
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
	t.Setenv("GIT_WORK_HOST", "m;x")
	if _, err := ResolveCallerOwner(context.Background(), exec); err == nil || !strings.Contains(err.Error(), "device identity is invalid") {
		t.Fatalf("malformed host: err = %v, want invalid device identity", err)
	}
	t.Setenv("GIT_WORK_HOST", "")
	if _, err := ResolveCallerOwner(context.Background(), exec); err == nil || !strings.Contains(err.Error(), "device identity is not configured") {
		t.Fatalf("missing host: err = %v, want unconfigured device identity", err)
	}
}
