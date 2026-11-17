// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeListConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, ".gz-git.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListDeclaredWorkspaces(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	// The nested config is broken on purpose: listing must not read it.
	if err := os.MkdirAll(filepath.Join(dir, "children"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeListConfig(t, filepath.Join(dir, "children"), "workspaces: [not, a, map\n")
	path := writeListConfig(t, dir, `kind: workspace
workspaces:
  zeta:
    path: ~/src/zeta
    url: ssh://host/zeta.git
    branch: master
    access: read-only
  alpha:
    url: ssh://host/alpha.git
    branch:
      defaultBranch: [main, develop]
  mirror:
    path: /abs/mirror
    type: forge
    source:
      provider: gitlab
      org: team
  children:
    path: ./children
    sync:
      recursive: true
`)

	got, err := ListDeclaredWorkspaces(path)
	if err != nil {
		t.Fatalf("ListDeclaredWorkspaces: %v", err)
	}
	want := []DeclaredWorkspace{
		{Name: "alpha", Type: WorkspaceTypeGit, Path: filepath.Join(dir, "alpha"), URL: "ssh://host/alpha.git", DefaultBranch: []string{"main", "develop"}, Access: WorkspaceAccessReadWrite},
		{Name: "children", Type: WorkspaceTypeConfig, Path: filepath.Join(dir, "children"), Access: WorkspaceAccessReadWrite},
		{Name: "mirror", Type: WorkspaceTypeForge, Path: "/abs/mirror", Access: WorkspaceAccessReadWrite},
		{Name: "zeta", Type: WorkspaceTypeGit, Path: filepath.Join(home, "src", "zeta"), URL: "ssh://host/zeta.git", DefaultBranch: []string{"master"}, Access: WorkspaceAccessReadOnly},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListDeclaredWorkspaces =\n%#v\nwant\n%#v", got, want)
	}
}

func TestListDeclaredWorkspacesRelativeConfigPath(t *testing.T) {
	dir := t.TempDir()
	writeListConfig(t, dir, "workspaces:\n  one:\n    url: ssh://host/one.git\n")
	t.Chdir(dir)

	got, err := ListDeclaredWorkspaces(".gz-git.yaml")
	if err != nil {
		t.Fatalf("ListDeclaredWorkspaces: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d workspaces, want 1", len(got))
	}
	// t.TempDir may sit behind a symlink (macOS /var -> /private/var), and
	// the working directory reports the resolved form.
	if gotDir, _ := filepath.EvalSymlinks(filepath.Dir(got[0].Path)); gotDir != resolved {
		t.Fatalf("path = %q, want it under %q", got[0].Path, resolved)
	}
}

func TestListDeclaredWorkspacesEmpty(t *testing.T) {
	path := writeListConfig(t, t.TempDir(), "branch:\n  integrationBranch: [main]\n")
	got, err := ListDeclaredWorkspaces(path)
	if err != nil {
		t.Fatalf("ListDeclaredWorkspaces: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty non-nil list", got)
	}
}

// Sync plans a workspaces map alongside any flat repositories and whatever
// the kind says, so each of these files lists its workspaces.
func TestListDeclaredWorkspacesListsWorkspacesBesideRepositories(t *testing.T) {
	cases := map[string]string{
		"mixed file":                 "kind: workspace\nrepositories:\n  - url: ssh://host/flat.git\nworkspaces:\n  one:\n    url: ssh://host/one.git\n",
		"repositories kind":          "kind: repositories\nworkspaces:\n  one:\n    url: ssh://host/one.git\n",
		"deprecated workspaces kind": "kind: workspaces\nworkspaces:\n  one:\n    url: ssh://host/one.git\n",
		// Parent configs only lend settings and are not read.
		"missing parent": "parent: ./absent.yaml\nworkspaces:\n  one:\n    url: ssh://host/one.git\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeListConfig(t, t.TempDir(), body)
			got, err := ListDeclaredWorkspaces(path)
			if err != nil {
				t.Fatalf("ListDeclaredWorkspaces: %v", err)
			}
			if len(got) != 1 || got[0].Name != "one" || got[0].URL != "ssh://host/one.git" {
				t.Fatalf("got %#v, want the one workspace", got)
			}
		})
	}
}

