// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestTaskResourceFenceExcludesSameTaskAcrossWorktrees(t *testing.T) {
	fx := readyTaskFixture(t)
	exec := gitcmd.NewExecutor()
	held, err := acquireTaskResourceFence(context.Background(), exec, fx.Worktree, "")
	if err != nil {
		t.Fatal(err)
	}
	defer held.release()
	// A caller in the primary selecting the task explicitly must contend with
	// a caller selecting it implicitly from the task worktree, independently
	// of which integration target or execution ID either caller will use.
	// A held fence refuses at once and names its holder; it never waits.
	for _, spelling := range []string{"dev/actor/feat/task", " refs/heads/dev/actor/feat/task "} {
		started := time.Now()
		_, err = acquireTaskResourceFence(context.Background(), exec, fx.Clone, spelling)
		if !errors.Is(err, ErrResourceBusy) {
			t.Fatalf("same task fence (%q) error = %v, want ErrResourceBusy", spelling, err)
		}
		if want := fmt.Sprintf("pid=%d", os.Getpid()); !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "dev/actor/feat/task") {
			t.Fatalf("same task fence (%q) error = %v, want it to name the task and holder %s", spelling, err, want)
		}
		if elapsed := time.Since(started); elapsed > 5*time.Second {
			t.Fatalf("held fence refusal took %s; it must not wait", elapsed)
		}
	}
	held.release()
	reacquired, err := acquireTaskResourceFence(context.Background(), exec, fx.Clone, "dev/actor/feat/task")
	if err != nil {
		t.Fatal(err)
	}
	reacquired.release()
}

func TestTaskResourceFenceKeepsUnrelatedTasksIndependent(t *testing.T) {
	repo := testutil.TempGitRepoWithCommit(t)
	exec := gitcmd.NewExecutor()
	first, err := acquireTaskResourceFence(context.Background(), exec, repo, "dev/codex/testhost/fix/one")
	if err != nil {
		t.Fatal(err)
	}
	defer first.release()
	second, err := acquireTaskResourceFence(context.Background(), exec, repo, "dev/codex/testhost/fix/two")
	if err != nil {
		t.Fatalf("unrelated task blocked: %v", err)
	}
	second.release()
}

// TestTaskResourceFenceSharesOneFileAcrossPathSpellings pins the fence's
// user-visible exclusion across repository-path spellings: a held fence
// refuses an acquisition made through a symlinked spelling with
// ErrResourceBusy. The sentinel below also watches the one path input git
// could ever report verbatim — the gitdir pointer file of a linked
// worktree, here rewritten through the symlink. Today git canonicalizes
// that report like it does the cwd (verified when this test was written),
// so no git-reported spelling reaches the fence un-canonicalized and the
// string-level derivation cannot be discriminated from out here; it is
// unit-pinned instead in TestCanonicalCommonDirResolvesSymlinkedSpellings,
// and the fence's canonicalCommonDir call keeps its derivation identical to
// the run lock's and the receipts'. If the sentinel ever fails, git has
// started reporting symlinked spellings, and this test must then also
// assert the derived lock path below the canonical common dir. The
// exclusion itself is flock-based and holds across spellings regardless.
func TestTaskResourceFenceSharesOneFileAcrossPathSpellings(t *testing.T) {
	repo := testutil.TempGitRepoWithCommit(t)
	base := t.TempDir()
	wt := filepath.Join(base, "task-wt")
	alias := filepath.Join(base, "repo-through-link")
	exec := gitcmd.NewExecutor()
	ctx := context.Background()

	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	held, err := acquireTaskResourceFence(ctx, exec, repo, "dev/codex/testhost/fix/alias")
	if err != nil {
		t.Fatal(err)
	}
	_, err = acquireTaskResourceFence(ctx, exec, alias, "dev/codex/testhost/fix/alias")
	if !errors.Is(err, ErrResourceBusy) || !strings.Contains(err.Error(), "dev/codex/testhost/fix/alias") {
		t.Fatalf("symlinked spelling error = %v, want ErrResourceBusy naming the task", err)
	}
	held.release()

	// Sentinel: rewrite the linked worktree's gitdir pointer through the
	// symlink — the one path input git could report verbatim.
	if _, err := newGitRepo(exec, repo).run(ctx, "worktree", "add", "--quiet", wt); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(alias, ".git", "worktrees", filepath.Base(wt))
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+linked+"\n"), 0o600); err != nil { //nolint:gosec // 0o600 satisfies G306; rewriting a gitdir pointer in a test fixture
		t.Fatal(err)
	}
	reported, err := newGitRepo(exec, wt).output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(filepath.Join(repo, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if reported != physical {
		t.Fatalf("git reported --git-common-dir as %q, not the canonical %q; git now surfaces symlinked spellings, so extend this test to assert the fence's derived lock path", reported, physical)
	}
	pointerHeld, err := acquireTaskResourceFence(ctx, exec, wt, "dev/codex/testhost/fix/alias")
	if err != nil {
		t.Fatal(err)
	}
	defer pointerHeld.release()
	if dir := filepath.Dir(pointerHeld.file.Name()); dir != filepath.Join(physical, taskFenceDirName) {
		t.Fatalf("fence lock dir = %s, want %s below the canonical common dir", dir, filepath.Join(physical, taskFenceDirName))
	}
	_, err = acquireTaskResourceFence(ctx, exec, repo, "dev/codex/testhost/fix/alias")
	if !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("physical-spelling acquisition error = %v, want ErrResourceBusy on the one derived file", err)
	}
}
