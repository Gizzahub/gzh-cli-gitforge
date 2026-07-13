// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposync

import (
	"context"
	"strings"
	"testing"
)

// TestCheckoutBranch_RejectsOptionInjection verifies that checkoutBranch — which
// bypasses pkg/repository and calls git directly — rejects config-derived branch
// names git could read as options, before any git process runs (AC4).
func TestCheckoutBranch_RejectsOptionInjection(t *testing.T) {
	ctx := context.Background()
	logger := nopGitLogger{}
	dir := t.TempDir() // not a git repo; rejection must precede any git op

	for _, br := range []string{"--upload-pack=/tmp/evil", "-x", "--output=/tmp/x"} {
		_, err := checkoutBranch(ctx, dir, br, logger)
		if err == nil || !strings.Contains(err.Error(), "invalid branch name") {
			t.Fatalf("branch %q: expected invalid branch name error, got %v", br, err)
		}
	}
}

// TestCheckoutBranch_AllowsValidBranch guards AC3: a legitimate comma-separated
// fallback list must pass the validator. On a non-repo dir it fails later with a
// "none of the specified branches exist" error, never "invalid branch name".
func TestCheckoutBranch_AllowsValidBranch(t *testing.T) {
	ctx := context.Background()
	logger := nopGitLogger{}
	dir := t.TempDir()

	_, err := checkoutBranch(ctx, dir, "develop,master", logger)
	if err != nil && strings.Contains(err.Error(), "invalid branch name") {
		t.Fatalf("valid branch list wrongly rejected: %v", err)
	}
}
