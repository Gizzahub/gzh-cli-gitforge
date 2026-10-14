//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os"
	"os/exec"
)

func readinessRunnerSupported() bool { return false }

func configureReadinessProcess(_ *exec.Cmd) {}

func killReadinessProcess(process *os.Process) error {
	if process == nil {
		return os.ErrProcessDone
	}
	return process.Kill()
}

func readinessTerminationRequest() TerminationRequest {
	// This unsupported runner path requests Process.Kill, not a signal.
	return TerminationRequest{Method: "process_kill", Signal: nil}
}
