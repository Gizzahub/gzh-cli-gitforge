// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// ndPreparationEnv fixes the build cache for this snapshot. Startup/Git overrides
// are sanitized identically for preparation and the target-owned runner.
func ndPreparationEnv(env []string, target string) []string {
	out := readinessEnv(env)
	filtered := out[:0]
	for _, entry := range out {
		if !strings.HasPrefix(entry, "CARGO_TARGET_DIR=") && !strings.HasPrefix(entry, "CARGO_TERM_COLOR=") && !strings.HasPrefix(entry, "GIT_TERMINAL_PROMPT=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "CARGO_TARGET_DIR="+target, "CARGO_TERM_COLOR=never", "GIT_TERMINAL_PROMPT=0")
}

// prepareNDReadinessPair compiles both immutable sibling snapshots. The unchanged
// V1 runner checks --source-dir, so it inherits only the source-side cache.
func prepareNDReadinessPair(ctx context.Context, g gitRepo, plan TargetPlan, profile, root, targetDir, sourceDir string, leases []SiblingLease, opts CheckOptions) (string, error) {
	if profile != config.NDStackCheckedTestsV1 {
		return "", fmt.Errorf("unsupported readiness preparation profile: %s", profile)
	}
	highLoad, err := prepareHighLoadForPlan(ctx, g, plan)
	if err != nil {
		return "", err
	}
	sides := []struct{ dir, sha, target string }{
		{targetDir, plan.TargetSHA, filepath.Join(root, "target-cargo")},
		{sourceDir, plan.BranchSHA, filepath.Join(root, "source-cargo")},
	}
	for _, side := range sides {
		canonicalDir, err := filepath.EvalSymlinks(side.dir)
		if err != nil {
			return "", err
		}
		var lease *SiblingLease
		for i := range leases {
			if leases[i].Root == canonicalDir && leases[i].Name == "ndea-rs" {
				if lease != nil {
					return "", fmt.Errorf("duplicate ndea-rs preparation lease")
				}
				lease = &leases[i]
			}
		}
		if lease == nil {
			return "", fmt.Errorf("%s requires a declared, provisioned ndea-rs sibling", profile)
		}
		guard, err := newPreparedRefGuard(ctx, g, plan, side.sha)
		if err != nil {
			return "", err
		}
		runCtx, release := ctx, func() {}
		if highLoad {
			started := time.Now()
			runCtx, release, err = holdMeasurementLock(ctx, g, opts)
			if opts.resourceWait != nil {
				*opts.resourceWait += time.Since(started)
			}
			if err != nil {
				return "", err
			}
		}
		err = prepareNDReadinessSide(runCtx, g, side.dir, side.target, *lease, guard, cargoPrepareTimeout)
		release()
		if err != nil {
			return "", fmt.Errorf("readiness preparation %s at %s: %w", profile, side.sha, err)
		}
	}
	return sides[1].target, nil
}

func prepareNDReadinessSide(parent context.Context, g gitRepo, dir, target string, lease SiblingLease, guard preparedRefGuard, timeout time.Duration) error {
	if err := inspectSiblingLease(lease); err != nil {
		return err
	}
	script := filepath.Join(lease.Destination, "scripts", "project-cargo")
	info, err := os.Lstat(script)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return errors.Join(fmt.Errorf("ndea-rs requires a regular executable scripts/project-cargo"), err)
	}
	if err := rejectPathSymlinks(filepath.Dir(script)); err != nil {
		return err
	}
	before, err := guard.snapshot(parent, g)
	if err != nil {
		return err
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		return fmt.Errorf("create isolated Cargo target: %w", err)
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	steps := [][]string{
		{"--", "test", "-p", "nd-common", "-p", "safend", "-p", "flownd", "--lib", "--no-run"},
		{"--", "test", "-p", "nd-common", "-p", "safend", "-p", "flownd", "--features", "nd-common/events", "--lib", "--no-run"},
		{"--", "test", "-p", "nd-events", "--no-run"},
	}
	for _, args := range steps {
		// #nosec G204 -- exact regular script from the verified immutable sibling; fixed arguments.
		cmd := exec.CommandContext(ctx, script, args...)
		cmd.Dir = lease.Destination
		cmd.Env = hostSlotsHeldForChild(parent, ndPreparationEnv(os.Environ(), target))
		var output bytes.Buffer
		cmd.Stdout = &limitedWriter{w: &output, n: 128 << 10}
		cmd.Stderr = cmd.Stdout
		started := time.Now()
		runErr := runPrepareCommand(cmd)
		accumulateValidation(parent, time.Since(started))
		integrityErr := errors.Join(before.verify(context.WithoutCancel(parent), g, dir), inspectSiblingLease(lease))
		if runErr != nil || integrityErr != nil || ctx.Err() != nil {
			return errors.Join(fmt.Errorf("project-cargo %s: %s", strings.Join(args, " "), strings.TrimSpace(output.String())), runErr, integrityErr, ctx.Err())
		}
	}
	return validatePreparedStatusAllowing(parent, dir, func(string) bool { return false })
}
