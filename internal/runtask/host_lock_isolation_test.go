// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package runtask

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points XDG_STATE_HOME at a private directory. Tests here drive the
// in-process integration engine, which takes the host measurement lock under
// $XDG_STATE_HOME/host-slots; no test may take, or wait on, this host's real
// lock.
func TestMain(m *testing.M) {
	state, err := os.MkdirTemp("", "gz-git-test-state-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create test state dir:", err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_STATE_HOME", state); err != nil {
		fmt.Fprintln(os.Stderr, "set XDG_STATE_HOME:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(state)
	os.Exit(code)
}
