// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// localFlagsSection returns the "Flags:" block of a command's help, stopping
// before "Global Flags:" so an inherited flag cannot satisfy the assertion.
func localFlagsSection(t *testing.T, usage string) string {
	t.Helper()
	_, after, ok := strings.Cut(usage, "\nFlags:\n")
	if !ok {
		t.Fatalf("help has no Flags: section:\n%s", usage)
	}
	section, _, _ := strings.Cut(after, "\nGlobal Flags:\n")
	return section
}

// The --no-fetch token is a wire contract read by CE from --help output, so
// it is asserted literally for each subcommand rather than through a loop
// over shared constants.
func TestIntegrateCheckHelpDeclaresNoFetch(t *testing.T) {
	cmd := findCommand(t, rootCmd, "integrate", "check")
	flag := cmd.Flags().Lookup("no-fetch")
	if flag == nil || flag.Hidden || flag.Deprecated != "" {
		t.Fatalf("integrate check --no-fetch must exist, visible and not deprecated: %+v", flag)
	}
	found := false
	for _, line := range strings.Split(localFlagsSection(t, cmd.UsageString()), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--no-fetch") {
			found = true
		}
	}
	if !found {
		t.Fatalf("integrate check help Flags: has no line beginning with --no-fetch:\n%s", cmd.UsageString())
	}
}

func TestIntegrateRunHelpDeclaresNoFetch(t *testing.T) {
	cmd := findCommand(t, rootCmd, "integrate", "run")
	flag := cmd.Flags().Lookup("no-fetch")
	if flag == nil || flag.Hidden || flag.Deprecated != "" {
		t.Fatalf("integrate run --no-fetch must exist, visible and not deprecated: %+v", flag)
	}
	found := false
	for _, line := range strings.Split(localFlagsSection(t, cmd.UsageString()), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--no-fetch") {
			found = true
		}
	}
	if !found {
		t.Fatalf("integrate run help Flags: has no line beginning with --no-fetch:\n%s", cmd.UsageString())
	}
}

// noFetchCommandFixture is a pushed task branch whose remote can be pushed to
// but not read from, so a successful command proves --no-fetch was honored
// end to end through flag parsing.
func noFetchCommandFixture(t *testing.T) *testutil.WorktreeOrigin {
	t.Helper()
	fx := testutil.TempWorktreeWithBareOrigin(t)
	runGit(t, fx.Clone, "branch", "develop")
	runGit(t, fx.Clone, "push", "-u", fx.Remote, "develop")
	runGit(t, fx.Worktree, "checkout", "-B", "dev/actor/feat/task", "develop")
	writeFile(t, fx.Worktree, "task.txt", "task\n")
	writeFile(t, fx.Worktree, ".gz-git.yaml", "branch:\n  integrationBranch: develop\n  taskPattern: dev/*\n")
	writeFile(t, fx.Worktree, "Makefile", "check:\n\t@true\n")
	runGit(t, fx.Worktree, "add", ".")
	runGit(t, fx.Worktree, "commit", "-m", "task")
	runGit(t, fx.Worktree, "push", "-u", fx.Remote, "HEAD")
	runGit(t, fx.Clone, "remote", "set-url", fx.Remote, filepath.Join(t.TempDir(), "unreadable.git"))
	runGit(t, fx.Clone, "config", "remote."+fx.Remote+".pushurl", fx.Origin)
	return fx
}

func TestIntegrateCheckNoFetchFlagSkipsFetch(t *testing.T) {
	restore := setIntegrateCheckGlobals(t)
	defer restore()

	fx := noFetchCommandFixture(t)
	t.Chdir(fx.Worktree)
	var out bytes.Buffer
	integrateCheckCmd.SetOut(&out)
	defer integrateCheckCmd.SetOut(nil)
	integrateCheckNoFetch = true
	if err := runIntegrateCheck(integrateCheckCmd, []string{"dev/actor/feat/task"}); err != nil {
		t.Fatalf("integrate check --no-fetch: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "without fetching") {
		t.Fatalf("check transcript must say freshness was judged without fetching:\n%s", out.String())
	}
}

func TestIntegrateRunNoFetchFlagIntegratesWithoutFetch(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()

	fx := noFetchCommandFixture(t)
	taskSHA := gitOutputForIntegrateRun(t, fx.Worktree, "rev-parse", "HEAD")
	t.Chdir(fx.Worktree)
	quiet = true
	integrateRunNoFetch = true
	if err := runIntegrateRun(integrateRunCmd, []string{"dev/actor/feat/task"}); err != nil {
		t.Fatalf("integrate run --no-fetch: %v", err)
	}
	if got := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop"); got != taskSHA {
		t.Fatalf("origin develop = %s, want %s", got, taskSHA)
	}
}

// The local tracking ref of the task branch is stale: the remote branch moved
// to another commit, so the leased delete is refused. A full run must carry
// --no-fetch through to reclaim, report the delete as unconfirmed rather than
// reading ls-remote, and exit with the incomplete-reclaim code.
func TestIntegrateRunNoFetchUnconfirmedRemoteDeleteExitsThree(t *testing.T) {
	restore := setIntegrateRunGlobals(t)
	defer restore()

	fx := noFetchCommandFixture(t)
	develop := gitOutputForIntegrateRun(t, fx.Origin, "rev-parse", "refs/heads/develop")
	runGit(t, fx.Origin, "update-ref", "refs/heads/dev/actor/feat/task", develop)
	t.Chdir(fx.Worktree)
	var out bytes.Buffer
	integrateRunCmd.SetOut(&out)
	defer integrateRunCmd.SetOut(nil)
	integrateRunNoFetch = true

	err := runIntegrateRun(integrateRunCmd, []string{"dev/actor/feat/task"})
	if got := cliutil.ExitCodeForError(err); got != cliutil.ExitReclaimIncomplete {
		t.Fatalf("unconfirmed remote delete exit = %d, want %d (%v)\n%s", got, cliutil.ExitReclaimIncomplete, err, out.String())
	}
	if !strings.Contains(out.String(), "RECLAIM incomplete: leased remote delete origin/dev/actor/feat/task (--no-fetch: not confirmed with ls-remote)") {
		t.Fatalf("run must report the unconfirmed no-fetch delete:\n%s", out.String())
	}
}
