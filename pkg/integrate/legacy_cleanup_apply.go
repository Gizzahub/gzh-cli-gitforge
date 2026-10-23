// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

// LegacyCleanupApplyOptions identifies an independently approved plan.
type LegacyCleanupApplyOptions struct {
	PlanPath       string
	ApproveDigest  string
	RuntimeAttempt string
}

// ErrLegacyCleanupPartial reports an incomplete or uncertain deletion outcome.
var ErrLegacyCleanupPartial = errors.New("legacy cleanup incomplete")

var errLegacyIntentMissing = errors.New("legacy cleanup intent is absent")

// LegacyCleanupExitCode distinguishes completion, partial removal, and refusal.
func LegacyCleanupExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrLegacyCleanupPartial) {
		return 3
	}
	return 2
}

type legacyCleanupIntent struct {
	SchemaVersion int    `json:"schema_version"`
	PlanDigest    string `json:"plan_digest"`
	Phase         string `json:"phase"`
}

const (
	legacyCleanupIntentSchema = 1
	legacyCleanupIntentDir    = "gz-git-legacy-cleanup"
	legacyPhaseIntent         = "intent"
	legacyPhaseWorktree       = "worktree_removed"
	legacyPhaseLocal          = "local_branch_removed"
	legacyPhaseRemote         = "remote_branch_removed"
	legacyPhaseComplete       = "complete"
)

func legacyPhaseCount(phase string) (int, error) {
	switch phase {
	case legacyPhaseIntent:
		return 0, nil
	case legacyPhaseWorktree:
		return 1, nil
	case legacyPhaseLocal:
		return 2, nil
	case legacyPhaseRemote, legacyPhaseComplete:
		return 3, nil
	default:
		return 0, fmt.Errorf("unknown legacy cleanup phase %q", phase)
	}
}

// ApplyLegacyCleanup supports standalone native integration only. A digest
// cannot override an unsupported host lifecycle or authorize a different owner.
func ApplyLegacyCleanup(ctx context.Context, exec *gitcmd.Executor, opts LegacyCleanupApplyOptions) error {
	if exec == nil {
		return fmt.Errorf("git executor is nil")
	}
	p, err := ReadLegacyCleanupPlan(opts.PlanPath)
	if err != nil {
		return fmt.Errorf("read cleanup plan: %w", err)
	}
	if opts.ApproveDigest == "" || opts.ApproveDigest != p.PlanDigest {
		return fmt.Errorf("explicit approval digest does not match prospective plan")
	}
	if legacyPathInside(p.TaskWorktree, opts.PlanPath) {
		return fmt.Errorf("plan must remain outside the task worktree")
	}
	caller, err := ResolveCallerOwner(ctx, exec)
	if err != nil {
		return fmt.Errorf("resolve cleanup owner: %w", err)
	}
	if !caller.sameAs(p.Owner) {
		return fmt.Errorf("cleanup owner does not match prospective plan")
	}
	g := newGitRepo(exec, p.RepositoryRoot)
	// Validate actual repository identity before writing locks or intent.
	if err := validateLegacyRepository(ctx, g, p); err != nil {
		return err
	}
	fence, err := acquireTaskResourceFence(ctx, exec, p.RepositoryRoot, p.TaskBranch)
	if err != nil {
		return err
	}
	defer fence.release()
	checker, err := newLegacyGuardChecker(p, opts.RuntimeAttempt)
	if err != nil {
		return err
	}
	return applyStandaloneLegacyCleanup(ctx, exec, p, checker)
}

func validateLegacyRepository(ctx context.Context, g gitRepo, p *LegacyCleanupPlan) error {
	root, err := g.toplevel(ctx)
	if err != nil {
		return fmt.Errorf("observe primary root: %w", err)
	}
	if root != p.RepositoryRoot || p.PrimaryRoot != root {
		return fmt.Errorf("primary root changed")
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if physical != root {
		return fmt.Errorf("primary root is not canonical")
	}
	common, err := canonicalCommonFromRepo(ctx, g)
	if err != nil {
		return err
	}
	if common != p.CommonDir {
		return fmt.Errorf("repository common directory changed")
	}
	dir, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return err
	}
	if dir != p.GitDir {
		return fmt.Errorf("primary git directory changed")
	}
	identity, err := legacyPhysicalIdentity(dir, true)
	if err != nil {
		return err
	}
	if p.WorktreeIdentity == nil || identity != p.WorktreeIdentity.PrimaryGitDirectory {
		return fmt.Errorf("primary git directory physical identity changed")
	}
	if err := validateLegacyRuntimeMode(p, root, common); err != nil {
		return err
	}
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return err
	}
	if len(trees) == 0 || trees[0].Path != root || p.TaskWorktree == root {
		return fmt.Errorf("primary and task registration is not exact")
	}
	branch, err := g.currentBranch(ctx)
	if err != nil {
		return err
	}
	if branch == p.TaskBranch {
		return fmt.Errorf("primary holds the task branch")
	}
	status, err := g.porcelain(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("primary worktree is dirty")
	}
	return nil
}

