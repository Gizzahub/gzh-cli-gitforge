// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import "testing"

// The no-fetch finish contract is declared through the --no-fetch flag on
// both integrate check and integrate run. A consumer probes each command's
// own Flags section on the rendered help, so the flag must be a registered,
// non-hidden flag on both commands. See TASK-221.
func TestIntegrateNoFetchFlagDeclaredOnCheckAndRun(t *testing.T) {
	for _, path := range [][]string{{"integrate", "check"}, {"integrate", "run"}} {
		cmd := findCommand(t, rootCmd, path...)
		if cmd.Flags().Lookup("no-fetch") == nil {
			t.Errorf("%s missing --no-fetch", cmd.CommandPath())
		}
	}
}
