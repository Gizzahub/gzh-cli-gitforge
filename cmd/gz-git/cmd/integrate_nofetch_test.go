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
// the rendered --help output, first confirming the Usage: section names
// "gz-git integrate <operation>" and only then trusting the Flags: section
// for a line whose first field is the literal "--no-fetch" token. A flag
// registered with Hidden: true still passes Lookup() != nil but never
// appears in that rendered section, so this test reproduces the render-based
// check CE actually performs, not just registration. See TASK-221, TASK-228
// (the Usage: precondition folds in TASK-221's F3 finding).
func TestIntegrateNoFetchFlagDeclaredOnCheckAndRun(t *testing.T) {
	for _, tc := range []struct {
		path      []string
		operation string
	}{
		{[]string{"integrate", "check"}, "check"},
		{[]string{"integrate", "run"}, "run"},
	} {
		cmd := findCommand(t, rootCmd, tc.path...)

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

		if !helpFlagsSectionDeclares(buf.String(), tc.operation, "--no-fetch") {
			t.Errorf("%s: rendered --help does not declare --no-fetch under a Usage: gz-git integrate %s Flags: section (a Hidden flag would fail this the same way)", cmd.CommandPath(), tc.operation)
		}
	}
}

// TestHelpFlagsSectionDeclaresRejectsWithoutUsagePrecondition exercises the
// Usage: precondition itself with crafted help text, not just real rendered
// --help output. Without this, deleting the usage check from
// helpFlagsSectionDeclares would leave the render-based test above green,
// silently regressing to the gap TASK-221's F3 finding identified.
func TestHelpFlagsSectionDeclaresRejectsWithoutUsagePrecondition(t *testing.T) {
	const flagsOnly = "Flags:\n  --no-fetch   skip fetching before finishing\n"
	const usageWrongOperation = "Usage:\n  gz-git integrate check [flags]\n\nFlags:\n  --no-fetch   skip fetching before finishing\n"
	const usageNotIndented = "Usage:\ngz-git integrate run [flags]\n\nFlags:\n  --no-fetch   skip fetching before finishing\n"
	const usageWrongBinary = "Usage:\n  gz integrate run [flags]\n\nFlags:\n  --no-fetch   skip fetching before finishing\n"
	const usageThenFlagsMissing = "Usage:\n  gz-git integrate run [flags]\n\nFlags:\n  --other-flag   unrelated\n"
	const wellFormed = "Usage:\n  gz-git integrate run [flags]\n\nFlags:\n  --no-fetch   skip fetching before finishing\n"

	for name, tc := range map[string]struct {
		help string
		want bool
	}{
		"flags section with no usage header at all":          {flagsOnly, false},
		"usage names a different operation":                  {usageWrongOperation, false},
		"usage line present but not indented":                {usageNotIndented, false},
		"usage names a different binary":                     {usageWrongBinary, false},
		"usage satisfied but flag absent from flags section": {usageThenFlagsMissing, false},
		"usage and flags both well formed":                   {wellFormed, true},
	} {
		if got := helpFlagsSectionDeclares(tc.help, "run", "--no-fetch"); got != tc.want {
			t.Errorf("%s: helpFlagsSectionDeclares() = %v, want %v", name, got, tc.want)
		}
	}
}

// helpFlagsSectionDeclares mirrors CE's declaresFlag (ce-agent-kit
// integration_provider.go): it first requires the Usage: section to name
// "gz-git integrate <operation>" — a mention in a description or another
// command's usage does not count — and only trusts the Flags: section that
// follows for a line whose first whitespace-separated field is the literal
// flag text.
func helpFlagsSectionDeclares(help, operation, flag string) bool {
	usageHeader, usage, inFlags := false, false, false
	scanner := bufio.NewScanner(strings.NewReader(help))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indented := line != strings.TrimLeft(line, " \t")
		if !indented && strings.HasSuffix(trimmed, ":") {
			usageHeader = trimmed == "Usage:"
			inFlags = trimmed == "Flags:" && usage
			continue
		}
		fields := strings.Fields(trimmed)
		if usageHeader && indented && len(fields) >= 3 && fields[0] == "gz-git" && fields[1] == "integrate" && fields[2] == operation {
			usage = true
			continue
		}
		if inFlags && indented && len(fields) > 0 && fields[0] == flag {
			return true
		}
	}
	return false
}
