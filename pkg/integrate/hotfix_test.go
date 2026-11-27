// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
)

// declHotfix keeps develop as the integration branch and lets reclaim match the
// fixture's task branch.
const declHotfix = "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n"

// hotfixFixture is a repository whose default branch main declares develop,
// whose develop holds one unreleased commit, and whose pushed task branch
// dev/task is cut from main.
func hotfixFixture(t *testing.T) *testutil.WorktreeOrigin {
	t.Helper()
	fx := gateFixture(t, declareOnMain(t, declHotfix), declHotfix)
	runGit(t, fx.Clone, "checkout", "-q", "-b", "advance-develop")
	writeRepoFile(t, fx.Clone, "unreleased.txt", "next release\n")
	runGit(t, fx.Clone, "add", ".")
	runGit(t, fx.Clone, "commit", "-m", "unreleased work")
	runGit(t, fx.Clone, "push", fx.Remote, "HEAD:refs/heads/develop")
	runGit(t, fx.Clone, "checkout", "-q", "main")
	runGit(t, fx.Clone, "branch", "-q", "-D", "advance-develop")
	return fx
}

func originHead(t *testing.T, fx *testutil.WorktreeOrigin, branch string) string {
	t.Helper()
	return gitOutput(t, fx.Origin, "rev-parse", "refs/heads/"+branch)
}

// hotfixOptions pins the current task head onto the current remote main.
func hotfixOptions(t *testing.T, fx *testutil.WorktreeOrigin) CheckOptions {
	t.Helper()
	opts := gateOptions(fx, "main")
	opts.Hotfix = true
	opts.ExpectSource = gitOutput(t, fx.Worktree, "rev-parse", "refs/heads/"+gateTaskBranch)
	opts.ExpectTarget = originHead(t, fx, "main")
	return opts
}

func hotfixRunOptions(t *testing.T, fx *testutil.WorktreeOrigin) RunOptions {
	t.Helper()
	opts := hotfixOptions(t, fx)
	return RunOptions{CheckOptions: opts, Execution: &ExecutionContext{
		ExecutionID:  "hotfix-1",
		Generation:   "g1",
		Owner:        OwnerIdentity{Actor: "claude", Host: "mbp", Kind: "agent"},
		ExpectSource: opts.ExpectSource,
	}}
}

func assertHotfixRefusal(t *testing.T, err error, want string) {
	t.Helper()
	if !errors.Is(err, ErrHotfixRefused) {
		t.Fatalf("error = %v, want ErrHotfixRefused", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want it to contain %q", err, want)
	}
}

func checkHotfixRefused(t *testing.T, opts CheckOptions, want string) {
	t.Helper()
	_, err := Check(context.Background(), gitcmd.NewExecutor(), opts)
	assertHotfixRefusal(t, err, want)
}

func TestRun_HotfixLandsOnDefaultBranch(t *testing.T) {
	t.Parallel()
	fx := hotfixFixture(t)
	developBefore := originHead(t, fx, "develop")
	opts := hotfixRunOptions(t, fx)

	report, err := Run(context.Background(), gitcmd.NewExecutor(), opts)
	if err != nil {
		t.Fatalf("Run: %v\n%s", err, FormatRun(report))
	}
	if got := originHead(t, fx, "main"); got != opts.ExpectSource {
		t.Fatalf("origin main = %s, want the hotfix %s", got, opts.ExpectSource)
	}
	if got := originHead(t, fx, "develop"); got != developBefore {
		t.Fatalf("origin develop = %s, want unchanged %s", got, developBefore)
	}
	if report.Receipt == nil || report.Receipt.Phase != evidencePhaseComplete {
		t.Fatalf("receipt = %+v, want a complete receipt", report.Receipt)
	}
	if report.Reclaim.Incomplete() || !report.Reclaim.RemovedWorktree ||
		!report.Reclaim.RemovedLocalBranch || !report.Reclaim.RemovedRemoteBranch {
		t.Fatalf("reclaim = %+v, want the worktree and both task refs removed", report.Reclaim)
	}
	if got := BuildRunProtocol(report, nil, RunExitOK).Landing; got != "hotfix" {
		t.Fatalf("protocol landing = %q, want hotfix", got)
	}
}

func TestRun_HotfixRefusesOptionsAndMissingContext(t *testing.T) {
	t.Parallel()
	fx := hotfixFixture(t)
	mainBefore, developBefore := originHead(t, fx, "main"), originHead(t, fx, "develop")
	for name, tc := range map[string]struct {
		mutate func(*RunOptions)
		want   string
	}{
		"release":           {func(o *RunOptions) { o.Release = true }, "combined with --release"},
		"release no target": {func(o *RunOptions) { o.Release, o.Target, o.ExpectTarget = true, "", "" }, "combined with --release"},
		"direct to default": {func(o *RunOptions) { o.DirectToDefault = true }, "combined with --direct-to-default"},
		"controller config": {func(o *RunOptions) { o.ControllerConfig = "controller.yaml" }, "combined with --controller-config"},
		"no fetch":          {func(o *RunOptions) { o.NoFetch = true }, "combined with --no-fetch"},
		"no execution":      {func(o *RunOptions) { o.Execution = nil }, "requires --source, --record and --owner"},
		"no target":         {func(o *RunOptions) { o.Target, o.ExpectTarget = "", "" }, "requires a pinned --target"},
		"unpinned target":   {func(o *RunOptions) { o.ExpectTarget = "" }, "requires a pinned --target"},
	} {
		opts := hotfixRunOptions(t, fx)
		tc.mutate(&opts)
		_, err := Run(context.Background(), gitcmd.NewExecutor(), opts)
		if !errors.Is(err, ErrHotfixRefused) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want a hotfix refusal containing %q", name, err, tc.want)
		}
		if originHead(t, fx, "main") != mainBefore || originHead(t, fx, "develop") != developBefore {
			t.Fatalf("%s: a refused hotfix moved a remote branch", name)
		}
	}
	unpinnedSource := hotfixOptions(t, fx)
	unpinnedSource.ExpectSource = ""
	checkHotfixRefused(t, unpinnedSource, "requires --source")
}

