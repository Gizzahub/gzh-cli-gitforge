// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runListCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := CommandFactory{}.newListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func writeWorkspaceListConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, ".gz-git.yaml")
	body := `kind: workspace
workspaces:
  beta:
    url: ssh://host/beta.git
    branch: master
    access: read-only
  alpha:
    path: ./nested/alpha
    url: ssh://host/alpha.git
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListCmdJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkspaceListConfig(t, dir)

	out, err := runListCmd(t, "-c", path, "--format", "json")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	var report struct {
		Config     string `json:"config"`
		Workspaces []struct {
			Name          string   `json:"name"`
			Type          string   `json:"type"`
			Path          string   `json:"path"`
			URL           string   `json:"url"`
			DefaultBranch []string `json:"defaultBranch"`
			Access        string   `json:"access"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out)
	}
	if report.Config != path {
		t.Errorf("config = %q, want %q", report.Config, path)
	}
	if len(report.Workspaces) != 2 {
		t.Fatalf("got %d workspaces, want 2: %s", len(report.Workspaces), out)
	}
	alpha, beta := report.Workspaces[0], report.Workspaces[1]
	if alpha.Name != "alpha" || alpha.Path != filepath.Join(dir, "nested", "alpha") || alpha.Access != "read-write" || alpha.Type != "git" {
		t.Errorf("alpha = %+v", alpha)
	}
	if beta.Name != "beta" || beta.Access != "read-only" || len(beta.DefaultBranch) != 1 || beta.DefaultBranch[0] != "master" {
		t.Errorf("beta = %+v", beta)
	}
}

func TestListCmdTable(t *testing.T) {
	dir := t.TempDir()
	path := writeWorkspaceListConfig(t, dir)

	out, err := runListCmd(t, "-c", path)
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header + 2 rows:\n%s", len(lines), out)
	}
	if fields := strings.Fields(lines[0]); strings.Join(fields, " ") != "NAME TYPE ACCESS BRANCH PATH" {
		t.Errorf("header = %q", lines[0])
	}
	if fields := strings.Fields(lines[1]); len(fields) != 5 || fields[0] != "alpha" || fields[3] != "-" {
		t.Errorf("alpha row = %q", lines[1])
	}
	if fields := strings.Fields(lines[2]); len(fields) != 5 || fields[0] != "beta" || fields[2] != "read-only" || fields[3] != "master" {
		t.Errorf("beta row = %q", lines[2])
	}
}

func TestListCmdEmptyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gz-git.yaml")
	if err := os.WriteFile(path, []byte("kind: workspace\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runListCmd(t, "-c", path)
	if err != nil || !strings.Contains(out, "No workspaces declared") {
		t.Fatalf("err = %v, out = %q", err, out)
	}
	out, err = runListCmd(t, "-c", path, "--format", "json")
	if err != nil || !strings.Contains(out, `"workspaces": []`) {
		t.Fatalf("json: err = %v, out = %q", err, out)
	}
}

func TestListCmdAutoDetectsConfig(t *testing.T) {
	dir := t.TempDir()
	writeWorkspaceListConfig(t, dir)
	t.Chdir(dir)

	out, err := runListCmd(t, "--format", "json")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"name": "alpha"`) {
		t.Fatalf("auto-detected config not listed: %s", out)
	}
}

func TestListCmdRelativeConfigIsReportedAbsolute(t *testing.T) {
	dir := t.TempDir()
	writeWorkspaceListConfig(t, dir)
	t.Chdir(dir)

	out, err := runListCmd(t, "-c", DefaultConfigFile, "--format", "json")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	var report struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !filepath.IsAbs(report.Config) || filepath.Base(report.Config) != DefaultConfigFile {
		t.Fatalf("config = %q, want an absolute path to %s", report.Config, DefaultConfigFile)
	}
}

func TestListCmdRefusals(t *testing.T) {
	path := writeWorkspaceListConfig(t, t.TempDir())
	if _, err := runListCmd(t, "-c", path, "--format", "csv"); err == nil || !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("csv format: err = %v", err)
	}
	if _, err := runListCmd(t, "-c", path, "extra"); err == nil {
		t.Error("positional argument: want an error")
	}
	if _, err := runListCmd(t, "-c", filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("missing config: want an error")
	}
}
