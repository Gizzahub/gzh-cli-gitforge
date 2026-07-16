// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

// Example: pre-merge conflict detection with ConflictDetector.
// Merge/rebase execution is intentionally left to plain git (bulk-first identity).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/branch"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/merge"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func main() {
	ctx := context.Background()

	repoPath := "."
	if len(os.Args) >= 2 {
		repoPath = os.Args[1]
	}

	repoClient := repository.NewClient()
	branchManager := branch.NewManager()
	executor := gitcmd.NewExecutor()
	conflictDetector := merge.NewConflictDetector(executor)

	repo, err := repoClient.Open(ctx, repoPath)
	if err != nil {
		log.Fatalf("Failed to open repository: %v", err)
	}

	fmt.Printf("Repository: %s\n\n", repo.Path)

	current, err := branchManager.Current(ctx, repo)
	if err != nil {
		log.Fatalf("Failed to get current branch: %v", err)
	}

	fmt.Printf("Current branch: %s\n\n", current.Name)

	fmt.Println("=== Pre-Merge Conflict Detection ===")

	branches, err := branchManager.List(ctx, repo, branch.ListOptions{})
	if err != nil {
		log.Printf("Warning: Failed to list branches: %v", err)
	}

	var targetBranch string
	for _, b := range branches {
		if !b.IsRemote && b.Name != current.Name && (b.Name == "main" || b.Name == "master" || b.Name == "develop") {
			targetBranch = b.Name
			break
		}
	}

	if targetBranch != "" {
		fmt.Printf("Checking for potential conflicts with '%s'...\n", targetBranch)

		report, err := conflictDetector.Detect(ctx, repo, merge.DetectOptions{
			Source: current.Name,
			Target: targetBranch,
		})
		if err != nil {
			log.Printf("Warning: Failed to detect conflicts: %v", err)
		} else {
			canFF, ffErr := conflictDetector.CanFastForward(ctx, repo, current.Name, targetBranch)
			if ffErr != nil {
				log.Printf("Warning: Failed to check fast-forward: %v", ffErr)
			}

			switch {
			case canFF:
				fmt.Println("✓ Can fast-forward (no merge commit needed)")
			case len(report.Conflicts) == 0:
				fmt.Println("✓ No conflicts detected - safe to merge")
			default:
				fmt.Printf("⚠️  %d potential conflicts detected:\n", len(report.Conflicts))
				for _, conflict := range report.Conflicts {
					fmt.Printf("  - %s (%s)\n", conflict.FilePath, conflict.ConflictType)
				}
			}

			if report.Difficulty != "" {
				fmt.Printf("Merge difficulty: %s\n", report.Difficulty)
			}
		}
	} else {
		fmt.Println("Skipping - no other branch found to test merge")
		fmt.Println()
		fmt.Println("Example usage:")
		fmt.Println("  report, err := conflictDetector.Detect(ctx, repo, merge.DetectOptions{")
		fmt.Println("      Source: \"feature/my-branch\",")
		fmt.Println("      Target: \"main\",")
		fmt.Println("  })")
	}
	fmt.Println()

	fmt.Println("Tip: use the CLI for conflict detection:")
	fmt.Println("  gz-git conflict detect <source> <target>")
	fmt.Println()
	fmt.Println("For actual merge/rebase, use plain git:")
	fmt.Println("  git merge <branch>")
	fmt.Println("  git rebase <onto>")
}