func TestCheck_HotfixRequiresCommittedDefaultGate(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		commitMain string
		want       string
	}{
		"no declaration":  {"", "no usable integrationBranch declaration"},
		"declares itself": {declMainOnly, "is the default branch main"},
		"unreadable":      {"branch: [\n", "no usable integrationBranch declaration"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var commitMain func(string)
			if tc.commitMain != "" {
				commitMain = declareOnMain(t, tc.commitMain)
			}
			// The task's working tree declares develop in every case: it must
			// never gate a hotfix.
			fx := gateFixture(t, commitMain, declHotfix)
			checkHotfixRefused(t, hotfixOptions(t, fx), tc.want)
		})
	}
	t.Run("caller supplied integration branch", func(t *testing.T) {
		t.Parallel()
		fx := hotfixFixture(t)
		opts := hotfixOptions(t, fx)
		opts.IntegrationConfig = []string{"develop"}
		checkHotfixRefused(t, opts, "no usable integrationBranch declaration")
	})
}

func TestCheck_HotfixRequiresGateCommitEqualsTarget(t *testing.T) {
	t.Parallel()
	fx := hotfixFixture(t)
	// A local branch spelled like the remote-tracking default shadows it when
	// the target is resolved, so the task would be judged against another
	// commit than the one whose declaration gates it.
	shadow := originHead(t, fx, "develop")
	runGit(t, fx.Clone, "branch", "origin/main", shadow)
	opts := hotfixOptions(t, fx)
	opts.ExpectTarget = shadow
	checkHotfixRefused(t, opts, "the gating declaration was read at default branch commit")
}

func TestCheck_HotfixRefusesMissingRemoteIntegrationRef(t *testing.T) {
	t.Parallel()
	fx := hotfixFixture(t)
	main := originHead(t, fx, "main")
	runGit(t, fx.Origin, "update-ref", "-d", "refs/heads/develop")
	// Local branches that a shorter or DWIM lookup would read instead: develop
	// itself, and branches spelled like the missing remote-tracking ref.
	runGit(t, fx.Clone, "branch", "develop", main)
	runGit(t, fx.Clone, "branch", "origin/develop", main)
	runGit(t, fx.Clone, "update-ref", "refs/heads/refs/remotes/origin/develop", main)
	checkHotfixRefused(t, hotfixOptions(t, fx), "remote integration ref refs/remotes/origin/develop is missing")
}

