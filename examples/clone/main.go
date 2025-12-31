// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

func main() {
	ctx := context.Background()

	// Get URL from command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <repository-url> [destination]")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  go run main.go https://github.com/user/repo.git")
		fmt.Println("  go run main.go https://github.com/user/repo.git /tmp/my-repo")
		os.Exit(1)
	}

	url := os.Args[1]
	destination := "/tmp/cloned-repo"
	if len(os.Args) >= 3 {
		destination = os.Args[2]
	}

	// Create repository client
	client := repository.NewClient()

	fmt.Printf("Cloning %s to %s...\n", url, destination)
	fmt.Println()

	// Clone repository with options
	repo, err := client.Clone(ctx, repository.CloneOptions{
		URL:          url,
		Destination:  destination,
		Depth:        1,    // Shallow clone for speed
		SingleBranch: true, // Only clone default branch
		Quiet:        false,
	})
	if err != nil {
		log.Fatalf("Failed to clone: %v", err)
	}

	fmt.Println()
	fmt.Printf("✓ Successfully cloned to: %s\n", repo.Path)

	// Get info about cloned repository
	info, err := client.GetInfo(ctx, repo)
	if err != nil {
		log.Printf("Warning: Failed to get info: %v", err)
		return
	}

	fmt.Println()
	fmt.Println("Repository Information:")
	fmt.Printf("  Branch:     %s\n", info.Branch)
	fmt.Printf("  Remote URL: %s\n", info.RemoteURL)
}
