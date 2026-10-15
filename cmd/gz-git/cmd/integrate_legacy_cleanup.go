package cmd

import "github.com/spf13/cobra"

var integrateLegacyCleanupCmd = &cobra.Command{
	Use:   "legacy-cleanup",
	Short: "Plan or apply prospective legacy cleanup",
	Args:  cobra.NoArgs,
}

func init() { integrateCmd.AddCommand(integrateLegacyCleanupCmd) }
