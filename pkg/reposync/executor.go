package reposync

import "context"

// Executor runs a Plan with concurrency, retries, and strategies.
type Executor interface {
	Execute(ctx context.Context, plan Plan, opts RunOptions, sink ProgressSink, store StateStore) (ExecutionResult, error)
}

// ExecutionResult captures aggregated outcomes from a run.
type ExecutionResult struct {
	Succeeded []ActionResult
	Failed    []ActionResult
	Skipped   []ActionResult
}

// ActionResult is a per-repo outcome.
type ActionResult struct {
	Action  Action
	Message string
	Error   error
}

// Strategy defines how updates are performed.
type Strategy string

const (
	StrategyReset Strategy = "reset"
	StrategyPull  Strategy = "pull"
	StrategyFetch Strategy = "fetch"
)
