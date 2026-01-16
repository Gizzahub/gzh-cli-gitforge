// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/scanner"
)

// ConfigScanOptions holds options for config scan command.
type ConfigScanOptions struct {
	Path           string
	Strategy       string
	Output         string
	PerDirName     string
	Depth          int
	ExcludePattern string
	IncludePattern string
	NoGitIgnore    bool
}

func (f CommandFactory) newConfigScanCmd() *cobra.Command {
	opts := &ConfigScanOptions{
		Strategy:   "unified",
		Output:     "sync-config.yaml",
		PerDirName: ".gz-git-sync.yaml",
		Depth:      2,
	}

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan local directory for git repositories and generate config",
		Long: `Scan a local directory tree for git repositories and generate configuration file(s).

This command recursively scans for .git directories and creates YAML configuration
files based on the selected strategy.

Strategies:
  unified        - Single config file containing all repositories
  per-directory  - Config file in each directory level with its children

Examples:
  # Scan ~/mydevbox and create unified config
  gz-git sync config scan ~/mydevbox --strategy unified -o sync-config.yaml

  # Scan with per-directory strategy (creates .gz-git-sync.yaml in each dir)
  gz-git sync config scan ~/mydevbox --strategy per-directory --depth 3

  # Exclude patterns (respects .gitignore by default)
  gz-git sync config scan ~/mydevbox --exclude "vendor,node_modules,tmp/*"

  # Force include despite .gitignore
  gz-git sync config scan ~/mydevbox --include "important-vendor/*"

  # Ignore .gitignore completely
  gz-git sync config scan ~/mydevbox --no-gitignore`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to current directory
			if len(args) > 0 {
				opts.Path = args[0]
			} else {
				opts.Path = "."
			}
			return f.runConfigScan(cmd, opts)
		},
	}

	// Strategy
	cmd.Flags().StringVar(&opts.Strategy, "strategy", opts.Strategy, "Scan strategy: unified | per-directory")

	// Output
	cmd.Flags().StringVarP(&opts.Output, "output", "o", opts.Output, "Output file path (unified mode)")
	cmd.Flags().StringVar(&opts.PerDirName, "per-dir-name", opts.PerDirName, "Config filename (per-directory mode)")

	// Scan options
	cmd.Flags().IntVar(&opts.Depth, "depth", opts.Depth, "Maximum scan depth")
	cmd.Flags().StringVar(&opts.ExcludePattern, "exclude", "", "Exclude patterns (comma-separated)")
	cmd.Flags().StringVar(&opts.IncludePattern, "include", "", "Force include patterns (comma-separated)")
	cmd.Flags().BoolVar(&opts.NoGitIgnore, "no-gitignore", false, "Ignore .gitignore patterns")

	return cmd
}

func (f CommandFactory) runConfigScan(cmd *cobra.Command, opts *ConfigScanOptions) error {
	ctx := cmd.Context()

	// Validate strategy
	if opts.Strategy != "unified" && opts.Strategy != "per-directory" {
		return fmt.Errorf("invalid strategy: %s (must be unified or per-directory)", opts.Strategy)
	}

	// Parse exclude/include patterns
	var excludePatterns []string
	if opts.ExcludePattern != "" {
		excludePatterns = strings.Split(opts.ExcludePattern, ",")
	}

	var includePatterns []string
	if opts.IncludePattern != "" {
		includePatterns = strings.Split(opts.IncludePattern, ",")
	}

	// Create scanner
	gitScanner := &scanner.GitRepoScanner{
		RootPath:         opts.Path,
		MaxDepth:         opts.Depth,
		RespectGitIgnore: !opts.NoGitIgnore,
		ExcludePatterns:  excludePatterns,
		IncludePatterns:  includePatterns,
	}

	// Scan for repositories
	fmt.Fprintf(cmd.OutOrStdout(), "Scanning %s (depth: %d)...\n", opts.Path, opts.Depth)
	repos, err := gitScanner.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(repos) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No git repositories found.\n")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d git repositories\n\n", len(repos))

	// Generate config based on strategy
	switch opts.Strategy {
	case "unified":
		return f.generateUnifiedConfig(cmd, opts, repos)
	case "per-directory":
		return f.generatePerDirectoryConfigs(cmd, opts, repos)
	default:
		return fmt.Errorf("unsupported strategy: %s", opts.Strategy)
	}
}

