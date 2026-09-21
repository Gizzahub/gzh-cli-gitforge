// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// The no-fetch finish contract is declared through the --no-fetch flag on
// both integrate check and integrate run. CE's real capability detector
// (declaresFlag) does not call Lookup on the live *pflag.FlagSet — it parses
// the rendered --help output's Flags: section and requires a line whose
// first field is the literal "--no-fetch" token. A flag registered with
// Hidden: true still passes Lookup() != nil but never appears in that
// rendered section, so this test reproduces the render-based check CE
// actually performs, not just registration. See TASK-221, TASK-228.
func TestIntegrateNoFetchFlagDeclaredOnCheckAndRun(t *testing.T) {
	for _, path := range [][]string{{"integrate", "check"}, {"integrate", "run"}} {
		cmd := findCommand(t, rootCmd, path...)

		if cmd.Flags().Lookup("no-fetch") == nil {
			t.Errorf("%s: no-fetch flag not registered", cmd.CommandPath())
			continue
		}

		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		if err := cmd.Help(); err != nil {
			t.Fatalf("%s: Help() error = %v", cmd.CommandPath(), err)
		}

		if !helpFlagsSectionDeclares(buf.String(), "--no-fetch") {
			t.Errorf("%s: rendered --help Flags: section does not declare --no-fetch (a Hidden flag would fail this the same way)", cmd.CommandPath())
		}
	}
}

// helpFlagsSectionDeclares mirrors CE's declaresFlag: it scans the Flags:
// section of rendered --help output (stopping at the next section header,
// e.g. Global Flags:) for a line whose first whitespace-separated field is
// the literal flag text.
func helpFlagsSectionDeclares(help, flag string) bool {
	scanner := bufio.NewScanner(strings.NewReader(help))
	inFlags := false
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			inFlags = trimmed == "Flags:"
			continue
		}
		if !inFlags {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 && fields[0] == flag {
			return true
		}
	}
	return false
}
