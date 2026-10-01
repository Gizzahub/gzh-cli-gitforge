// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMakeTargetBudget pins the make-probe budget contract: zero configured
// keeps the built-in default, an explicit budget passes through, and a probe
// that outlives its budget is killed with an error naming the budget — so a
// slow repository can raise the ceiling in its own declaration instead of
// being killed by the built-in one.
func TestMakeTargetBudget(t *testing.T) {
	t.Run("zero configured keeps the built-in default", func(t *testing.T) {
		if got := resolveMakeBudget(0); got != makeTargetTimeout {
			t.Fatalf("resolveMakeBudget(0) = %s, want the built-in default %s", got, makeTargetTimeout)
		}
	})

	t.Run("explicit budget passes through", func(t *testing.T) {
		budget := 90 * time.Minute
		if got := resolveMakeBudget(budget); got != budget {
			t.Fatalf("resolveMakeBudget(%s) = %s, want pass-through", budget, got)
		}
	})

	t.Run("probe outliving its budget is killed naming it", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("make process-group kill is POSIX-shaped")
		}
		dir := t.TempDir()
		writeRepoFile(t, dir, "Makefile", "check:\n\tsleep 5\n")
		start := time.Now()
		probe := runMakeTarget(context.Background(), dir, "check", 100*time.Millisecond)
		elapsed := time.Since(start)
		if !probe.Defined {
			t.Fatalf("timed-out probe must stay Defined\n%s", probe.Output)
		}
		if probe.Err == nil || !strings.Contains(probe.Err.Error(), "exceeded 100ms") {
			t.Fatalf("probe err = %v, want timeout naming the 100ms budget\n%s", probe.Err, probe.Output)
		}
		if elapsed > 3*time.Second {
			t.Fatalf("probe ran %s, want killed near the 100ms budget", elapsed)
		}
	})
}
