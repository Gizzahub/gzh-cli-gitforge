package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	gzhcligitforge "github.com/gizzahub/gzh-cli-gitforge"
)

var (
	verbose bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "gz-gitforge",
	Short: "Git platform management CLI",
	Long: `gz-gitforge is a CLI tool for managing Git platforms (GitHub, GitLab, Gitea).

It provides unified commands for:
  - Syncing repositories from organizations/groups
  - Managing webhooks across platforms
  - Repository configuration and auditing
  - Cross-platform operations`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")

	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "gz-gitforge %s\n", gzhcligitforge.Version)
		fmt.Fprintf(os.Stdout, "  Commit: %s\n", gzhcligitforge.GitCommit)
		fmt.Fprintf(os.Stdout, "  Built:  %s\n", gzhcligitforge.BuildDate)
	},
}
