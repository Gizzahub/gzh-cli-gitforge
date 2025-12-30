package reposync

import (
	"context"
	"fmt"
)

// NoopExecutor records actions and reports success without touching the
// filesystem or Git. It is intentionally side-effect free for dry-runs and
// early wiring.
type NoopExecutor struct{}

// Execute implements Executor.
func (NoopExecutor) Execute(ctx context.Context, plan Plan, opts RunOptions, sink ProgressSink, store StateStore) (ExecutionResult, error) {
	result := ExecutionResult{}

	if sink == nil {
		sink = NoopProgressSink{}
	}
	if store == nil {
		store = NewInMemoryStateStore()
	}

	state := RunState{Items: make([]RunStateItem, 0, len(plan.Actions))}

	for _, action := range plan.Actions {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		sink.OnStart(action)

		message := "noop executor: no git operations performed"
		if opts.DryRun {
			message = "dry-run: no git operations performed"
		}

		sink.OnProgress(action, message, 1.0)

		res := ActionResult{
			Action:  action,
			Message: message,
		}
		sink.OnComplete(res)
		result.Succeeded = append(result.Succeeded, res)

		state.Items = append(state.Items, RunStateItem{
			Repo:    action.Repo,
			Status:  RunStatusDone,
			Message: message,
		})
	}

	if err := store.Save(ctx, state); err != nil {
		return result, fmt.Errorf("save state: %w", err)
	}

	return result, nil
}
