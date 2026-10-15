package integrate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestLegacyCleanupResumeCrashGaps(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase string
	}{
		{"after-worktree", legacyPhaseIntent},
		{"after-local", legacyPhaseWorktree},
		{"after-remote", legacyPhaseLocal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, planPath, primary, task := legacySecurityPlan(t, true)
			intentPath := filepath.Join(plan.CommonDir, legacyCleanupIntentDir, plan.PlanDigest+".json")
			switch tc.phase {
			case legacyPhaseIntent:
				runGit(t, primary, "worktree", "remove", task)
			case legacyPhaseWorktree:
				runGit(t, primary, "worktree", "remove", task)
				runGit(t, primary, "update-ref", "-d", "--no-deref", "refs/heads/"+plan.TaskBranch, plan.TaskOID)
			case legacyPhaseLocal:
				runGit(t, primary, "worktree", "remove", task)
				runGit(t, primary, "update-ref", "-d", "--no-deref", "refs/heads/"+plan.TaskBranch, plan.TaskOID)
				runGit(t, primary, "push", "--force-with-lease=refs/heads/"+plan.TaskBranch+":"+plan.TaskOID, plan.Endpoint, ":refs/heads/"+plan.TaskBranch)
			}
			if err := writeLegacyCleanupIntent(intentPath, &legacyCleanupIntent{SchemaVersion: legacyCleanupIntentSchema, PlanDigest: plan.PlanDigest, Phase: tc.phase}); err != nil {
				t.Fatal(err)
			}
			if err := ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: plan.PlanDigest}); err != nil {
				t.Fatal(err)
			}
			intent, err := loadLegacyCleanupIntent(intentPath)
			if err != nil || intent == nil || intent.Phase != legacyPhaseComplete {
				t.Fatalf("intent=%+v err=%v", intent, err)
			}
		})
	}

	t.Run("reappeared-remote-is-partial", func(t *testing.T) {
		plan, planPath, primary, _ := legacySecurityPlan(t, true)
		if err := ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: plan.PlanDigest}); err != nil {
			t.Fatal(err)
		}
		runGit(t, primary, "push", plan.Endpoint, plan.TaskOID+":refs/heads/"+plan.TaskBranch)
		err := ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: plan.PlanDigest})
		if !errors.Is(err, ErrLegacyCleanupPartial) || LegacyCleanupExitCode(err) != 3 {
			t.Fatalf("err=%v code=%d", err, LegacyCleanupExitCode(err))
		}
	})

	t.Run("unknown-intent-preserves-resources", func(t *testing.T) {
		plan, planPath, _, task := legacySecurityPlan(t, true)
		intentPath := filepath.Join(plan.CommonDir, legacyCleanupIntentDir, plan.PlanDigest+".json")
		if err := writeLegacyCleanupIntent(intentPath, &legacyCleanupIntent{SchemaVersion: legacyCleanupIntentSchema, PlanDigest: plan.PlanDigest, Phase: "unknown"}); err != nil {
			t.Fatal(err)
		}
		err := ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: plan.PlanDigest})
		if err == nil {
			t.Fatal("unknown intent unexpectedly succeeded")
		}
		if _, statErr := os.Stat(task); statErr != nil {
			t.Fatalf("task worktree changed: %v", statErr)
		}
	})
}

func TestLegacyCleanupApplyStandaloneRealWorktree(t *testing.T) {
	t.Setenv("GZ_GIT_TASK_ACTOR", "codex")
	t.Setenv("GIT_WORK_HOST", "fixture")
	fx := testutil.TempWorktreeWithBareOrigin(t)
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(fx.Clone, ".gz-git.yaml"), "branch:\n  integrationBranch: develop\n  taskPattern:\n    - dev/*\n")
	runGit(t, fx.Clone, "add", ".gz-git.yaml")
	runGit(t, fx.Clone, "commit", "-m", "policy")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/main")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/develop")
	runGit(t, fx.Clone, "fetch", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-B", "dev/codex/cleanup", fx.Remote+"/develop")
	write(filepath.Join(fx.Worktree, "task.txt"), "task")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "dev/codex/cleanup")
	runGit(t, fx.Worktree, "push", fx.Remote, "HEAD:refs/heads/develop")
	runGit(t, fx.Clone, "fetch", fx.Remote)
	common := gitOutput(t, fx.Clone, "rev-parse", "--path-format=absolute", "--git-common-dir")
	taskSHA := gitOutput(t, fx.Worktree, "rev-parse", "HEAD")
	taskWorktree, err := filepath.EvalSymlinks(fx.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerIdentity{Actor: "codex", Host: "fixture", Kind: "agent"}
	done, released := filepath.Join(t.TempDir(), "done.json"), filepath.Join(t.TempDir(), "released.json")
	for path, status := range map[string]string{done: "DONE", released: "RELEASED"} {
		data, _ := json.Marshal(legacyCleanupArtifact{SchemaVersion: 1, Status: status, TaskSHA: taskSHA, TaskBranch: "dev/codex/cleanup", TaskWorktree: taskWorktree, CommonDir: common, Owner: owner})
		write(path, string(data))
	}
	plan, err := BuildLegacyCleanupPlan(context.Background(), gitcmd.NewExecutor(), LegacyCleanupPlanOptions{RepoPath: fx.Worktree, Target: "origin/develop", DoneArtifactPath: done, ReleasedArtifactPath: released, Owner: owner})
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := WriteLegacyCleanupPlan(planPath, plan); err != nil {
		t.Fatal(err)
	}
	if err := ApplyLegacyCleanup(context.Background(), gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: plan.PlanDigest}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fx.Worktree); !os.IsNotExist(err) {
		t.Fatalf("task worktree remains: %v", err)
	}
	if got := optionalGitOutput(t, fx.Clone, "show-ref", "refs/heads/dev/codex/cleanup"); got != "" {
		t.Fatalf("local branch remains: %s", got)
	}
	if got := optionalGitOutput(t, fx.Clone, "ls-remote", fx.Remote, "refs/heads/dev/codex/cleanup"); got != "" {
		t.Fatalf("remote branch remains: %s", got)
	}
}

func optionalGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 2) {
			return ""
		}
		t.Fatal(err)
	}
	return string(out)
}
