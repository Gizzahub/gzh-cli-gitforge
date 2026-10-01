// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLoadRepoRootTaskPattern_ThisRepoDeclaration guards this repository's own
// repo-root declaration. The post-integrate RECLAIM machinery stays dormant
// until LoadRepoRootTaskPattern finds a repo-root taskPattern, so deleting or
// loosening the declaration silently disables reclaim. The repo root is
// resolved from this test file's location, never from the working directory.
func TestLoadRepoRootTaskPattern_ThisRepoDeclaration(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	decl, err := LoadRepoRootTaskPattern(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoRootTaskPattern(%s): %v", repoRoot, err)
	}
	if len(decl.Patterns) == 0 {
		t.Fatalf("no taskPattern declaration: facts %v", decl.Facts)
	}

	wantIntegration := []string{"master"}
	if got := []string(decl.IntegrationBranch); !reflect.DeepEqual(got, wantIntegration) {
		t.Fatalf("integrationBranch = %v, want %v", got, wantIntegration)
	}

	mustMatch := []string{
		"feat/x",
		"test/info-branch-cell-colors",
		"agent/task/hermes-01", // agent/* must cover the multi-segment shape
		"dev/mac/fix/reclaim-dev-pattern",
	}
	for _, name := range mustMatch {
		if !MatchesAnyTaskPattern(name, decl.Patterns) {
			t.Errorf("%s must match taskPattern %v", name, decl.Patterns)
		}
	}

	mustNotMatch := []string{"develop", "master", "main"}
	for _, name := range mustNotMatch {
		if MatchesAnyTaskPattern(name, decl.Patterns) {
			t.Errorf("%s must not match taskPattern %v", name, decl.Patterns)
		}
	}
}

// TestRepoRootMakeTimeout pins branch.makeTimeout parsing on the repo-root
// declaration. Absent stays zero (the consumer's built-in default), a valid
// duration parses, and an unparsable or non-positive value fails the load —
// a repository that declares a budget its gate cannot meet must never
// silently run under the default it meant to lift.
func TestRepoRootMakeTimeout(t *testing.T) {
	writeDecl := func(t *testing.T, body string) string {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return root
	}

	t.Run("absent key means zero and the default applies", func(t *testing.T) {
		root := writeDecl(t, "branch:\n  integrationBranch: master\n")
		decl, err := LoadRepoRootTaskPattern(root)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if decl.MakeTimeout != 0 {
			t.Fatalf("absent makeTimeout = %s, want 0", decl.MakeTimeout)
		}
	})

	t.Run("declared duration parses", func(t *testing.T) {
		root := writeDecl(t, "branch:\n  integrationBranch: master\n  makeTimeout: 90m\n")
		decl, err := LoadRepoRootTaskPattern(root)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if decl.MakeTimeout != 90*time.Minute {
			t.Fatalf("makeTimeout = %s, want 90m", decl.MakeTimeout)
		}
	})

	t.Run("whitespace survives the yaml round trip and fails the load", func(t *testing.T) {
		root := writeDecl(t, "branch:\n  makeTimeout: \" 90m\"\n")
		if _, err := LoadRepoRootTaskPattern(root); err == nil || !strings.Contains(err.Error(), "branch.makeTimeout") {
			t.Fatalf("whitespace-padded makeTimeout err = %v, want branch.makeTimeout load error", err)
		}
	})

	t.Run("unparsable duration fails the load", func(t *testing.T) {
		root := writeDecl(t, "branch:\n  makeTimeout: banana\n")
		if _, err := LoadRepoRootTaskPattern(root); err == nil || !strings.Contains(err.Error(), "branch.makeTimeout") {
			t.Fatalf("unparsable makeTimeout err = %v, want branch.makeTimeout load error", err)
		}
	})

	for _, raw := range []string{"0s", "-5m"} {
		t.Run("non-positive "+raw+" fails the load", func(t *testing.T) {
			root := writeDecl(t, "branch:\n  makeTimeout: "+raw+"\n")
			if _, err := LoadRepoRootTaskPattern(root); err == nil || !strings.Contains(err.Error(), "must be positive") {
				t.Fatalf("makeTimeout %q err = %v, want positive-duration load error", raw, err)
			}
		})
	}

	t.Run("json declaration parses", func(t *testing.T) {
		root := t.TempDir()
		body := `{"branch": {"integrationBranch": "master", "makeTimeout": "45m"}}`
		if err := os.WriteFile(filepath.Join(root, ".gz-git.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		decl, err := LoadRepoRootTaskPattern(root)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if decl.MakeTimeout != 45*time.Minute {
			t.Fatalf("json makeTimeout = %s, want 45m", decl.MakeTimeout)
		}
	})

	t.Run("json numeric duration fails the load", func(t *testing.T) {
		root := t.TempDir()
		body := `{"branch": {"makeTimeout": 30}}`
		if err := os.WriteFile(filepath.Join(root, ".gz-git.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadRepoRootTaskPattern(root); err == nil || !strings.Contains(err.Error(), "must be a string") {
			t.Fatalf("numeric json makeTimeout err = %v, want string-type load error", err)
		}
	})
}
