// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func writeSelectionConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const selectionConfig = `selfSync:
  enabled: true
workspaces:
  app:
    url: https://example.invalid/app.git
  tools:
    path: lib/tools
    url: https://example.invalid/tools.git
  vendor:
    type: forge
    path: vendor
    source:
      provider: github
      org: example
  outside:
    path: ../elsewhere
    url: https://example.invalid/elsewhere.git
`

func TestResolveBulkSelection(t *testing.T) {
	dir := t.TempDir()
	writeSelectionConfig(t, dir, selectionConfig)
	emptyList := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(emptyList, []byte("# nothing selected\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		flags        BulkCommandFlags
		wantPaths    []string
		wantPrefixes []string
		wantErr      string
	}{
		{name: "default is all", flags: BulkCommandFlags{}},
		{name: "explicit all", flags: BulkCommandFlags{Select: "all"}},
		{name: "repo needs listed", flags: BulkCommandFlags{Repos: []string{"app"}}, wantErr: "require --select listed"},
		{name: "bad mode", flags: BulkCommandFlags{Select: "some"}, wantErr: "invalid --select"},
		{
			name:         "listed is the declared universe",
			flags:        BulkCommandFlags{Select: "listed"},
			wantPaths:    []string{".", "app", "lib/tools"},
			wantPrefixes: []string{"vendor"},
		},
		{
			name:      "repo narrows",
			flags:     BulkCommandFlags{Select: "listed", Repos: []string{"./app", "app", "."}},
			wantPaths: []string{"app", "."},
		},
		{
			name:      "repo under a forge workspace",
			flags:     BulkCommandFlags{Select: "listed", Repos: []string{"vendor/one"}},
			wantPaths: []string{"vendor/one"},
		},
		{
			name:         "forge workspace itself",
			flags:        BulkCommandFlags{Select: "listed", Repos: []string{"vendor"}},
			wantPrefixes: []string{"vendor"},
		},
		{name: "undeclared repo", flags: BulkCommandFlags{Select: "listed", Repos: []string{"app", "stray"}}, wantErr: "stray not declared"},
		{name: "escaping repo", flags: BulkCommandFlags{Select: "listed", Repos: []string{"../elsewhere"}}, wantErr: "escapes"},
		{name: "empty list file", flags: BulkCommandFlags{Select: "listed", ReposFrom: emptyList}, wantErr: "gave no repository paths"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := tt.flags
			got, err := resolveBulkSelection(dir, &flags)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			listed := flags.Select == "listed"
			if got.Listed != listed || !slices.Equal(got.Paths, tt.wantPaths) || !slices.Equal(got.Prefixes, tt.wantPrefixes) {
				t.Fatalf("got %+v, want listed=%v paths=%v prefixes=%v", got, listed, tt.wantPaths, tt.wantPrefixes)
			}
		})
	}
}

func TestResolveBulkSelectionNothingUnderScan(t *testing.T) {
	dir := t.TempDir()
	writeSelectionConfig(t, dir, selectionConfig)
	scan := filepath.Join(dir, "unrelated")
	if err := os.MkdirAll(scan, 0o755); err != nil {
		t.Fatal(err)
	}
	// The config declares repositories elsewhere, so an --repo here is not a
	// free-standing list: it must be refused, not run.
	for _, flags := range []BulkCommandFlags{{Select: "listed"}, {Select: "listed", Repos: []string{"stray"}}} {
		if _, err := resolveBulkSelection(scan, &flags); err == nil || !strings.Contains(err.Error(), "no workspace under this scan directory") {
			t.Fatalf("flags %+v: err = %v", flags, err)
		}
	}
}

func TestResolveBulkSelectionWithoutWorkspaces(t *testing.T) {
	dir := t.TempDir()
	writeSelectionConfig(t, dir, "branch:\n  integrationBranch: master\n")

	if _, err := resolveBulkSelection(dir, &BulkCommandFlags{Select: "listed"}); err == nil {
		t.Fatal("listed without workspaces or --repo must fail")
	}

	list := filepath.Join(t.TempDir(), "repos.txt")
	if err := os.WriteFile(list, []byte("# comment\n\nb\na\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolveBulkSelection(dir, &BulkCommandFlags{Select: "listed", Repos: []string{"a"}, ReposFrom: list})
	if err != nil {
		t.Fatal(err)
	}
	want := repository.RepoSelection{Listed: true, Paths: []string{"a", "b"}}
	if got.Listed != want.Listed || !slices.Equal(got.Paths, want.Paths) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRepoSelectionFlagsRegistered(t *testing.T) {
	for _, name := range []string{"commit", "diff", "fetch", "pull", "push", "status"} {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, flag := range []string{"select", "repo", "repos-from"} {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s has no --%s", name, flag)
			}
		}
		if !strings.Contains(cmd.Long, "Repository Selection:") {
			t.Errorf("%s help does not document repository selection", name)
		}
	}
	for _, name := range []string{"clone"} {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cmd.Flags().Lookup("select") != nil {
			t.Errorf("%s builds its own repository list and must not offer --select", name)
		}
	}
}
