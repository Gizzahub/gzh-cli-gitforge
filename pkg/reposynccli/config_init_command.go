// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const sampleConfig = `# gz-git sync configuration file
# Documentation: https://github.com/gizzahub/gzh-cli-gitforge

strategy: reset
parallel: 4
maxRetries: 3
cleanupOrphans: false

# Clone protocol (ssh or https)
cloneProto: ssh

# Custom SSH port (0 = auto-detect from GitLab API)
sshPort: 0

repositories:
  - name: example-repo
    url: https://github.com/example/repo.git
    targetPath: ./repos/example-repo
    # strategy: pull  # Optional: override default strategy
    # cloneProto: ssh # Optional: override default clone protocol

  # Add more repositories here
`

func (f CommandFactory) newConfigInitCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a sample configuration file",
		Long: `Create a sample configuration file with common settings.

The generated file includes:
  - Default sync strategy (reset)
  - Parallel worker settings
  - Clone protocol settings
  - Example repository entries

Examples:
  # Create config.yaml in current directory
  gz-git sync config init

  # Create with custom name
  gz-git sync config init -o my-sync.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				outputPath = "config.yaml"
			}

			// Check if file exists
			if _, err := os.Stat(outputPath); err == nil {
				return fmt.Errorf("file already exists: %s (use a different name)", outputPath)
			}

			// Write sample config
			if err := os.WriteFile(outputPath, []byte(sampleConfig), 0o644); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Created sample configuration: %s\n", outputPath)
			fmt.Fprintln(cmd.OutOrStdout(), "\nEdit the file and add your repositories, then run:")
			fmt.Fprintf(cmd.OutOrStdout(), "  gz-git sync from-config -c %s\n", outputPath)

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (default: config.yaml)")

	return cmd
}
