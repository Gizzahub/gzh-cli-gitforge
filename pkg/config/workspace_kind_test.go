// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		declared  WorkspaceType
		hasSource bool
		url       string
		recursive bool
		want      WorkspaceType
		wantErr   bool
	}{
		{name: "source infers forge", hasSource: true, want: WorkspaceTypeForge},
		{name: "forge with source", declared: WorkspaceTypeForge, hasSource: true, want: WorkspaceTypeForge},
		{name: "config with source conflicts", declared: WorkspaceTypeConfig, hasSource: true, recursive: true, wantErr: true},
		{name: "git with source conflicts", declared: WorkspaceTypeGit, hasSource: true, wantErr: true},
		{name: "explicit config", declared: WorkspaceTypeConfig, want: WorkspaceTypeConfig},
		{name: "explicit forge", declared: WorkspaceTypeForge, want: WorkspaceTypeForge},
		{name: "untyped url", url: "git@example.invalid:a/b.git", recursive: true, want: WorkspaceTypeGit},
		{name: "untyped recursive without url", recursive: true, want: WorkspaceTypeConfig},
		{name: "git recursive without url", declared: WorkspaceTypeGit, recursive: true, want: WorkspaceTypeConfig},
		{name: "untyped without url", want: WorkspaceTypeGit},
		{name: "unknown type unchanged", declared: "bogus", recursive: true, want: "bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifyWorkspace(tt.declared, tt.hasSource, tt.url, tt.recursive)
			if tt.wantErr {
				if !errors.Is(err, ErrWorkspaceTypeConflict) {
					t.Fatalf("err = %v, want ErrWorkspaceTypeConflict", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWorkspaceGettersShareClassification pins ISSUE-104: the sync planners'
// getters classify an untyped, url-less recursive workspace as config, and
// never plan a config workspace that also sets a source as forge.
func TestWorkspaceGettersShareClassification(t *testing.T) {
	cfg := &Config{Workspaces: map[string]*Workspace{
		"deep":  {Sync: &SyncConfig{Recursive: true}},
		"repo":  {URL: "git@example.invalid:a/b.git"},
		"org":   {Source: &ForgeSource{Provider: "github", Org: "example"}},
		"mixed": {Type: WorkspaceTypeConfig, Source: &ForgeSource{Provider: "github", Org: "example"}, Sync: &SyncConfig{Recursive: true}},
	}}

	if got := GetConfigWorkspaces(cfg); len(got) != 1 || got["deep"] == nil {
		t.Errorf("config workspaces = %v, want only deep", keys(got))
	}
	if got := GetGitWorkspaces(cfg); len(got) != 1 || got["repo"] == nil {
		t.Errorf("git workspaces = %v, want only repo", keys(got))
	}
	if got := GetForgeWorkspaces(cfg); len(got) != 1 || got["org"] == nil {
		t.Errorf("forge workspaces = %v, want only org", keys(got))
	}
}

// TestLoadConfigRecursive_RefusesConfigWorkspaceWithSource pins ISSUE-104:
// sync's recursive load fails on `type: config` with a source, as status does.
func TestLoadConfigRecursive_RefusesConfigWorkspaceWithSource(t *testing.T) {
	dir := t.TempDir()
	content := "workspaces:\n  mixed:\n    type: config\n    source:\n      provider: github\n      org: example\n"
	if err := os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfigRecursive(dir, ".gz-git.yaml")
	if !errors.Is(err, ErrWorkspaceTypeConflict) {
		t.Fatalf("LoadConfigRecursive error = %v, want ErrWorkspaceTypeConflict", err)
	}
}

func keys(m map[string]*Workspace) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
