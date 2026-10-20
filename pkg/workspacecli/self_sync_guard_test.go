// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
)

// selfSyncFixture returns a config directory and a symlink that reaches it
// through another spelling.
func selfSyncFixture(t *testing.T) (configDir, link string) {
	t.Helper()
	root := t.TempDir()
	configDir = filepath.Join(root, "devbox")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(root, "devbox-link")
	if err := os.Symlink(configDir, link); err != nil {
		t.Fatal(err)
	}
	return configDir, link
}

func TestPointsAtConfigDirMatchesFileIdentity(t *testing.T) {
	configDir, link := selfSyncFixture(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relConfigDir, err := filepath.Rel(cwd, configDir)
	if err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(filepath.Dir(configDir), "sibling")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, wsPath, configDir string
		want                    bool
	}{
		{"same string", configDir, configDir, true},
		{"symlink to config dir", link, configDir, true},
		{"relative config dir", configDir, relConfigDir, true},
		{"trailing dot segment", filepath.Join(configDir, "."), configDir, true},
		{"sibling directory", sibling, configDir, false},
		{"missing workspace", filepath.Join(configDir, "absent"), configDir, false},
	} {
		if got := pointsAtConfigDir(tc.wsPath, tc.configDir); got != tc.want {
			t.Errorf("%s: pointsAtConfigDir(%q, %q) = %t, want %t", tc.name, tc.wsPath, tc.configDir, got, tc.want)
		}
	}
}

// TestFlatConfigSkipsSymlinkToConfigDir covers the root repositories list,
// whose default strategy is reset: a repository whose path is a symlink to
// the config directory is dropped, and a sibling is still planned.
func TestFlatConfigSkipsSymlinkToConfigDir(t *testing.T) {
	configDir, link := selfSyncFixture(t)
	sibling := filepath.Join(filepath.Dir(configDir), "member")
	req := reposync.PlanRequest{Input: reposync.PlanInput{Repos: []reposync.RepoSpec{
		{Name: "self", CloneURL: "https://example.invalid/self.git", TargetPath: link},
		{Name: "member", CloneURL: "https://example.invalid/member.git", TargetPath: sibling},
	}}}

	actions := createActionsFromFlatConfig(req, configDir)
	if len(actions) != 1 || actions[0].Repo.Name != "member" {
		t.Fatalf("createActionsFromFlatConfig() = %+v, want only member", actions)
	}
}

// TestSyncPlannersSkipSymlinkToConfigDir covers each planner's guard: a
// workspace reaching the config directory through a symlink is skipped
// with the self-sync warning and plans nothing.
func TestSyncPlannersSkipSymlinkToConfigDir(t *testing.T) {
	configDir, link := selfSyncFixture(t)
	configPath := filepath.Join(configDir, DefaultConfigFile)
	deadOrigin := filepath.Join(filepath.Dir(configDir), "no-such-origin.git")
	// The devbox config lists a member, so a planner that reads the config
	// directory as a workspace plans it instead of skipping for a missing config.
	devboxConfig := "repositories:\n  - name: member\n    url: " + deadOrigin + "\n"
	if err := os.WriteFile(configPath, []byte(devboxConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		ws   *config.Workspace
		plan func(cfg *config.Config, out *bytes.Buffer) (int, error)
	}{
		{
			name: "git",
			ws:   &config.Workspace{Type: config.WorkspaceTypeGit, URL: deadOrigin, Path: link},
			plan: func(cfg *config.Config, out *bytes.Buffer) (int, error) {
				actions, err := planGitWorkspaces(ctx, cfg, configDir, out, "", true)
				return len(actions), err
			},
		},
		{
			name: "config",
			ws:   &config.Workspace{Path: link, Sync: &config.SyncConfig{Recursive: true}},
			plan: func(cfg *config.Config, out *bytes.Buffer) (int, error) {
				actions, err := planConfigWorkspacesWithDryRun(ctx, cfg, configDir, out, "", true)
				return len(actions), err
			},
		},
		{
			name: "forge",
			ws:   &config.Workspace{Path: link, Source: &config.ForgeSource{Provider: "gitlab", Org: "self"}},
			plan: func(cfg *config.Config, out *bytes.Buffer) (int, error) {
				actions, err := planForgeWorkspaces(ctx, cfg, out, "", false, true)
				return len(actions), err
			},
		},
	} {
		cfg := &config.Config{ConfigPath: configPath, Workspaces: map[string]*config.Workspace{"self": tc.ws}}
		var out bytes.Buffer
		n, err := tc.plan(cfg, &out)
		if err != nil {
			t.Fatalf("%s planner: %v\n%s", tc.name, err, out.String())
		}
		if n != 0 {
			t.Errorf("%s planner planned %d actions for a symlink to the config directory\n%s", tc.name, n, out.String())
		}
		if !strings.Contains(out.String(), "Skipping workspace 'self': path '"+link+"' points to config directory") {
			t.Errorf("%s planner output lacks the self-sync warning:\n%s", tc.name, out.String())
		}
	}
}

// TestConfigPlannerSkipsChildEntriesReachingConfigDir covers ISSUE-85: a
// child config workspace's own entries reaching the devbox that recursed
// into it, through `..` or a symlink, are skipped, and a sibling is planned.
func TestConfigPlannerSkipsChildEntriesReachingConfigDir(t *testing.T) {
	configDir, link := selfSyncFixture(t)
	configPath := filepath.Join(configDir, DefaultConfigFile)
	child := filepath.Join(configDir, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	deadOrigin := filepath.Join(filepath.Dir(configDir), "no-such-origin.git")
	sibling := filepath.Join(filepath.Dir(configDir), "member")
	ctx := context.Background()

	for _, tc := range []struct{ name, childConfig string }{
		{"git workspace dot-dot", "workspaces:\n  up:\n    type: git\n    path: ..\n    url: " + deadOrigin + "\n  member:\n    type: git\n    path: " + sibling + "\n    url: " + deadOrigin + "\n"},
		{"git workspace symlink", "workspaces:\n  up:\n    type: git\n    path: " + link + "\n    url: " + deadOrigin + "\n  member:\n    type: git\n    path: " + sibling + "\n    url: " + deadOrigin + "\n"},
		{"repositories symlink", "repositories:\n  - name: up\n    url: " + deadOrigin + "\n    path: " + link + "\n  - name: member\n    url: " + deadOrigin + "\n    path: " + sibling + "\n"},
	} {
		if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte(tc.childConfig), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{ConfigPath: configPath, Workspaces: map[string]*config.Workspace{
			"child": {Path: "child", Sync: &config.SyncConfig{Recursive: true}},
		}}
		var out bytes.Buffer
		actions, err := planConfigWorkspacesWithDryRun(ctx, cfg, configDir, &out, "", true)
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out.String())
		}
		if len(actions) != 1 || actions[0].Repo.Name != "member" {
			t.Errorf("%s: planned %+v, want only member\n%s", tc.name, actions, out.String())
		}
		if !strings.Contains(out.String(), "Skipping 'up' in workspace 'child'") {
			t.Errorf("%s: output lacks the self-sync warning:\n%s", tc.name, out.String())
		}
	}
}
