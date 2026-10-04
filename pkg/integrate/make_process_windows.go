//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// makeProcessTree owns every process started by make. Closing or terminating
// its Job Object also terminates recipe descendants, which Windows otherwise
// leaves alive when only the direct make process is killed.
type makeProcessTree struct {
	mu          sync.Mutex
	job         windows.Handle
	attached    bool
	canceled    bool
	gateDir     string
	releasePath string
}

func newMakeProcessTree() (*makeProcessTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create Job Object: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("set Job Object limits: %w", err)
	}
	return &makeProcessTree{job: job}, nil
}

// configure replaces direct make execution with a cmd.exe batch handshake.
// The batch file uses only cmd built-ins until release creates its marker, so
// no make recipe descendant can exist before attach assigns cmd.exe to the Job
// Object. Every process make creates after release then inherits that job.
func (p *makeProcessTree) configure(cmd *exec.Cmd) error {
	launch, err := gatedLaunchLine(cmd)
	if err != nil {
		return err
	}
	gateDir, err := os.MkdirTemp("", "gz-git-integrate-make-gate-")
	if err != nil {
		return fmt.Errorf("create make launch gate: %w", err)
	}
	p.gateDir = gateDir
	p.releasePath = filepath.Join(gateDir, "release")
	script := filepath.Join(gateDir, "run.cmd")
	targetDir := cmd.Dir
	if targetDir == "" {
		targetDir, err = os.Getwd()
		if err != nil {
			_ = os.RemoveAll(gateDir)
			p.gateDir, p.releasePath = "", ""
			return fmt.Errorf("resolve make directory: %w", err)
		}
	}
	contents := "@echo off\r\nsetlocal DisableDelayedExpansion\r\n:wait\r\nif not exist \"" +
		escapeBatchPath(p.releasePath) + "\" goto wait\r\n" + launch(escapeBatchPath(targetDir)) + "\r\n"
	if err := os.WriteFile(script, []byte(contents), 0o600); err != nil {
		_ = os.RemoveAll(gateDir)
		p.gateDir, p.releasePath = "", ""
		return fmt.Errorf("write make launch gate: %w", err)
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	cmd.Path = comspec
	// The /c payload has no generated path. cmd.exe finds run.cmd in gateDir,
	// so /s quote handling cannot reinterpret a TEMP path with spaces or %/!.
	cmd.Args = []string{comspec, "/d", "/v:off", "/s", "/c", "call run.cmd"}
	cmd.Dir = gateDir
	return nil
}

// gatedLaunchLine returns the batch line that runs cmd from its directory once
// the gate opens. `make -w <target>` keeps its historical `make -C` line; any
// other command (a preparation profile's fixed argv, e.g. pnpm install) is
// called by its resolved path after changing into the directory. Arguments
// are closed-set constants, so a quote or percent in one is refused rather
// than escaped.
func gatedLaunchLine(cmd *exec.Cmd) (func(dir string) string, error) {
	if len(cmd.Args) == 3 && cmd.Args[0] == "make" && cmd.Args[1] == "-w" {
		target := cmd.Args[2]
		return func(dir string) string { return "make -C \"" + dir + "\" -w " + target }, nil
	}
	if len(cmd.Args) == 0 || cmd.Path == "" {
		return nil, fmt.Errorf("unexpected gated command %q", cmd.Args)
	}
	for _, arg := range cmd.Args[1:] {
		if arg == "" || strings.ContainsAny(arg, "\"%!^&|<> \t\r\n") {
			return nil, fmt.Errorf("unexpected gated command argument %q", arg)
		}
	}
	path := escapeBatchPath(cmd.Path)
	args := strings.Join(cmd.Args[1:], " ")
	return func(dir string) string {
		// call: the resolved command may itself be a .cmd shim (pnpm.cmd).
		return "cd /d \"" + dir + "\" || exit /b 1\r\ncall \"" + path + "\" " + args
	}, nil
}

// escapeBatchPath writes a literal path inside a double-quoted batch operand.
// Quotes are not valid Windows path characters; percent is the remaining
// expansion character, so double it. Delayed expansion is disabled for !.
func escapeBatchPath(path string) string {
	return strings.ReplaceAll(path, "%", "%%")
}

func (p *makeProcessTree) attach(process *os.Process) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return os.ErrProcessDone
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(process.Pid))
	if err != nil {
		return fmt.Errorf("open make process: %w", err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.AssignProcessToJobObject(p.job, handle); err != nil {
		return fmt.Errorf("assign make process: %w", err)
	}
	p.attached = true
	if p.canceled {
		return windows.TerminateJobObject(p.job, 1)
	}
	return nil
}

func (p *makeProcessTree) cancel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.canceled = true
	if p.job == 0 {
		return os.ErrProcessDone
	}
	return windows.TerminateJobObject(p.job, 1)
}

func (p *makeProcessTree) close() {
	p.mu.Lock()
	if p.job != 0 {
		_ = windows.CloseHandle(p.job)
		p.job = 0
	}
	gateDir := p.gateDir
	p.gateDir, p.releasePath = "", ""
	p.mu.Unlock()
	if gateDir != "" {
		_ = os.RemoveAll(gateDir)
	}
}

func (p *makeProcessTree) release() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.attached || p.canceled || p.releasePath == "" {
		return os.ErrProcessDone
	}
	if err := os.WriteFile(p.releasePath, nil, 0o600); err != nil {
		return fmt.Errorf("release make launch gate: %w", err)
	}
	return nil
}