func validateLegacyRuntimeMode(p *LegacyCleanupPlan, root, common string) error {
	if p.RuntimeGuard == nil {
		if err := refuseLegacyRuntime(root, p.TaskWorktree); err != nil {
			return err
		}
		return refuseLegacyRuntimeState(common)
	}
	if err := validateLegacyCERuntime(root, p.TaskWorktree); err != nil {
		return err
	}
	return refuseLegacyNativeRuntimeState(common)
}

// legacyCleanupPresence proves all surviving identities. Absence must be a
// prefix of the approved deletion order. A decrease after durable intent can
// reconcile a crash before a phase write; recorded removals cannot reappear.
func validateLegacyCleanupDestination(ctx context.Context, g gitRepo, p *LegacyCleanupPlan) error {
	remote, err := detectRemote(ctx, g, p.TaskBranch)
	if err != nil {
		return err
	}
	endpoint, err := pushEndpoint(ctx, g, remote)
	if err != nil {
		return err
	}
	if remote != p.Remote || endpoint != p.Endpoint {
		return fmt.Errorf("push endpoint changed")
	}
	name := strings.TrimPrefix(p.TargetRef, "refs/heads/")
	tip, exists, err := lsRemoteHead(ctx, g, p.Endpoint, name)
	if err != nil {
		return err
	}
	if !exists || tip != p.TargetOID {
		return fmt.Errorf("remote target changed")
	}
	contains, err := g.isAncestor(ctx, p.TaskOID, tip)
	if err != nil {
		return err
	}
	if !contains {
		return fmt.Errorf("remote target does not contain the approved task")
	}
	tracking, ok, err := g.revParse(ctx, "refs/remotes/"+remote+"/"+name)
	if err != nil {
		return err
	}
	if !ok || tracking != p.LocalRemoteOID {
		return fmt.Errorf("target tracking ref changed")
	}
	decl, err := config.LoadRepoRootTaskPattern(p.RepositoryRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(decl.Patterns, p.TaskPatterns) || !reflect.DeepEqual(repository.ProtectedBranches, p.ProtectedBranches) {
		return fmt.Errorf("cleanup protections changed")
	}
	if repository.IsProtected(p.TaskBranch) || p.TaskBranch == name || !config.MatchesAnyTaskPattern(p.TaskBranch, p.TaskPatterns) {
		return fmt.Errorf("task branch is protected or outside approved patterns")
	}
	for _, a := range []struct{ path, hash, status string }{{p.DoneArtifactPath, p.DoneArtifactSHA256, "DONE"}, {p.ReleasedArtifactPath, p.ReleasedArtifactSHA256, "RELEASED"}} {
		hash, err := hashArtifact(a.path)
		if err != nil {
			return err
		}
		if hash != a.hash {
			return fmt.Errorf("%s artifact changed", a.status)
		}
		if err := validateLegacyCleanupArtifact(a.path, a.status, p); err != nil {
			return err
		}
	}
	return nil
}

func legacyCleanupPresence(ctx context.Context, exec *gitcmd.Executor, g gitRepo, p *LegacyCleanupPlan, recorded int) (int, error) {
	if err := validateLegacyRepository(ctx, g, p); err != nil {
		return 0, err
	}
	if err := validateLegacyCleanupDestination(ctx, g, p); err != nil {
		return 0, err
	}
	local, localPresent, err := probeLocalBranch(ctx, g, p.TaskBranch)
	if err != nil {
		return 0, err
	}
	if localPresent && local != p.TaskOID {
		return 0, fmt.Errorf("local task ref moved")
	}
	oid, remotePresent, err := lsRemoteHead(ctx, g, p.Endpoint, p.TaskBranch)
	if err != nil {
		return 0, err
	}
	if remotePresent && oid != p.TaskOID {
		return 0, fmt.Errorf("remote task ref moved")
	}
	worktreePresent, err := legacyWorktreePresence(ctx, exec, g, p)
	if err != nil {
		return 0, err
	}
	removed := 0
	presentSeen := false
	for _, present := range []bool{worktreePresent, localPresent, remotePresent} {
		if present {
			presentSeen = true
		} else {
			if presentSeen {
				return 0, fmt.Errorf("resource disappearance is outside approved cleanup order")
			}
			removed++
		}
	}
	if removed < recorded {
		return 0, fmt.Errorf("removed legacy resource reappeared")
	}
	return removed, nil
}

// legacyWorktreePresence reports whether the task worktree is still registered.
// A registered worktree must keep the planned physical identity and witness; an
// unregistered one must also be absent from disk.
func legacyWorktreePresence(ctx context.Context, exec *gitcmd.Executor, g gitRepo, p *LegacyCleanupPlan) (bool, error) {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return false, err
	}
	present := false
	for _, wt := range trees {
		if wt.Path == p.TaskWorktree || wt.Branch == p.TaskBranch {
			present = true
		}
	}
	if !present {
		return false, ensureLegacyWorktreeAbsent(ctx, g, p)
	}
	if p.WorktreeIdentity == nil || p.WorktreeIdentity.Witness == "" {
		return false, fmt.Errorf("plan predates the legacy cleanup witness; re-plan before removing the worktree")
	}
	fresh, err := captureLegacyWorktreeIdentity(ctx, exec, p)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(fresh, p.WorktreeIdentity) {
		return false, fmt.Errorf("task physical identity changed")
	}
	return true, nil
}

