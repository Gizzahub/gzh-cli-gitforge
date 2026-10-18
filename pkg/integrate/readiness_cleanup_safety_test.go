// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

func TestWorktreeRemoveForceReportsExitAndEmptyStderr(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "git-fail")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(binary)), t.TempDir()).worktreeRemoveForce(context.Background(), filepath.Join(t.TempDir(), "ready"))
	if err == nil || !strings.Contains(err.Error(), "exit 23") || !strings.Contains(err.Error(), "exit status 23") {
		t.Fatalf("empty-stderr removal failure = %v, want exit and command cause", err)
	}
}

func TestWorktreeRemoveForcePreservesCanceledContext(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "git-fail")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(binary)), t.TempDir()).worktreeRemoveForce(ctx, filepath.Join(t.TempDir(), "ready"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled removal error = %v, want errors.Is(context.Canceled)", err)
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("canceled removal error lacks context detail: %v", err)
	}
}

func TestReadinessCleanupRefusesReplacementAtSamePath(t *testing.T) {
	fx := readinessFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	path := filepath.Join(t.TempDir(), "target")
	head := gitOutput(t, fx.Worktree, "rev-parse", "develop")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
	identity, err := captureReadinessWorktree(context.Background(), g, path, head)
	if err != nil {
		t.Fatalf("capture identity: %v", err)
	}
	runGit(t, fx.Worktree, "worktree", "move", path, path+"-old")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)

	err = removeReadinessWorktree(context.Background(), g, identity)
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("replacement cleanup = %v, want physical identity refusal", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("replacement worktree was removed: %v", statErr)
	}
	runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
	runGit(t, fx.Worktree, "worktree", "remove", "--force", path+"-old")
}

func TestCaptureReadinessWorktreeRefusesEndpointSymlink(t *testing.T) {
	fx := readinessFixture(t)
	path := filepath.Join(t.TempDir(), "target")
	outside := t.TempDir()
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	_, err := captureReadinessWorktree(context.Background(), newGitRepo(gitcmd.NewExecutor(), fx.Worktree), path, gitOutput(t, fx.Worktree, "rev-parse", "develop"))
	if err == nil || !strings.Contains(err.Error(), "unsupported directory type") {
		t.Fatalf("endpoint symlink capture = %v, want refusal", err)
	}
}

func TestReadinessCleanupRefusesChangedHeadUnregisteredAndRemovalSymlink(t *testing.T) {
	fx := readinessFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	head := gitOutput(t, fx.Worktree, "rev-parse", "develop")
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(string)
	}{
		{"changed-head", "HEAD changed", func(path string) { runGit(t, path, "commit", "--allow-empty", "-m", "change detached head") }},
		{"unregistered", "foreign or replaced top-level", func(path string) {
			backup := path + "-kept"
			if err := os.Rename(path, backup); err != nil {
				t.Fatal(err)
			}
			runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
			if err := os.Rename(backup, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"endpoint-symlink", "unexpected physical resource type", func(path string) {
			runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "target")
			runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
			identity, err := captureReadinessWorktree(context.Background(), g, path, head)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(path)
			if err := removeReadinessWorktree(context.Background(), g, identity); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("replacement refusal = %v; want %q", err, tc.want)
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatalf("replacement was not preserved: %v", err)
			}
		})
	}
}

func TestReadinessCleanupDoesNotRetryRemoveFailure(t *testing.T) {
	fx := readinessFixture(t)
	real := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	path := filepath.Join(t.TempDir(), "target")
	head := gitOutput(t, fx.Worktree, "rev-parse", "develop")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
	identity, err := captureReadinessWorktree(context.Background(), real, path, head)
	if err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(t.TempDir(), "remove-count")
	binary := filepath.Join(t.TempDir(), "git")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = worktree ] && [ \"$2\" = remove ]; then echo refused >&2; printf x >> \"$GZ_TEST_REMOVE_LOG\"; exit 47; fi\nexec \"$GZ_TEST_REAL_GIT\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err = removeReadinessWorktree(context.Background(), newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(binary), gitcmd.WithEnv([]string{"GZ_TEST_REAL_GIT=" + realGit, "GZ_TEST_REMOVE_LOG=" + count})), fx.Worktree), identity)
	if err == nil || !strings.Contains(err.Error(), "exit 47") || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("remove failure = %v", err)
	}
	data, readErr := os.ReadFile(count)
	if readErr != nil || string(data) != "x" {
		t.Fatalf("remove attempts = %q, %v; want one", data, readErr)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("failed worktree was removed: %v", statErr)
	}
	runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
}

