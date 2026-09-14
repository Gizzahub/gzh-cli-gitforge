package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Both apply commands introduce or change a policy contract, and the digest
// they require is a human review acknowledgement. A pipe or CI runner has
// nobody to supply it, so neither may run without a terminal -- and neither may
// grow a --yes or environment bypass that puts the decision back in a script.
func TestIntegrateApplyRefusesWithoutTerminal(t *testing.T) {
	if stdinIsInteractive() {
		t.Skip("stdin is a terminal; this guard only asserts the non-interactive path")
	}
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"bootstrap apply", integrateBootstrapApplyCmd},
		{"readiness update apply", integrateReadinessUpdateApplyCmd},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cmd.RunE(tc.cmd, nil)
			if err == nil {
				t.Fatal("expected a refusal without a terminal")
			}
			if !strings.Contains(err.Error(), "interactive terminal") {
				t.Errorf("refused for the wrong reason: %v", err)
			}
		})
	}
}

// The refusal must not be reachable by supplying the flags, and no flag may
// stand in for the terminal.
func TestIntegrateApplyHasNoConfirmationBypassFlag(t *testing.T) {
	for _, cmd := range []*cobra.Command{integrateBootstrapApplyCmd, integrateReadinessUpdateApplyCmd} {
		for _, name := range []string{"yes", "force", "non-interactive", "batch"} {
			if cmd.Flags().Lookup(name) != nil {
				t.Errorf("%s defines --%s; the human confirmation must not be bypassable", cmd.CommandPath(), name)
			}
		}
	}
}
