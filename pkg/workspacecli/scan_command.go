// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/scanner"
)

// ScanOptions holds options for workspace scan command.
type ScanOptions struct {
	Path           string
	Output         string
	Depth          int
	ExcludePattern string
	IncludePattern string
	NoGitIgnore    bool
}

func (f CommandFactory) newScanCmd() *cobra.Command {
	opts := &ScanOptions{
		Output: DefaultConfigFile,
		Depth:  2,
	}

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan directory for git repos and generate config",
		Long: cliutil.QuickStartHelp(`  # Scan current directory
  gz-git workspace scan

  # Scan specific directory
  gz-git workspace scan ~/mydevbox

  # Scan with custom output
  gz-git workspace scan ~/mydevbox -c myworkspace.yaml

  # Scan with depth limit (default: 2)
  gz-git workspace scan ~/mydevbox --depth 3

  # Exclude patterns
  gz-git workspace scan ~/mydevbox --exclude "vendor,node_modules,tmp/*"

  # Ignore .gitignore
  gz-git workspace scan ~/mydevbox --no-gitignore`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Path = args[0]
			} else {
				opts.Path = "."
			}
			return f.runScan(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "config", "c", opts.Output, "Output config file")
	cmd.Flags().IntVar(&opts.Depth, "depth", opts.Depth, "Maximum scan depth")
	cmd.Flags().StringVar(&opts.ExcludePattern, "exclude", "", "Exclude patterns (comma-separated)")
	cmd.Flags().StringVar(&opts.IncludePattern, "include", "", "Force include patterns (comma-separated)")
	cmd.Flags().BoolVar(&opts.NoGitIgnore, "no-gitignore", false, "Ignore .gitignore patterns")

	return cmd
}

func (f CommandFactory) runScan(cmd *cobra.Command, opts *ScanOptions) error {
	ctx := cmd.Context()

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

	return f.generateConfig(cmd, opts, repos)
}

func (f CommandFactory) generateConfig(cmd *cobra.Command, opts *ScanOptions, repos []*scanner.ScannedRepo) error {
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
			entry["urls"] = repo.RemoteURLs
		}

		repoEntries = append(repoEntries, entry)
	}

	config := map[string]interface{}{
		"# Generated":  time.Now().Format(time.RFC3339),
		"# Path":       absPath,
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

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated config: %s (%d repositories)\n", outputPath, len(repos))
	fmt.Fprintf(cmd.OutOrStdout(), "\nRun:\n  gz-git workspace sync -c %s\n", outputPath)

	return nil
}
