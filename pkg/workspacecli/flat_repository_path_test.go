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

// TestFlatConfigResolvesRelativePathsAgainstConfigDir pins ISSUE-86: a
// relative `repositories:` path is placed under the directory of the config
// that lists it, not the working directory; absolute paths are kept.
func TestFlatConfigResolvesRelativePathsAgainstConfigDir(t *testing.T) {
	configDir := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	req := reposync.PlanRequest{Input: reposync.PlanInput{Repos: []reposync.RepoSpec{
		{Name: "lib", CloneURL: "https://example.invalid/lib.git", TargetPath: "lib"},
		{Name: "nested", CloneURL: "https://example.invalid/nested.git", TargetPath: filepath.Join("deps", "nested")},
		{Name: "abs", CloneURL: "https://example.invalid/abs.git", TargetPath: elsewhere},
	}}}

	want := map[string]string{
		"lib":    filepath.Join(configDir, "lib"),
		"nested": filepath.Join(configDir, "deps", "nested"),
		"abs":    elsewhere,
	}
	actions := createActionsFromFlatConfig(req, configDir)
	if len(actions) != len(want) {
		t.Fatalf("createActionsFromFlatConfig() = %+v, want %d actions", actions, len(want))
	}
	for _, action := range actions {
		if got := action.Repo.TargetPath; got != want[action.Repo.Name] {
			t.Errorf("%s: TargetPath = %q, want %q", action.Repo.Name, got, want[action.Repo.Name])
		}
	}
}

// TestConfigPlannerResolvesChildRepositoriesAgainstChildDir covers the
// child side of ISSUE-86: a child config's repository without a path is
// planned inside the child, and a child entry at `..` is the devbox seen
// from the child, so the enclosing-config guard drops it whatever the
// working directory is.
func TestConfigPlannerResolvesChildRepositoriesAgainstChildDir(t *testing.T) {
	f := childWorkspaceFixture(t)
	childConfig := "repositories:\n  - name: lib\n    url: " + f.deadOrigin + "\n  - name: up\n    url: " + f.deadOrigin + "\n    path: ..\n"
	if err := os.WriteFile(filepath.Join(f.child, DefaultConfigFile), []byte(childConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: f.configPath, Workspaces: map[string]*config.Workspace{
		"child": {Path: "child", Sync: &config.SyncConfig{Recursive: true}},
	}}

	var out bytes.Buffer
	actions, err := planConfigWorkspacesWithDryRun(context.Background(), cfg, f.configDir, &out, "", true)
	if err != nil {
		t.Fatalf("planConfigWorkspacesWithDryRun: %v\n%s", err, out.String())
	}
	if len(actions) != 1 || actions[0].Repo.Name != "lib" {
		t.Fatalf("planned %+v, want only lib\n%s", actions, out.String())
	}
	if got, want := actions[0].Repo.TargetPath, filepath.Join(f.child, "lib"); got != want {
		t.Errorf("lib TargetPath = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "Skipping 'up' in workspace 'child'") {
		t.Errorf("child entry at `..` was not refused:\n%s", out.String())
	}
}
