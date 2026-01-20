// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const sampleConfig = `# gz-git workspace configuration
# Documentation: https://github.com/gizzahub/gzh-cli-gitforge

strategy: reset
parallel: 4
maxRetries: 3
cleanupOrphans: false

# Clone protocol (ssh or https)
cloneProto: ssh

# Custom SSH port (0 = auto-detect)
sshPort: 0

repositories:
  - name: example-repo
    url: https://github.com/example/repo.git
    targetPath: ./repos/example-repo
    # strategy: pull  # Optional: override default strategy
    # cloneProto: ssh # Optional: override default clone protocol

  # Add more repositories here
`

func (f CommandFactory) newInitCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create empty workspace config file",
		Long: `Create a sample workspace configuration file.

The generated file includes:
  - Default sync strategy (reset)
  - Parallel worker settings
  - Clone protocol settings
  - Example repository entries

Examples:
  # Create .gz-git.yaml in current directory
  gz-git workspace init

  # Create with custom name
  gz-git workspace init -c myworkspace.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				outputPath = DefaultConfigFile
			}

			// Check if file exists
			if _, err := os.Stat(outputPath); err == nil {
				return fmt.Errorf("file already exists: %s (use a different name)", outputPath)
			}

			// Write sample config
			if err := os.WriteFile(outputPath, []byte(sampleConfig), 0o644); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Created workspace config: %s\n", outputPath)
			fmt.Fprintln(cmd.OutOrStdout(), "\nEdit the file and add your repositories, then run:")
			fmt.Fprintf(cmd.OutOrStdout(), "  gz-git workspace sync\n")

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "config", "c", "", "Output file path (default: "+DefaultConfigFile+")")

	return cmd
}
