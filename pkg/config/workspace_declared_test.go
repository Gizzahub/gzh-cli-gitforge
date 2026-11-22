// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeDeclaredConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFindDeclaredRepositories(t *testing.T) {
	root := t.TempDir()
	absUncloned := filepath.Join(root, "abs", "api")
	writeDeclaredConfig(t, root, `selfSync:
  enabled: true
workspaces:
  app:
    url: https://example.invalid/app.git
  group:
    type: config
    path: groups/one
    workspaces:
      svc:
        url: https://example.invalid/svc.git
  gitlab:
    type: forge
    source:
      provider: gitlab
      org: example
  absolute:
    path: `+absUncloned+`
    url: https://example.invalid/api.git
  sibling:
    path: ../elsewhere
    url: https://example.invalid/elsewhere.git
`)

	tests := []struct {
		name         string
		scan         string
		wantPaths    []string
		wantPrefixes []string
	}{
		{
			name:         "config directory",
			scan:         root,
			wantPaths:    []string{".", "abs/api", "app", "groups/one/svc"},
			wantPrefixes: []string{"gitlab", "groups/one"},
		},
		{
			name:         "inside a config workspace",
			scan:         filepath.Join(root, "groups"),
			wantPaths:    []string{"one/svc"},
			wantPrefixes: []string{"one"},
		},
		{
			name:         "inside a forge workspace",
			scan:         filepath.Join(root, "gitlab", "team"),
			wantPrefixes: []string{"."},
		},
		{
			name: "nothing under the scan",
			scan: filepath.Join(root, "unrelated"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(tt.scan, 0o755); err != nil {
				t.Fatal(err)
			}
			got, found, err := FindDeclaredRepositories(tt.scan)
			if err != nil {
				t.Fatal(err)
			}
			if !found || !got.DeclaresAny {
				t.Fatalf("found=%v declaresAny=%v, want both true", found, got.DeclaresAny)
			}
			if !slices.Equal(got.Paths, tt.wantPaths) || !slices.Equal(got.Prefixes, tt.wantPrefixes) {
				t.Fatalf("paths=%v prefixes=%v, want %v %v", got.Paths, got.Prefixes, tt.wantPaths, tt.wantPrefixes)
			}
		})
	}
}

func TestFindDeclaredRepositoriesWithoutDeclarations(t *testing.T) {
	root := t.TempDir()
	writeDeclaredConfig(t, root, "branch:\n  integrationBranch: master\n")
	got, found, err := FindDeclaredRepositories(root)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.DeclaresAny || !got.Empty() {
		t.Fatalf("found=%v got=%+v, want a config that declares nothing", found, got)
	}

	empty := t.TempDir()
	if _, found, err := FindDeclaredRepositories(empty); err != nil || found {
		t.Fatalf("found=%v err=%v, want no config", found, err)
	}
}

func TestCanonicalDeclaredPathResolvesMissingTail(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	resolvedReal, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalDeclaredPath(filepath.Join(link, "not", "cloned"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(resolvedReal, "not", "cloned"); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