func TestReadinessWorktreeIdentityRefusesAttachedHeadAndForeignCommonIdentity(t *testing.T) {
	fx := readinessFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	path := filepath.Join(t.TempDir(), "target")
	head := gitOutput(t, fx.Worktree, "rev-parse", "develop")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
	identity, err := captureReadinessWorktree(context.Background(), g, path, head)
	if err != nil {
		t.Fatalf("capture identity: %v", err)
	}
	runGit(t, path, "checkout", "-b", "dev/test/attached-readiness")
	if err := verifyReadinessWorktree(context.Background(), g, identity); err == nil || !strings.Contains(err.Error(), "expected detached HEAD") {
		t.Fatalf("attached worktree verification = %v, want detached-head refusal", err)
	}
	runGit(t, fx.Worktree, "worktree", "remove", "--force", path)

	path = filepath.Join(t.TempDir(), "common")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
	identity, err = captureReadinessWorktree(context.Background(), g, path, head)
	if err != nil {
		t.Fatalf("capture common identity: %v", err)
	}
	identity.common = LegacyFileIdentity{}
	if err := verifyReadinessWorktree(context.Background(), g, identity); err == nil || !strings.Contains(err.Error(), "common repository: physical identity changed") {
		t.Fatalf("common repository verification = %v, want physical identity refusal", err)
	}
	runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
}

func TestRemoveEmptyReadinessRootIgnoresUnrelatedStaleRegistration(t *testing.T) {
	fx := readinessFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	stale := filepath.Join(t.TempDir(), "unrelated")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", stale, "develop")
	if err := os.RemoveAll(stale); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "gz-git-readiness-owned")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	identity, err := legacyPhysicalIdentity(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := removeEmptyReadinessRoot(context.Background(), g, root, identity, false); err != nil {
		t.Fatalf("unrelated stale registration blocked root cleanup: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("owned root remains: %v", err)
	}
	if !strings.Contains(gitOutput(t, fx.Worktree, "worktree", "list", "--porcelain"), "worktree ") || !strings.Contains(gitOutput(t, fx.Worktree, "worktree", "list", "--porcelain"), "/unrelated\n") {
		t.Fatal("unrelated stale registration was changed")
	}
	runGit(t, fx.Worktree, "worktree", "prune", "--expire", "now")
}

func TestReadinessCleanupRunPreservesFailedWorktreesAndRoot(t *testing.T) {
	fx := readinessFixture(t)
	isolated := t.TempDir()
	t.Setenv("TMPDIR", isolated)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	removeLog := filepath.Join(t.TempDir(), "removals")
	binary := filepath.Join(t.TempDir(), "git-failing-remove")
	body := "#!/bin/sh\nif [ \"$1\" = worktree ] && [ \"$2\" = remove ]; then\n printf '%s\\n' \"$4\" >> \"$GZ_TEST_REMOVE_LOG\"\n echo synthetic-remove-refusal >&2\n exit 47\nfi\nexec \"$GZ_TEST_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(binary, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	g := newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(binary), gitcmd.WithEnv([]string{
		"GZ_TEST_REAL_GIT=" + realGit, "GZ_TEST_REMOVE_LOG=" + removeLog,
	})), fx.Worktree)
	plan := TargetPlan{BranchSHA: gitOutput(t, fx.Worktree, "rev-parse", "HEAD"), TargetSHA: gitOutput(t, fx.Worktree, "rev-parse", "develop")}
	contract, present, err := loadReadinessContract(context.Background(), g, plan.TargetSHA)
	if err != nil || !present {
		t.Fatalf("load contract: present=%v err=%v", present, err)
	}
	result, _, runErr := runContractWithTimeout(context.Background(), g, plan, contract, time.Minute)
	if runErr == nil || result.Status != "unavailable" || result.Summary != "target gate passed" {
		t.Fatalf("ready runner plus failed cleanup: result=%+v err=%v", result, runErr)
	}
	for _, detail := range []string{"exit 47", "synthetic-remove-refusal", "retained readiness workspace"} {
		if !strings.Contains(runErr.Error(), detail) {
			t.Errorf("cleanup error lacks %q: %v", detail, runErr)
		}
	}
	data, err := os.ReadFile(removeLog)
	if err != nil {
		t.Fatal(err)
	}
	paths := strings.Fields(string(data))
	if len(paths) != 2 || paths[0] == paths[1] {
		t.Fatalf("want one removal per distinct worktree, got %q", data)
	}
	registration := gitOutput(t, fx.Worktree, "worktree", "list", "--porcelain")
	for _, path := range paths {
		if !strings.Contains(runErr.Error(), path) || !strings.Contains(registration, "worktree "+path+"\n") {
			t.Errorf("failed path/registration missing: %s; error=%v", path, runErr)
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			t.Errorf("failed worktree files were erased: %s: %v", path, err)
		}
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			t.Errorf("failed root was erased: %v", err)
		}
		// Only this synthetic fixture's captured paths are removed after assertions.
		runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
	}
	if err := os.Remove(filepath.Dir(paths[0])); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreeRemoveForceDeadlineAndExecutorCauses(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "git-slow")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(binary)), t.TempDir()).worktreeRemoveForce(ctx, "owned")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("passed deadline cause lost: %v", err)
	}
	err = newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(binary), gitcmd.WithTimeout(20*time.Millisecond)), t.TempDir()).worktreeRemoveForce(context.Background(), "owned")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !strings.Contains(err.Error(), "exit -1") || strings.Contains(err.Error(), "no stderr") {
		t.Fatalf("executor termination cause lost: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing-git")
	err = newGitRepo(gitcmd.NewExecutor(gitcmd.WithGitBinary(missing)), t.TempDir()).worktreeRemoveForce(context.Background(), "owned")
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || !strings.Contains(err.Error(), "missing-git") || !strings.Contains(err.Error(), "exit -1") {
		t.Fatalf("start failure cause lost: %v", err)
	}
}

