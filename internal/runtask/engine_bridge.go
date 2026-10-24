// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// The in-process integration engine: pkg/integrate behind the lifecycle's
// engine boundary.

package runtask

import (
	"context"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/integrate"
)

// engineIntegrate adapts pkg/integrate to integrateEngine. It reproduces the
// observable behavior of running `gz-git integrate run --target <ref>` as a
// process: the same stdout formatting, the provenance notice on stderr, the
// same exit-code classification, and an empty stderr beyond the provenance
// wherever this binary's commands print nothing (cobra is configured with
// SilenceErrors, so a refused run says only what the command itself printed).
// An error result carries no Go error -- Finish treats a non-nil error as a
// provider invocation failure, and an engine that answered with an exit code
// ran, so its answer is the exit code.
type engineIntegrate struct{}

// NewEngine returns the in-process integration engine for this binary.
func NewEngine() integrateEngine { return engineIntegrate{} }

func (engineIntegrate) Run(ctx context.Context, dir, target string, noFetch bool) (CommandResult, error) {
	// The provenance line is the first thing the CLI's integrate run writes,
	// on the notice stream, so a captured log names the device and the build
	// that performed the mutation. The in-process engine owes receipts the
	// same attribution.
	device := integrate.ResolveDevice(ctx, gitcmd.NewExecutor(), dir)
	result := CommandResult{
		Command: integrationProviderGZGit,
		Args:    integrateArgs(target, noFetch),
		WorkDir: dir,
		Stderr:  integrate.FormatRunProvenance(device, runtimeVersion),
	}
	report, err := integrate.Run(ctx, gitcmd.NewExecutor(), integrate.RunOptions{CheckOptions: integrate.CheckOptions{RepoPath: dir, Target: target, NoFetch: noFetch}})
	// The command prints the report before classifying the error, so a run
	// that reached its own reporting still leaves its output on stdout.
	if report != nil {
		result.Stdout = integrate.FormatRun(report)
	}
	if err != nil {
		// The classifier is the shared pkg/integrate.RunExitCode -- the same
		// function the integrate run command answers with -- so the in-process
		// engine and the CLI cannot classify one outcome differently.
		result.ExitCode = integrate.RunExitCode(report, err)
		return result, nil
	}
	if report != nil && report.Integrated && report.Reclaim.Incomplete() {
		result.ExitCode = 3
	}
	return result, nil
}
