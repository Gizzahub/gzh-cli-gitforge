// Package main demonstrates basic repository operations using gzh-cli-git library.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gizzahub/gzh-cli-git/pkg/repository"
)

func main() {
	ctx := context.Background()

	// Create repository client
	client := repository.NewClient()

	// Open repository (current directory)
	repo, err := client.Open(ctx, "../..")
	if err != nil {
		log.Fatalf("Failed to open repository: %v", err)
	}

	fmt.Printf("Repository: %s\n", repo.Path)
	fmt.Println()

	// Get repository info
	info, err := client.GetInfo(ctx, repo)
	if err != nil {
		log.Fatalf("Failed to get info: %v", err)
	}

	fmt.Println("Repository Information:")
	fmt.Printf("  Branch:     %s\n", info.Branch)
	fmt.Printf("  Remote URL: %s\n", info.RemoteURL)
	if info.Upstream != "" {
		fmt.Printf("  Upstream:   %s\n", info.Upstream)
		if info.AheadBy > 0 || info.BehindBy > 0 {
			fmt.Printf("  Ahead/Behind: +%d/-%d\n", info.AheadBy, info.BehindBy)
		}
	}
	fmt.Println()

	// Get repository status
	status, err := client.GetStatus(ctx, repo)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}

	fmt.Println("Working Tree Status:")
	if status.IsClean {
		fmt.Println("  ✓ Working tree is clean")
	} else {
		fmt.Println("  ✗ Working tree has changes")
		if len(status.StagedFiles) > 0 {
			fmt.Printf("  Staged files:    %d\n", len(status.StagedFiles))
		}
		if len(status.ModifiedFiles) > 0 {
			fmt.Printf("  Modified files:  %d\n", len(status.ModifiedFiles))
		}
		if len(status.UntrackedFiles) > 0 {
			fmt.Printf("  Untracked files: %d\n", len(status.UntrackedFiles))
		}
	}
}
