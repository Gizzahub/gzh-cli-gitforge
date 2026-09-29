package runtask

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// hostResolutionError is deliberately limited to device identity faults so
// callers can give an actionable response without changing their public
// response schema.
type hostResolutionError struct {
	reason     string
	nextAction string
}

func (e *hostResolutionError) Error() string { return e.reason }

// Identity resolution for the run lifecycle. The chain is environment first,
// global Git config second, and nothing else: an agent names itself with
// GZ_GIT_TASK_ACTOR, a person inherits GIT_WORK_ACTOR or the global
// ce.workActor setting. There is no per-user identity file to fall back to --
// a value that only some checkouts can see is a value that silently differs
// between two worktrees of the same repository.
const (
	actorEnvAgent = "GZ_GIT_TASK_ACTOR"
	actorEnvHuman = "GIT_WORK_ACTOR"
	hostEnv       = "GIT_WORK_HOST"
	gitActorKey   = "ce.workActor"
	gitHostKey    = "ce.workHost"
)

func (s *Service) gitIdentity(ctx context.Context, gitKey string) (string, error) {
	result, err := s.command(ctx, s.root, "git", "config", "--global", "--includes", "--get-all", gitKey)
	if err != nil {
		return "", &identityError{result: result, err: fmt.Errorf("read Git work identity %s: %w", gitKey, err)}
	}
	if result.ExitCode == 1 {
		return "", nil
	}
	if result.ExitCode != 0 {
		return "", &identityError{result: result, err: fmt.Errorf("read Git work identity %s: %s", gitKey, strings.TrimSpace(result.Stderr))}
	}
	// Git terminates each configured value with one newline. Remove only that
	// framing byte: trimming all whitespace would collapse a second empty value
	// and turn an ambiguous declaration into an apparently unique one.
	lines := strings.Split(strings.TrimSuffix(result.Stdout, "\n"), "\n")
	if len(lines) != 1 {
		return "", &identityError{result: result, err: fmt.Errorf("global Git %s must have exactly one value; found %d", gitKey, len(lines))}
	}
	value := normalize(lines[0])
	if value == "" {
		return "", &identityError{result: result, err: fmt.Errorf("global Git %s has an invalid value", gitKey)}
	}
	return value, nil
}

func (s *Service) resolveHost(ctx context.Context) (string, error) {
	if value := normalize(os.Getenv(hostEnv)); value != "" {
		return value, nil
	}
	gitHost, err := s.gitIdentity(ctx, gitHostKey)
	if err != nil {
		return "", err
	}
	if gitHost != "" {
		return gitHost, nil
	}
	return "", &hostResolutionError{
		reason:     "device identity is not configured: " + hostEnv + " and global Git " + gitHostKey + " are both unset",
		nextAction: "set " + hostEnv + " or declare global Git " + gitHostKey + " (including its include files)",
	}
}

func (s *Service) resolveOwner(ctx context.Context) (TaskOwner, error) {
	actor, kind := normalize(os.Getenv(actorEnvAgent)), "agent"
	if actor == "" {
		actor, kind = normalize(os.Getenv(actorEnvHuman)), "human"
	}
	if actor == "" {
		var err error
		actor, err = s.gitIdentity(ctx, gitActorKey)
		if err != nil {
			return TaskOwner{}, err
		}
		kind = "human"
	}
	host, err := s.resolveHost(ctx)
	if err != nil {
		return TaskOwner{}, err
	}
	o := TaskOwner{Actor: actor, Host: host, Kind: kind}
	if !o.Valid() {
		return o, fmt.Errorf("task actor is not configured; set %s for an agent run, %s for a person, or global Git %s", actorEnvAgent, actorEnvHuman, gitActorKey)
	}
	return o, nil
}
