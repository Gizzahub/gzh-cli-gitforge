// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package runtask

import (
	"context"
	"fmt"
	"os"
)

const (
	integrationProviderGZGit = "gz-git"
	// integrationNoFetchFlag is the provider flag that makes check and run
	// judge freshness on local remote-tracking refs without reading a remote.
	integrationNoFetchFlag = "--no-fetch"
	// integrationNoFetchCapability is declared only when both check and run
	// support the flag. CE read it out of `integrate <op> --help`; here the
	// engine is this binary, so the capability is a build fact, not a probe.
	integrationNoFetchCapability = "integrate-no-fetch"
	// integrationReleaseCapability is the optional release-landing flag.
	// CE probed `integrate check --help` for it; here it is a build fact.
	integrationReleaseCapability = "integrate-release"
	// integrationTargetFlag pins run to the source recorded at run-start, so
	// a task worktree that changes its own integration declaration cannot
	// silently retarget the landing.
	integrationTargetFlag = "--target"
)

// integrateEngine is the in-process integration boundary. ADR-0055 makes
// gz-git the integration-provider, and the run lifecycle lives inside gz-git,
// so finish invokes the engine as a function call -- never by shelling out to
// a second copy of this binary and never by delegating to `ce`.
type integrateEngine interface {
	Run(ctx context.Context, dir string, target string, noFetch bool) (CommandResult, error)
}

// integrationProvider is the narrow boundary around the integration engine.
// The lifecycle records its evidence but does not reproduce the Git lifecycle
// itself.
type integrationProvider interface {
	// Run takes the worktree, the pinned target ref, and whether to skip
	// fetching. It measures readiness itself, so finish never calls check.
	Run(context.Context, string, string, bool) (CommandResult, error)
	Diagnose(context.Context, string) (IntegrationProviderDiagnostics, []CommandDiagnostics, error)
	// DiagnoseNoFetch is Diagnose that also fails unless the provider declares
	// the no-fetch finish capability, naming why it is not declared.
	DiagnoseNoFetch(context.Context, string) (IntegrationProviderDiagnostics, []CommandDiagnostics, error)
}

func integrationProviderFor(name string, engine integrateEngine) (integrationProvider, error) {
	if name != integrationProviderGZGit {
		return nil, fmt.Errorf("unsupported integration-provider %q", name)
	}
	return gzGitIntegrationProvider{engine: engine}, nil
}

type gzGitIntegrationProvider struct{ engine integrateEngine }

func (p gzGitIntegrationProvider) Run(ctx context.Context, dir, target string, noFetch bool) (CommandResult, error) {
	result, err := p.engine.Run(ctx, dir, target, noFetch)
	return p.stamped(result, dir, target, noFetch), err
}

// stamped restores the receipt-diagnostic shape an external provider would
// have produced: command name, argv, and working directory. Exit state and
// output come from the engine; Executable stays empty because the operation
// ran in-process, and receipts keep proving that by its absence.
func (p gzGitIntegrationProvider) stamped(result CommandResult, dir, target string, noFetch bool) CommandResult {
	result.Command = integrationProviderGZGit
	result.Args = integrateArgs(target, noFetch)
	if result.WorkDir == "" {
		result.WorkDir = dir
	}
	return result
}

// integrateArgs always names the target: gz-git accepts it only when it is the
// integration branch the worktree declares, so a declaration that drifted from
// the recorded source fails closed instead of landing elsewhere.
func integrateArgs(target string, noFetch bool) []string {
	args := []string{"integrate", "run", integrationTargetFlag, target}
	if noFetch {
		args = append(args, integrationNoFetchFlag)
	}
	return args
}

// Diagnose reports provider metadata and capabilities. CE probed the external
// binary's help output to discover them; the engine here is the running
// binary, so every capability is known at build time and no external probe
// runs -- which is also why this method contributes no diagnostics entries.
// It still never invokes integrate check or run: those operations are
// reserved for Finish.
func (p gzGitIntegrationProvider) Diagnose(ctx context.Context, dir string) (IntegrationProviderDiagnostics, []CommandDiagnostics, error) {
	report := p.diagnoseRequired()
	// The optional release capability is a build fact here, where CE's probe
	// read it from the help output; the no-fetch-only diagnosis keeps the
	// release answer out, exactly as CE's narrower probe did.
	report.Capabilities[integrationReleaseCapability] = true
	return report, nil, nil
}

func (p gzGitIntegrationProvider) DiagnoseNoFetch(context.Context, string) (IntegrationProviderDiagnostics, []CommandDiagnostics, error) {
	return p.diagnoseRequired(), nil, nil
}

func (p gzGitIntegrationProvider) diagnoseRequired() IntegrationProviderDiagnostics {
	// Naming this process is best-effort metadata; a failure to locate it must
	// not block the doctor answer, so the executable field stays empty instead.
	executable := ""
	if path, err := os.Executable(); err == nil {
		executable = path
	}
	return IntegrationProviderDiagnostics{
		Name:    integrationProviderGZGit,
		Present: true,
		Version: gzGitRuntimeVersion(),
		// The engine is this process; the executable names it anyway so the
		// report keeps the same fields a shelled-out provider filled in.
		Executable: executable,
		Capabilities: map[string]bool{
			"integrate-check":            true,
			"integrate-run":              true,
			integrationNoFetchCapability: true,
			integrationReleaseCapability: false,
		},
	}
}
