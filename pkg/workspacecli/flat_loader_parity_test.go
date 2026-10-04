// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// flatLoaderParityFixture is a flat repositories document that uses only the
// fields both flat loaders understand, with explicit values on every common
// field. The workspace and reposync parity tests keep an identical copy so
// both loaders are pinned against the same input.
const flatLoaderParityFixture = `
version: 1
kind: repositories
metadata:
  name: parity-fixture
  type: development
  owner: celee
strategy: reset
parallel: 6
maxRetries: 4
cleanupOrphans: true
roots:
  - ~/repos
branch: develop
repositories:
  - name: alpha
    description: primary service
    provider: gitlab
    url: https://gitlab.com/team/alpha.git
    additionalRemotes:
      upstream: https://gitlab.com/upstream/alpha.git
    path: ./repos/alpha
    branch: feature-x
    strictBranchCheckout: true
    strategy: pull
    enabled: false
    assumePresent: true
  - name: beta
    url: https://github.com/team/beta.git
`

// writeFlatParityConfig writes content to flat.yaml in a fresh temp
// directory and returns its path.
func writeFlatParityConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "flat.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write parity config %s: %v", path, err)
	}
	return path
}

// loadFlatParity runs the loader under test and fails the test on load errors.
func loadFlatParity(t *testing.T, path string) *ConfigData {
	t.Helper()

	result, err := FileSpecLoader{}.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	return result
}

