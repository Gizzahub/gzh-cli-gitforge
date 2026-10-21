// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
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
