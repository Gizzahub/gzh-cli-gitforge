// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
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
	for _, spelling := range []string{"dev/actor/feat/task", " refs/heads/dev/actor/feat/task "} {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err = acquireTaskResourceFence(ctx, exec, fx.Clone, spelling)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("same task fence (%q) error = %v, want context deadline", spelling, err)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	second, err := acquireTaskResourceFence(ctx, exec, repo, "dev/codex/testhost/fix/two")
	if err != nil {
		t.Fatalf("unrelated task blocked: %v", err)
	}
	second.release()
}
