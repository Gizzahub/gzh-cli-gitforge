// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// clearCallerIdentity leaves the native identity chain empty: no actor or
// host in the environment and an empty global Git config, so the result
// does not depend on the machine running the test.
func clearCallerIdentity(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GZ_GIT_TASK_ACTOR", "GIT_WORK_ACTOR", "GIT_WORK_HOST"} {
		t.Setenv(key, "")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
}

// An unconfigured caller is refused before cleanup reads a receipt or
// takes a lock: nothing is created and existing residuals stay intact.
func TestCleanupOnlyRetryRefusesUnconfiguredCaller(t *testing.T) {
	t.Run("no prior run creates no evidence root", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		clearCallerIdentity(t)
		_, err := f.cleanup(t, "g7")
		if err == nil || !strings.Contains(err.Error(), "caller identity is not configured") {
			t.Fatalf("err = %v, want the unconfigured caller refusal", err)
		}
		if _, statErr := os.Lstat(evidenceRoot(f.common)); !os.IsNotExist(statErr) {
			t.Fatalf("evidence root exists after refusal: %v", statErr)
		}
	})
	t.Run("crashed run keeps its residuals", func(t *testing.T) {
		f := newCleanupRetryFX(t, "@true")
		f.realRun(t)
		wt := f.crashState(t, crashOpts{phase: evidencePhasePushed})
		clearCallerIdentity(t)
		_, err := f.cleanup(t, "g7")
		if err == nil || !strings.Contains(err.Error(), "caller identity is not configured") {
			t.Fatalf("err = %v, want the unconfigured caller refusal", err)
		}
		f.assertResidualsIntact(t, wt)
	})
}

// Git exits 1 both for an unset key and for a global config it cannot read;
// only the second warns on stderr. An unreadable config must surface Git's
// warning rather than ask the caller to configure what is already there.
func TestResolveCallerOwnerReportsUnreadableGlobalConfig(t *testing.T) {
	clearCallerIdentity(t)
	t.Setenv("GIT_CONFIG_GLOBAL", t.TempDir())
	_, err := ResolveCallerOwner(context.Background(), gitcmd.NewExecutor())
	if err == nil || !strings.Contains(err.Error(), "read global Git ce.workActor") ||
		strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v, want Git's read failure for ce.workActor", err)
	}
}
