// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

func TestCheck_StaleFreshnessSkipsLegacyPreparationAndMake(t *testing.T) {
	fx := testutil.TempWorktreeWithBareOrigin(t)
	marker := filepath.Join(t.TempDir(), "commands-ran")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "cargo"), []byte("#!/bin/sh\nprintf cargo >> \"$MARKER\"\n"), 0o755); err != nil {
		t.Fatalf("write cargo fixture: %v", err)
	}
	t.Setenv("MARKER", marker)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	runGit(t, fx.Clone, "checkout", "-B", "develop")
	writeRepoFile(t, fx.Clone, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  prepareProfile: cargo-workspace-v1\n")
	writeRepoFile(t, fx.Clone, "Makefile", "check:\n\t@printf check >> \"$$MARKER\"\nlint:\n\t@printf lint >> \"$$MARKER\"\n")
	runGit(t, fx.Clone, "add", ".gz-git.yaml", "Makefile")
	runGit(t, fx.Clone, "commit", "-m", "declare integration prerequisites")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")

	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeRepoFile(t, fx.Worktree, "task.txt", "task\n")
	runGit(t, fx.Worktree, "add", "task.txt")
	runGit(t, fx.Worktree, "commit", "-m", "task work")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")

	writeRepoFile(t, fx.Clone, "later.txt", "later\n")
	runGit(t, fx.Clone, "add", "later.txt")
	runGit(t, fx.Clone, "commit", "-m", "develop moves")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || !hasCheckDetail(report, "freshness", checkFail, "commits behind") || report.Failures != 1 || report.GateMode != "" {
		t.Fatalf("stale source must report only freshness failure:\n%s", FormatCheck(report))
	}
	if report.ValidationDuration != 0 || report.ResourceWaitDuration != 0 {
		t.Fatalf("stale source must not record gate timing: validation=%s wait=%s", report.ValidationDuration, report.ResourceWaitDuration)
	}
	if report.ReadinessFailures != nil {
		t.Fatalf("stale source must not record gate failures before any gate starts: %+v", report.ReadinessFailures)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("stale source ran preparation or Make commands: stat marker = %v", err)
	}
}

func TestCheck_StaleFreshnessSkipsContractRunner(t *testing.T) {
	if !readinessRunnerSupported() {
		t.Skip("contract runners are unsupported on Windows")
	}
	fx := readinessFixture(t)
	marker := filepath.Join(t.TempDir(), "runner-ran")
	runner := "#!/bin/sh\nprintf runner >> " + strconv.Quote(marker) + "\nprintf '{\"version\":1,\"status\":\"ready\",\"summary\":\"ready\"}'\n"
	advanceReadinessTarget(t, fx, "", runner, "record readiness runner invocation")

	writeRepoFile(t, fx.Clone, "target-freshness.txt", "new target commit\n")
	runGit(t, fx.Clone, "add", "target-freshness.txt")
	runGit(t, fx.Clone, "commit", "-m", "advance target")
	runGit(t, fx.Clone, "push", fx.Remote, "develop")

	report, err := Check(context.Background(), gitcmd.NewExecutor(), CheckOptions{RepoPath: fx.Worktree, Branch: "dev/actor/feat/task"})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if report.Ready || !hasCheckDetail(report, "freshness", checkFail, "commits behind") || report.GateMode != "" {
		t.Fatalf("stale source must stop before the contract runner:\n%s", FormatCheck(report))
	}
	if report.ValidationDuration != 0 {
		t.Fatalf("validation duration = %s, want zero before runner execution", report.ValidationDuration)
	}
	if report.ReadinessFailures != nil {
		t.Fatalf("stale source must not record runner failures before it starts: %+v", report.ReadinessFailures)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("stale source ran the readiness runner: %v", err)
	}
}