func TestReadinessCleanupRevalidatesAfterExplicitRetry(t *testing.T) {
	fx := readinessFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	path := filepath.Join(t.TempDir(), "owned")
	head := gitOutput(t, fx.Worktree, "rev-parse", "develop")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
	identity, err := captureReadinessWorktree(context.Background(), g, path, head)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, fx.Worktree, "worktree", "lock", path)
	if err := removeReadinessWorktree(context.Background(), g, identity); err == nil {
		t.Fatal("locked registration accepted")
	}
	runGit(t, fx.Worktree, "worktree", "unlock", path)
	if err := removeReadinessWorktree(context.Background(), g, identity); err != nil {
		t.Fatalf("same identity retry failed after lock released: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("successful explicit retry retained worktree: %v", err)
	}
}

func TestReadinessCleanupRefusesPhysicalCommonRepositoryReplacement(t *testing.T) {
	fx := readinessFixture(t)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	path := filepath.Join(t.TempDir(), "owned")
	head := gitOutput(t, fx.Worktree, "rev-parse", "develop")
	runGit(t, fx.Worktree, "worktree", "add", "--detach", path, head)
	identity, err := captureReadinessWorktree(context.Background(), g, path, head)
	if err != nil {
		t.Fatal(err)
	}
	backup := identity.commonDir + "-captured-backup"
	if err := os.Rename(identity.commonDir, backup); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(identity.commonDir); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove synthetic replacement: %v", err)
			return
		}
		if err := os.Rename(backup, identity.commonDir); err != nil {
			t.Errorf("restore synthetic common repository: %v", err)
			return
		}
		runGit(t, fx.Worktree, "worktree", "remove", "--force", path)
	})
	if err := os.Mkdir(identity.commonDir, 0o700); err != nil {
		t.Fatal(err)
	}
	err = removeReadinessWorktree(context.Background(), g, identity)
	if err == nil || !strings.Contains(err.Error(), "common repository: physical identity changed") {
		t.Fatalf("same-path common repository replacement accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		t.Fatalf("worktree deleted after common repository replacement: %v", err)
	}
}

func TestReadinessCleanupRunCleansAfterParentCancellation(t *testing.T) {
	fx := readinessFixture(t)
	marker := filepath.Join(t.TempDir(), "runner-started")
	t.Setenv("GZ_TEST_RUNNER_STARTED", marker)
	advanceReadinessTarget(t, fx, "", "#!/bin/sh\nprintf started > \"$GZ_TEST_RUNNER_STARTED\"\nexec sleep 30\n", "cancel after runner starts")
	isolated := t.TempDir()
	t.Setenv("TMPDIR", isolated)
	g := newGitRepo(gitcmd.NewExecutor(), fx.Worktree)
	plan := TargetPlan{BranchSHA: gitOutput(t, fx.Worktree, "rev-parse", "HEAD"), TargetSHA: gitOutput(t, fx.Worktree, "rev-parse", "develop")}
	contract, present, err := loadReadinessContract(context.Background(), g, plan.TargetSHA)
	if err != nil || !present {
		t.Fatalf("load contract: present=%v err=%v", present, err)
	}
	before := gitOutput(t, fx.Worktree, "worktree", "list", "--porcelain")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	observerDone := make(chan struct{})
	go func() {
		defer close(observerDone)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(marker); err == nil {
					cancel()
					return
				}
			}
		}
	}()
	result, _, runErr := runContractWithTimeout(ctx, g, plan, contract, time.Minute)
	cancel()
	<-observerDone
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("runner never reached cancellation boundary: %v", err)
	}
	if !errors.Is(runErr, context.Canceled) || result.Status != "unavailable" || strings.Contains(runErr.Error(), "cleanup") {
		t.Fatalf("cancellation invented cleanup failure: result=%+v error=%v", result, runErr)
	}
	if after := gitOutput(t, fx.Worktree, "worktree", "list", "--porcelain"); after != before {
		t.Fatalf("cancellation leaked worktree registrations:\nbefore=%s\nafter=%s", before, after)
	}
	entries, err := os.ReadDir(isolated)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancellation leaked workspace roots: %v; err=%v", entries, err)
	}
}
