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
				actions, err := planGitWorkspaces(ctx, cfg, configDir, nil, out, "", true)
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
				actions, err := planForgeWorkspaces(ctx, cfg, nil, out, "", false, true)
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

func TestReachesEnclosingConfigDir(t *testing.T) {
	configDir, link := selfSyncFixture(t)
	child := filepath.Join(configDir, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(filepath.Dir(configDir), "sibling")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, path string
		enclosing  []string
		want       bool
	}{
		{"enclosing dir", configDir, []string{configDir}, true},
		{"symlink to enclosing dir", link, []string{configDir}, true},
		{"enclosing dir named through a symlink", configDir, []string{link}, true},
		{"ancestor", filepath.Dir(configDir), []string{configDir}, true},
		{"descendant", child, []string{configDir}, false},
		{"sibling", sibling, []string{configDir}, false},
		{"no enclosing dirs", configDir, nil, false},
	} {
		if got := reachesEnclosingConfigDir(tc.path, tc.enclosing); got != tc.want {
			t.Errorf("%s: reachesEnclosingConfigDir(%q, %q) = %t, want %t", tc.name, tc.path, tc.enclosing, got, tc.want)
		}
	}
}

// TestKeepActionsOffEnclosingConfigDirs covers the per-repository filter a
// child's forge actions pass through: a repository or orphan delete whose
// target is the devbox or an ancestor of it is dropped, a sibling is kept.
// No test seam stubs a forge provider, so this calls the filter directly.
func TestKeepActionsOffEnclosingConfigDirs(t *testing.T) {
	configDir, _ := selfSyncFixture(t)
	sibling := filepath.Join(filepath.Dir(configDir), "member")
	actions := []reposync.Action{
		{Repo: reposync.RepoSpec{Name: "devbox", TargetPath: configDir}, Type: reposync.ActionUpdate},
		{Repo: reposync.RepoSpec{Name: "parent", TargetPath: filepath.Dir(configDir)}, Type: reposync.ActionDelete},
		{Repo: reposync.RepoSpec{Name: "member", TargetPath: sibling}, Type: reposync.ActionUpdate},
	}

	var out bytes.Buffer
	kept := keepActionsOffEnclosingConfigDirs(actions, []string{configDir}, "child", &out)
	if len(kept) != 1 || kept[0].Repo.Name != "member" {
		t.Fatalf("kept %+v, want only member\n%s", kept, out.String())
	}
	for _, name := range []string{"devbox", "parent"} {
		if !strings.Contains(out.String(), "Skipping '"+name+"' in workspace 'child'") {
			t.Errorf("output lacks the warning for %s:\n%s", name, out.String())
		}
	}
}

// childWorkspace is a devbox config directory holding a child directory,
// with an origin that does not exist and a sibling of the devbox for
// entries that must still be planned.
type childWorkspace struct {
	configDir, link, configPath, child, deadOrigin, sibling string
}

