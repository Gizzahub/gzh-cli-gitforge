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

Key Features:

  ⚙ Configuration Profiles    - Switch contexts instantly (work/personal/client)
  🏥 Health Diagnostics        - Real-time repository health monitoring
  🔀 Refspec Support           - Push local:remote branch mapping
  📁 Recursive Configuration   - Hierarchical config for complex workspaces
  🌐 Network Resilience        - Timeout detection, smart retries
  🔍 Smart Recommendations     - Context-aware next actions
  🔐 Security First            - Input sanitization, safe execution

Quick Start:

  # Set up profiles
  gz-git config init
  gz-git config profile create work
  gz-git config profile use work

  # Check repository health
  gz-git status ~/projects

  # Sync from Git forge
  gz-git sync from-forge --provider gitlab --org myteam

This tool can also be used as a Go library for integrating Git operations
into your own applications.

Command Groups:

  Core Operations      clone, status, fetch, pull, push, diff, update
  Branch & Cleanup     branch, switch, merge, cleanup
  Automation           commit, sync, watch, config
  Analysis             history, info
  Maintenance          stash, tag

Common Workflows:

  Daily development:
    gz-git status              # Check all repos (health diagnostics)
    gz-git diff                # Review changes across repos
    gz-git commit --dry-run    # Preview bulk commits
    gz-git commit --yes        # Apply bulk commits

  Team sync:
    gz-git sync from-forge --provider gitlab --org myteam
    gz-git fetch               # Update remote refs

  Branch work:
    git checkout -b feature/x  # Create a branch (native git)
    gz-git push --refspec feature/x:release/x
    gz-git cleanup branch --merged`,
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
