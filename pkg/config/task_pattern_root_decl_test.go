// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
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

// TestRepoRootDeclarationShape keeps the authority-bearing root declaration
// unambiguous: several YAML documents or a repeated branch key are refused
// instead of being resolved by whichever value a decoder happens to keep.
func TestRepoRootDeclarationShape(t *testing.T) {
	for name, tc := range map[string]struct{ file, body string }{
		"yaml duplicate branch": {".gz-git.yaml", "branch:\n  integrationBranch: master\nbranch:\n  integrationBranch: develop\n"},
		"yaml multi document":   {".gz-git.yaml", "branch:\n  integrationBranch: master\n---\nbranch:\n  integrationBranch: develop\n"},
		"json duplicate branch": {".gz-git.json", `{"branch":{"integrationBranch":"master"},"branch":{"integrationBranch":"develop"}}`},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.file), []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadRepoRootTaskPattern(root); err == nil {
				t.Fatalf("ambiguous declaration accepted: %s", tc.body)
			}
		})
	}
}