func prepareLegacyCleanup(ctx context.Context, exec *gitcmd.Executor, g gitRepo, p *LegacyCleanupPlan, path string, checker legacyGuardChecker) (*legacyCleanupIntent, int, error) {
	intent, err := loadLegacyCleanupIntent(path)
	if err != nil && !errors.Is(err, errLegacyIntentMissing) {
		return nil, 0, err
	}
	recorded := 0
	if intent != nil {
		if intent.SchemaVersion != legacyCleanupIntentSchema || intent.PlanDigest != p.PlanDigest {
			return nil, 0, fmt.Errorf("cleanup intent identity mismatch")
		}
		recorded, err = legacyPhaseCount(intent.Phase)
		if err != nil {
			return nil, 0, err
		}
	}
	if err := attestLegacyCleanup(ctx, checker, "before resume"); err != nil {
		if recorded > 0 {
			return nil, 0, fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
		return nil, 0, err
	}
	removed, err := legacyCleanupPresence(ctx, exec, g, p, recorded)
	if err != nil {
		if recorded > 0 {
			return nil, 0, fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
		return nil, 0, err
	}
	if intent == nil {
		if removed != 0 {
			return nil, 0, fmt.Errorf("resources disappeared before durable cleanup intent")
		}
		intent = &legacyCleanupIntent{SchemaVersion: legacyCleanupIntentSchema, PlanDigest: p.PlanDigest, Phase: legacyPhaseIntent}
		if err := writeLegacyCleanupIntent(path, intent); err != nil {
			return nil, 0, err
		}
	}
	phases := []string{legacyPhaseIntent, legacyPhaseWorktree, legacyPhaseLocal, legacyPhaseRemote}
	if removed != recorded {
		intent.Phase = phases[removed]
		if err := writeLegacyCleanupIntent(path, intent); err != nil {
			return nil, 0, fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
	}
	return intent, removed, nil
}

func applyStandaloneLegacyCleanup(ctx context.Context, exec *gitcmd.Executor, p *LegacyCleanupPlan, checker legacyGuardChecker) error {
	g := newGitRepo(exec, p.RepositoryRoot)
	path := filepath.Join(p.CommonDir, legacyCleanupIntentDir, p.PlanDigest+".json")
	intent, removed, err := prepareLegacyCleanup(ctx, exec, g, p, path, checker)
	if err != nil {
		return err
	}
	if intent.Phase == legacyPhaseComplete {
		return ensureLegacyAbsence(ctx, g, p)
	}
	phases := []string{legacyPhaseIntent, legacyPhaseWorktree, legacyPhaseLocal, legacyPhaseRemote}
	for step := removed; step < 3; step++ {
		if err := attestLegacyCleanup(ctx, checker, fmt.Sprintf("before remove step %d", step)); err != nil {
			return fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
		actual, err := legacyCleanupPresence(ctx, exec, g, p, step)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
		if actual != step {
			return fmt.Errorf("%w: resource inventory changed during cleanup", ErrLegacyCleanupPartial)
		}
		switch step {
		case 0:
			_, err = g.output(ctx, "worktree", "remove", p.TaskWorktree)
		case 1:
			if err = refuseCheckedOutTaskBranch(ctx, g, p.TaskBranch); err == nil {
				_, err = g.output(ctx, "update-ref", "-d", "--no-deref", "refs/heads/"+p.TaskBranch, p.TaskOID)
			}
		case 2:
			ref := "refs/heads/" + p.TaskBranch
			_, err = g.output(ctx, "push", "--force-with-lease="+ref+":"+p.TaskOID, p.Endpoint, ":"+ref)
		}
		if err != nil {
			return fmt.Errorf("%w: remove step %d: %w", ErrLegacyCleanupPartial, step, err)
		}
		if err := attestLegacyCleanup(ctx, checker, fmt.Sprintf("after remove step %d", step)); err != nil {
			return fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
		actual, err = legacyCleanupPresence(ctx, exec, g, p, step+1)
		if err != nil {
			return fmt.Errorf("%w: verify removal: %w", ErrLegacyCleanupPartial, err)
		}
		if actual != step+1 {
			return fmt.Errorf("%w: unexpected inventory after removal", ErrLegacyCleanupPartial)
		}
		intent.Phase = phases[step+1]
		if err := writeLegacyCleanupIntent(path, intent); err != nil {
			return fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
		}
	}
	if err := ensureLegacyAbsence(ctx, g, p); err != nil {
		return fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
	}
	intent.Phase = legacyPhaseComplete
	if err := writeLegacyCleanupIntent(path, intent); err != nil {
		return fmt.Errorf("%w: %w", ErrLegacyCleanupPartial, err)
	}
	return nil
}

func attestLegacyCleanup(ctx context.Context, checker legacyGuardChecker, stage string) error {
	if err := checker(ctx); err != nil {
		return fmt.Errorf("runtime attestation %s: %w", stage, err)
	}
	return nil
}

func ensureLegacyWorktreeAbsent(ctx context.Context, g gitRepo, p *LegacyCleanupPlan) error {
	trees, err := g.listWorktrees(ctx)
	if err != nil {
		return err
	}
	for _, wt := range trees {
		if wt.Path == p.TaskWorktree || wt.Branch == p.TaskBranch {
			return fmt.Errorf("task worktree remains or reappeared")
		}
	}
	if _, err := os.Lstat(p.TaskWorktree); err == nil {
		return fmt.Errorf("unregistered task path remains")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ensureLegacyAbsence(ctx context.Context, g gitRepo, p *LegacyCleanupPlan) error {
	if err := ensureLegacyWorktreeAbsent(ctx, g, p); err != nil {
		return err
	}
	if _, present, err := probeLocalBranch(ctx, g, p.TaskBranch); err != nil {
		return err
	} else if present {
		return fmt.Errorf("local task ref remains")
	}
	if _, present, err := lsRemoteHead(ctx, g, p.Endpoint, p.TaskBranch); err != nil {
		return err
	} else if present {
		return fmt.Errorf("remote task ref remains")
	}
	return nil
}

func loadLegacyCleanupIntent(path string) (*legacyCleanupIntent, error) {
	present, err := validateLegacyIntentDirectory(filepath.Dir(path), false)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, errLegacyIntentMissing
	}
	data, err := readBoundedRegularFile(path, 16<<10, "legacy cleanup intent")
	if errors.Is(err, os.ErrNotExist) {
		return nil, errLegacyIntentMissing
	}
	if err != nil {
		return nil, err
	}
	var intent legacyCleanupIntent
	if err := jsonUnmarshalStrict(data, &intent); err != nil {
		return nil, err
	}
	return &intent, nil
}

func writeLegacyCleanupIntent(path string, intent *legacyCleanupIntent) error {
	if _, err := validateLegacyIntentDirectory(filepath.Dir(path), true); err != nil {
		return err
	}
	return atomicStateWrite(path, mustMarshalJSON(intent))
}

func validateLegacyIntentDirectory(dir string, create bool) (bool, error) {
	if create {
		if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
			return false, fmt.Errorf("create legacy intent directory: %w", err)
		}
	}
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) && !create {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect legacy intent directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return false, fmt.Errorf("legacy intent directory is not a private plain directory")
	}
	physical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false, fmt.Errorf("resolve legacy intent directory: %w", err)
	}
	if physical != dir {
		return false, fmt.Errorf("legacy intent directory is not canonical")
	}
	return true, nil
}

func legacyPathInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
