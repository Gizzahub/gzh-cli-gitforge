// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// TestIntegrationJSONSeparatesWaitAndValidation pins TASK-304's machine
// contract: integrate run --json carries the check's slot-admission wait and
// its gate runtime as two distinct explicit integer-millisecond fields, both
// present even when zero, so a caller can tell "waited, then validated",
// "validated without waiting", and "nothing ran" apart without guessing from
// absent keys.
func TestIntegrationJSONSeparatesWaitAndValidation(t *testing.T) {
	// runIntegrateRun reads package globals; save, set, and restore every one
	// this test touches so it cannot leak into or inherit from sibling tests.
	setJSONOutput := func(t *testing.T) func() {
		t.Helper()
		origJSON, origQuiet := integrateRunJSON, quiet
		origExec, origGen := integrateRunExecutionID, integrateRunGeneration
		origActor, origHost, origKind := integrateRunOwnerActor, integrateRunOwnerHost, integrateRunOwnerKind
		origExpect := integrateRunExpectSource
		integrateRunJSON, quiet = true, false
		integrateRunExecutionID, integrateRunGeneration = "", ""
		integrateRunOwnerActor, integrateRunOwnerHost, integrateRunOwnerKind = "", "", ""
		integrateRunExpectSource = ""
		return func() {
			integrateRunJSON, quiet = origJSON, origQuiet
			integrateRunExecutionID, integrateRunGeneration = origExec, origGen
			integrateRunOwnerActor, integrateRunOwnerHost, integrateRunOwnerKind = origActor, origHost, origKind
			integrateRunExpectSource = origExpect
		}
	}

	runJSON := func(t *testing.T, fx *testutil.WorktreeOrigin) (doc map[string]any, exit int) {
		t.Helper()
		t.Chdir(fx.Worktree)
		var buf bytes.Buffer
		integrateRunCmd.SetOut(&buf)
		defer integrateRunCmd.SetOut(nil)
		err := runIntegrateRun(integrateRunCmd, []string{"dev/actor/feat/task"})
		exit = cliutil.ExitCodeForError(err)
		if jsonErr := json.Unmarshal(buf.Bytes(), &doc); jsonErr != nil {
			t.Fatalf("run --json output is not one JSON document: %v\n%s", jsonErr, buf.String())
		}
		return doc, exit
	}

	timingField := func(t *testing.T, doc map[string]any, key string) float64 {
		t.Helper()
		raw, ok := doc[key]
		if !ok {
			t.Fatalf("JSON document omits %s even though zero must be explicit: %v", key, doc)
		}
		value, isNumber := raw.(float64)
		if !isNumber {
			t.Fatalf("%s = %#v, want an integer millisecond number", key, raw)
		}
		return value
	}

	t.Run("a legacy gate run reports validation milliseconds with an explicit zero wait", func(t *testing.T) {
		restore := setIntegrateRunGlobals(t)
		defer restore()
		restoreJSON := setJSONOutput(t)
		defer restoreJSON()

		fx := testutil.TempWorktreeWithBareOrigin(t)
		writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
		// A real no-op lint target: the legacy gate probes check and lint
		// alike, so an undefined lint would leave readiness resting on the
		// skipped-check path instead of reaching the integration itself.
		writeFile(t, fx.Clone, "Makefile", "check:\n\t@sleep 0.15\nlint:\n\t@:\n")
		runGit(t, fx.Clone, "add", ".")
		runGit(t, fx.Clone, "commit", "-m", "declare gate")
		runGit(t, fx.Clone, "branch", "develop")
		runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
		runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
		writeFile(t, fx.Worktree, "task.txt", "task\n")
		runGit(t, fx.Worktree, "add", ".")
		runGit(t, fx.Worktree, "commit", "-m", "task")
		runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

		doc, exit := runJSON(t, fx)
		if exit != 0 {
			t.Fatalf("run exit = %d, want 0 (integrated): %v", exit, doc["error"])
		}
		if wait := timingField(t, doc, "resource_wait_duration_ms"); wait != 0 {
			t.Fatalf("resource_wait_duration_ms = %v, want 0: no high-load declaration means no slot wait", wait)
		}
		if validation := timingField(t, doc, "validation_duration_ms"); validation < 100 {
			t.Fatalf("validation_duration_ms = %v, want at least the gate's 150ms run", validation)
		}
	})

	t.Run("a gate that never ran renders both timing fields as explicit zero", func(t *testing.T) {
		restore := setIntegrateRunGlobals(t)
		defer restore()
		restoreJSON := setJSONOutput(t)
		defer restoreJSON()

		// The catch-all guard satisfies check and lint through implicit
		// rules, so the check is not ready and no gate subprocess ran; the
		// protocol must still carry both fields as literal zeros.
		fx := testutil.TempWorktreeWithBareOrigin(t)
		writeFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n")
		writeFile(t, fx.Clone, "Makefile", "%:\n\t@:\n")
		runGit(t, fx.Clone, "add", ".")
		runGit(t, fx.Clone, "commit", "-m", "catch-all gate")
		runGit(t, fx.Clone, "branch", "develop")
		runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
		runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
		writeFile(t, fx.Worktree, "task.txt", "task\n")
		runGit(t, fx.Worktree, "add", ".")
		runGit(t, fx.Worktree, "commit", "-m", "task")
		runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

		doc, exit := runJSON(t, fx)
		if exit != 1 {
			t.Fatalf("run exit = %d, want 1 (not ready): %v", exit, doc["error"])
		}
		if integrated, ok := doc["integrated"].(bool); !ok || integrated {
			t.Fatalf("integrated = %#v, want false", doc["integrated"])
		}
		if wait := timingField(t, doc, "resource_wait_duration_ms"); wait != 0 {
			t.Fatalf("resource_wait_duration_ms = %v, want an explicit 0", wait)
		}
		if validation := timingField(t, doc, "validation_duration_ms"); validation != 0 {
			t.Fatalf("validation_duration_ms = %v, want an explicit 0 when no gate subprocess ran", validation)
		}
	})
}
