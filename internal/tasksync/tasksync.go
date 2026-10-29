// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

const (
	outputLimit = 1 << 20
	refPrefix   = "refs/gz-git/tasksync/"
)

// Outcome separates a known refusal or conflict from an uncertain effect.
type Outcome string

const (
	// OutcomePrepared describes a candidate whose pinned ref was created.
	OutcomePrepared Outcome = "prepared"
	// OutcomeRefused describes a known unsupported or unsafe input.
	OutcomeRefused Outcome = "refused"
	// OutcomeConflict describes a replay that produced merge conflicts.
	OutcomeConflict Outcome = "conflict"
	// OutcomeUncertain describes an operation whose effect cannot be known safely.
	OutcomeUncertain Outcome = "uncertain"
)

// Request pins every Git identity this local primitive may inspect.
type Request struct {
	CanonicalWorktree string
	TaskWorktree      string
	CommonDir         string
	TaskBranch        string
	ExpectedTaskHEAD  string
	SourceRef         string
	SourceOID         string
}

// PreparedCandidate is durable only through Handle's private local ref.  It
// is a candidate, not a synced task branch or an integration authorization.
type PreparedCandidate struct {
	Handle       string
	CandidateOID string
	SourceOID    string
	OriginalHEAD string
	CommonDir    string
}

// CandidateDescriptor is the durable, local-only identity of a prepared ref.
// Restore verifies it against a repository before a later process may release it.
type CandidateDescriptor struct {
	Handle, CandidateOID, SourceOID, OriginalHEAD, CommonDir string
}

// Descriptor returns the durable local descriptor for this prepared candidate.
func (candidate PreparedCandidate) Descriptor() CandidateDescriptor {
	return CandidateDescriptor(candidate)
}

// Result reports the observed local effect. OriginalUnchanged is true only
// after the task HEAD and branch were rechecked at the end of the operation.
type Result struct {
	Outcome             Outcome
	Reason              string
	Prepared            *PreparedCandidate
	PossibleCandidate   *CandidateDescriptor
	OriginalUnchanged   bool
	RetainedScratch     bool
	RetainedScratchPath string
}

// PrepareClean replays the pinned task head on the pinned source in an owned,
// detached scratch worktree. It never applies that replay to the original task
// worktree or branch and never contacts a remote.
func PrepareClean(ctx context.Context, request Request) Result {
	return prepareClean(ctx, request, randomHandle)
}

