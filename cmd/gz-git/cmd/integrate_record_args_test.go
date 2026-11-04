// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// recordCommands are the commands that address one existing execution; each
// takes --record <id>@<generation> and nothing else (ADR-0161 cutover).
func recordCommands() map[string]struct {
	cmd    *cobra.Command
	record *string
} {
	return map[string]struct {
		cmd    *cobra.Command
		record *string
	}{
		"cleanup": {integrateCleanupCmd, &integrateCleanupRecord},
		"receipt": {integrateReceiptCmd, &integrateReceiptRecord},
	}
}

// resetRecordFlags returns every flag of a record command to its default,
// Changed included, so a test can parse a fresh argument list.
func resetRecordFlags(t *testing.T, c *cobra.Command) {
	t.Helper()
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if setErr := f.Value.Set(f.DefValue); setErr != nil {
			t.Fatalf("reset --%s: %v", f.Name, setErr)
		}
		f.Changed = false
	})
}

func resolveExecutionRecord(t *testing.T, name string, flags ...string) (id, gen string, err error) {
	t.Helper()
	c := recordCommands()[name]
	resetRecordFlags(t, c.cmd)
	t.Cleanup(func() { resetRecordFlags(t, c.cmd) })
	if parseErr := c.cmd.ParseFlags(flags); parseErr != nil {
		t.Fatalf("%s ParseFlags(%q) = %v", name, flags, parseErr)
	}
	return executionRecordFromFlags(*c.record)
}

func TestIntegrateExecutionRecordSplitsAtTheLastAt(t *testing.T) {
	for name := range recordCommands() {
		id, gen, err := resolveExecutionRecord(t, name, "--record", "run@host@g7")
		if err != nil || id != "run@host" || gen != "g7" {
			t.Errorf("%s: = (%q, %q, %v), want run@host/g7", name, id, gen, err)
		}
	}
}

func TestIntegrateExecutionRecordRefusesMissingOrTruncatedForms(t *testing.T) {
	cases := map[string][]string{
		"nothing":            {},
		"record without @":   {"--record", "run-42"},
		"record without id":  {"--record", "@g7"},
		"record without gen": {"--record", "run-42@"},
		"empty record":       {"--record="},
	}
	for name := range recordCommands() {
		for label, flags := range cases {
			if _, _, err := resolveExecutionRecord(t, name, flags...); err == nil {
				t.Errorf("%s %s (%q): error = nil, want refusal", name, label, flags)
			}
		}
	}
}

// The handlers turn the refusal into exit 2 before reading any receipt:
// a record that is absent or malformed never reaches the evidence.
func TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"cleanup": runIntegrateCleanup,
		"receipt": runIntegrateReceipt,
	} {
		if _, _, err := resolveExecutionRecord(t, name); err == nil {
			t.Fatalf("%s: setup should refuse a missing record", name)
		}
		err := run(recordCommands()[name].cmd, nil)
		var exitErr *cliutil.ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "--record <id>@<generation> is required") {
			t.Errorf("%s: error = %v, want exit 2 missing-record refusal", name, err)
		}
	}
}

// TestIntegrateHandlerRefusesMalformedRecord exercises the handlers directly
// with a supplied but malformed record (no <id>@<generation> shape): the
// refusal is exit 2 and names the record grammar.
func TestIntegrateHandlerRefusesMalformedRecord(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"cleanup": runIntegrateCleanup,
		"receipt": runIntegrateReceipt,
	} {
		if _, _, err := resolveExecutionRecord(t, name, "--record", "run-42"); err == nil {
			t.Fatalf("%s: setup should refuse a malformed record", name)
		}
		err := run(recordCommands()[name].cmd, nil)
		var exitErr *cliutil.ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "--record must be <id>@<generation>") {
			t.Errorf("%s: error = %v, want exit 2 record-grammar refusal", name, err)
		}
	}
}

// TestIntegrateHandlerRefusesIncompleteRecord exercises the handlers with no
// record at all: the refusal is exit 2 and names the requirement.
func TestIntegrateHandlerRefusesIncompleteRecord(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"cleanup": runIntegrateCleanup,
		"receipt": runIntegrateReceipt,
	} {
		resetRecordFlags(t, recordCommands()[name].cmd)
		err := run(recordCommands()[name].cmd, nil)
		var exitErr *cliutil.ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "--record <id>@<generation> is required") {
			t.Errorf("%s: error = %v, want exit 2 missing-record refusal", name, err)
		}
	}
}
