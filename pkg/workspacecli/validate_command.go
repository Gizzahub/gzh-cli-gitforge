// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package workspacecli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (f CommandFactory) newValidateCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate workspace config file",
		Long: `Validate a workspace configuration file for correctness.

Checks:
  - YAML syntax
  - Required fields (name, url, targetPath)
  - Duplicate targetPath entries
  - URL format
  - Strategy values

Examples:
  # Validate config file
  gz-git workspace validate -c myworkspace.yaml

  # Auto-detect config in current directory
  gz-git workspace validate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Auto-detect config file if not specified
			if configPath == "" {
				detected, err := detectConfigFile(".")
				if err != nil {
					return fmt.Errorf("no config file specified and auto-detection failed: %w", err)
				}
				configPath = detected
				fmt.Fprintf(cmd.OutOrStdout(), "Using config: %s\n", configPath)
			}

			loader := FileSpecLoader{}
			_, err := loader.Load(ctx, configPath)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration is valid: %s\n", configPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file (auto-detects "+DefaultConfigFile+")")

	return cmd
}
