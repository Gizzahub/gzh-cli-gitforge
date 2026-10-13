// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"testing"
)

// CE detects immutable request capabilities by parsing the rendered --help
// Flags section, rather than guessing from a binary version.
func TestIntegrateExpectedCommitFlagsDeclaredOnCheckAndRun(t *testing.T) {
	for _, tc := range []struct {
		path      []string
		operation string
	}{
		{[]string{"integrate", "check"}, "check"},
		{[]string{"integrate", "run"}, "run"},
	} {
		cmd := findCommand(t, rootCmd, tc.path...)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		if err := cmd.Help(); err != nil {
			t.Fatalf("%s: Help() error = %v", cmd.CommandPath(), err)
		}
		for _, flag := range []string{"--expect-source", "--expect-target"} {
			if !helpFlagsSectionDeclares(buf.String(), tc.operation, flag) {
				t.Errorf("%s: rendered --help does not declare %s", cmd.CommandPath(), flag)
			}
		}
	}
}
