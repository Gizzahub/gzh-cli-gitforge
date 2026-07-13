// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/wizard"
)

// TestSaveWizardConfig_RoundTrips is the AC4 core for task 12. A config written
// by `forge setup` must load and pass validation. The wizard used to write a
// flat map (provider/organization/target/...) that no loader could consume: the
// sync loader detected it as a gzh.yaml (it had an "organization" key) and then
// rejected it for having no repositories array. Reshaping the output to the
// hierarchical config.Config fixes that drift. This asserts the written file
// (a) routes to the workspace loader, not the gzh.yaml path, (b) never leaks a
// raw token, (c) unmarshals into config.Config and passes config.Validator, and
// (d) preserves the wizard's forge-source settings.
func TestSaveWizardConfig_RoundTrips(t *testing.T) {
	const rawToken = "secret-should-not-persist"

	for _, tc := range []struct {
		name string
		opts wizard.SyncSetupOptions
	}{
		{
			name: "gitlab with subgroups and custom ssh port",
			opts: wizard.SyncSetupOptions{
				Provider:         "gitlab",
				Organization:     "myorg",
				BaseURL:          "https://gitlab.example.com",
				Token:            rawToken,
				CloneProto:       "ssh",
				SSHPort:          2224,
				IncludeSubgroups: true,
				SubgroupMode:     "flat",
				TargetPath:       "/tmp/repos",
				IncludePrivate:   true,
				Parallel:         4,
			},
		},
		{
			name: "github minimal",
			opts: wizard.SyncSetupOptions{
				Provider:       "github",
				Organization:   "acme",
				CloneProto:     "https",
				TargetPath:     "/tmp/acme",
				IncludePrivate: true,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			opts.ConfigPath = filepath.Join(t.TempDir(), "sync.yaml")

			if err := saveWizardConfig(&opts); err != nil {
				t.Fatalf("saveWizardConfig: %v", err)
			}

			raw, err := os.ReadFile(opts.ConfigPath)
			if err != nil {
				t.Fatalf("read written config: %v", err)
			}

			// (a) The saved file must route to the workspace loader, never the
			// gzh.yaml path that requires a repositories array.
			if kind := detectConfigKind(raw, opts.ConfigPath); kind != config.KindWorkspace {
				t.Errorf("detected kind = %q, want %q\n%s", kind, config.KindWorkspace, raw)
			}

			// (b) The raw token must never be written to disk.
			if opts.Token != "" && strings.Contains(string(raw), rawToken) {
				t.Errorf("raw token leaked into config:\n%s", raw)
			}

			// (c) The written config must load into config.Config and pass the
			// shared validator — the round-trip AC4 requires.
			var loaded config.Config
			if err := yaml.Unmarshal(raw, &loaded); err != nil {
				t.Fatalf("unmarshal written config: %v", err)
			}
			if err := config.NewValidator().ValidateConfig(&loaded); err != nil {
				t.Fatalf("written config failed validation: %v\n%s", err, raw)
			}

			// (d) Key wizard settings must survive the round-trip.
			ws, ok := loaded.Workspaces[opts.Organization]
			if !ok {
				t.Fatalf("workspace %q missing from loaded config; got %v", opts.Organization, loaded.Workspaces)
			}
			if ws.Source == nil {
				t.Fatal("workspace source is nil")
			}
			if ws.Source.Provider != opts.Provider {
				t.Errorf("provider = %q, want %q", ws.Source.Provider, opts.Provider)
			}
			if ws.Source.Org != opts.Organization {
				t.Errorf("org = %q, want %q", ws.Source.Org, opts.Organization)
			}
			if ws.Path != opts.TargetPath {
				t.Errorf("path = %q, want %q", ws.Path, opts.TargetPath)
			}
			if opts.SSHPort != 0 && ws.SSHPort != opts.SSHPort {
				t.Errorf("sshPort = %d, want %d", ws.SSHPort, opts.SSHPort)
			}
		})
	}
}

// TestSaveWizardConfig_RejectsInvalid confirms saveWizardConfig routes through
// the validator: an empty org (which the forge source requires) must fail before
// anything is written to disk, rather than persisting an unloadable config.
func TestSaveWizardConfig_RejectsInvalid(t *testing.T) {
	opts := wizard.SyncSetupOptions{
		Provider:     "gitlab",
		Organization: "", // missing org — ValidateForgeSource must reject
		TargetPath:   "/tmp/repos",
		ConfigPath:   filepath.Join(t.TempDir(), "sync.yaml"),
	}

	if err := saveWizardConfig(&opts); err == nil {
		t.Fatal("expected validation error for empty org, got nil")
	}
	if _, err := os.Stat(opts.ConfigPath); !os.IsNotExist(err) {
		t.Errorf("config must not be written when validation fails, stat err=%v", err)
	}
}
