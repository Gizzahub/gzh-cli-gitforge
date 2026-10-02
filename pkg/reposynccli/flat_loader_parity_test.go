// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// flatLoaderParityFixture mirrors the workspace package's copy: a flat
// repositories document using only fields both flat loaders understand, with
// explicit values on every common field, so both loaders are pinned against
// the same input.
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

// writeFlatParityConfig writes content to a uniquely named file in a fresh
// temp directory and returns its path.
func writeFlatParityConfig(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write parity config %s: %v", path, err)
	}
	return path
}

// loadFlatParity runs the loader under test and fails the test on load errors.
func loadFlatParity(t *testing.T, loader FileSpecLoader, path string) ConfigData {
	t.Helper()

	result, err := loader.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	return result
}

// TestReposyncFlatLoaderCompatibility pins the reposync adapter's flat-loader
// behavior: explicit common fields (including the fields only this loader
// applies), the optional/omitted/explicit-retry-0 distinction with its
// configurable defaults, empty-list rejection, duplicate-path rejection, and
// the surviving parent/kind-workspace/profiles/gzh.yaml routing that must not
// be folded into the flat path.
func TestReposyncFlatLoaderCompatibility(t *testing.T) {
	t.Run("explicit common fields", func(t *testing.T) {
		result := loadFlatParity(t, FileSpecLoader{}, writeFlatParityConfig(t, "flat.yaml", flatLoaderParityFixture))

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
		// Unlike the workspace loader, paths are cleaned.
		if alpha.TargetPath != "repos/alpha" {
			t.Errorf("alpha TargetPath = %q, want cleaned repos/alpha", alpha.TargetPath)
		}
		if alpha.Branch != "feature-x" {
			t.Errorf("alpha Branch = %q, want feature-x", alpha.Branch)
		}
		if alpha.Strategy != reposync.StrategyPull {
			t.Errorf("alpha Strategy = %q, want pull", alpha.Strategy)
		}
		if !alpha.StrictBranchCheckout {
			t.Error("alpha StrictBranchCheckout = false, want true (per-repo override)")
		}
		if alpha.Provider != "gitlab" {
			t.Errorf("alpha Provider = %q, want gitlab", alpha.Provider)
		}
		if alpha.Enabled == nil || *alpha.Enabled {
			t.Errorf("alpha Enabled = %v, want explicit false", alpha.Enabled)
		}
		if !alpha.AssumePresent {
			t.Error("alpha AssumePresent = false, want true")
		}

		beta := repos[1]
		if beta.Name != "beta" || beta.TargetPath != "beta" {
			t.Errorf("beta name/path = %q/%q, want beta/beta", beta.Name, beta.TargetPath)
		}
		// Unlike the workspace loader, no top-level branch fallback and the
		// default strategy is materialized onto every spec.
		if beta.Branch != "" {
			t.Errorf("beta Branch = %q, want empty (no top-level fallback)", beta.Branch)
		}
		if beta.Strategy != reposync.StrategyReset {
			t.Errorf("beta Strategy = %q, want reset (default materialized)", beta.Strategy)
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
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("user home dir: %v", err)
		}
		if got := result.Plan.Options.Roots; len(got) != 1 || got[0] != filepath.Join(home, "repos") {
			t.Errorf("Roots = %v, want cleaned [%s]", got, filepath.Join(home, "repos"))
		}

		if result.Run.Parallel != 6 {
			t.Errorf("Run.Parallel = %d, want 6", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 4 {
			t.Errorf("Run.MaxRetries = %d, want 4", result.Run.MaxRetries)
		}
	})

	t.Run("empty repository list is rejected", func(t *testing.T) {
		path := writeFlatParityConfig(t, "flat.yaml", `
strategy: reset
repositories: []
`)
		_, err := FileSpecLoader{}.Load(context.Background(), path)
		if err == nil {
			t.Fatal("expected error for empty repositories, got nil")
		}
		if !strings.Contains(err.Error(), "no repositories") {
			t.Errorf("error %q does not mention no repositories", err.Error())
		}
	})

	t.Run("omitted parallel and retries default to 10 and 1", func(t *testing.T) {
		result := loadFlatParity(t, FileSpecLoader{}, writeFlatParityConfig(t, "flat.yaml", `
repositories:
  - name: only
    url: https://github.com/team/only.git
`))

		if result.Run.Parallel != 10 {
			t.Errorf("default parallel = %d, want 10", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 1 {
			t.Errorf("default maxRetries = %d, want 1", result.Run.MaxRetries)
		}
	})

	t.Run("explicit retry zero is preserved, not defaulted", func(t *testing.T) {
		result := loadFlatParity(t, FileSpecLoader{}, writeFlatParityConfig(t, "flat.yaml", `
maxRetries: 0
repositories:
  - name: only
    url: https://github.com/team/only.git
`))

		// Legacy reposync semantics: an explicit 0 is a real value and must
		// survive (contrast with the workspace loader's default-to-3).
		if result.Run.MaxRetries != 0 {
			t.Errorf("explicit maxRetries 0 = %d, want 0 preserved", result.Run.MaxRetries)
		}
	})

	t.Run("negative retries are rejected", func(t *testing.T) {
		path := writeFlatParityConfig(t, "flat.yaml", `
maxRetries: -1
repositories:
  - name: only
    url: https://github.com/team/only.git
`)
		_, err := FileSpecLoader{}.Load(context.Background(), path)
		if err == nil {
			t.Fatal("expected error for negative maxRetries, got nil")
		}
		if !strings.Contains(err.Error(), "maxRetries must be >= 0") {
			t.Errorf("error %q does not mention maxRetries must be >= 0", err.Error())
		}
	})

	t.Run("configurable loader defaults apply when the file omits values", func(t *testing.T) {
		loader := FileSpecLoader{
			DefaultStrategy: reposync.StrategyPull,
			DefaultParallel: 7,
			DefaultRetries:  2,
		}
		result := loadFlatParity(t, loader, writeFlatParityConfig(t, "flat.yaml", `
repositories:
  - name: only
    url: https://github.com/team/only.git
`))

		if result.Plan.Options.DefaultStrategy != reposync.StrategyPull {
			t.Errorf("DefaultStrategy = %q, want pull (configured)", result.Plan.Options.DefaultStrategy)
		}
		if result.Plan.Input.Repos[0].Strategy != reposync.StrategyPull {
			t.Errorf("spec Strategy = %q, want pull (configured default materialized)", result.Plan.Input.Repos[0].Strategy)
		}
		if result.Run.Parallel != 7 {
			t.Errorf("Run.Parallel = %d, want 7 (configured)", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 2 {
			t.Errorf("Run.MaxRetries = %d, want 2 (configured)", result.Run.MaxRetries)
		}

		// A per-repo strategy still overrides the configured default.
		overridden := loadFlatParity(t, loader, writeFlatParityConfig(t, "flat.yaml", `
repositories:
  - name: only
    url: https://github.com/team/only.git
    strategy: reset
`))
		if overridden.Plan.Input.Repos[0].Strategy != reposync.StrategyReset {
			t.Errorf("spec Strategy = %q, want reset (per-repo override)", overridden.Plan.Input.Repos[0].Strategy)
		}
	})

	t.Run("duplicate target paths are rejected", func(t *testing.T) {
		path := writeFlatParityConfig(t, "flat.yaml", `
repositories:
  - name: dup
    url: https://github.com/team/dup.git
    path: ./repos/dup
  - name: dup-2
    url: https://github.com/team/dup-2.git
    path: repos/dup
`)
		_, err := FileSpecLoader{}.Load(context.Background(), path)
		if err == nil {
			t.Fatal("expected error for duplicate paths, got nil")
		}
		if !strings.Contains(err.Error(), "duplicate path detected") {
			t.Errorf("error %q does not mention duplicate path detected", err.Error())
		}
	})

	t.Run("parent inheritance is preserved", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "parent.yaml")
		child := filepath.Join(dir, "child.yaml")
		parentYAML := `
strategy: reset
parallel: 2
maxRetries: 4
repositories:
  - name: parent-repo
    url: https://github.com/team/parent-repo.git
`
		childYAML := `
parent: parent.yaml
repositories:
  - name: child-repo
    url: https://github.com/team/child-repo.git
`
		if err := os.WriteFile(parent, []byte(parentYAML), 0o600); err != nil {
			t.Fatalf("write parent: %v", err)
		}
		if err := os.WriteFile(child, []byte(childYAML), 0o600); err != nil {
			t.Fatalf("write child: %v", err)
		}

		result := loadFlatParity(t, FileSpecLoader{}, child)
		if len(result.Plan.Input.Repos) != 1 || result.Plan.Input.Repos[0].Name != "child-repo" {
			t.Fatalf("repos = %+v, want only child-repo (child list replaces parent's)", result.Plan.Input.Repos)
		}
		if result.Plan.Options.DefaultStrategy != reposync.StrategyReset {
			t.Errorf("DefaultStrategy = %q, want reset inherited from parent", result.Plan.Options.DefaultStrategy)
		}
		if result.Run.Parallel != 2 {
			t.Errorf("Run.Parallel = %d, want 2 inherited from parent", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 4 {
			t.Errorf("Run.MaxRetries = %d, want 4 inherited from parent", result.Run.MaxRetries)
		}
	})

	t.Run("parent inheritance child override uses absolute parent path", func(t *testing.T) {
		dir := t.TempDir()
		parent := filepath.Join(dir, "parent.yaml")
		parentYAML := `
strategy: reset
parallel: 2
maxRetries: 4
repositories:
  - name: parent-repo
    url: https://github.com/team/parent-repo.git
`
		if err := os.WriteFile(parent, []byte(parentYAML), 0o600); err != nil {
			t.Fatalf("write parent: %v", err)
		}
		childYAML := `
parent: ` + parent + `
strategy: pull
parallel: 9
maxRetries: 0
repositories:
  - name: child-repo
    url: https://github.com/team/child-repo.git
`
		result := loadFlatParity(t, FileSpecLoader{}, writeFlatParityConfig(t, "child.yaml", childYAML))
		if result.Plan.Options.DefaultStrategy != reposync.StrategyPull {
			t.Errorf("DefaultStrategy = %q, want pull (child overrides parent)", result.Plan.Options.DefaultStrategy)
		}
		if result.Run.Parallel != 9 {
			t.Errorf("Run.Parallel = %d, want 9 (child overrides parent)", result.Run.Parallel)
		}
		if result.Run.MaxRetries != 0 {
			t.Errorf("Run.MaxRetries = %d, want 0 (explicit child zero overrides parent 4)", result.Run.MaxRetries)
		}
	})

	t.Run("kind workspace routing is preserved", func(t *testing.T) {
		// A kind:workspace document must reach the workspaces branch, not the
		// flat repositories path: an empty workspace tree fails with the
		// workspaces-specific error.
		path := writeFlatParityConfig(t, "ws.yaml", `
kind: workspace
workspaces:
  main:
    path: .
`)
		_, err := FileSpecLoader{}.Load(context.Background(), path)
		if err == nil {
			t.Fatal("expected error for workspace config without git repos, got nil")
		}
		if !strings.Contains(err.Error(), "no git repositories found in workspaces") {
			t.Errorf("error %q does not mention no git repositories found in workspaces", err.Error())
		}
	})

	t.Run("profiles content detection routing is preserved", func(t *testing.T) {
		// A document carrying only a profiles map is detected as the
		// hierarchical format by content and must not fall into the flat
		// path's empty-list rejection.
		path := writeFlatParityConfig(t, "profiles.yaml", `
profiles:
  work:
    provider: gitlab
`)
		_, err := FileSpecLoader{}.Load(context.Background(), path)
		if err == nil {
			t.Fatal("expected error for profiles-only config, got nil")
		}
		if !strings.Contains(err.Error(), "no workspaces defined") {
			t.Errorf("error %q does not mention no workspaces defined", err.Error())
		}
	})

	t.Run("gzh.yaml routing is preserved", func(t *testing.T) {
		path := writeFlatParityConfig(t, "gzh.yaml", `
provider: gitlab
sync_mode:
  cleanup_orphans: false
repositories:
  - name: gz-repo
    clone_url: https://gitlab.com/team/gz-repo.git
`)
		result := loadFlatParity(t, FileSpecLoader{}, path)
		repos := result.Plan.Input.Repos
		if len(repos) != 1 {
			t.Fatalf("got %d repos, want 1", len(repos))
		}
		if repos[0].Name != "gz-repo" || repos[0].CloneURL != "https://gitlab.com/team/gz-repo.git" {
			t.Errorf("gzh repo = %q/%q, want gz-repo with clone_url", repos[0].Name, repos[0].CloneURL)
		}
		if repos[0].Provider != "gitlab" {
			t.Errorf("gzh repo Provider = %q, want gitlab from top-level provider", repos[0].Provider)
		}
	})
}