func TestListDeclaredWorkspacesEmptyRepositoriesList(t *testing.T) {
	for name, body := range map[string]string{
		"empty list": "repositories: []\n",
		"null value": "repositories:\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ListDeclaredWorkspaces(writeListConfig(t, t.TempDir(), body))
			if err != nil || got == nil || len(got) != 0 {
				t.Fatalf("got %#v, err %v; want an empty list", got, err)
			}
		})
	}
}

func TestListDeclaredWorkspacesJSONConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.json")
	body := `{"workspaces": {"one": {"url": "ssh://host/one.git", "access": "read-only"}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListDeclaredWorkspaces(path)
	if err != nil {
		t.Fatalf("ListDeclaredWorkspaces: %v", err)
	}
	want := []DeclaredWorkspace{{Name: "one", Type: WorkspaceTypeGit, Path: filepath.Join(dir, "one"), URL: "ssh://host/one.git", Access: WorkspaceAccessReadOnly}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestListDeclaredWorkspacesRefusals(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"repositories kind": {
			body: "kind: repositories\nrepositories:\n  - url: ssh://host/a.git\n",
			want: `is kind "repositories"`,
		},
		"repository kind alias": {
			body: "kind: repository\nrepositories: []\n",
			want: `is kind "repository"`,
		},
		"repositories without kind": {
			body: "repositories:\n  - url: ssh://host/a.git\n",
			want: "has a repositories list and no workspaces key",
		},
		"clone config kind": {
			body: "kind: groups\nbackend:\n  target: ./backend\n",
			want: `has kind "groups", which is not a workspace config`,
		},
		"unknown kind with workspaces": {
			body: "kind: groups\nworkspaces:\n  one:\n    url: ssh://host/one.git\n",
			want: `has kind "groups", which is not a workspace config`,
		},
		"unknown type": {
			body: "workspaces:\n  bad:\n    type: gti\n    url: ssh://host/a.git\n",
			want: `has unknown type "gti"`,
		},
		"type conflicts with source": {
			body: "workspaces:\n  bad:\n    type: git\n    source:\n      provider: gitlab\n      org: team\n",
			want: `workspace "bad"`,
		},
		"invalid access": {
			body: "workspaces:\n  bad:\n    url: ssh://host/a.git\n    access: write-only\n",
			want: `invalid access "write-only"`,
		},
		"empty entry": {
			body: "workspaces:\n  bad:\n",
			want: `workspace "bad"`,
		},
		"malformed yaml": {
			body: "workspaces: [unclosed\n",
			want: "failed to parse config",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeListConfig(t, t.TempDir(), tc.body)
			_, err := ListDeclaredWorkspaces(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestListDeclaredWorkspacesTypeConflictIsTyped(t *testing.T) {
	path := writeListConfig(t, t.TempDir(), "workspaces:\n  bad:\n    type: config\n    source:\n      provider: gitlab\n      org: team\n")
	_, err := ListDeclaredWorkspaces(path)
	if !errors.Is(err, ErrWorkspaceTypeConflict) {
		t.Fatalf("err = %v, want ErrWorkspaceTypeConflict", err)
	}
}

func TestListDeclaredWorkspacesMissingAndEmptyPath(t *testing.T) {
	if _, err := ListDeclaredWorkspaces(" "); err == nil {
		t.Fatal("empty path: want an error")
	}
	_, err := ListDeclaredWorkspaces(filepath.Join(t.TempDir(), ".gz-git.yaml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: err = %v, want os.ErrNotExist", err)
	}
}
