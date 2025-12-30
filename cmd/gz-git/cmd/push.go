package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/commit"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	pushForce       bool
	pushSetUpstream bool
	pushTags        bool
	pushDryRun      bool
	pushAllRemotes  bool
	pushSkipChecks  bool
)

// pushCmd represents the push command (single repository, git-style)
var pushCmd = &cobra.Command{
	Use:   "push [remote] [refspec]",
	Short: "Push commits to remote repository",
	Long: `Push local commits to a remote repository.

This command works like 'git push' with enhanced safety checks.
It operates on the current directory's Git repository.

Arguments:
  remote   - Remote name (default: origin)
  refspec  - Branch or refspec to push (e.g., "develop" or "develop:master")

Refspec format:
  branch           - Push current branch to same-named remote branch
  local:remote     - Push local branch to different remote branch
  :remote          - Delete remote branch

For bulk operations across multiple repositories, use 'push-bulk' instead.`,
	Example: `  # Push current branch to origin
  gz-git push

  # Push to specific remote
  gz-git push upstream

  # Push current branch to origin
  gz-git push origin main

  # Push local develop to remote master
  gz-git push origin develop:master

  # Push feature branch to origin with upstream tracking
  gz-git push -u origin feature/login

  # Force push with lease (safer than --force)
  gz-git push --force origin develop

  # Push all tags
  gz-git push --tags origin

  # Push to all configured remotes
  gz-git push --all-remotes

  # Dry run - show what would be pushed
  gz-git push --dry-run origin main

  # Delete remote branch
  gz-git push origin :old-branch`,
	Args: cobra.MaximumNArgs(2),
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	// Push flags
	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false, "force push with lease (use with caution!)")
	pushCmd.Flags().BoolVarP(&pushSetUpstream, "set-upstream", "u", false, "set upstream for the branch")
	pushCmd.Flags().BoolVarP(&pushTags, "tags", "t", false, "push all tags to remote")
	pushCmd.Flags().BoolVarP(&pushDryRun, "dry-run", "n", false, "dry run - show what would be pushed")
	pushCmd.Flags().BoolVar(&pushAllRemotes, "all-remotes", false, "push to all configured remotes")
	pushCmd.Flags().BoolVar(&pushSkipChecks, "skip-checks", false, "skip safety checks (use with caution!)")
}

func runPush(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	startTime := time.Now()

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Parse arguments
	remote, refspec := parseRemoteRefspec(args)

	// Check if current directory is a git repository
	client := repository.NewClient()
	repo, err := client.Open(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Create smart push
	smartPush := commit.NewSmartPush()

	// Build push options
	opts := commit.PushOptions{
		Remote:      remote,
		Refspec:     refspec,
		Force:       pushForce,
		SetUpstream: pushSetUpstream,
		AllRemotes:  pushAllRemotes,
		DryRun:      pushDryRun,
		SkipChecks:  pushSkipChecks,
	}

	// Show what we're about to do
	if !quiet {
		displayPushIntent(remote, refspec, opts)
	}

	// Run safety checks first (unless skipped or dry run)
	if !pushSkipChecks && !pushDryRun {
		check, err := smartPush.CanPush(ctx, repo)
		if err != nil {
			return fmt.Errorf("safety check failed: %w", err)
		}

		if verbose {
			fmt.Print(commit.FormatPushCheck(check))
		}

		// Show blocking issues
		if !check.Safe {
			for _, issue := range check.Issues {
				if issue.Blocker {
					fmt.Fprintf(os.Stderr, "✗ %s\n", issue.Message)
				}
			}
			for _, rec := range check.Recommendations {
				fmt.Fprintf(os.Stderr, "  → %s\n", rec)
			}
			return fmt.Errorf("push blocked by safety checks (use --skip-checks to override)")
		}
	}

	// Execute push
	if err := smartPush.Push(ctx, repo, opts); err != nil {
		return err
	}

	// Show success
	duration := time.Since(startTime)
	if !quiet {
		displayPushSuccess(remote, refspec, opts, duration)
	}

	return nil
}

// parseRemoteRefspec parses command arguments into remote and refspec
func parseRemoteRefspec(args []string) (remote, refspec string) {
	remote = "origin" // default remote

	switch len(args) {
	case 0:
		// No args: push current branch to origin
		return remote, ""
	case 1:
		// One arg: could be remote or refspec
		arg := args[0]
		if isLikelyRefspec(arg) {
			// It's a refspec (contains ":" or looks like a branch)
			return remote, arg
		}
		// It's a remote name
		return arg, ""
	case 2:
		// Two args: remote and refspec
		return args[0], args[1]
	}

	return remote, ""
}

// isLikelyRefspec determines if an argument looks like a refspec rather than a remote
func isLikelyRefspec(arg string) bool {
	// Contains ":" → definitely a refspec (e.g., "develop:master" or ":delete-branch")
	if strings.Contains(arg, ":") {
		return true
	}

	// Common remote names that should NOT be treated as refspecs
	commonRemotes := map[string]bool{
		"origin":   true,
		"upstream": true,
		"fork":     true,
		"backup":   true,
	}

	if commonRemotes[arg] {
		return false
	}

	// If it contains "/" it's likely a branch (e.g., "feature/login")
	if strings.Contains(arg, "/") {
		return true
	}

	// Default: treat as remote name
	return false
}

// displayPushIntent shows what the push command will do
func displayPushIntent(remote, refspec string, opts commit.PushOptions) {
	var action string
	if opts.DryRun {
		action = "Would push"
	} else {
		action = "Pushing"
	}

	var target string
	if refspec != "" {
		if strings.HasPrefix(refspec, ":") {
			// Delete branch
			target = fmt.Sprintf("(deleting %s)", strings.TrimPrefix(refspec, ":"))
		} else if strings.Contains(refspec, ":") {
			// Full refspec
			parts := strings.Split(refspec, ":")
			target = fmt.Sprintf("%s → %s", parts[0], parts[1])
		} else {
			target = refspec
		}
	} else {
		target = "(current branch)"
	}

	remotePart := remote
	if opts.AllRemotes {
		remotePart = "(all remotes)"
	}

	flags := []string{}
	if opts.Force {
		flags = append(flags, "--force")
	}
	if opts.SetUpstream {
		flags = append(flags, "-u")
	}

	flagStr := ""
	if len(flags) > 0 {
		flagStr = " " + strings.Join(flags, " ")
	}

	fmt.Printf("%s to %s: %s%s\n", action, remotePart, target, flagStr)
}

// displayPushSuccess shows push completion message
func displayPushSuccess(remote, refspec string, opts commit.PushOptions, duration time.Duration) {
	if opts.DryRun {
		fmt.Println("✓ Dry run complete (no changes made)")
		return
	}

	var target string
	if refspec != "" {
		if strings.HasPrefix(refspec, ":") {
			target = fmt.Sprintf("Deleted %s", strings.TrimPrefix(refspec, ":"))
		} else {
			target = fmt.Sprintf("Pushed %s", refspec)
		}
	} else {
		target = "Pushed"
	}

	remotePart := remote
	if opts.AllRemotes {
		remotePart = "all remotes"
	}

	fmt.Printf("✓ %s to %s (%s)\n", target, remotePart, duration.Round(time.Millisecond))
}
