package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// legacySecurityPlan creates real primary, linked-task, and bare-remote state.
// The artifacts are deliberately bound to the plan identity; no old receipt is
// treated as an authorization token.
func legacySecurityPlan(t *testing.T, targetContainsTask bool) (plan *LegacyCleanupPlan, planPath, primary, task string) {
	t.Helper()
	t.Setenv("GZ_GIT_TASK_ACTOR", "codex")
	t.Setenv("GIT_WORK_HOST", "testhost")
	fx := readyTaskFixture(t)
	policy := "branch:\n  integrationBranch: develop\n  taskPattern:\n    - dev/*\n"
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", policy)
	runGit(t, fx.Clone, "add", ".gz-git.yaml")
	runGit(t, fx.Clone, "commit", "-m", "declare legacy cleanup pattern")
	writeRepoFile(t, fx.Worktree, ".gz-git.yaml", policy)
	runGit(t, fx.Worktree, "add", ".gz-git.yaml")
	runGit(t, fx.Worktree, "commit", "-m", "declare legacy cleanup pattern")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	if targetContainsTask {
		runGit(t, fx.Worktree, "push", fx.Remote, "HEAD:refs/heads/develop")
		runGit(t, fx.Clone, "fetch", fx.Remote, "develop")
		runGit(t, fx.Clone, "update-ref", "refs/heads/develop", "refs/remotes/origin/develop")
	}
	task = resolvePath(t, fx.Worktree)
	primary = resolvePath(t, fx.Clone)
	common := gitOutput(t, task, "rev-parse", "--path-format=absolute", "--git-common-dir")
	sha := gitOutput(t, task, "rev-parse", "HEAD")
	owner := OwnerIdentity{Actor: "codex", Host: "testhost", Kind: "agent"}
	artifactDir := t.TempDir()
	artifact := func(name, status string) string {
		path := filepath.Join(artifactDir, name)
		body := fmt.Sprintf(`{"schema_version":1,"status":%q,"task_sha":%q,"task_branch":%q,"task_worktree":%q,"common_dir":%q,"owner":{"actor":"%s","host":"%s","kind":"agent"}}
`, status, sha, "dev/actor/feat/task", task, common, owner.Actor, owner.Host)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	exec := gitcmd.NewExecutor()
	var err error
	plan, err = BuildLegacyCleanupPlan(context.Background(), exec, LegacyCleanupPlanOptions{
		RepoPath: fx.Worktree, Target: "develop", DoneArtifactPath: artifact("done.json", "DONE"), ReleasedArtifactPath: artifact("released.json", "RELEASED"), Owner: owner,
	})
	if err != nil {
		if !targetContainsTask {
			if !strings.Contains(strings.ToLower(err.Error()), "contain") {
				t.Fatalf("target containment fixture failed for unrelated reason: %v", err)
			}
			return nil, "", primary, task
		}
		t.Fatalf("BuildLegacyCleanupPlan: %v", err)
	}
	planPath = filepath.Join(artifactDir, "plan.json")
	if err := WriteLegacyCleanupPlan(planPath, plan); err != nil {
		t.Fatalf("WriteLegacyCleanupPlan: %v", err)
	}
	return plan, planPath, primary, task
}

func applyLegacySecurity(t *testing.T, plan *LegacyCleanupPlan, planPath string) error {
	t.Helper()
	return applyLegacySecurityContext(t, context.Background(), plan, planPath)
}

func applyLegacySecurityContext(t *testing.T, ctx context.Context, plan *LegacyCleanupPlan, planPath string) error {
	t.Helper()
	data, err := ReadLegacyCleanupPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if data.PlanDigest != plan.PlanDigest {
		t.Fatalf("plan digest changed after round trip: %q != %q", data.PlanDigest, plan.PlanDigest)
	}
	return ApplyLegacyCleanup(ctx, gitcmd.NewExecutor(), LegacyCleanupApplyOptions{PlanPath: planPath, ApproveDigest: plan.PlanDigest})
}

func TestLegacyCleanupSecurityBuildPlanApplyAndResourceFence(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	if err := applyLegacySecurity(t, plan, planPath); err != nil {
		t.Fatalf("standalone BuildPlan -> Apply: %v", err)
	}
	if _, err := os.Stat(task); !os.IsNotExist(err) {
		t.Fatalf("task worktree remains after successful cleanup: %v", err)
	}
	if refExists(t, primary, "refs/heads/dev/actor/feat/task") {
		t.Fatal("local task branch remains after successful cleanup")
	}
	if got := gitOutput(t, primary, "ls-remote", "--heads", "origin", "dev/actor/feat/task"); got != "" {
		t.Fatalf("remote task branch remains after successful cleanup: %q", got)
	}
}

func TestLegacyCleanupSecurityRejectsCEAndPreservesAllResources(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	if err := os.MkdirAll(filepath.Join(primary, ".ce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(primary, ".ce", "task-runtime.yaml"), []byte("schema_version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := applyLegacySecurity(t, plan, planPath)
	if !errors.Is(err, ErrLegacyCleanupUnsupportedNative) {
		t.Fatalf("CE declaration was not refused: %v", err)
	}
	assertLegacyResourcesPresent(t, primary, task)
}

func TestLegacyCleanupSecurityRejectsTaskOutsideTargetHistory(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, false)
	if plan != nil || planPath != "" {
		t.Fatal("task outside target history unexpectedly produced an apply plan")
	}
	assertLegacyResourcesPresent(t, primary, task)
}

func TestLegacyCleanupSecurityRejectsMovedLocalAndRemoteRefs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *LegacyCleanupPlan, string)
	}{
		{"local task ref", func(t *testing.T, p *LegacyCleanupPlan, primary string) {
			t.Helper()
			replacement := gitOutput(t, primary, "rev-parse", p.TaskOID+"^")
			runGit(t, primary, "update-ref", "refs/heads/"+p.TaskBranch, replacement)
		}},
		{"remote task ref", func(t *testing.T, p *LegacyCleanupPlan, primary string) {
			t.Helper()
			replacement := gitOutput(t, primary, "rev-parse", p.TaskOID+"^")
			runGit(t, primary, "push", "--force", "origin", replacement+":refs/heads/"+p.TaskBranch)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, planPath, primary, task := legacySecurityPlan(t, true)
			tc.mutate(t, plan, primary)
			if err := applyLegacySecurity(t, plan, planPath); err == nil {
				t.Fatal("cleanup accepted moved task ref")
			}
			assertLegacyResourcesPresent(t, primary, task)
		})
	}
}

