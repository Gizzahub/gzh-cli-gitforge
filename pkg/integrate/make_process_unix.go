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

type makeProcessTree struct {
	mu       sync.Mutex
	process  *os.Process
	canceled bool
}

func newMakeProcessTree() (*makeProcessTree, error) { return &makeProcessTree{}, nil }

func (p *makeProcessTree) configure(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func (p *makeProcessTree) attach(process *os.Process) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.process = process
	if p.canceled {
		return killMakeProcess(process)
	}
	return nil
}

// cancel terminates the process group created for a make target so
// recipe descendants cannot keep Cmd.Wait's stdout/stderr pipes open.
func (p *makeProcessTree) cancel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.canceled = true
	return killMakeProcess(p.process)
}

func (p *makeProcessTree) close() {}

func (p *makeProcessTree) release() error { return nil }

func killMakeProcess(process *os.Process) error {
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
