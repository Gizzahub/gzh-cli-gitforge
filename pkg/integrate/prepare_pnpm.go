// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// pnpmFrozenLockfilePrepareV1 installs a pnpm project's locked dependencies
// so the readiness probes measure the commit, not an empty node_modules. A
// fresh worktree has none, and a Makefile that runs `pnpm exec tsc` there
// fails on both sides for a reason neither commit owns.
const pnpmFrozenLockfilePrepareV1 = "pnpm-frozen-lockfile-v1"

// pnpmPrepareTimeout bounds one install. The content-addressed store makes a
// warm install a linking pass; the ceiling is for a cold store download.
const pnpmPrepareTimeout = 10 * time.Minute

// pnpmPrepareWaitDelay bounds Cmd.Wait after cancellation. Dependency
// lifecycle scripts (binary downloads in postinstall) fork children that can
// outlive a killed pnpm and keep the output pipe open; see makeTargetWaitDelay.
const pnpmPrepareWaitDelay = 10 * time.Second

// preparePnpmFrozenLockfile runs `pnpm install --frozen-lockfile`.
//
// The environment is the inherited one plus CI=true, LC_ALL=C and
// GIT_TERMINAL_PROMPT=0 — the same pnpm, store and registry the make probe
// will resolve, without interactive prompts. --frozen-lockfile makes a
// lockfile that disagrees with package.json a preparation failure instead of
// a silent re-resolution.
//
// This is process isolation, not a security sandbox. As with legacy Make,
// the selected revision is trusted to execute repository-owned code:
// dependency lifecycle scripts the project allows run here exactly as they
// run in its own install.
func preparePnpmFrozenLockfile(ctx context.Context, g gitRepo, dir string) error {
	before, err := g.refNames(ctx)
	if err != nil {
		return err
	}
	if err := runPnpmFrozenInstall(ctx, dir, pnpmPrepareTimeout); err != nil {
		return err
	}
	after, err := g.refNames(ctx)
	if err != nil {
		return err
	}
	if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
		return fmt.Errorf("preparation changed git refs")
	}
	// A workspace links one node_modules per package, so the output is every
	// ignored node_modules directory, not one prefix.
	return validatePreparedStatusAllowing(ctx, dir, isNodeModulesPath)
}

func isNodeModulesPath(path string) bool {
	return strings.HasPrefix(path, "node_modules/") || strings.Contains(path, "/node_modules/")
}

// runPnpmFrozenInstall runs the install in its own process tree, the way
// runMakeTargetOnce runs a make target: cancellation kills the whole group so
// a lifecycle-script descendant cannot keep Cmd.Wait blocked on the pipe, and
// WaitDelay bounds the wait if one survives anyway.
func runPnpmFrozenInstall(ctx context.Context, dir string, budget time.Duration) error {
	runCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "pnpm", "install", "--frozen-lockfile") // #nosec G204 -- fixed closed profile.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI=true", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	processTree, err := newMakeProcessTree()
	if err != nil {
		return fmt.Errorf("create pnpm process tree: %w", err)
	}
	defer processTree.close()
	if err := processTree.configure(cmd); err != nil {
		return fmt.Errorf("prepare pnpm process tree: %w", err)
	}
	cmd.Cancel = processTree.cancel
	cmd.WaitDelay = pnpmPrepareWaitDelay
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: 128 << 10}
	cmd.Stderr = cmd.Stdout
	err = cmd.Start()
	if err == nil {
		if attachErr := processTree.attach(cmd.Process); attachErr != nil {
			err = makeProcessAttachFailure(cmd, processTree, attachErr)
		} else if releaseErr := processTree.release(); releaseErr != nil {
			err = makeProcessAttachFailure(cmd, processTree, releaseErr)
		} else {
			err = cmd.Wait()
		}
	}
	if err == nil {
		return nil
	}
	if runCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
		return fmt.Errorf("pnpm install --frozen-lockfile exceeded %s and was killed: %s", budget, strings.TrimSpace(out.String()))
	}
	if ctx.Err() != nil {
		return fmt.Errorf("pnpm install --frozen-lockfile canceled: %w", ctx.Err())
	}
	return fmt.Errorf("pnpm install --frozen-lockfile failed: %w: %s", err, strings.TrimSpace(out.String()))
}
