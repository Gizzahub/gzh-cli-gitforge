// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"testing"
)

// CE detects immutable request capabilities by parsing the rendered --help
// Flags section, rather than guessing from a binary version.
func TestIntegrateSourceApprovalFlagsDeclaredOnRun(t *testing.T) {
	cmd := findCommand(t, rootCmd, "integrate", "run")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Help(); err != nil {
		t.Fatalf("%s: Help() error = %v", cmd.CommandPath(), err)
	}
	for _, flag := range []string{"--source", "--target"} {
		if !helpFlagsSectionDeclares(buf.String(), "run", flag) {
			t.Errorf("%s: rendered --help does not declare %s", cmd.CommandPath(), flag)
		}
	}
}
