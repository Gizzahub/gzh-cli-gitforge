// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

// AddOptions holds options for the add command.
type AddOptions struct {
	ConfigFile  string
	Name        string
	URL         string
	Path        string
	FromCurrent bool
}

func (f CommandFactory) newAddCmd() *cobra.Command {
	opts := &AddOptions{}

	cmd := &cobra.Command{
		Use:   "add [url]",
		Short: "Add repository to workspace config",
		Long: cliutil.QuickStartHelp(`  # Add repository by URL
  gz-git workspace add https://github.com/user/repo.git

  # Add with custom name and path
  gz-git workspace add --name myrepo --url git@github.com:user/repo.git --path ./repos/myrepo

  # Add current directory's repo to config
  gz-git workspace add --from-current

  # Add to specific config file
  gz-git workspace add https://github.com/user/repo.git -c myworkspace.yaml`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.URL = args[0]
			}
			return f.runAdd(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.ConfigFile, "config", "c", "", "Config file (default: "+DefaultConfigFile+")")
	cmd.Flags().StringVar(&opts.Name, "name", "", "Repository name")
	cmd.Flags().StringVar(&opts.URL, "url", "", "Repository URL")
	cmd.Flags().StringVar(&opts.Path, "path", "", "Path for clone")
	cmd.Flags().BoolVar(&opts.FromCurrent, "from-current", false, "Add current directory's repo")

	return cmd
}

func (f CommandFactory) runAdd(cmd *cobra.Command, opts *AddOptions) error {
	// Determine config file
	configPath := opts.ConfigFile
	if configPath == "" {
		detected, err := detectConfigFile(".")
		if err != nil {
			// Create new config if not found
			configPath = DefaultConfigFile
			fmt.Fprintf(cmd.OutOrStdout(), "Creating new config: %s\n", configPath)
		} else {
			configPath = detected
		}
	}

	// Handle --from-current
	if opts.FromCurrent {
		return f.addFromCurrent(cmd, configPath)
	}

	// Validate inputs
	if opts.URL == "" {
		return fmt.Errorf("repository URL required (use --url or pass as argument)")
	}

	// Auto-detect name from URL if not provided
	name := opts.Name
	if name == "" {
		name = extractRepoName(opts.URL)
	}

	// Auto-generate target path if not provided
	targetPath := opts.Path
	if targetPath == "" {
		targetPath = "./" + name
	}

	// Load existing config or create new
	config, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	// Add repository
	newRepo := map[string]interface{}{
		"name":       name,
		"url":        opts.URL,
		"targetPath": targetPath,
	}

	repos, ok := config["repositories"].([]interface{})
	if !ok {
		repos = []interface{}{}
	}

	// Check for duplicates
	for _, r := range repos {
		if rm, ok := r.(map[string]interface{}); ok {
			if rm["targetPath"] == targetPath {
				return fmt.Errorf("repository with targetPath %q already exists", targetPath)
			}
		}
	}

	repos = append(repos, newRepo)
	config["repositories"] = repos

	// Write config
	if err := writeConfig(configPath, config); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Added %s to %s\n", name, configPath)
	return nil
}

func (f CommandFactory) addFromCurrent(cmd *cobra.Command, configPath string) error {
	// Check if current directory is a git repo
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// Get remote URL
	// This is a simple implementation - could be enhanced
	absPath, err := filepath.Abs(".")
	if err != nil {
		return err
	}

	name := filepath.Base(absPath)

	config, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}

	newRepo := map[string]interface{}{
		"name":       name,
		"url":        "", // Will be filled by user
		"targetPath": absPath,
	}

	repos, ok := config["repositories"].([]interface{})
	if !ok {
		repos = []interface{}{}
	}
	repos = append(repos, newRepo)
	config["repositories"] = repos

	if err := writeConfig(configPath, config); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Added %s to %s\n", name, configPath)
	fmt.Fprintln(cmd.OutOrStdout(), "  Note: Please set the repository URL in the config file")
	return nil
}

func loadOrCreateConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Create default config structure
		return map[string]interface{}{
			"strategy":     "reset",
			"parallel":     4,
			"maxRetries":   3,
			"cloneProto":   "ssh",
			"sshPort":      0,
			"repositories": []interface{}{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return config, nil
}

func writeConfig(path string, config map[string]interface{}) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

func extractRepoName(url string) string {
	// Handle various URL formats
	// git@github.com:user/repo.git -> repo
	// https://github.com/user/repo.git -> repo
	// https://github.com/user/repo -> repo

	base := filepath.Base(url)

	// Remove .git suffix
	if len(base) > 4 && base[len(base)-4:] == ".git" {
		base = base[:len(base)-4]
	}

	return base
}
