// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"
)

// TestParseFlatRepositoriesContract pins the neutral flat-repositories
// parser: it decodes every field of the shared schema losslessly, keeps
// optional values distinguishable from explicit zeros, performs the common
// format validation (URL presence), and applies no defaults of its own.
func TestParseFlatRepositoriesContract(t *testing.T) {
	t.Run("explicit fields decode losslessly", func(t *testing.T) {
		doc, err := ParseFlatRepositories([]byte(`
version: 1
kind: repositories
metadata:
  name: parity-fixture
  type: development
  owner: celee
strategy: reset
parallel: 6
maxRetries: 4
resume: true
dryRun: true
cleanupOrphans: true
strictBranchCheckout: true
branch: develop
roots:
  - ~/repos
  - /opt/git
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
`))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		if doc.Version != 1 {
			t.Errorf("Version = %d, want 1", doc.Version)
		}
		if doc.Kind != KindRepositories {
			t.Errorf("Kind = %q, want repositories", doc.Kind)
		}
		if doc.Metadata == nil || doc.Metadata.Name != "parity-fixture" || doc.Metadata.Type != "development" || doc.Metadata.Owner != "celee" {
			t.Errorf("Metadata = %+v, want name/type/owner decoded", doc.Metadata)
		}
		if doc.Strategy != "reset" {
			t.Errorf("Strategy = %q, want reset", doc.Strategy)
		}
		if doc.Parallel == nil || *doc.Parallel != 6 {
			t.Errorf("Parallel = %v, want explicit 6", doc.Parallel)
		}
		if doc.MaxRetries == nil || *doc.MaxRetries != 4 {
			t.Errorf("MaxRetries = %v, want explicit 4", doc.MaxRetries)
		}
		if !doc.Resume || !doc.DryRun {
			t.Errorf("Resume/DryRun = %v/%v, want true/true", doc.Resume, doc.DryRun)
		}
		if doc.CleanupOrphans == nil || !*doc.CleanupOrphans {
			t.Errorf("CleanupOrphans = %v, want explicit true", doc.CleanupOrphans)
		}
		if !doc.StrictBranchCheckout {
			t.Error("StrictBranchCheckout = false, want true")
		}
		if doc.Branch != "develop" {
			t.Errorf("Branch = %q, want develop", doc.Branch)
		}
		if got := doc.Roots; len(got) != 2 || got[0] != "~/repos" || got[1] != "/opt/git" {
			t.Errorf("Roots = %v, want [~/repos /opt/git]", got)
		}

		if len(doc.Repositories) != 2 {
			t.Fatalf("got %d repositories, want 2", len(doc.Repositories))
		}
		alpha := doc.Repositories[0]
		if alpha.Name != "alpha" || alpha.Description != "primary service" || alpha.Provider != "gitlab" {
			t.Errorf("alpha identity = %q/%q/%q", alpha.Name, alpha.Description, alpha.Provider)
		}
		if alpha.URL != "https://gitlab.com/team/alpha.git" {
			t.Errorf("alpha URL = %q", alpha.URL)
		}
		if len(alpha.AdditionalRemotes) != 1 || alpha.AdditionalRemotes["upstream"] != "https://gitlab.com/upstream/alpha.git" {
			t.Errorf("alpha AdditionalRemotes = %v", alpha.AdditionalRemotes)
		}
		if alpha.Path != "./repos/alpha" {
			t.Errorf("alpha Path = %q, want ./repos/alpha", alpha.Path)
		}
		if alpha.Branch != "feature-x" {
			t.Errorf("alpha Branch = %q, want feature-x", alpha.Branch)
		}
		if alpha.StrictBranchCheckout == nil || !*alpha.StrictBranchCheckout {
			t.Errorf("alpha StrictBranchCheckout = %v, want explicit true", alpha.StrictBranchCheckout)
		}
		if alpha.Strategy != "pull" {
			t.Errorf("alpha Strategy = %q, want pull", alpha.Strategy)
		}
		if alpha.Enabled == nil || *alpha.Enabled {
			t.Errorf("alpha Enabled = %v, want explicit false", alpha.Enabled)
		}
		if !alpha.AssumePresent {
			t.Error("alpha AssumePresent = false, want true")
		}

		beta := doc.Repositories[1]
		if beta.Name != "beta" || beta.URL != "https://github.com/team/beta.git" {
			t.Errorf("beta = %q/%q", beta.Name, beta.URL)
		}
		if beta.Enabled != nil || beta.StrictBranchCheckout != nil {
			t.Errorf("beta optional pointers = %v/%v, want nil/nil (omitted)", beta.Enabled, beta.StrictBranchCheckout)
		}
	})

	t.Run("optional fields distinguish omitted from explicit zero", func(t *testing.T) {
		doc, err := ParseFlatRepositories([]byte(`
parallel: 0
maxRetries: 0
cleanupOrphans: false
repositories:
  - name: only
    url: https://github.com/team/only.git
    enabled: false
    strictBranchCheckout: false
`))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		// Explicit zeros and falses are present pointers, not omitted nils:
		// this is what lets the reposync loader honor maxRetries: 0 while
		// the workspace loader defaults it.
		if doc.Parallel == nil || *doc.Parallel != 0 {
			t.Errorf("Parallel = %v, want pointer to 0", doc.Parallel)
		}
		if doc.MaxRetries == nil || *doc.MaxRetries != 0 {
			t.Errorf("MaxRetries = %v, want pointer to 0", doc.MaxRetries)
		}
		if doc.CleanupOrphans == nil || *doc.CleanupOrphans {
			t.Errorf("CleanupOrphans = %v, want pointer to false", doc.CleanupOrphans)
		}
		repo := doc.Repositories[0]
		if repo.Enabled == nil || *repo.Enabled {
			t.Errorf("Enabled = %v, want pointer to false", repo.Enabled)
		}
		if repo.StrictBranchCheckout == nil || *repo.StrictBranchCheckout {
			t.Errorf("StrictBranchCheckout = %v, want pointer to false", repo.StrictBranchCheckout)
		}

		omitted, err := ParseFlatRepositories([]byte(`
repositories:
  - url: https://github.com/team/only.git
`))
		if err != nil {
			t.Fatalf("parse omitted: %v", err)
		}
		if omitted.Parallel != nil || omitted.MaxRetries != nil || omitted.CleanupOrphans != nil {
			t.Errorf("omitted optionals = %v/%v/%v, want nil/nil/nil",
				omitted.Parallel, omitted.MaxRetries, omitted.CleanupOrphans)
		}
		if omitted.Repositories[0].Enabled != nil || omitted.Repositories[0].StrictBranchCheckout != nil {
			t.Errorf("omitted repo optionals = %v/%v, want nil/nil",
				omitted.Repositories[0].Enabled, omitted.Repositories[0].StrictBranchCheckout)
		}
	})

	t.Run("branch accepts string and map forms", func(t *testing.T) {
		doc, err := ParseFlatRepositories([]byte(`
branch:
  defaultBranch: [dev, main]
repositories:
  - url: https://github.com/team/only.git
    branch: develop,master
`))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if doc.Branch != "dev,main" {
			t.Errorf("top-level Branch = %q, want dev,main (map form)", doc.Branch)
		}
		if doc.Repositories[0].Branch != "develop,master" {
			t.Errorf("repo Branch = %q, want develop,master (comma string form)", doc.Repositories[0].Branch)
		}
	})

	t.Run("missing URL is rejected with the entry index", func(t *testing.T) {
		_, err := ParseFlatRepositories([]byte(`
repositories:
  - name: first
    url: https://github.com/team/first.git
  - name: second
`))
		if err == nil {
			t.Fatal("expected error for missing URL, got nil")
		}
		if !strings.Contains(err.Error(), "repository[1]: missing URL") {
			t.Errorf("error %q does not identify repository[1] missing URL", err.Error())
		}
	})

	t.Run("invalid YAML is rejected", func(t *testing.T) {
		if _, err := ParseFlatRepositories([]byte("strategy: [invalid")); err == nil {
			t.Fatal("expected error for invalid YAML, got nil")
		}
	})

	t.Run("no defaults are applied", func(t *testing.T) {
		doc, err := ParseFlatRepositories([]byte("{}\n"))
		if err != nil {
			t.Fatalf("parse empty document: %v", err)
		}
		if doc.Strategy != "" || doc.Parallel != nil || doc.MaxRetries != nil || doc.CleanupOrphans != nil {
			t.Errorf("empty document got %+v, want zero values without defaults", doc)
		}
		if doc.Resume || doc.DryRun || doc.StrictBranchCheckout {
			t.Error("empty document booleans are not all false")
		}
		if doc.Roots != nil || doc.Repositories != nil {
			t.Errorf("empty document lists = %v/%v, want nil/nil", doc.Roots, doc.Repositories)
		}
	})
}
