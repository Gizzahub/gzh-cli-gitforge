// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"fmt"
	"os"
	"testing"
	"time"
)

const testWaitTimeout = 10 * time.Second

// TestMain points XDG state at a private directory, so no test in this
// package writes into, or reads from, this host's state.
func TestMain(m *testing.M) {
	reclaimDeadTestStateDirs(os.TempDir())
	state, err := newTestStateDir()
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
