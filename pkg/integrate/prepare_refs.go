// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// preparedRefGuard is the invariant every preparation profile is held to:
// preparation may not move the refs the verdict is measured against, nor the
// commit of the worktree it prepares.
//
// The bound refs are the source, the target, and the target's remote-tracking
// ref, compared by name AND commit. Refs outside that set are deliberately
// not bound: another session creating or deleting an unrelated branch, or a
// fetch --prune, while a long preparation runs says nothing about the
// preparation, and a names-only snapshot of every ref would fail it. The
// reverse blind spot is closed by comparing commits: a bound ref that moved
// to a different commit under the same name is a change.
type preparedRefGuard struct {
	refs []string
	// head is the commit the prepared worktree must still be detached at.
	// Empty skips the check.
	head string
}

// newPreparedRefGuard binds the refs of plan. The target's local branch is
// bound only when it exists: a repository that never had one must not fail
// because another session created it.
func newPreparedRefGuard(ctx context.Context, g gitRepo, plan TargetPlan, head string) (preparedRefGuard, error) {
	var refs []string
	switch {
	case plan.Branch == "":
	case plan.releasesRemoteRef():
		refs = append(refs, "refs/remotes/"+plan.Branch)
	default:
		refs = append(refs, "refs/heads/"+plan.Branch)
	}
	if name := targetBranchName(plan.Target, plan.Remote); name != "" {
		local := "refs/heads/" + name
		_, ok, err := g.revParse(ctx, local)
		if err != nil {
			return preparedRefGuard{}, err
		}
		if ok {
			refs = append(refs, local)
		}
		if plan.Remote != "" {
			refs = append(refs, "refs/remotes/"+plan.Remote+"/"+name)
		}
	}
	sort.Strings(refs)
	unique := make([]string, 0, len(refs))
	for _, ref := range refs {
		if len(unique) == 0 || unique[len(unique)-1] != ref {
			unique = append(unique, ref)
		}
	}
	return preparedRefGuard{refs: unique, head: head}, nil
}

// refStates maps each named ref to its commit; a missing ref maps to "".
func (g gitRepo) refStates(ctx context.Context, names ...string) (map[string]string, error) {
	states := make(map[string]string, len(names))
	for _, name := range names {
		sha, ok, err := g.revParse(ctx, name)
		if err != nil {
			return nil, err
		}
		if !ok {
			sha = ""
		}
		states[name] = sha
	}
	return states, nil
}

// preparedRefSnapshot is a guard's bound refs as they stood before preparation.
type preparedRefSnapshot struct {
	guard  preparedRefGuard
	states map[string]string
}

func (guard preparedRefGuard) snapshot(ctx context.Context, g gitRepo) (preparedRefSnapshot, error) {
	states, err := g.refStates(ctx, guard.refs...)
	if err != nil {
		return preparedRefSnapshot{}, err
	}
	return preparedRefSnapshot{guard: guard, states: states}, nil
}

// verify fails when a bound ref or the prepared worktree's HEAD differs from
// the snapshot, naming what moved.
func (s preparedRefSnapshot) verify(ctx context.Context, g gitRepo, dir string) error {
	after, err := g.refStates(ctx, s.guard.refs...)
	if err != nil {
		return err
	}
	var moved []string
	for _, name := range s.guard.refs {
		if after[name] != s.states[name] {
			moved = append(moved, fmt.Sprintf("%s (%s -> %s)", name, shortState(s.states[name]), shortState(after[name])))
		}
	}
	if len(moved) > 0 {
		return fmt.Errorf("preparation changed git refs: %s", strings.Join(moved, ", "))
	}
	if s.guard.head == "" {
		return nil
	}
	head, err := newGitRepo(g.exec, dir).headSHA(ctx)
	if err != nil {
		return err
	}
	if head != s.guard.head {
		return fmt.Errorf("preparation moved the prepared worktree HEAD: %s -> %s", shortState(s.guard.head), shortState(head))
	}
	return nil
}

func shortState(sha string) string {
	switch {
	case sha == "":
		return "absent"
	case len(sha) > 12:
		return sha[:12]
	}
	return sha
}