func prepareClean(ctx context.Context, request Request, newHandle func() (string, error)) Result {
	if err := validateRequest(request); err != nil {
		return refused(err)
	}
	exec, err := newExecutor()
	if err != nil {
		return refused(err)
	}
	state, err := inspect(ctx, exec, request)
	if err != nil {
		return refused(err)
	}
	ownedScratch, result := createScratch(ctx, exec, request.CanonicalWorktree, state.TaskHEAD)
	if result != nil {
		return *result
	}
	cleanup := func() error {
		return cleanupScratch(ctx, exec, request.CanonicalWorktree, ownedScratch.root, ownedScratch.path, ownedScratch.adminID)
	}
	if result := replayScratch(ctx, exec, request, ownedScratch.path, cleanup); result != nil {
		return *result
	}
	candidate, err := oid(ctx, exec, ownedScratch.path, "HEAD")
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return Result{Outcome: OutcomeUncertain, Reason: "candidate identity and scratch cleanup failed", RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
		}
		return refused(fmt.Errorf("resolve scratch candidate: %w", err))
	}
	if !originalUnchanged(ctx, exec, request) {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return Result{Outcome: OutcomeUncertain, Reason: "original drift and scratch cleanup failed", RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
		}
		return Result{Outcome: OutcomeRefused, Reason: "task or source changed during candidate preparation"}
	}
	handle, err := newHandle()
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return Result{Outcome: OutcomeUncertain, Reason: "candidate handle and scratch cleanup failed", RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
		}
		return refused(err)
	}
	ref := refPrefix + handle
	if err := safeCommonSurfaces(request.CommonDir, handle); err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return Result{Outcome: OutcomeUncertain, Reason: "candidate metadata validation cleanup failed", RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
		}
		return refused(err)
	}
	zero := strings.Repeat("0", len(candidate))
	update, updateErr := runResult(ctx, exec, request.CanonicalWorktree, "update-ref", "--no-deref", ref, candidate, zero)
	if updateErr != nil || update == nil || update.ExitCode < 0 {
		possible := &CandidateDescriptor{Handle: handle, CandidateOID: candidate, SourceOID: request.SourceOID, OriginalHEAD: request.ExpectedTaskHEAD, CommonDir: request.CommonDir}
		return Result{Outcome: OutcomeUncertain, Reason: "candidate ref creation outcome is uncertain", PossibleCandidate: possible, RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
	}
	if update.ExitCode != 0 {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return Result{Outcome: OutcomeUncertain, Reason: "candidate ref rejection cleanup failed", RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
		}
		return refused(fmt.Errorf("candidate ref creation was rejected"))
	}
	if !originalUnchanged(ctx, exec, request) {
		prepared := &PreparedCandidate{Handle: handle, CandidateOID: candidate, SourceOID: request.SourceOID, OriginalHEAD: request.ExpectedTaskHEAD, CommonDir: request.CommonDir}
		if cleanupErr := cleanup(); cleanupErr != nil {
			return Result{Outcome: OutcomeUncertain, Reason: "original drift after candidate ref creation and scratch cleanup failed", Prepared: prepared, RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
		}
		return Result{Outcome: OutcomeUncertain, Reason: "original drift after candidate ref creation", Prepared: prepared}
	}
	if err := cleanup(); err != nil {
		return Result{Outcome: OutcomeUncertain, Reason: "candidate retained but scratch cleanup failed", Prepared: &PreparedCandidate{Handle: handle, CandidateOID: candidate, SourceOID: request.SourceOID, OriginalHEAD: request.ExpectedTaskHEAD, CommonDir: request.CommonDir}, RetainedScratch: true, RetainedScratchPath: ownedScratch.path}
	}
	return Result{Outcome: OutcomePrepared, Prepared: &PreparedCandidate{Handle: handle, CandidateOID: candidate, SourceOID: request.SourceOID, OriginalHEAD: request.ExpectedTaskHEAD, CommonDir: request.CommonDir}, OriginalUnchanged: true}
}

type scratchWorktree struct {
	root, path, adminID string
}

func createScratch(ctx context.Context, exec *gitcmd.Executor, canonical, taskHead string) (*scratchWorktree, *Result) {
	root, err := os.MkdirTemp("", "gzh-task-sync-")
	if err != nil {
		return nil, &Result{Outcome: OutcomeUncertain, Reason: "create owned scratch root failed"}
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, &Result{Outcome: OutcomeUncertain, Reason: "resolve owned scratch root failed", RetainedScratch: true, RetainedScratchPath: root}
	}
	path := filepath.Join(physicalRoot, "worktree")
	added, err := runResult(ctx, exec, canonical, "worktree", "add", "--detach", "--", path, taskHead)
	if err != nil || added == nil || added.ExitCode < 0 {
		return nil, &Result{Outcome: OutcomeUncertain, Reason: "scratch worktree creation outcome is uncertain", RetainedScratch: true, RetainedScratchPath: path}
	}
	if added.ExitCode != 0 {
		if cleanupErr := cleanupScratch(ctx, exec, canonical, physicalRoot, path, ""); cleanupErr != nil {
			return nil, &Result{Outcome: OutcomeUncertain, Reason: "scratch worktree creation cleanup failed", RetainedScratch: true, RetainedScratchPath: path}
		}
		return nil, &Result{Outcome: OutcomeRefused, Reason: "create scratch worktree was rejected"}
	}
	adminID, err := output(ctx, exec, path, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return nil, &Result{Outcome: OutcomeUncertain, Reason: "scratch administrative identity unavailable", RetainedScratch: true, RetainedScratchPath: path}
	}
	return &scratchWorktree{root: physicalRoot, path: path, adminID: adminID}, nil
}

func replayScratch(ctx context.Context, exec *gitcmd.Executor, request Request, scratch string, cleanup func() error) *Result {
	// Rebase can create commits in this detached, owned worktree only. Hooks,
	// rerere, autostash, signing, replacement objects and ambient config are
	// disabled or rejected before reaching this point.
	res, err := runResult(ctx, exec, scratch,
		"-c", "core.hooksPath=/dev/null", "-c", "rebase.autoStash=false",
		"-c", "rerere.enabled=false", "-c", "commit.gpgsign=false",
		"-c", "core.attributesFile=/dev/null", "rebase", "--no-autostash",
		"--no-update-refs", "--no-autosquash", "--no-fork-point", "--no-verify",
		"--no-gpg-sign", request.SourceOID)
	if err != nil {
		return &Result{Outcome: OutcomeUncertain, Reason: "scratch replay execution outcome is uncertain", RetainedScratch: true, RetainedScratchPath: scratch}
	}
	if res.ExitCode == 0 {
		return nil
	}
	conflicted, checkErr := ownedRebaseConflict(ctx, exec, scratch)
	if checkErr != nil {
		return &Result{Outcome: OutcomeUncertain, Reason: "scratch replay failure cannot be classified", RetainedScratch: true, RetainedScratchPath: scratch}
	}
	if !conflicted {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return &Result{Outcome: OutcomeUncertain, Reason: "non-conflict scratch replay cleanup failed", RetainedScratch: true, RetainedScratchPath: scratch}
		}
		return &Result{Outcome: OutcomeRefused, Reason: "pinned replay failed without a merge conflict"}
	}
	if _, abortErr := run(ctx, exec, scratch, "rebase", "--abort"); abortErr != nil {
		return &Result{Outcome: OutcomeUncertain, Reason: "scratch rebase outcome cannot be restored", RetainedScratch: true, RetainedScratchPath: scratch}
	}
	if cleanupErr := cleanup(); cleanupErr != nil {
		return &Result{Outcome: OutcomeUncertain, Reason: "scratch conflict cleanup failed", RetainedScratch: true, RetainedScratchPath: scratch}
	}
	return &Result{Outcome: OutcomeConflict, Reason: "pinned replay conflicted", OriginalUnchanged: originalUnchanged(ctx, exec, request)}
}

