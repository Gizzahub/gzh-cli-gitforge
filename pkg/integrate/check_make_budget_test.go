// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
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

// TestMakeTimeoutIsNotMaskedAsRC0 pins that a probe killed by its budget is
// judged as a timeout. A killed make has no exit code, so comparing it with
// the baseline rendered "failed here (rc=0) but target tip passes" and threw
// the cause away; a baseline killed the same way left a truncated output to
// be counted as if it were a finished measurement.
func TestMakeTimeoutIsNotMaskedAsRC0(t *testing.T) {
	ctx := context.Background()
	timedOut := func(side string) makeProbe {
		return makeProbe{
			Target: "check", Defined: true, TimedOut: true,
			Err: errors.New(side + ": make check exceeded 15m0s and was killed"),
		}
	}
	failed := makeProbe{Target: "check", Defined: true, Code: 2, Err: errors.New("exit status 2")}
	passed := makeProbe{Target: "check", Defined: true}

	for _, allowSkipped := range []bool{false, true} {
		branch := timedOut("branch")
		// The zero gitRepo cannot build a baseline worktree, so reaching the
		// baseline at all would surface as a different failure detail.
		legacy := judgeMakeLegacy(ctx, gitRepo{}, TargetPlan{}, branch, allowSkipped, 0)
		against := judgeMakeAgainstProbe(ctx, gitRepo{}, TargetPlan{}, branch, allowSkipped, passed, 0)
		for name, item := range map[string]CheckItem{"legacy": legacy, "against-probe": against} {
			if item.Status != checkFail {
				t.Errorf("%s branch timeout (allowSkipped=%t) status = %s, want fail: %+v", name, allowSkipped, item.Status, item)
			}
			if !strings.Contains(item.Detail, "branch: make check exceeded 15m0s") {
				t.Errorf("%s branch timeout detail lost the cause: %q", name, item.Detail)
			}
			if strings.Contains(item.Detail, "rc=0") || strings.Contains(item.Detail, "target tip passes") {
				t.Errorf("%s branch timeout compared against the baseline: %q", name, item.Detail)
			}
		}
	}

	t.Run("baseline timeout is unmeasurable, not a comparison", func(t *testing.T) {
		item := judgeMakeAgainstProbe(ctx, gitRepo{}, TargetPlan{}, failed, false, timedOut("baseline"), 0)
		if item.Status != checkFail || !strings.Contains(item.Detail, "baseline unmeasurable") ||
			!strings.Contains(item.Detail, "baseline: make check exceeded 15m0s") {
			t.Fatalf("baseline timeout = %+v, want unmeasurable naming the timeout", item)
		}
		downgraded := judgeMakeAgainstProbe(ctx, gitRepo{}, TargetPlan{}, failed, true, timedOut("baseline"), 0)
		if downgraded.Status != checkWarn {
			t.Fatalf("baseline timeout with --allow-skipped-checks = %+v, want warn", downgraded)
		}
	})

	t.Run("exit code is reported when there is one", func(t *testing.T) {
		item := judgeMakeAgainstProbe(ctx, gitRepo{}, TargetPlan{}, failed, false, passed, 0)
		if item.Status != checkFail || !strings.Contains(item.Detail, "failed here (rc=2) but target tip passes") {
			t.Fatalf("ordinary failure = %+v, want the rc=2 baseline verdict", item)
		}
	})

	t.Run("a failure without an exit code names its error", func(t *testing.T) {
		noCode := makeProbe{Target: "check", Defined: true, Err: errors.New("start make: not found")}
		item := judgeMakeAgainstProbe(ctx, gitRepo{}, TargetPlan{}, noCode, false, passed, 0)
		if item.Status != checkFail || strings.Contains(item.Detail, "rc=0") || !strings.Contains(item.Detail, "start make: not found") {
			t.Fatalf("exit-code-less failure = %+v, want its error instead of rc=0", item)
		}
	})

	t.Run("runMakeTarget marks a killed probe", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("make process-group kill is POSIX-shaped")
		}
		dir := t.TempDir()
		writeRepoFile(t, dir, "Makefile", "check:\n\tsleep 5\n")
		if probe := runMakeTarget(ctx, dir, "check", 100*time.Millisecond); !probe.TimedOut {
			t.Fatalf("killed probe TimedOut = false: %+v", probe)
		}
		writeRepoFile(t, dir, "Makefile", "check:\n\texit 3\n")
		// make reports a failed recipe as its own exit status 2.
		if probe := runMakeTarget(ctx, dir, "check", time.Minute); probe.TimedOut || probe.Code != 2 {
			t.Fatalf("ordinary failure probe = %+v, want make's Code 2 and not TimedOut", probe)
		}
	})
}
