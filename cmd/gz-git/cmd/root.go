// Package cmd implements the CLI commands for gz-git.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// version is set by main.go
	appVersion string

	// Global flags
	verbose         bool
	quiet           bool
	profileOverride string // Override active profile
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gz-git",
	Short: "Advanced Git operations CLI",
	Long: `gz-git is a bulk-first Git CLI that runs safe operations across many repositories in parallel.

[1;36mQuick Start:[0m
  # Initialize workspace and check status
  gz-git config init
  gz-git status

See 'gz-git schema' for configuration reference.`,
	Version: appVersion,
	// Uncomment the following line if your application requires Cobra to
	// check for a config file.
	// PersistentPreRun: initConfig,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	appVersion = version
	rootCmd.Version = version

	setCommandGroups(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setCommandGroups(cmd *cobra.Command) {
	// ANSI color codes
	const (
		colorCyanBold = "\033[1;36m"
		colorReset    = "\033[0m"
	)

	coreGroup := &cobra.Group{ID: "core", Title: colorCyanBold + "Core Git Operations" + colorReset}
	mgmtGroup := &cobra.Group{ID: "mgmt", Title: colorCyanBold + "Management & Configuration" + colorReset}
	toolGroup := &cobra.Group{ID: "tool", Title: colorCyanBold + "Additional Tools" + colorReset}

	cmd.AddGroup(coreGroup, mgmtGroup, toolGroup)

	for _, c := range cmd.Commands() {
		// Skip internal commands
		if c.Name() == "help" || c.Name() == "completion" || c.Name() == "version" {
			continue
		}

		switch c.Name() {
		case "clone", "status", "fetch", "pull", "push", "switch", "commit", "update":
			c.GroupID = coreGroup.ID
		case "workspace", "config", "sync", "schema", "cleanup":
			c.GroupID = mgmtGroup.ID
		default:
			c.GroupID = toolGroup.ID
		}
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (errors only)")
	rootCmd.PersistentFlags().StringVar(&profileOverride, "profile", "", "override active profile (e.g., --profile work)")

	// Version template
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
`)
}

// initConfig reads in config file and ENV variables if set.
// Configuration file support is deferred to Phase 2 (Commit Automation).
// See: specs/10-commit-automation.md for configuration requirements
func initConfig() {
	// Configuration file support deferred to Phase 2
	// Will implement with Viper when needed for:
	// - Commit message templates
	// - Default Git options
	// - User preferences
}