func childWorkspaceFixture(t *testing.T) childWorkspace {
	t.Helper()
	configDir, link := selfSyncFixture(t)
	child := filepath.Join(configDir, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	return childWorkspace{
		configDir:  configDir,
		link:       link,
		configPath: filepath.Join(configDir, DefaultConfigFile),
		child:      child,
		deadOrigin: filepath.Join(filepath.Dir(configDir), "no-such-origin.git"),
		sibling:    filepath.Join(filepath.Dir(configDir), "member"),
	}
}

// TestConfigPlannerSkipsChildEntriesReachingConfigDir covers ISSUE-85: a
// child config workspace's entries that reach the devbox recursing into
// it, through `..`, a symlink or an ancestor, are skipped, and a sibling
// is still planned.
func TestConfigPlannerSkipsChildEntriesReachingConfigDir(t *testing.T) {
	f := childWorkspaceFixture(t)
	configDir, link, configPath, child, deadOrigin, sibling := f.configDir, f.link, f.configPath, f.child, f.deadOrigin, f.sibling
	member := "  member:\n    type: git\n    path: " + sibling + "\n    url: " + deadOrigin + "\n"
	const workspaceSkip = "Skipping workspace 'up': path '"
	const reaches = "' reaches an enclosing config directory"
	ctx := context.Background()

	for _, tc := range []struct{ name, childConfig, warning string }{
		{"git workspace dot-dot", "workspaces:\n  up:\n    type: git\n    path: ..\n    url: " + deadOrigin + "\n" + member, workspaceSkip},
		{"git workspace symlink", "workspaces:\n  up:\n    type: git\n    path: " + link + "\n    url: " + deadOrigin + "\n" + member, workspaceSkip},
		{"git workspace ancestor", "workspaces:\n  up:\n    type: git\n    path: ../..\n    url: " + deadOrigin + "\n" + member, workspaceSkip},
		{"forge workspace dot-dot", "workspaces:\n  up:\n    path: ..\n    source:\n      provider: gitlab\n      org: self\n" + member, workspaceSkip},
		{"repositories symlink", "repositories:\n  - name: up\n    url: " + deadOrigin + "\n    path: " + link + "\n  - name: member\n    url: " + deadOrigin + "\n    path: " + sibling + "\n", "Skipping 'up' in workspace 'child': path '"},
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
		if !strings.Contains(out.String(), tc.warning) || !strings.Contains(out.String(), reaches) {
			t.Errorf("%s: output lacks the enclosing-config warning:\n%s", tc.name, out.String())
		}
	}
}

// TestConfigPlannerSkipsChildWorkspaceBeforeConfigLink covers the real run
// of ISSUE-85: a child git workspace reaching the devbox is skipped before
// its configLink replaces the devbox config.
func TestConfigPlannerSkipsChildWorkspaceBeforeConfigLink(t *testing.T) {
	f := childWorkspaceFixture(t)
	configDir, configPath, child, deadOrigin := f.configDir, f.configPath, f.child, f.deadOrigin
	const devboxConfig = "repositories: []\n"
	if err := os.WriteFile(configPath, []byte(devboxConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(filepath.Dir(configDir), "linked.yaml")
	if err := os.WriteFile(linked, []byte("repositories: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	childConfig := "workspaces:\n  up:\n    type: git\n    path: ..\n    url: " + deadOrigin + "\n    configLink: " + linked + "\n"
	if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte(childConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: configPath, Workspaces: map[string]*config.Workspace{
		"child": {Path: "child", Sync: &config.SyncConfig{Recursive: true}},
	}}

	var out bytes.Buffer
	actions, err := planConfigWorkspacesWithDryRun(context.Background(), cfg, configDir, &out, "", false)
	if err != nil {
		t.Fatalf("planConfigWorkspacesWithDryRun: %v\n%s", err, out.String())
	}
	if len(actions) != 0 {
		t.Errorf("planned %+v, want nothing\n%s", actions, out.String())
	}
	info, err := os.Lstat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("devbox config was replaced by a symlink\n%s", out.String())
	}
	if got, err := os.ReadFile(configPath); err != nil || string(got) != devboxConfig {
		t.Errorf("devbox config = %q, %v; want %q unchanged", got, err, devboxConfig)
	}
}

// TestRunChildSyncSkipsWorkspaceReachingEnclosingConfigDir covers the
// post-execution recursion of ISSUE-85: a child synced after the root run
// does not plan its workspace that reaches the root devbox.
func TestRunChildSyncSkipsWorkspaceReachingEnclosingConfigDir(t *testing.T) {
	f := childWorkspaceFixture(t)
	configDir, child, deadOrigin := f.configDir, f.child, f.deadOrigin
	childConfig := "workspaces:\n  up:\n    type: git\n    path: ..\n    url: " + deadOrigin + "\n"
	if err := os.WriteFile(filepath.Join(child, DefaultConfigFile), []byte(childConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runChildSync(context.Background(), child, recursiveSyncOpts{
		MaxDepth:  3,
		Visited:   map[string]bool{configDir: true},
		Enclosing: []string{configDir},
		Out:       &out,
	})
	if !strings.Contains(out.String(), "Skipping workspace 'up'") {
		t.Errorf("output lacks the enclosing-config warning:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "No repositories found in child workspace.") {
		t.Errorf("child sync planned an action for the devbox:\n%s", out.String())
	}
}
