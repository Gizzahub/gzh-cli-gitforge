package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

var (
	cloneBranch       string
	cloneDepth        int
	cloneSingleBranch bool
	cloneRecursive    bool
	cloneBare         bool
	cloneMirror       bool
)

// cloneCmd represents the clone command
var cloneCmd = &cobra.Command{
	Use:   "clone <repository> [directory]",
	Short: "Clone a repository into a new directory",
	Long: `Clone a repository from a remote URL into a local directory.

Supported URL formats:
  - HTTPS: https://github.com/user/repo.git
  - SSH: git@github.com:user/repo.git
  - Git: git://github.com/user/repo.git
  - File: /path/to/repo or file:///path/to/repo

If directory is not specified, the repository name is used.`,
	Example: `  # Clone a repository
  gz-git clone https://github.com/user/repo.git

  # Clone into specific directory
  gz-git clone https://github.com/user/repo.git my-repo

  # Clone specific branch
  gz-git clone -b develop https://github.com/user/repo.git

  # Shallow clone (only latest commit)
  gz-git clone --depth 1 https://github.com/user/repo.git

  # Clone with submodules
  gz-git clone --recursive https://github.com/user/repo.git

  # Clone only single branch (faster)
  gz-git clone --single-branch https://github.com/user/repo.git`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runClone,
}

func init() {
	rootCmd.AddCommand(cloneCmd)

	// Flags
	cloneCmd.Flags().StringVarP(&cloneBranch, "branch", "b", "", "checkout specific branch")
	cloneCmd.Flags().IntVar(&cloneDepth, "depth", 0, "create a shallow clone with truncated history")
	cloneCmd.Flags().BoolVar(&cloneSingleBranch, "single-branch", false, "clone only one branch")
	cloneCmd.Flags().BoolVar(&cloneRecursive, "recursive", false, "initialize submodules in the clone")
	cloneCmd.Flags().BoolVar(&cloneBare, "bare", false, "create a bare repository")
	cloneCmd.Flags().BoolVar(&cloneMirror, "mirror", false, "create a mirror repository (all refs)")
}

func runClone(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse arguments
	url := args[0]
	destination := ""

	if len(args) > 1 {
		destination = args[1]
	} else {
		// Extract repository name from URL
		destination = extractRepoName(url)
	}

	if !quiet {
		fmt.Printf("Cloning into '%s'...\n", destination)
	}

	// Create client
	client := repository.NewClient()

	// Build clone options
	opts := repository.CloneOptions{
		URL:          url,
		Destination:  destination,
		Branch:       cloneBranch,
		Depth:        cloneDepth,
		SingleBranch: cloneSingleBranch,
		Recursive:    cloneRecursive,
		Bare:         cloneBare,
		Mirror:       cloneMirror,
		Quiet:        quiet,
	}

	// Clone repository
	repo, err := client.Clone(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if !quiet {
		fmt.Printf("Successfully cloned into '%s'\n", repo.Path)

		// Show basic info
		info, err := client.GetInfo(ctx, repo)
		if err == nil && info.Branch != "" {
			fmt.Printf("Branch: %s\n", info.Branch)
		}
	}

	return nil
}

// extractRepoName extracts the repository name from a URL.
// Example: https://github.com/user/repo.git -> repo
func extractRepoName(url string) string {
	// Remove trailing .git
	name := url
	if len(name) > 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}

	// Find last slash
	lastSlash := -1
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == '\\' {
			lastSlash = i
			break
		}
	}

	if lastSlash >= 0 && lastSlash < len(name)-1 {
		name = name[lastSlash+1:]
	}

	// Remove colon (for SSH URLs like git@github.com:user/repo)
	if lastColon := -1; lastColon >= 0 {
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == ':' {
				lastColon = i
				break
			}
		}
		if lastColon >= 0 && lastColon < len(name)-1 {
			name = name[lastColon+1:]
		}
	}

	if name == "" {
		return "repository"
	}

	return name
}
