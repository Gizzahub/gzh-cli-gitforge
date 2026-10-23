// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package runtask

import (
	"context"
	"fmt"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/internal/workidentity"
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
// internal/workidentity owns the names and the Git read, which integrate
// cleanup shares, so both accept and refuse the same configuration; the
// normalization below is this package's own.
const (
	actorEnvAgent = workidentity.ActorEnvAgent
	actorEnvHuman = workidentity.ActorEnvHuman
	hostEnv       = workidentity.HostEnv
	gitActorKey   = workidentity.ActorGitKey
	gitHostKey    = workidentity.HostGitKey
)

func (s *Service) gitIdentity(ctx context.Context, gitKey string) (string, error) {
	result, err := s.command(ctx, s.root, "git", workidentity.GitConfigArgs(gitKey)...)
	if err != nil {
		return "", &identityError{result: result, err: fmt.Errorf("read Git work identity %s: %w", gitKey, err)}
	}
	raw, set, err := workidentity.ParseGitConfig(gitKey, result.ExitCode, result.Stdout, result.Stderr)
	if err != nil {
		return "", &identityError{result: result, err: err}
	}
	if !set {
		return "", nil
	}
	value := normalize(raw)
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
