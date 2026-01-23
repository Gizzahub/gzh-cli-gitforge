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

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/scanner"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/templates"
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
  gz-git workspace scan ~/mydevbox -o myworkspace.yaml

  # Scan with depth limit (default: 2)
  gz-git workspace scan ~/mydevbox --scan-depth 3

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

	cmd.Flags().StringVarP(&opts.Output, "output", "o", opts.Output, "Output config file")
	cmd.Flags().StringVarP(&opts.Output, "config", "c", opts.Output, "Deprecated: use --output")
	_ = cmd.Flags().MarkDeprecated("config", "use --output instead")
	cmd.Flags().IntVarP(&opts.Depth, "scan-depth", "d", opts.Depth, "Directory scan depth")
	cmd.Flags().IntVar(&opts.Depth, "depth", opts.Depth, "[DEPRECATED] use --scan-depth")
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

	// Build repository entries for template
	repoEntries := make([]templates.ScannedRepoData, 0, len(repos))
	for _, repo := range repos {
		entry := templates.ScannedRepoData{
			Name: repo.Name,
			Path: repo.Path,
		}

		// Handle remote URLs
		if len(repo.RemoteURLs) == 1 {
			entry.URL = repo.RemoteURLs[0]
		} else if len(repo.RemoteURLs) > 1 {
			entry.URLs = repo.RemoteURLs
		}

		repoEntries = append(repoEntries, entry)
	}

	// Render template
	data := templates.ScannedData{
		ScannedAt:    time.Now().Format(time.RFC3339),
		Path:         absPath,
		Count:        len(repos),
		Strategy:     "reset",
		Parallel:     4,
		MaxRetries:   3,
		CloneProto:   "ssh",
		SSHPort:      0,
		Repositories: repoEntries,
	}

	content, err := templates.Render(templates.RepositoriesScanned, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	outputPath := opts.Output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(opts.Path, outputPath)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated config: %s (%d repositories)\n", outputPath, len(repos))
	fmt.Fprintf(cmd.OutOrStdout(), "\nRun:\n  gz-git workspace sync -c %s\n", outputPath)

	return nil
}
