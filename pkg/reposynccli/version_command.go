package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (f CommandFactory) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.Version == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "version: development")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", f.Version)
			if f.Commit != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", f.Commit)
			}
			if f.BuildDate != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "built: %s\n", f.BuildDate)
			}
			return nil
		},
	}
}