// Release removes exactly this package's candidate ref only when it still
// names the supplied candidate. It never deletes a branch, remote ref, or a
// ref whose value changed after preparation.
func Release(ctx context.Context, canonicalWorktree string, candidate PreparedCandidate) error {
	_, err := Restore(ctx, canonicalWorktree, candidate.Descriptor())
	if err != nil {
		return err
	}
	exec, err := newExecutor()
	if err != nil {
		return err
	}
	_, err = run(ctx, exec, canonicalWorktree, "update-ref", "-d", "--no-deref", refPrefix+candidate.Handle, candidate.CandidateOID)
	return err
}

// Restore verifies a descriptor retained from a prior Prepared result without
// changing repository state. It is not an authorization mechanism: callers
// must not use PossibleCandidate from an uncertain result as owned evidence.
func Restore(ctx context.Context, canonicalWorktree string, descriptor CandidateDescriptor) (*PreparedCandidate, error) {
	if err := safeWorktreePath(canonicalWorktree); err != nil {
		return nil, err
	}
	if len(descriptor.Handle) != 32 || !hexOnly(descriptor.Handle) || !validOID(descriptor.CandidateOID) || !validOID(descriptor.SourceOID) || !validOID(descriptor.OriginalHEAD) || descriptor.CommonDir == "" {
		return nil, fmt.Errorf("invalid candidate descriptor")
	}
	exec, err := newExecutor()
	if err != nil {
		return nil, err
	}
	common, err := output(ctx, exec, canonicalWorktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || common != descriptor.CommonDir {
		return nil, fmt.Errorf("candidate is not bound to this repository")
	}
	if err := safeCommonSurfaces(common, descriptor.Handle); err != nil {
		return nil, err
	}
	actual, err := oid(ctx, exec, canonicalWorktree, refPrefix+descriptor.Handle)
	if err != nil || actual != descriptor.CandidateOID {
		return nil, fmt.Errorf("candidate ref no longer names descriptor object")
	}
	return &PreparedCandidate{Handle: descriptor.Handle, CandidateOID: descriptor.CandidateOID, SourceOID: descriptor.SourceOID, OriginalHEAD: descriptor.OriginalHEAD, CommonDir: descriptor.CommonDir}, nil
}

func refused(err error) Result { return Result{Outcome: OutcomeRefused, Reason: err.Error()} }

func validateRequest(r Request) error {
	for _, path := range []string{r.CanonicalWorktree, r.TaskWorktree, r.CommonDir} {
		if err := safeWorktreePath(path); err != nil {
			return err
		}
	}
	if r.CanonicalWorktree == r.TaskWorktree || !validOID(r.ExpectedTaskHEAD) || !validOID(r.SourceOID) {
		return fmt.Errorf("invalid pinned worktree or object identity")
	}
	for _, ref := range []string{r.TaskBranch, r.SourceRef} {
		if err := gitcmd.SanitizeBranchName(ref); err != nil {
			return fmt.Errorf("invalid ref: %w", err)
		}
	}
	return nil
}

func inspect(ctx context.Context, exec *gitcmd.Executor, r Request) (snapshot, error) {
	for _, dir := range []string{r.CanonicalWorktree, r.TaskWorktree} {
		common, err := output(ctx, exec, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil || common != r.CommonDir {
			return snapshot{}, fmt.Errorf("worktree common directory differs from request")
		}
		top, err := output(ctx, exec, dir, "rev-parse", "--show-toplevel")
		if err != nil || top != dir {
			return snapshot{}, fmt.Errorf("worktree root differs from request")
		}
	}
	if err := safeCommonSurfaces(r.CommonDir, ""); err != nil {
		return snapshot{}, err
	}
	gitDir, err := output(ctx, exec, r.CanonicalWorktree, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil || gitDir != r.CommonDir {
		return snapshot{}, fmt.Errorf("canonical worktree is not the primary control checkout")
	}
	branch, err := output(ctx, exec, r.TaskWorktree, "symbolic-ref", "--quiet", "HEAD")
	if err != nil || branch != "refs/heads/"+r.TaskBranch {
		return snapshot{}, fmt.Errorf("task worktree branch differs from request")
	}
	taskHead, err := oid(ctx, exec, r.TaskWorktree, "HEAD")
	if err != nil || taskHead != r.ExpectedTaskHEAD {
		return snapshot{}, fmt.Errorf("task HEAD differs from request")
	}
	branchHead, err := oid(ctx, exec, r.TaskWorktree, "refs/heads/"+r.TaskBranch)
	if err != nil || branchHead != taskHead {
		return snapshot{}, fmt.Errorf("task branch differs from checked-out HEAD")
	}
	if err := rejectUnsupported(ctx, exec, r); err != nil {
		return snapshot{}, err
	}
	source, err := oid(ctx, exec, r.CanonicalWorktree, r.SourceRef)
	if err != nil || source != r.SourceOID {
		return snapshot{}, fmt.Errorf("source ref differs from request")
	}
	if err := cleanStatus(ctx, exec, r); err != nil {
		return snapshot{}, err
	}
	base, err := output(ctx, exec, r.TaskWorktree, "merge-base", r.SourceOID, r.ExpectedTaskHEAD)
	if err != nil || !validOID(base) {
		return snapshot{}, fmt.Errorf("task and source do not share a merge base")
	}
	merges, err := output(ctx, exec, r.TaskWorktree, "rev-list", "--merges", base+".."+r.ExpectedTaskHEAD)
	if err != nil || merges != "" {
		return snapshot{}, fmt.Errorf("task history contains unsupported merge commits")
	}
	return snapshot{TaskHEAD: taskHead}, nil
}

type snapshot struct{ TaskHEAD string }

func originalUnchanged(ctx context.Context, exec *gitcmd.Executor, r Request) bool {
	state, err := inspect(ctx, exec, r)
	return err == nil && state.TaskHEAD == r.ExpectedTaskHEAD
}

func cleanStatus(ctx context.Context, exec *gitcmd.Executor, r Request) error {
	status, err := output(ctx, exec, r.TaskWorktree, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	ignored := []string{}
	for _, record := range strings.Split(status, "\x00") {
		if record == "" {
			continue
		}
		if strings.HasPrefix(record, "!! ") {
			ignored = append(ignored, strings.TrimPrefix(record, "!! "))
			continue
		}
		return fmt.Errorf("task worktree has tracked or untracked changes")
	}
	changed, err := output(ctx, exec, r.TaskWorktree, "diff", "--name-only", "-z", r.SourceOID, r.ExpectedTaskHEAD)
	if err != nil {
		return err
	}
	for _, ignoredPath := range ignored {
		for _, changedPath := range strings.Split(changed, "\x00") {
			if changedPath == ignoredPath || strings.HasPrefix(changedPath, strings.TrimSuffix(ignoredPath, "/")+"/") {
				return fmt.Errorf("ignored path collides with replay input")
			}
		}
	}
	return nil
}

func rejectUnsupported(ctx context.Context, exec *gitcmd.Executor, r Request) error {
	config, err := output(ctx, exec, r.TaskWorktree, "config", "--local", "--null", "--name-only", "--includes", "--list")
	if err != nil {
		return err
	}
	for _, key := range strings.Split(config, "\x00") {
		key = strings.ToLower(key)
		for _, prefix := range []string{"include.", "includeif.", "filter.", "submodule.", "rerere.", "rebase.", "notes.", "core.hookspath", "core.attributesfile", "core.sshcommand", "core.fsmonitor", "gpg.", "commit.gpgsign", "sequence.", "url.", "merge.", "diff.", "gc.", "maintenance.", "credential."} {
			if strings.HasPrefix(key, prefix) {
				return fmt.Errorf("unsupported local git configuration")
			}
		}
		if strings.HasSuffix(key, ".promisor") || strings.HasSuffix(key, ".partialclonefilter") || key == "extensions.partialclone" || key == "extensions.worktreeconfig" {
			return fmt.Errorf("unsupported partial clone configuration")
		}
	}
	for _, marker := range []string{"rebase-merge", "rebase-apply", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "sequencer", "BISECT_START", "index.lock", "locked", "info/grafts", "info/sparse-checkout", "config.worktree", "objects/info/alternates"} {
		location, err := output(ctx, exec, r.TaskWorktree, "rev-parse", "--path-format=absolute", "--git-path", marker)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(location); err == nil {
			return fmt.Errorf("unsupported git operation or metadata is present")
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect git metadata: %w", err)
		}
	}
	replaceRefs, err := output(ctx, exec, r.TaskWorktree, "for-each-ref", "--format=%(refname)", "refs/replace/")
	if err != nil || replaceRefs != "" {
		return fmt.Errorf("replace refs are unsupported")
	}
	return nil
}

func ownedRebaseConflict(ctx context.Context, exec *gitcmd.Executor, scratch string) (bool, error) {
	for _, marker := range []string{"rebase-merge", "rebase-apply"} {
		path, err := output(ctx, exec, scratch, "rev-parse", "--path-format=absolute", "--git-path", marker)
		if err != nil {
			return false, err
		}
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("rebase state cannot be verified")
		}
		unmerged, err := output(ctx, exec, scratch, "diff", "--name-only", "--diff-filter=U", "-z")
		if err != nil {
			return false, err
		}
		return unmerged != "", nil
	}
	return false, nil
}

func oid(ctx context.Context, exec *gitcmd.Executor, dir, ref string) (string, error) {
	value, err := output(ctx, exec, dir, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil || !validOID(value) {
		return "", fmt.Errorf("resolve commit identity")
	}
	return value, nil
}

func output(ctx context.Context, exec *gitcmd.Executor, dir string, args ...string) (string, error) {
	result, err := run(ctx, exec, dir, args...)
	if err != nil || result.ExitCode != 0 {
		return "", fmt.Errorf("git inspection failed")
	}
	return strings.TrimSuffix(result.Stdout, "\n"), nil
}

func run(ctx context.Context, exec *gitcmd.Executor, dir string, args ...string) (*gitcmd.Result, error) {
	result, err := runResult(ctx, exec, dir, args...)
	if err != nil || result.ExitCode != 0 {
		return result, fmt.Errorf("git command failed")
	}
	return result, nil
}

func runResult(ctx context.Context, exec *gitcmd.Executor, dir string, args ...string) (*gitcmd.Result, error) {
	base := []string{"--no-replace-objects", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "rerere.enabled=false"}
	result, overflow, err := exec.RunWithOutputLimitCleanEnv(ctx, dir, cleanEnv(), outputLimit, append(base, args...)...)
	if err != nil || overflow || result == nil || result.ExitCode < 0 || ctx.Err() != nil {
		return result, fmt.Errorf("git command unavailable or output exceeded limit")
	}
	return result, nil
}

// cleanupScratch removes only a worktree rooted in the private directory this
// invocation created.  A failed command alone is not evidence of absence:
// worktree add/remove can leave administrative state behind.
func cleanupScratch(ctx context.Context, exec *gitcmd.Executor, canonical, root, scratch, adminID string) error {
	if filepath.Dir(scratch) != root {
		return fmt.Errorf("scratch path is not owned by its root")
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("owned scratch root cannot be verified")
	}
	listed, err := scratchListed(ctx, exec, canonical, scratch)
	if err != nil {
		return err
	}
	if listed {
		if adminID == "" {
			return fmt.Errorf("scratch worktree identity cannot be verified")
		}
		info, err := os.Lstat(scratch)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("scratch worktree path cannot be verified")
		}
		actualAdmin, err := output(ctx, exec, scratch, "rev-parse", "--path-format=absolute", "--git-dir")
		if err != nil || actualAdmin != adminID {
			return fmt.Errorf("scratch worktree administrative identity changed")
		}
		status, err := output(ctx, exec, scratch, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--ignore-submodules=none")
		if err != nil || status != "" {
			return fmt.Errorf("scratch worktree is not clean for removal")
		}
		if _, err := run(ctx, exec, canonical, "worktree", "remove", "--", scratch); err != nil {
			return fmt.Errorf("remove owned scratch worktree: %w", err)
		}
		listed, err = scratchListed(ctx, exec, canonical, scratch)
		if err != nil || listed {
			return fmt.Errorf("scratch worktree administrative state remains")
		}
	}
	if _, err := os.Lstat(scratch); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("scratch path remains after cleanup")
	}
	if err := os.Remove(root); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove owned scratch root: %w", err)
	}
	return nil
}

func scratchListed(ctx context.Context, exec *gitcmd.Executor, canonical, scratch string) (bool, error) {
	worktrees, err := output(ctx, exec, canonical, "worktree", "list", "--porcelain")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(worktrees, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		if strings.TrimPrefix(line, "worktree ") == scratch {
			return true, nil
		}
	}
	return false, nil
}

func newExecutor() (*gitcmd.Executor, error) {
	// Resolve the caller's already-selected Git once before cleanEnv removes
	// PATH. The resolved executable is then absolute for every Git invocation.
	path, err := osexec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("resolve configured git executable: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("make git executable path absolute: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolve git executable symlink: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("configured git executable is not runnable")
	}
	return gitcmd.NewExecutor(gitcmd.WithGitBinary(path), gitcmd.WithTimeout(45*time.Second)), nil
}

func cleanEnv() []string {
	return []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_ATTR_NOSYSTEM=1", "GIT_OPTIONAL_LOCKS=0", "GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true", "GIT_PAGER=cat"}
}

func safeWorktreePath(path string) error {
	if err := gitcmd.SanitizePath(path); err != nil {
		return fmt.Errorf("unsafe worktree path: %w", err)
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("worktree path must be a clean absolute path")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect worktree path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("worktree path contains symlink")
		}
		if !info.IsDir() {
			return fmt.Errorf("worktree path component is not a directory")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func randomHandle() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate candidate handle: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	return hexOnly(value)
}

func hexOnly(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}
