//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Let cooperative gates cancel their separately grouped bindings before the
// outer group is killed. This cleanup grace does not extend the runner budget.
const readinessTerminationGrace = 2 * time.Second

func readinessRunnerSupported() bool { return true }

func configureReadinessProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killReadinessProcess(process *os.Process) error {
	if process == nil {
		return os.ErrProcessDone
	}
	if process.Pid <= 1 {
		return os.ErrProcessDone
	}
	if err := syscall.Kill(-process.Pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	deadline := time.Now().Add(readinessTerminationGrace)
	for time.Now().Before(deadline) {
		// A leader can exit before its children; probe the captured group.
		if err := syscall.Kill(-process.Pid, 0); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func readinessTerminationRequest() TerminationRequest {
	// The receipt records the initial request, not proof of signal delivery.
	// A surviving group is synchronously escalated to SIGKILL after the grace.
	signal := "SIGTERM"
	return TerminationRequest{Method: "process_group", Signal: &signal}
}
