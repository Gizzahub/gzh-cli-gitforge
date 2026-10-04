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
	runCtx, cancel := context.WithTimeout(ctx, pnpmPrepareTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "pnpm", "install", "--frozen-lockfile") // #nosec G204 -- fixed closed profile.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI=true", "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: 128 << 10}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return fmt.Errorf("preparation timed out")
		}
		return fmt.Errorf("pnpm install --frozen-lockfile failed: %w: %s", err, strings.TrimSpace(out.String()))
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
