//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const (
	makeProcessHelperMode = "GZH_MAKE_PROCESS_HELPER"
	makeProcessChildPID   = "GZH_MAKE_PROCESS_CHILD_PID"
	makeProcessParentGate = "GZH_MAKE_PROCESS_PARENT_GATE"
)

func TestMakeProcessTreeKillsDescendants(t *testing.T) {
	switch os.Getenv(makeProcessHelperMode) {
	case "parent":
		runMakeProcessParentHelper(t)
		return
	case "child":
		time.Sleep(24 * time.Hour)
		return
	}

	pidFile := filepath.Join(t.TempDir(), "child.pid")
	parentGate := filepath.Join(t.TempDir(), "parent-release")
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestMakeProcessTreeKillsDescendants$")
	cmd.Env = append(os.Environ(), makeProcessHelperMode+"=parent", makeProcessChildPID+"="+pidFile, makeProcessParentGate+"="+parentGate)
	processTree, err := newMakeProcessTree()
	if err != nil {
		t.Fatalf("create process tree: %v", err)
	}
	defer processTree.close()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start parent helper: %v", err)
	}
	if err := processTree.attach(cmd.Process); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("attach parent helper: %v", err)
	}
	if err := os.WriteFile(parentGate, nil, 0o600); err != nil {
		_ = processTree.cancel()
		_ = cmd.Wait()
		t.Fatalf("release parent helper: %v", err)
	}
	childPID := waitForMakeProcessChildPID(t, pidFile)
	childHandle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, childPID)
	if err != nil {
		t.Fatalf("open child process before cancellation: %v", err)
	}
	defer windows.CloseHandle(childHandle)
	if err := processTree.cancel(); err != nil {
		t.Fatalf("cancel process tree: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("terminated parent helper must not exit successfully")
	}
	if err := waitForWindowsProcessExit(childHandle, childPID, 5*time.Second); err != nil {
		t.Cleanup(func() { terminateWindowsProcess(childPID) })
		t.Fatal(err)
	}
}

func TestMakeProcessTreeHandshakeWaitsForJobAttach(t *testing.T) {
	dir := t.TempDir()
	gateRoot := filepath.Join(dir, "gate space % !")
	if err := os.Mkdir(gateRoot, 0o700); err != nil {
		t.Fatalf("create special-character temp root: %v", err)
	}
	t.Setenv("TMP", gateRoot)
	t.Setenv("TEMP", gateRoot)
	marker := filepath.Join(dir, "make-started")
	makeCmd := filepath.Join(dir, "make.cmd")
	if err := os.WriteFile(makeCmd, []byte("@echo launched > \""+marker+"\"\r\n"), 0o600); err != nil {
		t.Fatalf("write fake make: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), "make", "-w", "check")
	cmd.Env = append(withoutEnv(os.Environ(), "PATH"), "PATH="+dir)
	processTree, err := newMakeProcessTree()
	if err != nil {
		t.Fatalf("create process tree: %v", err)
	}
	defer processTree.close()
	if err := processTree.configure(cmd); err != nil {
		t.Fatalf("configure process tree: %v", err)
	}
	if !strings.HasPrefix(processTree.gateDir, gateRoot+string(os.PathSeparator)) {
		t.Fatalf("gate directory %q is not under special-character temp root %q", processTree.gateDir, gateRoot)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start handshake: %v", err)
	}
	defer func() {
		_ = processTree.cancel()
		_ = cmd.Wait()
	}()

	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("make launched before Job Object attach, stat err = %v", err)
	}
	if err := processTree.release(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("release before attach = %v, want os.ErrProcessDone", err)
	}
	if err := processTree.attach(cmd.Process); err != nil {
		t.Fatalf("attach handshake: %v", err)
	}
	if err := processTree.release(); err != nil {
		t.Fatalf("release handshake: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait for fake make: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("make did not launch after release: %v", err)
	}
}

func TestMakeProcessTreeAttachFailureDoesNotReleaseMake(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "make-started")
	makeCmd := filepath.Join(dir, "make.cmd")
	if err := os.WriteFile(makeCmd, []byte("@echo launched > \""+marker+"\"\r\n"), 0o600); err != nil {
		t.Fatalf("write fake make: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), "make", "-w", "check")
	cmd.Env = append(withoutEnv(os.Environ(), "PATH"), "PATH="+dir)
	cmd.WaitDelay = time.Second
	processTree, err := newMakeProcessTree()
	if err != nil {
		t.Fatalf("create process tree: %v", err)
	}
	if err := processTree.configure(cmd); err != nil {
		processTree.close()
		t.Fatalf("configure process tree: %v", err)
	}
	if err := cmd.Start(); err != nil {
		processTree.close()
		t.Fatalf("start handshake: %v", err)
	}
	// A closed job makes attach fail. The failure handler must kill the waiting
	// cmd.exe and, because release was never created, must never run make.
	processTree.close()
	if err := makeProcessAttachFailure(cmd, processTree, errors.New("simulated attach failure")); err == nil {
		t.Fatal("attach failure must be reported")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("make launched after attach failure, stat err = %v", err)
	}
}

func runMakeProcessParentHelper(t *testing.T) {
	pidFile := os.Getenv(makeProcessChildPID)
	parentGate := os.Getenv(makeProcessParentGate)
	if pidFile == "" || parentGate == "" {
		t.Fatal("missing parent helper paths")
	}
	waitForMakeProcessParentRelease(t, parentGate)
	child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestMakeProcessTreeKillsDescendants$")
	child.Env = append(os.Environ(), makeProcessHelperMode+"=child")
	if err := child.Start(); err != nil {
		t.Fatalf("start child helper: %v", err)
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		t.Fatalf("write child pid: %v", err)
	}
	time.Sleep(24 * time.Hour)
}

func waitForMakeProcessParentRelease(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatalf("read parent release: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("parent helper was not released")
}

func waitForMakeProcessChildPID(t *testing.T, path string) uint32 {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
			if err != nil {
				t.Fatalf("parse child pid: %v", err)
			}
			return uint32(pid)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("read child pid: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child helper did not publish its PID")
	return 0
}

func waitForWindowsProcessExit(handle windows.Handle, pid uint32, timeout time.Duration) error {
	event, err := windows.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
	if err != nil {
		return fmt.Errorf("wait for child process: %w", err)
	}
	if event != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("child process %d survived cancellation", pid)
	}
	return nil
}

func terminateWindowsProcess(pid uint32) {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err == nil {
		defer windows.CloseHandle(handle)
		_ = windows.TerminateProcess(handle, 1)
	}
}
