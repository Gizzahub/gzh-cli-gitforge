// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeclaredWorkspaceClones(t *testing.T) {
	dir := t.TempDir()
	body := `workspaces:
  by-key:
    url: ssh://host/a.git
  by-path:
    path: ./nested/../deps/b
    url: ssh://host/b.git
    type: git
  absolute:
    path: /srv/c
    url: ssh://host/c.git
  home:
    path: ~/c
    url: ssh://host/c.git
  escapes:
    path: ../d
    url: ssh://host/d.git
  forge:
    path: f
    type: forge
    url: ssh://host/f.git
  no-url:
    path: g
`
	if err := os.WriteFile(filepath.Join(dir, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := DeclaredWorkspaceClones(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"by-key": "ssh://host/a.git", "deps/b": "ssh://host/b.git"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestDeclaredWorkspaceClonesMissingConfig(t *testing.T) {
	got, err := DeclaredWorkspaceClones(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}
