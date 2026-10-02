// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/internal/testutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/identity"
)

// foreignProbeFormat is the --format value findForeignCommits passes to git
// log. Matching it — rather than the subcommand "log" alone — is what lets the
// wrapper fail the foreign-work probe and nothing else: GetInfo runs its own
// git log for the last commit, and that one has to keep working.
const foreignProbeFormat = "--format=%H%x1f%s%x1f%B%x1e"

// newForeignProbeExecutor returns an executor whose git binary is a wrapper
// that records every invocation and delegates to the real git — except the
// foreign-work probe, which fails the way failure dictates: "exit128"
// simulates git dying on a repository error, "killed" a probe process that
// never answered, and "" lets everything through.
func newForeignProbeExecutor(t *testing.T, probeFailure string) (executor *gitcmd.Executor, logPath string) {
	t.Helper()

	var probeBlock string
	switch probeFailure {
	case "exit128":
		probeBlock = `for arg in "$@"; do
	if [ "$arg" = '` + foreignProbeFormat + `' ]; then
		echo "fatal: simulated object store failure" >&2
		exit 128
	fi
done
`
	case "killed":
		probeBlock = `for arg in "$@"; do
	if [ "$arg" = '` + foreignProbeFormat + `' ]; then
		kill -9 $$
	fi
done
`
	case "":
	default:
		t.Fatalf("unknown probe failure mode %q", probeFailure)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("look up git: %v", err)
	}

	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	wrapper := filepath.Join(dir, "git")

	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$GIT_ARG_LOG\"\n" +
		probeBlock +
		"exec \"" + realGit + "\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}

	return gitcmd.NewExecutor(
		gitcmd.WithGitBinary(wrapper),
		gitcmd.WithEnv([]string{"GIT_ARG_LOG=" + logPath}),
	), logPath
}

// newPushProcessClient builds the concrete client processPushRepository lives
// on, running every git call through the given executor.
func newPushProcessClient(t *testing.T, executor *gitcmd.Executor) *client {
	t.Helper()

	c, ok := NewClient(WithExecutor(executor)).(*client)
	if !ok {
		t.Fatalf("NewClient returned %T, want *client", c)
	}
	return c
}

// wrapperInvocations returns the git invocations the wrapper recorded, one
// line per call with the arguments space-joined.
func wrapperInvocations(t *testing.T, logPath string) []string {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read wrapper log: %v", err)
	}
	return strings.Split(string(data), "\n")
}

// wrapperPushed reports whether a push was invoked through the wrapper.
func wrapperPushed(t *testing.T, logPath string) bool {
	t.Helper()

	for _, line := range wrapperInvocations(t, logPath) {
		if strings.HasPrefix(line, "push ") {
			return true
		}
	}
	return false
}

// wrapperRanProbe reports whether the foreign-work probe was invoked at all.
func wrapperRanProbe(t *testing.T, logPath string) bool {
	t.Helper()

	for _, line := range wrapperInvocations(t, logPath) {
		if strings.Contains(line, foreignProbeFormat) {
			return true
		}
	}
	return false
}

// newForeignDivergence builds the situation the gate exists for: a clone whose
// branch has diverged from a temp local bare origin, with the remote side
// holding a commit signed by another device that a force push would discard.
// The clone has fetched, so --force-with-lease alone no longer protects that
// commit. Nothing here touches any real remote.
func newForeignDivergence(t *testing.T) (root, clone, bare string) {
	t.Helper()

	root = t.TempDir()
	bare = filepath.Join(root, "origin.git")
	runGit(t, "", "init", "--bare", bare)

	clone = filepath.Join(root, "clone")
	runGit(t, "", "clone", bare, clone)
	configureCloneIdentity(t, clone)

	runGit(t, clone, "checkout", "-b", "feat/task")
	commit(t, clone, "base.txt", "chore: shared base\n\nDevice: dave-office\n")
	runGit(t, clone, "push", "-u", "origin", "feat/task")
	runGit(t, bare, "symbolic-ref", "HEAD", "refs/heads/feat/task")

	// A second clone stands in for the other machine: it lands a commit the
	// first clone never makes, straight onto the remote branch.
	other := filepath.Join(root, "other")
	runGit(t, "", "clone", bare, other)
	configureCloneIdentity(t, other)

	runGit(t, other, "checkout", "feat/task")
	commit(t, other, "theirs.txt", "chore(wip): theirs\n\nDevice: dave-laptop\n")
	runGit(t, other, "push", "origin", "feat/task")

	// The fetch moves the clone's tracking ref onto the foreign tip, which is
	// exactly what satisfies --force-with-lease afterwards.
	runGit(t, clone, "fetch", "origin")
	commit(t, clone, "mine.txt", "chore(wip): mine\n\nDevice: dave-office\n")

	return root, clone, bare
}