func TestRunExitCode_HotfixRefusalIsNotReady(t *testing.T) {
	t.Parallel()
	// The text matches none of the phrases the classifier otherwise reads.
	if got := RunExitCode(nil, hotfixRefusal("boom")); got != RunExitToolError {
		t.Fatalf("hotfix refusal exit = %d, want %d", got, RunExitToolError)
	}
	if got := RunExitCode(nil, errors.New("boom")); got != RunExitInterrupted {
		t.Fatalf("control exit = %d, want %d", got, RunExitInterrupted)
	}
	if got := RunExitCode(nil, fmt.Errorf("%w: --target moved", ErrTargetMismatch)); got != RunExitSourceMismatch {
		t.Fatalf("target mismatch exit = %d, want %d", got, RunExitSourceMismatch)
	}
}

func TestCheck_HotfixRequiresDefaultTargetAndTaskSource(t *testing.T) {
	t.Parallel()
	fx := hotfixFixture(t)
	develop := originHead(t, fx, "develop")
	runGit(t, fx.Clone, "branch", "develop", develop)

	toDevelop := hotfixOptions(t, fx)
	toDevelop.Target, toDevelop.ExpectTarget = fx.Remote+"/develop", develop
	checkHotfixRefused(t, toDevelop, "lands only on the default branch main")

	for source, want := range map[string]string{
		"develop":              "source develop is not a task branch",
		"main":                 "source main is not a task branch",
		fx.Remote + "/develop": "is not a local task branch",
	} {
		opts := hotfixOptions(t, fx)
		opts.Branch = source
		opts.ExpectSource = gitOutput(t, fx.Clone, "rev-parse", source)
		checkHotfixRefused(t, opts, want)
	}

	report, err := Check(context.Background(), gitcmd.NewExecutor(), hotfixOptions(t, fx))
	if err != nil || !report.Ready {
		t.Fatalf("control: the task branch onto main must be READY: err=%v report=%+v", err, report)
	}
}

func TestCheck_HotfixRefusesIntegrationHistory(t *testing.T) {
	t.Parallel()
	fx := hotfixFixture(t)
	develop := originHead(t, fx, "develop")

	runGit(t, fx.Worktree, "fetch", "-q", fx.Remote)
	runGit(t, fx.Worktree, "checkout", "-q", "-B", gateTaskBranch, develop)
	writeRepoFile(t, fx.Worktree, "task.txt", "fix\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "fix cut from develop")
	runGit(t, fx.Worktree, "push", "-q", "--force", "-u", fx.Remote, gateTaskBranch)
	// A local branch spelled like the remote ref, without the unreleased
	// commit, must not stand in for the remote integration branch.
	runGit(t, fx.Clone, "branch", "origin/develop", originHead(t, fx, "main"))
	checkHotfixRefused(t, hotfixOptions(t, fx), "carries history from refs/remotes/origin/develop")
	runGit(t, fx.Clone, "branch", "-q", "-D", "origin/develop")

	// A cherry-pick of the same change is a new commit and is accepted.
	gateTaskFromMain(t, fx, declHotfix)
	runGit(t, fx.Worktree, "cherry-pick", develop)
	runGit(t, fx.Worktree, "push", "-q", fx.Remote, gateTaskBranch)
	report, err := Check(context.Background(), gitcmd.NewExecutor(), hotfixOptions(t, fx))
	if err != nil {
		t.Fatalf("Check of a cherry-picked hotfix: %v", err)
	}
	if !report.Ready {
		t.Fatalf("want READY, got\n%s", FormatCheck(report))
	}
}

func TestCheck_HotfixRefusesDeclarationChange(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct{ taskDecl, want string }{
		"changed": {"branch:\n  integrationBranch: main\n  taskPattern: dev/*\n", "from develop to main"},
		"removed": {"branch:\n  taskPattern: dev/*\n", "from develop to (none)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := gateFixture(t, declareOnMain(t, declHotfix), tc.taskDecl)
			checkHotfixRefused(t, hotfixOptions(t, fx), tc.want)
		})
	}
}

func TestBuildRunProtocol_HotfixLanding(t *testing.T) {
	t.Parallel()
	for _, hotfix := range []bool{true, false} {
		report := &RunReport{Check: &CheckReport{Plan: TargetPlan{Remote: "origin", Target: "origin/main", Hotfix: hotfix}}}
		data, err := json.Marshal(BuildRunProtocol(report, nil, RunExitOK))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if got := strings.Contains(string(data), `"landing":"hotfix"`); got != hotfix {
			t.Fatalf("hotfix=%t protocol = %s", hotfix, data)
		}
		if !hotfix && strings.Contains(string(data), `"landing"`) {
			t.Fatalf("a non-hotfix document must omit landing: %s", data)
		}
	}
}
