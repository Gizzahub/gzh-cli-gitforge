// Copyright (c) 2025 Archmagece
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/workspacecli"
)

func init() {
	planner := reposync.FSPlanner{}
	executor := reposync.GitExecutor{}
	state := reposync.NewInMemoryStateStore()
	orchestrator := reposync.NewOrchestrator(planner, executor, state)

	factory := workspacecli.CommandFactory{
		Orchestrator: orchestrator,
	}

	// Register `gz-git sync` as a top-level shortcut for `gz-git workspace sync`.
	// Auto-inits .gz-git.yaml when it is missing.
	syncCmd := factory.NewQuickSyncCmd()
	syncCmd.GroupID = "core" // show in Core Git Operations group
	rootCmd.AddCommand(syncCmd)
}