// configureCloneIdentity gives a freshly cloned fixture the local config
// commits need; a clone inherits no user identity from its origin.
func configureCloneIdentity(t *testing.T, dir string) {
	t.Helper()

	for _, args := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		runGit(t, dir, args...)
	}
}

// TestForeignWorkProbeFailure pins the fail-closed contract of the foreign-work
// gate: a probe that cannot answer is not a probe that passed. Block mode must
// refuse the force push without invoking it and leave the remote tip alone,
// while a verified absent remote ref (a brand-new branch) and an explicit
// allow keep their existing behavior. The push contrast runs against a temp
// local bare origin only; no real remote is contacted.
func TestForeignWorkProbeFailure(t *testing.T) {
	mine := identity.Identity{Device: "dave-office"}

	t.Run("a probe that dies is an error, not silence", func(t *testing.T) {
		dir := testutil.TempGitRepoWithCommit(t)
		runGit(t, dir, "checkout", "-b", "remote")
		commit(t, dir, "theirs.txt", "chore(wip): theirs\n\nDevice: dave-laptop\n")
		runGit(t, dir, "branch", "local", "HEAD~1")

		executor, _ := newForeignProbeExecutor(t, "exit128")
		got, err := findForeignCommits(context.Background(), executor, dir, "local", "remote", mine)
		if err == nil {
			t.Fatalf("a dead probe must surface as an error, got %v", got)
		}
		if got != nil {
			t.Fatalf("a failed probe must not report commits it never read: %v", got)
		}
	})

	t.Run("a probe that never executes is an error", func(t *testing.T) {
		dir := testutil.TempGitRepoWithCommit(t)
		runGit(t, dir, "checkout", "-b", "remote")
		commit(t, dir, "theirs.txt", "chore(wip): theirs\n\nDevice: dave-laptop\n")
		runGit(t, dir, "branch", "local", "HEAD~1")

		executor, _ := newForeignProbeExecutor(t, "killed")
		got, err := findForeignCommits(context.Background(), executor, dir, "local", "remote", mine)
		if err == nil {
			t.Fatalf("a probe that never answered must surface as an error, got %v", got)
		}
		if got != nil {
			t.Fatalf("an unanswered probe must not report commits: %v", got)
		}
	})

	t.Run("a verified absent remote ref stays a normal empty result", func(t *testing.T) {
		dir := testutil.TempGitRepoWithCommit(t)

		// The probe is made to fail here on purpose: absence must be decided
		// by verifying the ref, not by absorbing the probe's death.
		executor, _ := newForeignProbeExecutor(t, "exit128")
		got, err := findForeignCommits(context.Background(), executor, dir, "HEAD", "origin/nothing-here", mine)
		if err != nil {
			t.Fatalf("a branch not yet on the remote is the normal first push, not an error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("found %v, want nothing", got)
		}
	})

	t.Run("a working probe still reports the other writer", func(t *testing.T) {
		dir := testutil.TempGitRepoWithCommit(t)
		runGit(t, dir, "checkout", "-b", "remote")
		commit(t, dir, "theirs.txt", "chore(wip): theirs\n\nDevice: dave-laptop\n")
		runGit(t, dir, "branch", "local", "HEAD~1")

		executor, logPath := newForeignProbeExecutor(t, "")
		got, err := findForeignCommits(context.Background(), executor, dir, "local", "remote", mine)
		if err != nil {
			t.Fatalf("findForeignCommits returned %v", err)
		}
		if len(got) != 1 || got[0].Identity.Device != "dave-laptop" {
			t.Fatalf("found %v, want the single dave-laptop commit", got)
		}
		if !wrapperRanProbe(t, logPath) {
			t.Fatal("the probe must have run through the wrapper for this to prove anything")
		}
	})

	t.Run("block mode refuses the force push when the probe fails", func(t *testing.T) {
		root, clone, bare := newForeignDivergence(t)
		executor, logPath := newForeignProbeExecutor(t, "exit128")
		c := newPushProcessClient(t, executor)
		tipBefore := gitOut(t, bare, "rev-parse", "refs/heads/feat/task")

		res := c.processPushRepository(context.Background(), root, clone, BulkPushOptions{
			Force:    true,
			Identity: mine,
			Policy:   &PushPolicy{ForeignWork: ForeignWorkBlock},
			Logger:   NewNoopLogger(),
		}, NewNoopLogger())

		if res.Status != StatusBlocked {
			t.Fatalf("status = %q (%s), want %q: an unverifiable check is not a passed one", res.Status, res.Message, StatusBlocked)
		}
		if res.Error == nil || !strings.Contains(res.Error.Error(), string(PushRuleForeignWork)) {
			t.Fatalf("error = %v, want the %s rule", res.Error, PushRuleForeignWork)
		}
		if wrapperPushed(t, logPath) {
			t.Fatal("the push must not run while what the remote holds is unknown")
		}
		if tipAfter := gitOut(t, bare, "rev-parse", "refs/heads/feat/task"); tipAfter != tipBefore {
			t.Fatalf("remote tip moved %s -> %s; the other machine's commit was discarded", tipBefore, tipAfter)
		}
	})

	t.Run("block mode refuses under dry-run too", func(t *testing.T) {
		root, clone, _ := newForeignDivergence(t)
		executor, logPath := newForeignProbeExecutor(t, "exit128")
		c := newPushProcessClient(t, executor)

		res := c.processPushRepository(context.Background(), root, clone, BulkPushOptions{
			Force:    true,
			DryRun:   true,
			Identity: mine,
			Policy:   &PushPolicy{ForeignWork: ForeignWorkBlock},
			Logger:   NewNoopLogger(),
		}, NewNoopLogger())

		if res.Status != StatusBlocked {
			t.Fatalf("status = %q (%s), want %q: dry-run reports the refusal too", res.Status, res.Message, StatusBlocked)
		}
		if res.Error == nil || !strings.Contains(res.Error.Error(), string(PushRuleForeignWork)) {
			t.Fatalf("error = %v, want the %s rule", res.Error, PushRuleForeignWork)
		}
		if wrapperPushed(t, logPath) {
			t.Fatal("no push may be invoked, not even behind a dry run")
		}
	})

	t.Run("allow mode keeps its explicit bypass", func(t *testing.T) {
		root, clone, bare := newForeignDivergence(t)
		executor, logPath := newForeignProbeExecutor(t, "exit128")
		c := newPushProcessClient(t, executor)
		wantTip := gitOut(t, clone, "rev-parse", "feat/task")

		res := c.processPushRepository(context.Background(), root, clone, BulkPushOptions{
			Force:    true,
			Identity: mine,
			Policy:   &PushPolicy{ForeignWork: ForeignWorkAllow},
			Logger:   NewNoopLogger(),
		}, NewNoopLogger())

		if res.Status != StatusPushed {
			t.Fatalf("status = %q (%s), want %q: an explicit allow must keep pushing", res.Status, res.Message, StatusPushed)
		}
		if !wrapperPushed(t, logPath) {
			t.Fatal("allow mode must still push")
		}
		if got := gitOut(t, bare, "rev-parse", "refs/heads/feat/task"); got != wantTip {
			t.Fatalf("remote tip = %s, want the local tip %s", got, wantTip)
		}
	})

	t.Run("a brand-new remote branch still pushes on verified absence", func(t *testing.T) {
		root, clone, bare := newForeignDivergence(t)
		// The probe is broken here too: a target the remote has never held
		// must push anyway, because absence is verified, not assumed.
		executor, _ := newForeignProbeExecutor(t, "exit128")
		c := newPushProcessClient(t, executor)
		wantTip := gitOut(t, clone, "rev-parse", "feat/task")

		res := c.processPushRepository(context.Background(), root, clone, BulkPushOptions{
			Force:    true,
			Refspec:  "feat/task:brand-new",
			Identity: mine,
			Policy:   &PushPolicy{ForeignWork: ForeignWorkBlock},
			Logger:   NewNoopLogger(),
		}, NewNoopLogger())

		if res.Status != StatusPushed {
			t.Fatalf("status = %q (%s), want %q: a new branch is the normal first push", res.Status, res.Message, StatusPushed)
		}
		if got := gitOut(t, bare, "rev-parse", "refs/heads/brand-new"); got != wantTip {
			t.Fatalf("brand-new tip = %s, want the local tip %s", got, wantTip)
		}
	})
}
