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
// takes --record <id>@<generation> next to the legacy pair (TASK-349 step 3).
func recordCommands() map[string]struct {
	cmd    *cobra.Command
	record *string
	id     *string
	gen    *string
} {
	return map[string]struct {
		cmd    *cobra.Command
		record *string
		id     *string
		gen    *string
	}{
		"cleanup": {integrateCleanupCmd, &integrateCleanupRecord, &integrateCleanupExecutionID, &integrateCleanupGeneration},
		"receipt": {integrateReceiptCmd, &integrateReceiptRecord, &integrateReceiptExecutionID, &integrateReceiptGeneration},
	}
}

func resolveExecutionRecord(t *testing.T, name string, flags ...string) (id, gen string, err error) {
	t.Helper()
	c := recordCommands()[name]
	reset := func() {
		c.cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if setErr := f.Value.Set(f.DefValue); setErr != nil {
				t.Fatalf("reset --%s: %v", f.Name, setErr)
			}
			f.Changed = false
		})
	}
	reset()
	t.Cleanup(reset)
	if parseErr := c.cmd.ParseFlags(flags); parseErr != nil {
		t.Fatalf("%s ParseFlags(%q) = %v", name, flags, parseErr)
	}
	return executionRecordFromFlags(c.cmd, *c.record, *c.id, *c.gen)
}

func TestIntegrateExecutionRecordBothFormsResolveToTheSamePair(t *testing.T) {
	for name := range recordCommands() {
		newID, newGen, err := resolveExecutionRecord(t, name, "--record", "run-42@g7")
		if err != nil {
			t.Fatalf("%s --record: %v", name, err)
		}
		oldID, oldGen, err := resolveExecutionRecord(t, name, "--execution-id", "run-42", "--generation", "g7")
		if err != nil {
			t.Fatalf("%s legacy: %v", name, err)
		}
		if newID != "run-42" || newGen != "g7" || newID != oldID || newGen != oldGen {
			t.Errorf("%s: --record = (%q, %q), legacy = (%q, %q), want run-42/g7 for both", name, newID, newGen, oldID, oldGen)
		}
	}
}

func TestIntegrateExecutionRecordSplitsAtTheLastAt(t *testing.T) {
	for name := range recordCommands() {
		id, gen, err := resolveExecutionRecord(t, name, "--record", "run@host@g7")
		if err != nil || id != "run@host" || gen != "g7" {
			t.Errorf("%s: = (%q, %q, %v), want run@host/g7", name, id, gen, err)
		}
	}
}

func TestIntegrateExecutionRecordRefusesMixingTheForms(t *testing.T) {
	mixes := [][]string{
		{"--record", "run-42@g7", "--execution-id", "run-42"},
		{"--record", "run-42@g7", "--generation", "g7"},
		{"--record", "run-42@g7", "--execution-id", "run-42", "--generation", "g7"},
	}
	for name := range recordCommands() {
		for _, flags := range mixes {
			if _, _, err := resolveExecutionRecord(t, name, flags...); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
				t.Errorf("%s %q: error = %v, want mutual exclusion", name, flags, err)
			}
		}
	}
}

func TestIntegrateExecutionRecordRefusesMissingOrTruncatedForms(t *testing.T) {
	cases := map[string][]string{
		"nothing":                 {},
		"only execution-id":       {"--execution-id", "run-42"},
		"only generation":         {"--generation", "g7"},
		"empty legacy generation": {"--execution-id", "run-42", "--generation="},
		"record without @":        {"--record", "run-42"},
		"record without id":       {"--record", "@g7"},
		"record without gen":      {"--record", "run-42@"},
		"empty record":            {"--record="},
	}
	for name := range recordCommands() {
		for label, flags := range cases {
			if _, _, err := resolveExecutionRecord(t, name, flags...); err == nil {
				t.Errorf("%s %s (%q): error = nil, want refusal", name, label, flags)
			}
		}
	}
}

// The handlers turn the refusal into exit 2 before reading any receipt.
func TestIntegrateRecordRefusalExitsTwoBeforeTouchingEvidence(t *testing.T) {
	t.Chdir(t.TempDir())
	for name, run := range map[string]func(*cobra.Command, []string) error{
		"cleanup": runIntegrateCleanup,
		"receipt": runIntegrateReceipt,
	} {
		if _, _, err := resolveExecutionRecord(t, name, "--record", "run-42@g7", "--generation", "g7"); err == nil {
			t.Fatalf("%s: setup should be refused", name)
		}
		err := run(recordCommands()[name].cmd, nil)
		var exitErr *cliutil.ExitError
		if !errors.As(err, &exitErr) || exitErr.Code != 2 || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("%s: error = %v, want exit 2 mutual-exclusion refusal", name, err)
		}
	}
}
