// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func TestNormalizeCloneKind(t *testing.T) {
	cases := []struct {
		in      string
		want    CloneConfigKind
		wantErr bool
	}{
		{"groups", CloneKindGroups, false},
		{"flat", CloneKindFlat, false},
		{"", "", false},
		{"bogus", "", true},
	}
	for _, tc := range cases {
		got, _, err := NormalizeCloneKind(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("NormalizeCloneKind(%q) err=%v wantErr=%v", tc.in, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("NormalizeCloneKind(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsValidCloneStrategy(t *testing.T) {
	for _, s := range ValidCloneStrategies {
		if !IsValidCloneStrategy(s) {
			t.Errorf("expected valid: %s", s)
		}
	}
	if IsValidCloneStrategy("nope") {
		t.Error("expected invalid strategy")
	}
}

func TestValidateStrategyAndFlatRepos(t *testing.T) {
	if err := validateStrategy(""); err != nil {
		t.Errorf("empty strategy ok: %v", err)
	}
	if err := validateStrategy("pull"); err != nil {
		t.Errorf("pull: %v", err)
	}
	if err := validateStrategy("bad"); err == nil {
		t.Error("expected bad strategy error")
	}

	if err := validateFlatRepositories(nil, ""); err == nil {
		t.Error("expected empty repos error")
	}
	repos := []CloneRepoSpec{{URL: "https://github.com/a/b.git"}}
	if err := validateFlatRepositories(repos, ""); err != nil {
		t.Errorf("valid repo: %v", err)
	}
	if err := validateFlatRepositories([]CloneRepoSpec{{URL: ""}}, "g"); err == nil {
		t.Error("expected empty url error")
	}
}

func TestValidateCloneConfig(t *testing.T) {
	if err := validateCloneConfig(&CloneConfig{
		Repositories: []CloneRepoSpec{{URL: "https://github.com/a/b.git"}},
	}); err != nil {
		t.Errorf("flat: %v", err)
	}
	if err := validateCloneConfig(&CloneConfig{}); err == nil {
		t.Error("empty config should fail")
	}
	if err := validateCloneConfig(&CloneConfig{
		Strategy:     "bad",
		Repositories: []CloneRepoSpec{{URL: "https://github.com/a/b.git"}},
	}); err == nil {
		t.Error("bad strategy should fail")
	}
	// grouped
	cfg := &CloneConfig{
		Groups: map[string]*CloneGroup{
			"core": {
				Target:       "./core",
				Repositories: []CloneRepoSpec{{URL: "https://github.com/a/b.git"}},
			},
		},
	}
	if err := validateCloneConfig(cfg); err != nil {
		t.Errorf("grouped: %v", err)
	}
}

func TestExtractFlexBranchAndResolveStrategy(t *testing.T) {
	fb := extractFlexBranch("main")
	if fb.String() == "" {
		t.Logf("flex branch empty for main: %q", fb.String())
	}
	_ = extractFlexBranch([]any{"develop", "main"})
	_ = extractFlexBranch(nil)
	_ = extractFlexBranch(42)

	if got := resolveCloneStrategy("pull", ""); got != repository.StrategyPull {
		t.Errorf("cli pull => %q", got)
	}
	if got := resolveCloneStrategy("", "rebase"); got != repository.StrategyRebase {
		t.Errorf("yaml rebase => %q", got)
	}
	if got := resolveCloneStrategy("skip", "pull"); got != repository.StrategySkip {
		t.Errorf("cli skip wins => %q", got)
	}
	if got := resolveCloneStrategy("", ""); got != repository.StrategySkip {
		t.Errorf("default skip => %q", got)
	}
}

func TestParseCloneConfig_FlatYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clone.yaml")
	content := `
strategy: pull
repositories:
  - url: https://github.com/example/one.git
    name: one
  - url: https://github.com/example/two.git
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseCloneConfig(path, false)
	if err != nil {
		t.Fatalf("parseCloneConfig: %v", err)
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("repos=%d", len(cfg.Repositories))
	}
	if err := validateCloneConfig(cfg); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestParseCloneConfig_GroupedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "groups.yaml")
	content := `
strategy: pull
libs:
  target: ./libs
  strategy: fetch
  repositories:
    - url: https://github.com/example/lib.git
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseCloneConfig(path, false)
	if err != nil {
		t.Fatalf("parse grouped: %v", err)
	}
	if err := validateCloneConfig(cfg); err != nil {
		t.Fatalf("validate grouped: %v", err)
	}
}

func TestParseCloneHooksAndMerge(t *testing.T) {
	raw := map[string]any{
		"before": []any{"echo a"},
		"after":  []any{"echo b"},
	}
	h := parseCloneHooks(raw)
	if h == nil {
		t.Fatal("expected hooks")
	}
	g := &CloneHooks{Before: []string{"g-before"}, After: []string{"g-after"}}
	r := &CloneHooks{Before: []string{"r-before"}}
	m := mergeHooks(g, r)
	if m == nil {
		t.Fatal("merge nil")
	}
	_ = mergeHooks(nil, r)
	_ = mergeHooks(g, nil)
	_ = mergeHooks(nil, nil)
	_ = config.DefaultProfileName
}
