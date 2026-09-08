// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestAdhocPushScopeAxes pins which `.gz-git.yaml` declarations narrow the
// target set of an ad-hoc `gz-git push`, and -- just as importantly -- which
// ones do not.
//
// The table is the whole point. A test that only asserted `access: read-only`
// works would leave the other three rows to be rediscovered by someone reading
// a key's name and assuming it applies here. `discovery.mode` and
// `sync.strategy` both name things that sound like scope, and neither one is,
// for two different reasons this test keeps separated:
//
//   - `sync.strategy` is a real key on a different axis. Its only non-test
//     reader is pkg/workspacecli/sync_command.go, the declarative `workspace
//     sync` engine. It governs how a repository is reconciled with its remote
//     during a sync, not whether a push may target it.
//   - `discovery.mode` is read by nothing at all. pkg/config/validator.go
//     accepts and validates it, and no other non-test file consults it. A key
//     that validates and is never read is worse than an absent one: the
//     validator's silence reads as confirmation.
//
// Both scoping axes that do exist are covered here: exclusion at scan time
// (defaults.scan.exclude, which every ad-hoc bulk command applies) and refusal
// at write time (access: read-only, which push, `workspace sync --push` and
// `handoff end` honor).
func TestAdhocPushScopeAxes(t *testing.T) {
	tests := []struct {
		name string
		// declaration is spliced under the workspace entry for the repository.
		declaration  string
		wantExcluded bool
		wantAllowed  bool
		why          string
	}{
		{
			name:         "no declaration",
			declaration:  "",
			wantExcluded: false,
			wantAllowed:  true,
			why:          "an undeclared repository under the tree is a push target",
		},
		{
			name:         "access read-only",
			declaration:  "    access: read-only\n",
			wantExcluded: false,
			wantAllowed:  false,
			why:          "the write-axis refusal: still scanned, but never pushed",
		},
		{
			name:         "discovery mode explicit",
			declaration:  "    discovery:\n      mode: explicit\n",
			wantExcluded: false,
			wantAllowed:  true,
			why:          "no code reads discovery.mode; it cannot narrow anything",
		},
		{
			name:         "sync strategy skip",
			declaration:  "    sync:\n      strategy: skip\n",
			wantExcluded: false,
			wantAllowed:  true,
			why:          "sync.strategy belongs to `workspace sync`, not to push",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			repoPath := filepath.Join(root, "target")
			if err := os.MkdirAll(repoPath, 0o755); err != nil {
				t.Fatal(err)
			}
			body := "version: \"1.0\"\nworkspaces:\n  target:\n    path: target\n" + tt.declaration
			if err := os.WriteFile(filepath.Join(root, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}

			if got := scanExcludes(t, root, "target"); got != tt.wantExcluded {
				t.Errorf("scan exclusion = %v, want %v (%s)", got, tt.wantExcluded, tt.why)
			}

			allowed, reason, err := configuredWorkspacePushAccess(repoPath)
			if err != nil {
				t.Fatalf("configuredWorkspacePushAccess: %v", err)
			}
			if allowed != tt.wantAllowed {
				t.Errorf("push allowed = %v (reason %q), want %v (%s)", allowed, reason, tt.wantAllowed, tt.why)
			}
		})
	}
}

// TestAdhocPushScopeExcludeIsDeclarable is the counterpart the table above
// deliberately leaves out of its rows: the repository-owned way to keep a
// directory out of an ad-hoc run entirely.
//
// It exists because the alternative -- carrying an --exclude regex on every
// invocation -- puts the declaration in the caller's shell history, where it
// grows a branch per piece of debris and is lost the moment someone runs the
// command from memory. defaults.scan.exclude puts the same regex in the tree
// that owns the debris.
func TestAdhocPushScopeExcludeIsDeclarable(t *testing.T) {
	root := t.TempDir()
	body := "version: \"1.0\"\ndefaults:\n  scan:\n    exclude:\n      - /tmp/\n      - /legacy/\n"
	if err := os.WriteFile(filepath.Join(root, ".gz-git.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	if !scanExcludes(t, root, "tmp/e2e-probe-15026") {
		t.Error("declared /tmp/ exclusion did not remove a temporary clone from the scan")
	}
	if !scanExcludes(t, root, "legacy/proxynd-core") {
		t.Error("declared /legacy/ exclusion did not remove an archived lineage from the scan")
	}
	if scanExcludes(t, root, "active-project") {
		t.Error("a declared exclusion removed a repository it does not name")
	}
}

// scanExcludes reports whether the exclusion regex the CLI hands the scanner
// matches relPath. It goes through resolveScanExclude rather than reading the
// config directly, so it measures what push actually filters on.
func scanExcludes(t *testing.T, root, relPath string) bool {
	t.Helper()
	pattern := resolveScanExclude(root, "")
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("resolved exclude %q does not compile: %v", pattern, err)
	}
	return re.MatchString("/" + relPath)
}