func TestLegacyCleanupSecurityRejectsUnidentifiedRuntimeState(t *testing.T) {
	cases := []struct {
		name string
		seed func(*testing.T, *LegacyCleanupPlan, string, string)
	}{
		{"primary legacy declaration", func(t *testing.T, _ *LegacyCleanupPlan, primary, _ string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(primary, ".gz-git-task.yaml"), []byte("legacy\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"task legacy declaration", func(t *testing.T, _ *LegacyCleanupPlan, _, task string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(task, ".gz-git-task.yaml"), []byte("legacy\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"dangling CE parent", func(t *testing.T, _ *LegacyCleanupPlan, primary, _ string) {
			t.Helper()
			if err := os.Symlink(filepath.Join(primary, "missing-ce"), filepath.Join(primary, ".ce")); err != nil {
				t.Fatal(err)
			}
		}},
		{"common CE runtime state", func(t *testing.T, p *LegacyCleanupPlan, _, _ string) {
			t.Helper()
			path := filepath.Join(p.CommonDir, "ce", "task-runtime")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("state\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"common gz-git runtime state", func(t *testing.T, p *LegacyCleanupPlan, _, _ string) {
			t.Helper()
			path := filepath.Join(p.CommonDir, "gz-git", "task-runtime")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("state\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, planPath, primary, task := legacySecurityPlan(t, true)
			tc.seed(t, plan, primary, task)
			if err := applyLegacySecurity(t, plan, planPath); err == nil {
				t.Fatal("cleanup accepted unidentified runtime state")
			}
			assertLegacyResourcesPresent(t, primary, task)
		})
	}
}

func TestLegacyCleanupSecurityRejectsIntentDirectorySymlink(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	external := t.TempDir()
	intentDir := filepath.Join(plan.CommonDir, legacyCleanupIntentDir)
	if err := os.Symlink(external, intentDir); err != nil {
		t.Fatal(err)
	}
	if err := applyLegacySecurity(t, plan, planPath); err == nil {
		t.Fatal("cleanup followed an external intent-directory symlink")
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("cleanup wrote outside the repository through intent symlink: %v", entries)
	}
	assertLegacyResourcesPresent(t, primary, task)
}

// TestLegacyCleanupSecurityHonorsTaskFence holds the sibling task fence. The
// legacy cleanup must refuse at once, without a deadline forcing it out of a
// wait, and leave every resource in place.
func TestLegacyCleanupSecurityHonorsTaskFence(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	fence, err := acquireTaskResourceFence(context.Background(), gitcmd.NewExecutor(), primary, plan.TaskBranch)
	if err != nil {
		t.Fatal(err)
	}
	defer fence.release()
	if err := applyLegacySecurityContext(t, context.Background(), plan, planPath); !errors.Is(err, ErrResourceBusy) {
		t.Fatalf("cleanup under a held task fence = %v, want ErrResourceBusy", err)
	}
	assertLegacyResourcesPresent(t, primary, task)
}

func TestLegacyCleanupSecurityRejectsChangedEndpointTargetDirtyNestedAndForeignMarker(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, *LegacyCleanupPlan, string, string)
	}{
		{"endpoint", func(t *testing.T, _ *LegacyCleanupPlan, primary, _ string) {
			t.Helper()
			runGit(t, primary, "config", "remote.origin.pushurl", "https://changed.invalid/repository.git")
		}},
		{"target", func(t *testing.T, p *LegacyCleanupPlan, primary, _ string) {
			t.Helper()
			parent := gitOutput(t, primary, "rev-parse", p.TargetRef+"^")
			runGit(t, primary, "update-ref", "refs/remotes/origin/develop", parent)
		}},
		{"nested repository", func(t *testing.T, _ *LegacyCleanupPlan, _, task string) {
			t.Helper()
			nested := filepath.Join(task, "nested")
			if err := os.MkdirAll(nested, 0o700); err != nil {
				t.Fatal(err)
			}
			runGit(t, nested, "init")
			if err := os.WriteFile(filepath.Join(nested, "dirty"), []byte("changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"foreign marker", func(t *testing.T, p *LegacyCleanupPlan, _, task string) {
			t.Helper()
			if err := writeRegistrationMarker(context.Background(), gitcmd.NewExecutor(), task, registrationMarker{SchemaVersion: registrationSchemaVersion, ExecutionID: "foreign", Generation: "foreign", TaskSHA: p.TaskOID}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, planPath, primary, task := legacySecurityPlan(t, true)
			tc.mutate(t, plan, primary, task)
			err := applyLegacySecurity(t, plan, planPath)
			if err == nil {
				t.Fatal("unsafe cleanup unexpectedly succeeded")
			}
			assertLegacyResourcesPresent(t, primary, task)
		})
	}
}

func TestLegacyCleanupSecurityRejectsGitDirABASubstitutionBeforeDeletion(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	runGit(t, primary, "worktree", "remove", "--force", task)
	runGit(t, primary, "worktree", "add", task, plan.TaskBranch)
	err := applyLegacySecurity(t, plan, planPath)
	if err == nil {
		t.Fatal("gitdir substitution unexpectedly authorized cleanup")
	}
	if !strings.Contains(err.Error(), "physical identity changed") {
		t.Fatalf("gitdir substitution failed for an unrelated reason: %v", err)
	}
	if _, statErr := os.Stat(task); statErr != nil {
		t.Fatalf("replacement worktree disappeared after ABA rejection: %v", statErr)
	}
	if !refExists(t, primary, "refs/heads/dev/actor/feat/task") {
		t.Fatal("local task ref was deleted after gitdir substitution")
	}
	if got := gitOutput(t, primary, "ls-remote", "--heads", "origin", "dev/actor/feat/task"); got == "" {
		t.Fatal("remote task ref was deleted after gitdir substitution")
	}
}

// The substitution above is caught only while the filesystem happens to give
// the re-created worktree new numbers. This rewrites the plan with the
// replacement's device, inode, and birth time, which is what an ext4 host
// reusing freed inodes within one coarse timestamp tick observes, so only the
// witness token the plan wrote into the removed Git directory can refuse it.
func TestLegacyCleanupSecurityRejectsSubstitutionWithReusedPhysicalIdentity(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	runGit(t, primary, "worktree", "remove", "--force", task)
	runGit(t, primary, "worktree", "add", task, plan.TaskBranch)
	fresh, err := captureLegacyWorktreeIdentity(context.Background(), gitcmd.NewExecutor(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Witness != "" {
		t.Fatalf("replacement worktree inherited a witness: %q", fresh.Witness)
	}
	fresh.Witness = plan.WorktreeIdentity.Witness
	plan.WorktreeIdentity = fresh
	if plan.PlanDigest, err = legacyCleanupPlanDigest(plan); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(planPath); err != nil {
		t.Fatal(err)
	}
	if err := WriteLegacyCleanupPlan(planPath, plan); err != nil {
		t.Fatal(err)
	}
	err = applyLegacySecurity(t, plan, planPath)
	if err == nil || !strings.Contains(err.Error(), "physical identity changed") {
		t.Fatalf("substitution with a reused physical identity was not refused: %v", err)
	}
	assertLegacyResourcesPresent(t, primary, task)
}

// A plan written before witnesses existed must not remove a present worktree,
// since its identity can no longer tell a substitute from the planned tree.
func TestLegacyCleanupSecurityRejectsWitnesslessPlanWhileWorktreePresent(t *testing.T) {
	plan, planPath, primary, task := legacySecurityPlan(t, true)
	plan.WorktreeIdentity.Witness = ""
	var err error
	if plan.PlanDigest, err = legacyCleanupPlanDigest(plan); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(planPath); err != nil {
		t.Fatal(err)
	}
	if err := WriteLegacyCleanupPlan(planPath, plan); err != nil {
		t.Fatal(err)
	}
	err = applyLegacySecurity(t, plan, planPath)
	if err == nil || !strings.Contains(err.Error(), "predates the legacy cleanup witness") {
		t.Fatalf("witnessless plan was not refused: %v", err)
	}
	assertLegacyResourcesPresent(t, primary, task)
}

func assertLegacyResourcesPresent(t *testing.T, primary, task string) {
	t.Helper()
	if _, err := os.Stat(task); err != nil {
		t.Fatalf("task worktree was removed: %v", err)
	}
	if !refExists(t, primary, "refs/heads/dev/actor/feat/task") {
		t.Fatal("local task ref was removed")
	}
	if got := gitOutput(t, primary, "ls-remote", "--heads", "origin", "dev/actor/feat/task"); got == "" {
		t.Fatal("remote task ref was removed")
	}
}
