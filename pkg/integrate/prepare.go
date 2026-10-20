// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

func runPrepareProfileWithResource(ctx context.Context, g gitRepo, dir, profile string, inputs []prepareInput, guard preparedRefGuard, highLoad bool, opts CheckOptions) error {
	if !highLoad {
		return measurePreparation(ctx, func() error { return runPrepareProfileWithInputs(ctx, g, dir, profile, inputs, guard) })
	}
	started := time.Now()
	heldCtx, release, err := holdMeasurementLock(ctx, g, opts)
	if opts.resourceWait != nil {
		*opts.resourceWait += time.Since(started)
	}
	if err != nil {
		return err
	}
	defer release()
	return measurePreparation(heldCtx, func() error { return runPrepareProfileWithInputs(heldCtx, g, dir, profile, inputs, guard) })
}

// measurePreparation records one profile run after its slot was granted.
func measurePreparation(ctx context.Context, run func() error) error {
	started := time.Now()
	err := run()
	accumulatePreparation(ctx, time.Since(started))
	return err
}

// runPrepareProfileWithInputs runs one closed preparation profile that a
// readiness contract selected. This is process isolation, not a security
// sandbox: the selected revision is trusted to execute its own code.
func runPrepareProfileWithInputs(parent context.Context, g gitRepo, dir, profile string, inputs []prepareInput, guard preparedRefGuard) error {
	if profile == infraOpsPythonUVV1 {
		return prepareInfraOpsPythonUV(parent, g, dir, inputs, guard)
	}
	return fmt.Errorf("unsupported preparation profile %q", profile)
}

func runPrepareCommand(cmd *exec.Cmd) error {
	processTree, err := newPreparationProcessTree()
	if err != nil {
		return fmt.Errorf("create preparation process tree: %w", err)
	}
	defer processTree.close()
	if err := processTree.configure(cmd); err != nil {
		return fmt.Errorf("configure preparation process tree: %w", err)
	}
	cmd.Cancel = processTree.cancel
	cmd.WaitDelay = preparationWaitDelay
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := processTree.attach(cmd.Process); err != nil {
		return preparationProcessAttachFailure(cmd, processTree, err)
	}
	if err := processTree.release(); err != nil {
		return preparationProcessAttachFailure(cmd, processTree, err)
	}
	return cmd.Wait()
}

// validatePreparedStatusAllowing fails when preparation changed tracked files
// or left ignored output that allowed does not accept as the profile's
// output.
func validatePreparedStatusAllowing(ctx context.Context, dir string, allowed func(path string) bool) error {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("inspect prepared tree: %w", err)
	}
	return validatePreparedStatusRecords(raw, allowed)
}

func validatePreparedStatusRecords(raw []byte, allowed func(path string) bool) error {
	for _, record := range bytes.Split(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		if len(record) < 4 {
			return fmt.Errorf("invalid preparation status")
		}
		state, path := string(record[:2]), string(record[3:])
		if state != "!!" || !allowed(path) {
			return fmt.Errorf("preparation changed forbidden path: %s", path)
		}
	}
	return nil
}

type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return len(p), nil
	}
	if len(p) > w.n {
		p = p[:w.n]
	}
	w.n -= len(p)
	return w.w.Write(p)
}
