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

// InitOptions holds options for workspace init command.
type InitOptions struct {
	Path           string
	Output         string
	Depth          int
	ExcludePattern string
	IncludePattern string
	NoGitIgnore    bool
	Force          bool
	Template       bool
}

func (f CommandFactory) newInitCmd() *cobra.Command {
	opts := &InitOptions{
		Output: DefaultConfigFile,
		Depth:  2,
	}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize workspace config by scanning for git repos",
		Long: cliutil.QuickStartHelp(`  # Show usage guide
  gz-git workspace init

  # Scan current directory and create config
  gz-git workspace init .

  # Scan specific directory
  gz-git workspace init ~/mydevbox

  # Scan with depth limit (default: 2)
  gz-git workspace init ~/mydevbox -d 3

  # Exclude patterns
  gz-git workspace init . --exclude "vendor,node_modules,tmp"

  # Overwrite existing config
  gz-git workspace init . --force

  # Create empty template (no scanning)
  gz-git workspace init . --template`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// No arguments: show usage guide
			if len(args) == 0 {
				return f.showInitGuide(cmd)
			}

			opts.Path = args[0]
			return f.runInit(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "output", "o", opts.Output, "Output config file name")
	cmd.Flags().IntVarP(&opts.Depth, "scan-depth", "d", opts.Depth, "Directory scan depth")
	cmd.Flags().StringVar(&opts.ExcludePattern, "exclude", "", "Exclude patterns (comma-separated)")
	cmd.Flags().StringVar(&opts.IncludePattern, "include", "", "Force include patterns (comma-separated)")
	cmd.Flags().BoolVar(&opts.NoGitIgnore, "no-gitignore", false, "Ignore .gitignore patterns")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Overwrite existing config file")
	cmd.Flags().BoolVar(&opts.Template, "template", false, "Create empty template without scanning")

	return cmd
}

func (f CommandFactory) showInitGuide(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "📁 Workspace Init - Create config from existing git repositories")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  gz-git workspace init <path>    Scan directory and generate config")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  gz-git workspace init .              # Scan current directory")
	fmt.Fprintln(out, "  gz-git workspace init ~/mydevbox     # Scan specific directory")
	fmt.Fprintln(out, "  gz-git workspace init . -d 3         # Scan 3 levels deep")
	fmt.Fprintln(out, "  gz-git workspace init . --exclude vendor,tmp")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Options:")
	fmt.Fprintln(out, "  -d, --scan-depth int   Scan depth (default: 2)")
	fmt.Fprintln(out, "  -o, --output string    Output file (default: .gz-git.yaml)")
	fmt.Fprintln(out, "  -f, --force            Overwrite existing config")
	fmt.Fprintln(out, "      --template         Create empty template (no scanning)")
	fmt.Fprintln(out, "      --exclude string   Exclude patterns (comma-separated)")
	fmt.Fprintln(out)

	// Check if config already exists in current directory
	if _, err := os.Stat(DefaultConfigFile); err == nil {
		fmt.Fprintf(out, "⚠️  Config file already exists: %s\n", DefaultConfigFile)
		fmt.Fprintln(out, "   Use --force to overwrite:")
		fmt.Fprintln(out, "   gz-git workspace init . --force")
		fmt.Fprintln(out)
	}

	return nil
}

func (f CommandFactory) runInit(cmd *cobra.Command, opts *InitOptions) error {
	// Resolve output path
	outputPath := opts.Output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(opts.Path, outputPath)
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil {
		if !opts.Force {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Config file already exists: %s\n\n", outputPath)
			fmt.Fprintf(cmd.OutOrStdout(), "View:      cat %s\n", outputPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Overwrite: gz-git workspace init %s --force\n", opts.Path)
			return nil
		}
	}

	// Template mode: create empty template without scanning
	if opts.Template {
		return f.createTemplate(cmd, outputPath)
	}

	// Scan mode: scan directory and generate config
	return f.scanAndGenerate(cmd, opts, outputPath)
}

func (f CommandFactory) createTemplate(cmd *cobra.Command, outputPath string) error {
	sampleContent, err := templates.GetRaw(templates.RepositoriesSample)
	if err != nil {
		return fmt.Errorf("failed to load sample template: %w", err)
	}

	if err := os.WriteFile(outputPath, sampleContent, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Created empty template: %s\n", outputPath)
	fmt.Fprintln(cmd.OutOrStdout(), "\nEdit the file and add your repositories, then run:")
	fmt.Fprintln(cmd.OutOrStdout(), "  gz-git workspace sync")

	return nil
}

func (f CommandFactory) scanAndGenerate(cmd *cobra.Command, opts *InitOptions, outputPath string) error {
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
		fmt.Fprintln(cmd.OutOrStdout(), "No git repositories found.")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "To create an empty template instead:")
		fmt.Fprintf(cmd.OutOrStdout(), "  gz-git workspace init %s --template\n", opts.Path)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d git repositories\n\n", len(repos))

	return f.generateConfigFromScan(cmd, opts, repos, outputPath)
}

func (f CommandFactory) generateConfigFromScan(cmd *cobra.Command, opts *InitOptions, repos []*scanner.ScannedRepo, outputPath string) error {
	// Build repository entries for template
	repoEntries := make([]templates.ScannedRepoData, 0, len(repos))
	for _, repo := range repos {
		entry := templates.ScannedRepoData{
			Name: repo.Name,
			Path: repo.Path,
		}

		// Handle remotes: origin → URL, others → additionalRemotes
		if len(repo.Remotes) > 0 {
			// Use origin as primary URL
			if originURL, ok := repo.Remotes["origin"]; ok {
				entry.URL = originURL
				// Add other remotes as additionalRemotes
				if len(repo.Remotes) > 1 {
					entry.AdditionalRemotes = make(map[string]string)
					for name, url := range repo.Remotes {
						if name != "origin" {
							entry.AdditionalRemotes[name] = url
						}
					}
				}
			} else {
				// No origin, use first remote as URL, rest as additional
				first := true
				for name, url := range repo.Remotes {
					if first {
						entry.URL = url
						first = false
					} else {
						if entry.AdditionalRemotes == nil {
							entry.AdditionalRemotes = make(map[string]string)
						}
						entry.AdditionalRemotes[name] = url
					}
				}
			}
		}

		repoEntries = append(repoEntries, entry)
	}

	// Render template
	// BasePath "." means paths are relative to config file location
	data := templates.ScannedData{
		ScannedAt:    time.Now().Format(time.RFC3339),
		BasePath:     ".",
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

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated config: %s (%d repositories)\n", outputPath, len(repos))
	fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  gz-git workspace sync        # Clone/update repositories")
	fmt.Fprintln(cmd.OutOrStdout(), "  gz-git workspace status      # Check workspace health")

	return nil
}
