// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/gizzahub/gzh-cli-gitforge/pkg/workspacecli"
)

func init() {
	factory := workspacecli.CommandFactory{}

	// Register `gz-git sync` as a top-level shortcut for `gz-git workspace sync`.
	// Auto-inits .gz-git.yaml when it is missing.
	syncCmd := factory.NewQuickSyncCmd()
	syncCmd.GroupID = "core" // show in Core Git Operations group
	rootCmd.AddCommand(syncCmd)
}
