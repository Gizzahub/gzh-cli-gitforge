// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package reposynccli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (f CommandFactory) newConfigValidateCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file format",
		Long: `Validate a sync configuration file for correctness.

Checks:
  - YAML syntax
  - Required fields (name, url, targetPath)
  - Duplicate targetPath entries
  - URL format
  - Strategy values

Examples:
  # Validate config file
  gz-git sync config validate -c sync.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			loader := f.SpecLoader
			if loader == nil {
				loader = FileSpecLoader{}
			}

			_, err := loader.Load(ctx, configPath)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Configuration is valid: %s\n", configPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config file [required]")
	_ = cmd.MarkFlagRequired("config")

	return cmd
}
