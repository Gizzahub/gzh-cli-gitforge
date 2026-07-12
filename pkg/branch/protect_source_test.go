// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package branch

import (
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// TestProtection_SingleSource proves the single-repo cleanup path shares one
// protected-branch source with the bulk path: a pattern added only to
// repository.ProtectedBranches is honored by branch.IsProtected without a second
// edit. Paired with repository.TestBulkIsProtectedUsesSharedSource, this shows a
// new protected pattern closes both deletion paths at once.
func TestProtection_SingleSource(t *testing.T) {
	const custom = "sandbox/keep-me"

	if IsProtected(custom) {
		t.Fatalf("precondition failed: %q already protected", custom)
	}

	orig := repository.ProtectedBranches
	repository.ProtectedBranches = append(append([]string{}, orig...), custom)
	t.Cleanup(func() { repository.ProtectedBranches = orig })

	if !IsProtected(custom) {
		t.Errorf("branch.IsProtected(%q) = false; single-repo path did not honor the shared protected set", custom)
	}
}
