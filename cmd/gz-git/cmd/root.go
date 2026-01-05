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
	verbose bool
	quiet   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gz-git",
	Short: "Advanced Git operations CLI",
	Long: `gz-git is a bulk-first Git CLI that runs safe operations across many repositories in parallel:
  - Bulk status/fetch/pull/push/update/diff
  - Bulk commit automation (preview by default)
  - Branch switching and cleanup helpers
  - Git history analysis
  - Repository synchronization (filesystem and forge providers)
  - Real-time monitoring (watch)

This tool can also be used as a Go library for integrating Git operations
into your own applications.

Command Groups:

  Core Operations      clone, status, fetch, pull, push, diff, update
  Branch & Cleanup     branch, switch, merge, cleanup
  Automation           commit, sync, watch
  Analysis             history, info
  Maintenance          stash, tag

Common Workflows:

  Daily development:
    gz-git status              # Check all repos in current directory
    gz-git diff                # Review changes across repos
    gz-git commit --dry-run    # Preview bulk commits
    gz-git commit --yes        # Apply bulk commits

  Team sync:
    gz-git sync forge          # Sync repos from GitHub/GitLab/Gitea org/group/user
    gz-git fetch               # Update remote refs

  Branch work:
    git checkout -b feature/x  # Create a branch (native git)
    gz-git merge detect feature/x main
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
