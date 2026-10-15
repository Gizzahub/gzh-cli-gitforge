// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestNDReadinessNativePreparation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX project-cargo fixture")
	}
	for _, mode := range []string{"ready", "ready-heavy", "failure", "source-failure", "cancel", "ref-drift", "sibling-drift"} {
		t.Run(mode, func(t *testing.T) {
			ready := strings.HasPrefix(mode, "ready")
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv(hostSlotsHeldEnv, "aux")
			slots, err := hostSlotsDir()
			if err != nil {
				t.Fatal(err)
			}
			sidecar := filepath.Join(slots, integrateLockName+".json")
			t.Setenv("ND_TEST_SLOT_FILE", sidecar)
			dependency, dep := siblingFixture(t)
			script := "#!/bin/sh\nset -eu\nprintf '%s|%s\\n' \"$CARGO_TARGET_DIR\" \"$*\" >> \"$ND_TEST_TRACE\"\n"
			switch mode {
			case "failure":
				script += "exit 7\n"
			case "cancel":
				script += "sleep 60\n"
			case "source-failure":
				script += "case \"$CARGO_TARGET_DIR\" in */source-cargo) exit 7;; esac\nprintf '%s\\n' \"$*\" >> \"$CARGO_TARGET_DIR/compiled\"\n"
			case "ref-drift":
				script += "git -C \"$ND_TEST_WORKTREE\" update-ref refs/heads/dev/actor/feat/task \"$ND_TEST_TARGET_SHA\"\n"
			case "sibling-drift":
				script += "printf 'changed\\n' > payload.txt\n"
			default:
				script += "printf '%s\\n' \"$*\" >> \"$CARGO_TARGET_DIR/compiled\"\n"
			}
			if mode == "ready-heavy" {
				script += "test \"$HOST_SLOTS_HELD\" = aux,integrate\ntest -f \"$ND_TEST_SLOT_FILE\"\n"
			}
			if err := os.MkdirAll(filepath.Join(dependency.Clone, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeRepoFile(t, dependency.Clone, "scripts/project-cargo", script)
			if err := os.Chmod(filepath.Join(dependency.Clone, "scripts", "project-cargo"), 0o755); err != nil {
				t.Fatal(err)
			}
			runGit(t, dependency.Clone, "add", "scripts/project-cargo")
			runGit(t, dependency.Clone, "commit", "-m", "fixed compiler fixture")
			runGit(t, dependency.Clone, "push", dependency.Origin, "HEAD:master")
			fx := readinessFixture(t)
			trace := filepath.Join(t.TempDir(), "trace")
			t.Setenv("ND_TEST_TRACE", trace)
			t.Setenv("CARGO_TARGET_DIR", filepath.Join(t.TempDir(), "ambient-must-not-be-used"))
			primary := filepath.Join(filepath.Dir(fx.Clone), "ndea-rs")
			runGit(t, fx.Clone, "clone", dependency.Origin, primary)
			runGit(t, primary, "checkout", "-B", "master", "origin/master")
			runGit(t, primary, "remote", "set-url", "origin", dep.Remote)
			runGit(t, primary, "remote", "set-url", "--push", "origin", dep.Remote)
			manifest := fmt.Sprintf("branch:\n  integrationBranch: develop\n  readiness:\n    version: 1\n    runner: .gz-git/readiness/check\n    prepareProfile: nd-stack-checked-tests-v1\n  siblingProvision:\n    version: 1\n    siblings:\n      - name: ndea-rs\n        remote: %q\n        path: ../ndea-rs\n        sourceBranch: master\n        revision: source-tip\n", dep.Remote)
			if mode == "ready-heavy" {
				manifest += "  prepareHighLoad: true\n"
			}
			runner := "#!/bin/sh\nset -eu\ntest \"$#\" = 8\ntest -f \"$CARGO_TARGET_DIR/compiled\"\ntest \"$(wc -l < \"$CARGO_TARGET_DIR/compiled\" | tr -d ' ')\" = 3\ncase \"$CARGO_TARGET_DIR\" in */source-cargo) ;; *) exit 1;; esac\nprintf 'runner\\n' >> \"$ND_TEST_TRACE\"\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"prepared source bindings\"}'\n"
			advanceReadinessTarget(t, fx, manifest, runner, "declare ND preparation")
			g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
			before := runGitInTest(t, fx.Worktree, "worktree", "list", "--porcelain")
			plan := TargetPlan{TargetSHA: strings.TrimSpace(runGitInTest(t, fx.Clone, "rev-parse", "HEAD")), BranchSHA: strings.TrimSpace(runGitInTest(t, fx.Worktree, "rev-parse", "HEAD"))}
			plan.Branch, plan.Target = "dev/actor/feat/task", "origin/develop"
			t.Setenv("ND_TEST_WORKTREE", fx.Worktree)
			t.Setenv("ND_TEST_TARGET_SHA", plan.TargetSHA)
			contract, _, err := loadReadinessContract(context.Background(), g, plan.TargetSHA)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if mode == "cancel" {
				go func() {
					for ctx.Err() == nil {
						if _, err := os.Stat(trace); err == nil {
							cancel()
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
				}()
			}
			result, _, err := runContractWithTimeout(ctx, g, plan, contract, time.Second)
			if ready && (err != nil || result.Status != "ready") {
				t.Fatalf("native result=%+v err=%v", result, err)
			}
			if !ready && (err == nil || result.Status != "unavailable") {
				t.Fatalf("failed preparation admitted runner: %+v %v", result, err)
			}
			if mode == "ref-drift" && !strings.Contains(err.Error(), "ref") {
				t.Fatalf("wrong refusal for bound ref mutation: %v", err)
			}
			data, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if mode == "sibling-drift" {
				buildTarget := strings.SplitN(lines[0], "|", 2)[0]
				retainedRoot := filepath.Dir(buildTarget)
				if !strings.HasPrefix(filepath.Base(retainedRoot), "gz-git-readiness-") {
					t.Fatalf("unexpected retained workspace: %s", retainedRoot)
				}
				contents, err := os.ReadFile(filepath.Join(retainedRoot, "target-side", "ndea-rs", "payload.txt"))
				if err != nil || string(contents) != "changed\n" {
					t.Fatalf("changed evidence lost: %q %v", contents, err)
				}
				// Only this test's newly-created synthetic workspace is reclaimed.
				t.Cleanup(func() {
					if err := os.RemoveAll(retainedRoot); err != nil {
						t.Error(err)
					}
				})
			}
			switch {
			case ready:
				if len(lines) != 7 || lines[6] != "runner" || !strings.Contains(lines[1], "--features nd-common/events") || !strings.Contains(lines[2], "-p nd-events --no-run") {
					t.Fatalf("wrong preparation coverage/order: %s", data)
				}
				for i, line := range lines[:6] {
					want := "/target-cargo|"
					if i >= 3 {
						want = "/source-cargo|"
					}
					if !strings.Contains(line, want) {
						t.Fatalf("cache mismatch: %s", line)
					}
				}
			case mode == "source-failure":
				if len(lines) != 4 {
					t.Fatalf("source failure did not stop runner: %s", data)
				}
			default:
				if len(lines) != 1 {
					t.Fatalf("failure did not stop source preparation/runner: %s", data)
				}
			}
			if mode == "ref-drift" {
				runGit(t, fx.Worktree, "update-ref", "refs/heads/dev/actor/feat/task", plan.BranchSHA)
			}
			if after := runGitInTest(t, fx.Worktree, "worktree", "list", "--porcelain"); after != before {
				t.Fatalf("readiness worktree leak: %s", after)
			}
			if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
				t.Fatalf("host slot not released: %v", err)
			}
		})
	}
}
