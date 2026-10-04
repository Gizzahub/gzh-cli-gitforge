//go:build windows

// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGatedLaunchLineRunsNonMakeCommandsByPath(t *testing.T) {
	cmd := &exec.Cmd{Path: `C:\tools\pnpm.cmd`, Args: []string{"pnpm", "install", "--frozen-lockfile"}}
	launch, err := gatedLaunchLine(cmd)
	if err != nil {
		t.Fatal(err)
	}
	line := launch(`C:\work dir`)
	if !strings.Contains(line, `cd /d "C:\work dir"`) || !strings.Contains(line, `call "C:\tools\pnpm.cmd" install --frozen-lockfile`) {
		t.Fatalf("launch line = %q", line)
	}
	makeCmd := &exec.Cmd{Path: "make", Args: []string{"make", "-w", "check"}}
	launch, err = gatedLaunchLine(makeCmd)
	if err != nil || launch(`C:\r`) != `make -C "C:\r" -w check` {
		t.Fatalf("make line changed: %v", err)
	}
	bad := &exec.Cmd{Path: "x.exe", Args: []string{"x", "a&b"}}
	if _, err := gatedLaunchLine(bad); err == nil {
		t.Fatal("metacharacter argument accepted")
	}
}
