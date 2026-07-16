// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package repository

import "testing"

// TestIsProtected covers the built-in protected set and trailing-wildcard match.
func TestIsProtected(t *testing.T) {
	for _, name := range []string{"main", "master", "develop", "development", "release/1.0", "hotfix/urgent"} {
		if !IsProtected(name) {
			t.Errorf("IsProtected(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"feature/x", "my-branch", "released"} {
		if IsProtected(name) {
			t.Errorf("IsProtected(%q) = true, want false", name)
		}
	}
}

// TestBulkIsProtectedUsesSharedSource proves the bulk cleanup predicate resolves
// built-in protection through the shared ProtectedBranches source: appending a
// pattern there makes the bulk path refuse a matching branch with no change to
// the bulk code itself.
func TestBulkIsProtectedUsesSharedSource(t *testing.T) {
	c := &client{}
	const custom = "sandbox/keep-me"

	if c.isProtectedBranch(custom, "", nil) {
		t.Fatalf("precondition failed: %q already protected", custom)
	}

	orig := ProtectedBranches
	ProtectedBranches = append(append([]string{}, orig...), custom)
	t.Cleanup(func() { ProtectedBranches = orig })

	if !c.isProtectedBranch(custom, "", nil) {
		t.Errorf("bulk isProtectedBranch(%q) = false; bulk path did not honor the shared protected set", custom)
	}
}
