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

	rootCmd.AddCommand(factory.NewRootCmd())
}
