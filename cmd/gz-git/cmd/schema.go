// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
)

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Show configuration schema reference",
	Long: `Print a documented example configuration showing all available options and structure.

This schema applies to:
  - Project config: .gz-git.yaml
  - Global config:  ~/.config/gz-git/config.yaml
  - Profile config: ~/.config/gz-git/profiles/*.yaml

Example:
  # View schema
  gz-git schema

  # Save default config template
  gz-git schema > .gz-git.yaml`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.ExampleConfig)
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
