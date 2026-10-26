// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
)

func TestBranchConfigUnmarshalWorktreeSync(t *testing.T) {
	var project ProjectConfig
	if err := yaml.Unmarshal([]byte(`branch:
  worktreeSync:
    upstream: ff-only
    rules:
      - match: feature/*
        parent: develop
        strategy: rebase
`), &project); err != nil {
		t.Fatalf("unmarshal project config: %v", err)
	}
	if project.Branch == nil || project.Branch.WorktreeSync == nil {
		t.Fatal("worktreeSync was not decoded")
	}
	rules := project.Branch.WorktreeSync.Rules
	if project.Branch.WorktreeSync.Upstream != "ff-only" || len(rules) != 1 || rules[0].Parent != "develop" {
		t.Fatalf("decoded worktreeSync = %#v", project.Branch.WorktreeSync)
	}
}

func TestLoadAtWorktreeSyncPrecedenceAndEmptyOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	mgr, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveGlobalConfig(&GlobalConfig{Branch: &BranchConfig{WorktreeSync: &branch.WorktreeSyncConfig{
		Upstream: "ff-only", Rules: []branch.WorktreeSyncRule{{Match: "feature/*", Parent: "main", Strategy: "rebase"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SaveProfile(&Profile{Name: "work", Branch: &BranchConfig{WorktreeSync: &branch.WorktreeSyncConfig{
		Rules: []branch.WorktreeSyncRule{{Match: "feature/*", Parent: "develop", Strategy: "rebase"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(home, "projects", "repo", "nested-worktree")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	loader, err := NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadAt(projectDir); err != nil {
		t.Fatalf("LoadAt: %v", err)
	}
	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := effective.Branch.WorktreeSync.Rules[0].Parent; got != "main" {
		t.Fatalf("global worktreeSync parent = %q, want main", got)
	}

	paths, err := NewPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.SetActiveProfile("work"); err != nil {
		t.Fatal(err)
	}
	loader, err = NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadAt(projectDir); err != nil {
		t.Fatalf("LoadAt with profile: %v", err)
	}
	effective, err = loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := effective.Branch.WorktreeSync.Rules[0].Parent; got != "develop" {
		t.Fatalf("profile worktreeSync parent = %q, want develop", got)
	}

	projectConfig := `profile: work
branch:
  worktreeSync:
    rules:
      - match: feature/*
        parent: release
        strategy: rebase
`
	projectPath := filepath.Join(home, "projects", "repo", ".gz-git.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	loader, err = NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadAt(projectDir); err != nil {
		t.Fatalf("LoadAt with project policy: %v", err)
	}
	effective, err = loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := effective.Branch.WorktreeSync.Rules[0].Parent; got != "release" {
		t.Fatalf("project worktreeSync parent = %q, want release", got)
	}

	if err := os.WriteFile(projectPath, []byte("profile: work\nbranch:\n  worktreeSync: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loader, err = NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadAt(projectDir); err != nil {
		t.Fatalf("LoadAt with empty project policy: %v", err)
	}
	effective, err = loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Branch.WorktreeSync == nil {
		t.Fatal("expected explicit project worktreeSync policy")
	}
	if effective.Branch.WorktreeSync.Upstream != "" || len(effective.Branch.WorktreeSync.Rules) != 0 {
		t.Fatalf("project empty policy must replace inherited rules, got %#v", effective.Branch.WorktreeSync)
	}
}

func TestLoadAtUsesAnchoredProjectInlineProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	anchor := filepath.Join(home, "repo", "linked-worktree")
	if err := os.MkdirAll(anchor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "repo", ".gz-git.yaml"), []byte(`profile: local
profiles:
  local:
    branch:
      worktreeSync:
        upstream: ff-only
`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Loading from HOME proves LoadAt does not use the process CWD to discover
	// either the project config or the inline profile it selects.
	t.Chdir(home)
	loader, err := NewLoader()
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadAt(anchor); err != nil {
		t.Fatalf("LoadAt: %v", err)
	}
	effective, err := loader.ResolveConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if effective.Branch.WorktreeSync == nil || effective.Branch.WorktreeSync.Upstream != "ff-only" {
		t.Fatalf("anchored inline profile policy = %#v", effective.Branch.WorktreeSync)
	}
}

func TestMergeParentBranchConfigInheritsWorktreeSyncUnlessOverridden(t *testing.T) {
	parent := &BranchConfig{WorktreeSync: &branch.WorktreeSyncConfig{Upstream: "ff-only"}}
	child := &BranchConfig{}
	mergeParentBranchConfig(child, parent)
	if child.WorktreeSync != parent.WorktreeSync {
		t.Fatal("child did not inherit parent worktreeSync")
	}
	override := &branch.WorktreeSyncConfig{}
	child.WorktreeSync = override
	mergeParentBranchConfig(child, parent)
	if child.WorktreeSync != override {
		t.Fatal("explicit child worktreeSync was overwritten")
	}
}

func TestValidateProjectConfigRejectsInvalidWorktreeSync(t *testing.T) {
	err := NewValidator().ValidateProjectConfig(&ProjectConfig{Branch: &BranchConfig{WorktreeSync: &branch.WorktreeSyncConfig{Upstream: "rebase"}}})
	if err == nil {
		t.Fatal("invalid worktreeSync upstream was accepted")
	}
}
