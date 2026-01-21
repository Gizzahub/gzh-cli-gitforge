package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var infoFlags BulkCommandFlags

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info [directory]",
	Short: "Display repository information",
	Long: `Display information about Git repositories in the specified directory.

Scans for repositories and shows metadata such as current branch, remote URL,
and status summary.

By default:
  - Scans 1 directory level deep
  - Processes 10 repositories in parallel
  - Shows result in a table format`,
	Args: cobra.MaximumNArgs(1),
	Example: `  # Show info for current directory
  gz-git info

  # Show info for specific repository
  gz-git info /path/to/repo

  # Verbose output with more details
  gz-git info --verbose`,
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)

	// Common bulk operation flags
	addBulkFlags(infoCmd, &infoFlags)
}

func runInfo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config with profile support
	effective, _ := LoadEffectiveConfig(cmd, nil)
	if effective != nil {
		if !cmd.Flags().Changed("parallel") && effective.Parallel > 0 {
			infoFlags.Parallel = effective.Parallel
		}
		if verbose {
			PrintConfigSources(cmd, effective)
		}
	}

	// Validate and parse directory
	directory, err := validateBulkDirectory(args)
	if err != nil {
		return err
	}

	// Validate depth
	if err := validateBulkDepth(cmd, infoFlags.Depth); err != nil {
		return err
	}

	// Create client
	client := repository.NewClient()

	// Create logger for verbose mode
	logger := createBulkLogger(verbose)

	if shouldShowProgress(infoFlags.Format, quiet) {
		printScanningMessage(directory, infoFlags.Depth, infoFlags.Parallel, false)
	}

	// Setup bulk scan options
	bulkOpts := repository.BulkStatusOptions{
		Directory:         directory,
		Parallel:          infoFlags.Parallel,
		MaxDepth:          infoFlags.Depth,
		Verbose:           verbose,
		IncludeSubmodules: infoFlags.IncludeSubmodules,
		IncludePattern:    infoFlags.Include,
		ExcludePattern:    infoFlags.Exclude,
		Logger:            logger,
	}

	// Execute scan
	result, err := client.BulkStatus(ctx, bulkOpts)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Display results
	if infoFlags.Format == "json" {
		displayInfoResultsJSON(result)
		return nil
	}

	displayInfoResultsTable(result)

	return nil
}

func displayInfoResultsTable(result *repository.BulkStatusResult) {
	if len(result.Repositories) == 0 {
		fmt.Println("No repositories found.")
		return
	}

	fmt.Println()
	fmt.Printf("found %d repositories (scanned in %s)\n", len(result.Repositories), result.Duration.Round(10*time.Millisecond))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	// Header
	if verbose {
		fmt.Fprintln(w, "REPOSITORY\tBRANCH (SHA)\tVERSION\tSTATUS\tUPDATE\tAUTHOR\tREMOTE")
	} else {
		fmt.Fprintln(w, "REPOSITORY\tBRANCH\tVERSION\tSTATUS\tUPDATE\tREMOTE")
	}

	for _, repo := range result.Repositories {
		// Format Path (basename unless verbose)
		path := filepath.Base(repo.Path)
		if verbose {
			path = repo.RelativePath
		}

		// Format Status
		status := "clean"
		if repo.Status != "clean" {
			status = repo.Status
			if repo.UncommittedFiles > 0 {
				status = fmt.Sprintf("dirty (%d)", repo.UncommittedFiles)
			}
		}
		// Add stash count to status
		if repo.StashCount > 0 {
			status += fmt.Sprintf(" (%d stash)", repo.StashCount)
		}

		// Format Remote (shorten if too long)
		remote := repo.RemoteURL
		if remote == "" {
			remote = "-"
		} else {
			// Add indicator for multiple remotes
			if len(repo.Remotes) > 1 {
				remote += fmt.Sprintf(" (+%d)", len(repo.Remotes)-1)
			}

			if !verbose && len(remote) > 40 {
				remote = "..." + remote[len(remote)-37:]
			}
		}

		// Format Branch & SHA
		branch := repo.Branch
		if branch == "" {
			branch = "DETACHED"
		}
		if repo.HeadSHA != "" {
			branch += fmt.Sprintf(" (%s)", repo.HeadSHA)
		}
		// Add branch count if meaningful
		if repo.LocalBranchCount > 1 {
			// We iterate local branches, so >1 means there are others.
			// But showing (+N) might clutter the branch column too much if SHA is there.
			// Let's keep it simple or show only in verbose.
		}

		// Format Version (Describe)
		version := repo.Describe
		if version == "" {
			version = "-"
		}

		// Format Update (Time)
		update := repo.LastCommitDate
		if update == "" {
			update = "-"
		}

		if verbose {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				path, branch, version, status, update, repo.LastCommitAuthor, remote)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				path, branch, version, status, update, remote)
		}
	}
	w.Flush()
	fmt.Println()
}

func displayInfoResultsJSON(result *repository.BulkStatusResult) {
	// Re-use status JSON output format as it contains all info
	displayStatusResultsJSON(result)
}
