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
)

// integrateEngine is the in-process integration boundary. ADR-0055 makes
// gz-git the integration-provider, and the run lifecycle lives inside gz-git,
// so finish invokes the engine as a function call -- never by shelling out to
// a second copy of this binary and never by delegating to `ce`.
type integrateEngine interface {
	Check(ctx context.Context, dir string, noFetch bool) (CommandResult, error)
	Run(ctx context.Context, dir string, noFetch bool) (CommandResult, error)
}

// integrationProvider is the narrow boundary around the integration engine.
// The lifecycle records its evidence but does not reproduce the Git lifecycle
// itself.
type integrationProvider interface {
	Check(context.Context, string, bool) (CommandResult, error)
	Run(context.Context, string, bool) (CommandResult, error)
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

func (p gzGitIntegrationProvider) Check(ctx context.Context, dir string, noFetch bool) (CommandResult, error) {
	result, err := p.engine.Check(ctx, dir, noFetch)
	return p.stamped(result, dir, "check", noFetch), err
}

func (p gzGitIntegrationProvider) Run(ctx context.Context, dir string, noFetch bool) (CommandResult, error) {
	result, err := p.engine.Run(ctx, dir, noFetch)
	return p.stamped(result, dir, "run", noFetch), err
}

// stamped restores the receipt-diagnostic shape an external provider would
// have produced: command name, argv, and working directory. Exit state and
// output come from the engine; Executable stays empty because the operation
// ran in-process, and receipts keep proving that by its absence.
func (p gzGitIntegrationProvider) stamped(result CommandResult, dir, operation string, noFetch bool) CommandResult {
	result.Command = integrationProviderGZGit
	result.Args = integrateArgs(operation, noFetch)
	if result.WorkDir == "" {
		result.WorkDir = dir
	}
	return result
}

func integrateArgs(operation string, noFetch bool) []string {
	args := []string{"integrate", operation}
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
func (p gzGitIntegrationProvider) Diagnose(context.Context, string) (IntegrationProviderDiagnostics, []CommandDiagnostics, error) {
	return p.diagnoseRequired(), nil, nil
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
		},
	}
}
