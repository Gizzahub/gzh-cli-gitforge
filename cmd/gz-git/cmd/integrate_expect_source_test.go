// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"testing"
)

// CE detects the expect-source capability the same way it detects
// --no-fetch: by parsing the rendered --help Flags section.
func TestIntegrateExpectSourceFlagDeclaredOnCheckAndRun(t *testing.T) {
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
		if !helpFlagsSectionDeclares(buf.String(), tc.operation, "--expect-source") {
			t.Errorf("%s: rendered --help does not declare --expect-source", cmd.CommandPath())
		}
	}
}
