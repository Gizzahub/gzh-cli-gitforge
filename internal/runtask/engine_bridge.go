// The in-process integration engine: pkg/integrate behind the lifecycle's
// engine boundary.

package runtask

import (
	"context"
	"errors"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// engineIntegrate adapts pkg/integrate to integrateEngine. It reproduces the
// observable behavior of running `gz-git integrate check|run` as a process:
// the same stdout formatting, the same exit-code classification, and an empty
// stderr wherever this binary's commands print nothing (cobra is configured
// with SilenceErrors, so a refused check says only what the command itself
// printed). An error result carries no Go error -- Finish treats a non-nil
// error as a provider invocation failure, and an engine that answered with an
// exit code ran, so its answer is the exit code.
type engineIntegrate struct{}

// NewEngine returns the in-process integration engine for this binary.
func NewEngine() integrateEngine { return engineIntegrate{} }

func (engineIntegrate) Check(ctx context.Context, dir string, noFetch bool) (CommandResult, error) {
	result := CommandResult{Command: integrationProviderGZGit, Args: integrateArgs("check", noFetch), WorkDir: dir}
	report, err := integrate.Check(ctx, gitcmd.NewExecutor(), integrate.CheckOptions{RepoPath: dir, NoFetch: noFetch})
	// Check may return a report together with an error. Preserve the report
	// for the bridge caller.
	if report != nil {
		result.Stdout = integrate.FormatCheck(report)
	}
	if err != nil {
		msg := err.Error()
		switch {
		case errors.Is(err, integrate.ErrImplicitSourceIsTarget) || strings.Contains(msg, "--target") || strings.Contains(msg, "integration branch"):
			result.ExitCode = 1
			result.Stderr = "integrate check: " + msg + "\n"
		default:
			result.ExitCode = 2
		}
		return result, nil
	}
	if !report.Ready {
		result.ExitCode = 1
	}
	return result, nil
}

func (engineIntegrate) Run(ctx context.Context, dir string, noFetch bool) (CommandResult, error) {
	result := CommandResult{Command: integrationProviderGZGit, Args: integrateArgs("run", noFetch), WorkDir: dir}
	report, err := integrate.Run(ctx, gitcmd.NewExecutor(), integrate.RunOptions{CheckOptions: integrate.CheckOptions{RepoPath: dir, NoFetch: noFetch}})
	// The command prints the report before classifying the error, so a run
	// that reached its own reporting still leaves its output on stdout.
	if report != nil {
		result.Stdout = integrate.FormatRun(report)
	}
	if err != nil {
		msg := err.Error()
		switch {
		case errors.Is(err, integrate.ErrImplicitSourceIsTarget) || strings.Contains(msg, "not ready") || strings.Contains(msg, "--target") || strings.Contains(msg, "integration branch"):
			result.ExitCode = 1
		default:
			result.ExitCode = 2
		}
		return result, nil
	}
	if report != nil && report.Integrated && report.Reclaim.Incomplete() {
		result.ExitCode = 3
	}
	return result, nil
}
