package reposync

import (
	"context"
	"errors"
	"fmt"
)

// Runner encapsulates a full plan + execute lifecycle.
type Runner interface {
	Run(ctx context.Context, req RunRequest) (ExecutionResult, error)
}

// RunRequest contains everything required for a run.
type RunRequest struct {
	PlanRequest PlanRequest
	RunOptions  RunOptions

	Progress ProgressSink
	State    StateStore
}

// RunOptions control execution behaviour.
type RunOptions struct {
	Parallel   int
	MaxRetries int
	Resume     bool
	DryRun     bool
}

// Orchestrator wires Planner/Executor/StateStore to implement Runner.
type Orchestrator struct {
	Planner    Planner
	Executor   Executor
	StateStore StateStore
}

// ErrMissingDependency is returned when required collaborators are unset.
var ErrMissingDependency = errors.New("missing dependency")

// NewOrchestrator creates a Runner from injected collaborators.
func NewOrchestrator(planner Planner, executor Executor, state StateStore) *Orchestrator {
	return &Orchestrator{
		Planner:    planner,
		Executor:   executor,
		StateStore: state,
	}
}

// Run executes the plan/execution lifecycle.
func (o *Orchestrator) Run(ctx context.Context, req RunRequest) (ExecutionResult, error) {
	if o.Planner == nil || o.Executor == nil {
		return ExecutionResult{}, ErrMissingDependency
	}

	stateStore := req.State
	if stateStore == nil {
		stateStore = o.StateStore
	}
	if stateStore == nil {
		stateStore = NewInMemoryStateStore()
	}

	var previousState RunState
	if req.RunOptions.Resume {
		prev, err := stateStore.Load(ctx)
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("load state: %w", err)
		}
		previousState = prev
	}

	planReq := req.PlanRequest
	if planReq.Options.DefaultStrategy == "" {
		planReq.Options.DefaultStrategy = StrategyReset
	}

	plan, err := o.Planner.Plan(ctx, planReq)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("plan: %w", err)
	}

	if req.RunOptions.Resume && len(previousState.Items) > 0 {
		plan = filterCompleted(plan, previousState)
	}

	progress := req.Progress
	if progress == nil {
		progress = NoopProgressSink{}
	}

	return o.Executor.Execute(ctx, plan, req.RunOptions, progress, stateStore)
}

func filterCompleted(plan Plan, previous RunState) Plan {
	if len(previous.Items) == 0 {
		return plan
	}

	done := make(map[string]RunStatus, len(previous.Items))
	for _, item := range previous.Items {
		done[item.Repo.TargetPath] = item.Status
	}

	var remaining []Action
	for _, action := range plan.Actions {
		status, ok := done[action.Repo.TargetPath]
		if !ok {
			remaining = append(remaining, action)
			continue
		}
		if status != RunStatusDone {
			remaining = append(remaining, action)
		}
	}

	return Plan{Actions: remaining}
}
