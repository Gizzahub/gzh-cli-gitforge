// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Package workidentity holds the native work identity chain that the run
// lifecycle and integrate cleanup both resolve: environment first
// (GZ_GIT_TASK_ACTOR for an agent, GIT_WORK_ACTOR for a person, GIT_WORK_HOST
// for the device), global Git config second (ce.workActor, ce.workHost).
//
// Cleanup authorizes by comparing the caller with the owner the run recorded,
// so the two must read the same value from the same configuration. Each keeps
// its own command runner and normalization; this package fixes the names, the
// Git command, and how its result is read.
package workidentity

import (
	"fmt"
	"strings"
)

// Environment variables and global Git keys of the chain, in resolution
// order: the agent actor wins over the person, environment over Git.
const (
	ActorEnvAgent = "GZ_GIT_TASK_ACTOR"
	ActorEnvHuman = "GIT_WORK_ACTOR"
	HostEnv       = "GIT_WORK_HOST"
	ActorGitKey   = "ce.workActor"
	HostGitKey    = "ce.workHost"
)

// GitConfigArgs returns the git arguments, without "git", that read every
// value of key from the global config and its include files.
func GitConfigArgs(key string) []string {
	return []string{"config", "--global", "--includes", "--get-all", key}
}

// ParseGitConfig reads the result of the GitConfigArgs command. set is false,
// with no error, when the key is unset; when there is exactly one value it is
// returned with its framing newline removed, even if empty, so the caller
// can refuse an empty declaration instead of reading it as unset. Anything
// else is an error.
//
// Git exits 1 both for an unset key and for a global config it cannot read;
// only the second warns on stderr, and that must not read as unconfigured.
// Several values are refused rather than resolved to the last one: an
// ambiguous identity is not a configured one.
func ParseGitConfig(key string, exitCode int, stdout, stderr string) (value string, set bool, err error) {
	stderr = strings.TrimSpace(stderr)
	switch {
	case exitCode == 1 && stderr == "":
		return "", false, nil
	case exitCode != 0 && stderr == "":
		return "", false, fmt.Errorf("read global Git %s: exit %d", key, exitCode)
	case exitCode != 0:
		return "", false, fmt.Errorf("read global Git %s: exit %d: %s", key, exitCode, stderr)
	}
	// Git terminates each configured value with one newline. Remove only that
	// framing byte: trimming all whitespace would collapse a second empty value
	// and turn an ambiguous declaration into an apparently unique one.
	values := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(values) != 1 {
		return "", false, fmt.Errorf("global Git %s must have exactly one value; found %d", key, len(values))
	}
	return values[0], true, nil
}