// TestFlatLoaderCompatibility pins the workspace loader's behavior on the
// shared flat repositories schema: explicit common fields, the
// optional/omitted/explicit-zero distinction as this loader historically
// treats it, the empty-list allowance with its 10/3 defaults, and the split
// that keeps duplicate planning prevention out of the loader (the loader
// keeps duplicates; the command layer deduplicates planned actions).
func TestFlatLoaderCompatibility(t *testing.T) {
	t.Run("explicit common fields", func(t *testing.T) {
		result := loadFlatParity(t, writeFlatParityConfig(t, flatLoaderParityFixture))

		repos := result.Plan.Input.Repos
		if len(repos) != 2 {
			t.Fatalf("got %d repos, want 2", len(repos))
		}

		alpha := repos[0]
		if alpha.Name != "alpha" || alpha.Description != "primary service" {
			t.Errorf("alpha identity = %q/%q, want alpha/primary service", alpha.Name, alpha.Description)
		}
		if alpha.CloneURL != "https://gitlab.com/team/alpha.git" {
			t.Errorf("alpha CloneURL = %q", alpha.CloneURL)
		}
		if len(alpha.AdditionalRemotes) != 1 || alpha.AdditionalRemotes["upstream"] != "https://gitlab.com/upstream/alpha.git" {
			t.Errorf("alpha AdditionalRemotes = %v", alpha.AdditionalRemotes)
		}
		// The workspace loader keeps the configured path verbatim.
		if alpha.TargetPath != "./repos/alpha" {
			t.Errorf("alpha TargetPath = %q, want ./repos/alpha", alpha.TargetPath)
		}
		if alpha.Branch != "feature-x" {
			t.Errorf("alpha Branch = %q, want feature-x", alpha.Branch)
		}
		if alpha.Strategy != reposync.StrategyPull {
			t.Errorf("alpha Strategy = %q, want pull", alpha.Strategy)
		}
		if alpha.Enabled == nil || *alpha.Enabled {
			t.Errorf("alpha Enabled = %v, want explicit false", alpha.Enabled)
		}
		if !alpha.AssumePresent {
			t.Error("alpha AssumePresent = false, want true")
		}
		// Provider and per-repo strictBranchCheckout were never decoded by
		// this loader and must stay unapplied.
		if alpha.Provider != "" {
			t.Errorf("alpha Provider = %q, want empty (workspace ignores provider)", alpha.Provider)
		}
		if alpha.StrictBranchCheckout {
			t.Error("alpha StrictBranchCheckout = true, want false (workspace ignores it)")
		}

		beta := repos[1]
		if beta.Name != "beta" || beta.TargetPath != "beta" {
			t.Errorf("beta name/path = %q/%q, want beta/beta", beta.Name, beta.TargetPath)
		}
		// Workspace-only behavior: per-repo branch falls back to the
		// top-level branch, and an unspecified per-repo strategy stays empty
		// for the planner to fill from the default.
		if beta.Branch != "develop" {
			t.Errorf("beta Branch = %q, want develop (top-level fallback)", beta.Branch)
		}
		if beta.Strategy != "" {
			t.Errorf("beta Strategy = %q, want empty (default applied at plan time)", beta.Strategy)
		}
		if beta.Enabled != nil {
			t.Errorf("beta Enabled = %v, want nil (omitted)", beta.Enabled)
		}

		if result.Plan.Options.DefaultStrategy != reposync.StrategyReset {
			t.Errorf("DefaultStrategy = %q, want reset", result.Plan.Options.DefaultStrategy)
		}
		if !result.Plan.Options.CleanupOrphans {
			t.Error("CleanupOrphans = false, want true")
		}
		// Roots are passed through verbatim, unlike the reposync loader.
		if got := result.Plan.Options.Roots; len(got) != 1 || got[0] != "~/repos" {
			t.Errorf("Roots = %v, want [~/repos] verbatim", got)
		}

		if result.Run.Parallel != 6 {
			t.Errorf("Run.Parallel = %d, want 6", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 4 {
			t.Errorf("Run.MaxRetries = %d, want 4", result.Run.MaxRetries)
		}
	})

	t.Run("omitted parallel and retries default to 10 and 3", func(t *testing.T) {
		result := loadFlatParity(t, writeFlatParityConfig(t, `
repositories:
  - name: only
    url: https://github.com/team/only.git
`))

		if result.Run.Parallel != 10 {
			t.Errorf("default parallel = %d, want 10", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 3 {
			t.Errorf("default maxRetries = %d, want 3", result.Run.MaxRetries)
		}
	})

	t.Run("explicit zero retries are indistinguishable from omitted", func(t *testing.T) {
		result := loadFlatParity(t, writeFlatParityConfig(t, `
parallel: 0
maxRetries: 0
repositories:
  - name: only
    url: https://github.com/team/only.git
`))

		// Legacy workspace semantics: an explicit 0 falls through to the
		// 10/3 defaults just like an omitted value.
		if result.Run.Parallel != 10 {
			t.Errorf("parallel 0 = %d, want default 10", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 3 {
			t.Errorf("maxRetries 0 = %d, want default 3", result.Run.MaxRetries)
		}
	})

	t.Run("empty repository list is allowed", func(t *testing.T) {
		result := loadFlatParity(t, writeFlatParityConfig(t, `
strategy: reset
repositories: []
`))

		if len(result.Plan.Input.Repos) != 0 {
			t.Errorf("got %d repos, want 0", len(result.Plan.Input.Repos))
		}
		if result.Run.Parallel != 10 || result.Run.MaxRetries != 3 {
			t.Errorf("defaults = %d/%d, want 10/3", result.Run.Parallel, result.Run.MaxRetries)
		}
	})

	t.Run("duplicate planning is prevented at the command layer, not the loader", func(t *testing.T) {
		// The loader itself historically keeps duplicate target paths: the
		// same entry planned twice is a command-layer concern.
		result := loadFlatParity(t, writeFlatParityConfig(t, `
repositories:
  - name: dup
    url: https://github.com/team/dup.git
    path: ./repos/dup
  - name: dup-2
    url: https://github.com/team/dup-2.git
    path: ./repos/dup
`))

		if len(result.Plan.Input.Repos) != 2 {
			t.Fatalf("got %d repos, want 2 (loader keeps duplicates)", len(result.Plan.Input.Repos))
		}

		// deduplicateActions is the workspace double-planning guard: when
		// several planners emit actions for one path, the first wins.
		actions := []reposync.Action{
			{Repo: reposync.RepoSpec{Name: "first", TargetPath: "repos/dup"}},
			{Repo: reposync.RepoSpec{Name: "second", TargetPath: "repos/dup"}},
			{Repo: reposync.RepoSpec{Name: "other", TargetPath: "repos/other"}},
		}
		deduped := deduplicateActions(actions)
		if len(deduped) != 2 {
			t.Fatalf("deduplicateActions kept %d actions, want 2", len(deduped))
		}
		if deduped[0].Repo.Name != "first" || deduped[1].Repo.Name != "other" {
			t.Errorf("deduplicateActions order = %q,%q, want first,other (first wins per path)",
				deduped[0].Repo.Name, deduped[1].Repo.Name)
		}
	})
}

// TestFlatLoaderDecodeContract pins the common-schema contract introduced by
// TASK-265, including validation of fields this workspace adapter does not apply.
func TestFlatLoaderDecodeContract(t *testing.T) {
	const validRepo = "repositories:\n  - url: https://github.com/team/repo.git\n"
	tests := []struct {
		name      string
		yaml      string
		wantError string
	}{
		{"resume type", "resume: notabool\n" + validRepo, "parse flat repositories YAML:"},
		{"dryRun type", "dryRun: notabool\n" + validRepo, "parse flat repositories YAML:"},
		{"repository strict checkout type", validRepo + "    strictBranchCheckout: notabool\n", "parse flat repositories YAML:"},
		{"repository provider type", validRepo + "    provider: [oops]\n", "parse flat repositories YAML:"},
		{"valid unapplied fields", "resume: true\ndryRun: true\n" + validRepo + "    provider: gitlab\n    strictBranchCheckout: true\n", ""},
		{"unknown sshPort ignored", "sshPort: [oops]\n" + validRepo, ""},
		{"unknown cloneProto ignored", "cloneProto: [ssh]\n" + validRepo, ""},
		{"malformed YAML prefix", "repositories: [", "parse flat repositories YAML:"},
		{"URL validated before strategy", "strategy: bogus\nrepositories:\n  - name: missing\n", "repository[0]: missing URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (FileSpecLoader{}).Load(context.Background(), writeFlatParityConfig(t, tt.yaml))
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("load: %v", err)
				}
			} else if err == nil || !strings.HasPrefix(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want prefix %q", err, tt.wantError)
			}
		})
	}
}
