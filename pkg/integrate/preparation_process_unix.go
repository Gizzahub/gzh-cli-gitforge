//go:build !windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type preparationProcessTree struct {
	mu       sync.Mutex
	process  *os.Process
	canceled bool
}

func newPreparationProcessTree() (*preparationProcessTree, error) {
	return &preparationProcessTree{}, nil
}

func (p *preparationProcessTree) configure(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func (p *preparationProcessTree) attach(process *os.Process) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.process = process
	if p.canceled {
		return killPreparationProcess(process)
	}
	return nil
}

// cancel terminates the process group created for a preparation command so
// recipe descendants cannot keep Cmd.Wait's stdout/stderr pipes open.
func (p *preparationProcessTree) cancel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.canceled = true
	return killPreparationProcess(p.process)
}

func (p *preparationProcessTree) close() {}

func (p *preparationProcessTree) release() error { return nil }

func killPreparationProcess(process *os.Process) error {
	if process == nil {
		return os.ErrProcessDone
	}
	if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}
