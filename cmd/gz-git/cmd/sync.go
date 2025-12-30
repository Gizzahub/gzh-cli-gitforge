package cmd

import (
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposync"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/reposynccli"
)

func init() {
	planner := reposync.FSPlanner{}
	executor := reposync.GitExecutor{}
	state := reposync.NewInMemoryStateStore()
	orchestrator := reposync.NewOrchestrator(planner, executor, state)

	factory := reposynccli.CommandFactory{
		Use:          "sync",
		Short:        "Git repository synchronization",
		Orchestrator: orchestrator,
		SpecLoader:   reposynccli.FileSpecLoader{},
	}

	rootCmd.AddCommand(factory.NewRootCmd())
}
