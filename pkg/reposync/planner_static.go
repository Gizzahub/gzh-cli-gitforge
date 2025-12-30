package reposync

import (
	"context"
	"errors"
	"fmt"
)

// StaticPlanner produces a trivial plan that maps every RepoSpec to a single
// action (clone or update) using the provided defaults. It is primarily useful
// for early wiring, tests, and dry-runs.
type StaticPlanner struct{}

// ErrNoRepositories is returned when no repositories are provided.
var ErrNoRepositories = errors.New("no repositories provided")

// Plan implements Planner.
func (StaticPlanner) Plan(_ context.Context, req PlanRequest) (Plan, error) {
	if len(req.Input.Repos) == 0 {
		return Plan{}, ErrNoRepositories
	}

	defaultStrategy := req.Options.DefaultStrategy
	if defaultStrategy == "" {
		defaultStrategy = StrategyReset
	}

	actions := make([]Action, 0, len(req.Input.Repos))
	for _, repo := range req.Input.Repos {
		strategy := repo.Strategy
		if strategy == "" {
			strategy = defaultStrategy
		}

		actionType := ActionClone
		if repo.AssumePresent {
			actionType = ActionUpdate
		}

		actions = append(actions, Action{
			Repo:      repo,
			Type:      actionType,
			Strategy:  strategy,
			Reason:    "static planner",
			PlannedBy: "static",
		})
	}

	return Plan{
		Actions: actions,
	}, nil
}

// Describe returns a short description of what would be planned; useful for
// logging in CLI integrations.
func (StaticPlanner) Describe(req PlanRequest) string {
	strategy := req.Options.DefaultStrategy
	if strategy == "" {
		strategy = StrategyReset
	}
	return fmt.Sprintf("static plan for %d repositories (default strategy=%s)", len(req.Input.Repos), strategy)
}