func (f CommandFactory) generateUnifiedConfig(cmd *cobra.Command, opts *ConfigScanOptions, repos []*scanner.ScannedRepo) error {
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		absPath = opts.Path
	}

	// Build config
	repoEntries := make([]map[string]interface{}, 0, len(repos))

	for _, repo := range repos {
		entry := map[string]interface{}{
			"name":       repo.Name,
			"targetPath": repo.Path,
		}

		// Handle remote URLs
		if len(repo.RemoteURLs) == 0 {
			entry["url"] = ""
		} else if len(repo.RemoteURLs) == 1 {
			entry["url"] = repo.RemoteURLs[0]
		} else {
			// Multiple remotes
			entry["urls"] = repo.RemoteURLs
		}

		// Add comment for nested repos
		if repo.Depth > 0 {
			entry["# depth"] = repo.Depth
		}

		repoEntries = append(repoEntries, entry)
	}

	config := map[string]interface{}{
		"# Generated":  time.Now().Format(time.RFC3339),
		"# Path":       absPath,
		"# Strategy":   "unified",
		"# Scanned":    len(repos),
		"strategy":     "reset",
		"parallel":     4,
		"maxRetries":   3,
		"cloneProto":   "ssh",
		"sshPort":      0,
		"repositories": repoEntries,
	}

	// Write to file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}

	outputPath := opts.Output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(opts.Path, outputPath)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated unified config: %s (%d repositories)\n", outputPath, len(repos))
	fmt.Fprintf(cmd.OutOrStdout(), "\nRun:\n  gz-git sync from-config -c %s\n", outputPath)

	return nil
}

func (f CommandFactory) generatePerDirectoryConfigs(cmd *cobra.Command, opts *ConfigScanOptions, repos []*scanner.ScannedRepo) error {
	// Group repositories by parent directory
	dirGroups := make(map[string][]*scanner.ScannedRepo)

	for _, repo := range repos {
		parentDir := filepath.Dir(repo.Path)
		dirGroups[parentDir] = append(dirGroups[parentDir], repo)
	}

	// Generate config for each directory
	configCount := 0
	for dir, dirRepos := range dirGroups {
		configPath := filepath.Join(dir, opts.PerDirName)

		// Build config for this directory
		repoEntries := make([]map[string]interface{}, 0, len(dirRepos))

		for _, repo := range dirRepos {
			entry := map[string]interface{}{
				"name":       repo.Name,
				"targetPath": repo.Path,
			}

			// Handle remote URLs
			if len(repo.RemoteURLs) == 0 {
				entry["url"] = ""
			} else if len(repo.RemoteURLs) == 1 {
				entry["url"] = repo.RemoteURLs[0]
			} else {
				entry["urls"] = repo.RemoteURLs
			}

			repoEntries = append(repoEntries, entry)
		}

		config := map[string]interface{}{
			"# Generated":  time.Now().Format(time.RFC3339),
			"# Directory":  dir,
			"# Strategy":   "per-directory",
			"strategy":     "reset",
			"parallel":     4,
			"maxRetries":   3,
			"cloneProto":   "ssh",
			"sshPort":      0,
			"repositories": repoEntries,
		}

		// Write config file
		data, err := yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("marshal YAML for %s: %w", dir, err)
		}

		if err := os.WriteFile(configPath, data, 0o644); err != nil {
			return fmt.Errorf("write config file %s: %w", configPath, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ %s (%d repos)\n", configPath, len(dirRepos))
		configCount++
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Generated %d config files\n", configCount)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRun in each directory:\n  gz-git sync from-config -c %s\n", opts.PerDirName)

	return nil
}
