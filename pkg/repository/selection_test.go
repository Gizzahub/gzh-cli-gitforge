// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeSelectionPath(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "a", want: "a"},
		{in: " a/b/ ", want: "a/b"},
		{in: "./a//b", want: "a/b"},
		{in: ".", want: "."},
		{in: "a/..", want: "."},
		{in: "", wantErr: true},
		{in: "  ", wantErr: true},
		{in: "/abs/a", wantErr: true},
		{in: "..", wantErr: true},
		{in: "../x", wantErr: true},
		{in: "a/../../x", wantErr: true},
	}
	for _, tt := range tests {
		got, err := NormalizeSelectionPath(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("NormalizeSelectionPath(%q) = %q, want error", tt.in, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("NormalizeSelectionPath(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestRequiredDepth(t *testing.T) {
	sel := RepoSelection{Listed: true, Paths: []string{".", "a", "x/y/z"}, Prefixes: []string{"p/q"}}
	if got := sel.requiredDepth(); got != 3 {
		t.Fatalf("requiredDepth = %d, want 3", got)
	}
	if got := (RepoSelection{Listed: true, Prefixes: []string{"forge"}}).requiredDepth(); got != 2 {
		t.Fatalf("prefix requiredDepth = %d, want 2", got)
	}
}

func TestApplySelection(t *testing.T) {
	root := filepath.FromSlash("/w")
	repos := []string{root, filepath.FromSlash("/w/a"), filepath.FromSlash("/w/b"), filepath.FromSlash("/w/forge/x"), filepath.FromSlash("/w/forge/y")}

	t.Run("zero value keeps everything", func(t *testing.T) {
		got, err := applySelection(root, repos, RepoSelection{})
		if err != nil || len(got) != len(repos) {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	t.Run("exact paths and prefix", func(t *testing.T) {
		got, err := applySelection(root, repos, RepoSelection{Listed: true, Paths: []string{".", "b"}, Prefixes: []string{"forge"}})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{root, filepath.FromSlash("/w/b"), filepath.FromSlash("/w/forge/x"), filepath.FromSlash("/w/forge/y")}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("listed with no entries selects nothing", func(t *testing.T) {
		got, err := applySelection(root, repos, RepoSelection{Listed: true})
		if err != nil || len(got) != 0 {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	t.Run("missing entries are all reported", func(t *testing.T) {
		_, err := applySelection(root, repos, RepoSelection{Listed: true, Paths: []string{"a", "nope"}, Prefixes: []string{"empty"}})
		if !errors.Is(err, ErrSelectionUnmatched) {
			t.Fatalf("err = %v, want ErrSelectionUnmatched", err)
		}
		for _, name := range []string{"nope", "empty/"} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name %q", err, name)
			}
		}
	})

	t.Run("a prefix is not a name prefix", func(t *testing.T) {
		_, err := applySelection(root, []string{filepath.FromSlash("/w/forgery")}, RepoSelection{Listed: true, Prefixes: []string{"forge"}})
		if !errors.Is(err, ErrSelectionUnmatched) {
			t.Fatalf("err = %v, want ErrSelectionUnmatched", err)
		}
	})
}

func TestFilterRepositoriesMatchesRelativePath(t *testing.T) {
	root := filepath.FromSlash("/home/u/ws")
	repos := []string{root, filepath.FromSlash("/home/u/ws/api"), filepath.FromSlash("/home/u/ws/web/api")}

	got, err := filterRepositories(root, repos, "^api$", "", &noopLogger{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{filepath.FromSlash("/home/u/ws/api")}) {
		t.Fatalf("include ^api$ = %v", got)
	}

	// Exclusion no longer sees the parent directories of the scan root: "u"
	// appears in every absolute path but in no relative one.
	got, err = filterRepositories(root, repos, "", "u", &noopLogger{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(repos) {
		t.Fatalf("exclude u dropped repositories: %v", got)
	}

	got, err = filterRepositories(root, repos, `^\.$`, "", &noopLogger{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{root}) {
		t.Fatalf(`include ^\.$ = %v`, got)
	}
}

// selectionFixture creates a, b and nested/c, each with one uncommitted
// change, under a fresh directory.
func selectionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"a", "b", filepath.Join("nested", "c")} {
		repo := newRepoIn(t, root, name)
		if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func dirty(t *testing.T, repo string) bool {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain") //nolint:noctx // test helper
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status in %s: %v", repo, err)
	}
	return len(out) > 0
}

func TestScanRepositoriesListed(t *testing.T) {
	root := selectionFixture(t)
	client := NewClient()

	// The default depth (1) does not reach nested/c; listing it must.
	result, err := client.ScanRepositories(context.Background(), ScanOptions{
		Directory: root,
		Selection: RepoSelection{Listed: true, Paths: []string{"b", "nested/c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	rel := make([]string, 0, len(result.Paths))
	for _, p := range result.Paths {
		rel = append(rel, relativeRepoPath(result.Directory, p))
	}
	slices.Sort(rel)
	if !slices.Equal(rel, []string{"b", "nested/c"}) {
		t.Fatalf("selected %v", rel)
	}

	_, err = client.ScanRepositories(context.Background(), ScanOptions{
		Directory:      root,
		ExcludePattern: "^b$",
		Selection:      RepoSelection{Listed: true, Paths: []string{"a", "b"}},
	})
	if !errors.Is(err, ErrSelectionUnmatched) || !strings.Contains(err.Error(), "b excluded") {
		t.Fatalf("exclude contradicting the list: err = %v", err)
	}
}

func TestBulkCommitListedLeavesOthersUntouched(t *testing.T) {
	root := selectionFixture(t)
	client := NewClient()

	_, err := client.BulkCommit(context.Background(), BulkCommitOptions{
		Directory: root,
		Message:   "test: listed",
		Yes:       true,
		Selection: RepoSelection{Listed: true, Paths: []string{"a", "missing"}},
	})
	if !errors.Is(err, ErrSelectionUnmatched) {
		t.Fatalf("missing entry: err = %v", err)
	}
	if !dirty(t, filepath.Join(root, "a")) {
		t.Fatal("a was committed although the selection was refused")
	}

	result, err := client.BulkCommit(context.Background(), BulkCommitOptions{
		Directory: root,
		Message:   "test: listed",
		Yes:       true,
		Selection: RepoSelection{Listed: true, Paths: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Repositories) != 1 || result.Repositories[0].RelativePath != "a" {
		t.Fatalf("processed %+v", result.Repositories)
	}
	if dirty(t, filepath.Join(root, "a")) {
		t.Fatal("a still dirty")
	}
	if !dirty(t, filepath.Join(root, "b")) || !dirty(t, filepath.Join(root, "nested", "c")) {
		t.Fatal("an unlisted repository was committed")
	}
}

func TestCheckSelectionFiltered(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "ws")
	abs := func(rels ...string) []string {
		out := make([]string, 0, len(rels))
		for _, rel := range rels {
			out = append(out, filepath.Join(root, filepath.FromSlash(rel)))
		}
		return out
	}

	tests := []struct {
		name     string
		sel      RepoSelection
		selected []string
		filtered []string
		wantErr  string
	}{
		{
			name:     "filter narrows a prefix",
			sel:      RepoSelection{Listed: true, Prefixes: []string{"vendor"}},
			selected: abs("vendor/one", "vendor/archive"),
			filtered: abs("vendor/one"),
		},
		{
			name:     "filter narrows the scan-root prefix",
			sel:      RepoSelection{Listed: true, Prefixes: []string{"."}},
			selected: abs("one", "archive"),
			filtered: abs("one"),
		},
		{
			name:     "filter drops an exact path",
			sel:      RepoSelection{Listed: true, Paths: []string{"app"}, Prefixes: []string{"vendor"}},
			selected: abs("app", "vendor/one"),
			filtered: abs("vendor/one"),
			wantErr:  "app excluded",
		},
		{
			name:     "filter empties a prefix",
			sel:      RepoSelection{Listed: true, Paths: []string{"app"}, Prefixes: []string{"vendor"}},
			selected: abs("app", "vendor/archive"),
			filtered: abs("app"),
			wantErr:  "vendor/ (every repository under it) excluded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSelectionFiltered(root, tt.selected, tt.filtered, tt.sel)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, ErrSelectionUnmatched) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
