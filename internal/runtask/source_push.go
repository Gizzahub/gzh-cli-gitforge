// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Source-push and containment evidence for the finish receipt.

package runtask

import (
	"fmt"
	"strings"
)

type sourcePushEvidence struct {
	head string
}

// verifySourcePush proves the current source tip is present on origin. Allow
// policy asks origin directly; no-fetch retains the local tracking-ref proof.
func verifySourcePush(probe func(string, ...string) (CommandResult, error), dir, source string, policy IntegrationNetworkPolicy) (sourcePushEvidence, error) {
	ref := "refs/heads/" + source
	head := ""
	if policy == IntegrationNetworkPolicyNoFetch {
		result, err := probe(dir, "rev-parse", "--verify", sourceTrackingRef(source))
		if err != nil || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) == "" {
			return sourcePushEvidence{}, sourcePushError("resolve local source tracking ref", result, err)
		}
		head = strings.TrimSpace(result.Stdout)
	} else {
		result, err := probe(dir, "ls-remote", "--exit-code", "--heads", "origin", ref)
		if err != nil || result.ExitCode != 0 {
			return sourcePushEvidence{}, sourcePushError("query origin source ref", result, err)
		}
		fields := strings.Fields(strings.TrimSpace(result.Stdout))
		if len(fields) != 2 || !validOID(fields[0]) || fields[1] != ref {
			return sourcePushEvidence{}, fmt.Errorf("query origin source ref returned malformed evidence")
		}
		head = fields[0]
		known, err := probe(dir, "cat-file", "-e", head+"^{commit}")
		if err != nil || known.ExitCode != 0 {
			return sourcePushEvidence{}, sourcePushError("verify origin source commit exists locally", known, err)
		}
	}
	local, err := probe(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+source)
	if err != nil || (local.ExitCode != 0 && local.ExitCode != 1) {
		return sourcePushEvidence{}, sourcePushError("inspect local source branch", local, err)
	}
	if local.ExitCode == 0 {
		ahead, err := probe(dir, "rev-list", "--count", head+"..refs/heads/"+source)
		if err != nil || ahead.ExitCode != 0 || strings.TrimSpace(ahead.Stdout) != "0" {
			return sourcePushEvidence{}, sourcePushError("verify local source branch is not ahead", ahead, err)
		}
	}
	return sourcePushEvidence{head: head}, nil
}

// verifySourceContainsTask proves the captured task commit is an ancestor of
// the observed pushed source tip. A source ref can be pushed while an attempted
// task push was rejected, so source evidence alone cannot establish this.
// Rebase integrations rewrite the captured commit and therefore fail closed
// here unless their provider supplies a separate binding proof.
func verifySourceContainsTask(probe func(string, ...string) (CommandResult, error), dir, taskHead, sourceHead string) error {
	contained, err := probe(dir, "merge-base", "--is-ancestor", taskHead, sourceHead)
	if err != nil || contained.ExitCode != 0 {
		return sourcePushError("verify task head is contained in source", contained, err)
	}
	return nil
}

func sourcePushError(stage string, result CommandResult, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if message == "" {
		return fmt.Errorf("%s: exit %d", stage, result.ExitCode)
	}
	return fmt.Errorf("%s: exit %d: %s", stage, result.ExitCode, message)
}

// validOID reports whether v is a full SHA-1 or SHA-256 object name.
func validOID(v string) bool {
	if len(v) != 40 && len(v) != 64 {
		return false
	}
	for _, r := range strings.ToLower(v) {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
