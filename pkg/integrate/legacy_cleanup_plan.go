// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/config"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

const (
	legacyCleanupPlanSchema   = 1
	legacyCleanupCapability   = "integrate-legacy-cleanup-plan-v1"
	legacyRetiredDisposition  = "retired"
	legacyCleanupMaxPlanBytes = 1 << 20
	legacyCleanupMaxArtifact  = 256 << 20
)

// ErrLegacyCleanupUnsupportedNative is returned by apply until the host has a
// native CE registration/claim fence. A caller cannot turn this into support
// by passing a flag or token.
var ErrLegacyCleanupUnsupportedNative = errors.New("legacy cleanup apply is unsupported without the native CE registration fence")

// LegacyCleanupPlan is a prospective, read-only authority. It never upgrades
// an old receipt or marker into permission to mutate a repository.
type LegacyCleanupPlan struct {
	SchemaVersion          int                     `json:"schema_version"`
	Capability             string                  `json:"capability"`
	Owner                  OwnerIdentity           `json:"owner"`
	RepositoryRoot         string                  `json:"repository_root"`
	PrimaryRoot            string                  `json:"primary_root"`
	CommonDir              string                  `json:"common_dir"`
	GitDir                 string                  `json:"git_dir"`
	TaskWorktree           string                  `json:"task_worktree"`
	TaskBranch             string                  `json:"task_branch"`
	TaskOID                string                  `json:"task_oid"`
	Remote                 string                  `json:"remote"`
	Endpoint               string                  `json:"endpoint"`
	TargetRef              string                  `json:"target_ref"`
	TargetOID              string                  `json:"target_oid"`
	LocalRemoteOID         string                  `json:"local_remote_oid"`
	TaskPatterns           []string                `json:"task_patterns"`
	ProtectedBranches      []string                `json:"protected_branches"`
	RetiredDisposition     string                  `json:"retired_disposition"`
	DoneArtifactPath       string                  `json:"done_artifact_path"`
	DoneArtifactSHA256     string                  `json:"done_artifact_sha256"`
	ReleasedArtifactPath   string                  `json:"released_artifact_path"`
	ReleasedArtifactSHA256 string                  `json:"released_artifact_sha256"`
	PlanDigest             string                  `json:"plan_digest"`
	RuntimeGuard           *LegacyRuntimeGuard     `json:"runtime_guard,omitempty"`
	WorktreeIdentity       *LegacyWorktreeIdentity `json:"worktree_identity,omitempty"`
}

// LegacyCleanupPlanOptions selects a target and the exact artifacts being
// retired. It intentionally has no mutation or force option.
type LegacyCleanupPlanOptions struct {
	RepoPath             string
	Target               string
	DoneArtifactPath     string
	ReleasedArtifactPath string
	Owner                OwnerIdentity
	RuntimeGuard         *LegacyRuntimeGuard
}

type legacyCleanupArtifact struct {
	SchemaVersion int           `json:"schema_version"`
	Status        string        `json:"status"`
	TaskSHA       string        `json:"task_sha"`
	TaskBranch    string        `json:"task_branch"`
	TaskWorktree  string        `json:"task_worktree"`
	CommonDir     string        `json:"common_dir"`
	Owner         OwnerIdentity `json:"owner"`
}

type legacyCleanupSource struct {
	repo                 gitRepo
	root, common, branch string
}

func resolveLegacyCleanupSource(ctx context.Context, exec *gitcmd.Executor, repoPath string) (legacyCleanupSource, error) {
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return legacyCleanupSource{}, fmt.Errorf("resolve repository root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return legacyCleanupSource{}, fmt.Errorf("resolve repository identity: %w", err)
	}
	g := newGitRepo(exec, root)
	actualRoot, err := g.toplevel(ctx)
	if err != nil {
		return legacyCleanupSource{}, fmt.Errorf("observe repository root: %w", err)
	}
	if filepath.Clean(actualRoot) != filepath.Clean(root) {
		return legacyCleanupSource{}, fmt.Errorf("repository root identity mismatch")
	}
	common, err := canonicalCommonFromRepo(ctx, g)
	if err != nil {
		return legacyCleanupSource{}, err
	}
	branch, err := g.currentBranch(ctx)
	if err != nil {
		return legacyCleanupSource{}, err
	}
	if err := gitcmd.SanitizeBranchName(branch); err != nil {
		return legacyCleanupSource{}, fmt.Errorf("invalid task branch: %w", err)
	}
	return legacyCleanupSource{repo: g, root: root, common: common, branch: branch}, nil
}

func observeLegacyCleanupPlan(ctx context.Context, exec *gitcmd.Executor, opts LegacyCleanupPlanOptions, source legacyCleanupSource) (*LegacyCleanupPlan, error) {
	g, root, common, branch := source.repo, source.root, source.common, source.branch
	if opts.Target == "" {
		return nil, fmt.Errorf("target is required")
	}
	remote, err := detectRemote(ctx, g, branch)
	if err != nil {
		return nil, err
	}
	targetName := targetBranchName(opts.Target, remote)
	if err := gitcmd.SanitizeBranchName(targetName); err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}
	endpoint, err := pushEndpoint(ctx, g, remote)
	if err != nil {
		return nil, fmt.Errorf("resolve target endpoint: %w", err)
	}
	taskOID, ok, err := g.revParse(ctx, "HEAD")
	if err != nil || !ok {
		if err != nil {
			return nil, fmt.Errorf("resolve task OID: %w", err)
		}
		return nil, fmt.Errorf("task OID is absent")
	}
	targetOID, exists, err := lsRemoteHead(ctx, g, endpoint, targetName)
	if err != nil {
		return nil, fmt.Errorf("resolve remote target OID: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("remote target %s does not exist", targetName)
	}
	localOID, ok, err := g.revParse(ctx, "refs/remotes/"+remote+"/"+targetName)
	if err != nil || !ok {
		return nil, fmt.Errorf("local remote-tracking target is absent")
	}
	ancestor, err := g.isAncestor(ctx, taskOID, targetOID)
	if err != nil {
		return nil, fmt.Errorf("prove remote target containment: %w", err)
	}
	if !ancestor {
		return nil, fmt.Errorf("task commit is not contained by the captured integration target")
	}
	worktree, err := findTaskWorktree(ctx, g, branch)
	if err != nil {
		return nil, fmt.Errorf("resolve task worktree: %w", err)
	}
	if worktree == "" {
		return nil, fmt.Errorf("task worktree is absent")
	}
	trees, err := g.listWorktrees(ctx)
	if err != nil || len(trees) == 0 || trees[0].Path == worktree {
		return nil, fmt.Errorf("task worktree must be distinct from a stable primary worktree")
	}
	primaryRoot := trees[0].Path
	primaryGitDir, err := newGitRepo(exec, primaryRoot).output(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("resolve primary git directory: %w", err)
	}
	patternDecl, err := config.LoadRepoRootTaskPattern(root)
	if err != nil {
		return nil, err
	}
	if len(patternDecl.Patterns) == 0 || !config.MatchesAnyTaskPattern(branch, patternDecl.Patterns) {
		return nil, fmt.Errorf("task branch does not match a declared cleanup pattern")
	}
	plan := &LegacyCleanupPlan{
		SchemaVersion: legacyCleanupPlanSchema, Capability: legacyCleanupCapability, Owner: opts.Owner,
		RepositoryRoot: primaryRoot, PrimaryRoot: primaryRoot, CommonDir: common, GitDir: primaryGitDir, TaskWorktree: worktree,
		TaskBranch: branch, TaskOID: taskOID, Remote: remote, Endpoint: endpoint,
		TargetRef: "refs/heads/" + targetName, TargetOID: targetOID, LocalRemoteOID: localOID,
		TaskPatterns: append([]string(nil), patternDecl.Patterns...), ProtectedBranches: append([]string(nil), repository.ProtectedBranches...),
		RetiredDisposition: legacyRetiredDisposition, DoneArtifactPath: cleanAbsolute(opts.DoneArtifactPath), DoneArtifactSHA256: "",
		ReleasedArtifactPath: cleanAbsolute(opts.ReleasedArtifactPath), ReleasedArtifactSHA256: "",
		RuntimeGuard: opts.RuntimeGuard,
	}
	return plan, nil
}

// BuildLegacyCleanupPlan observes a prospective cleanup without modifying Git resources.
func BuildLegacyCleanupPlan(ctx context.Context, exec *gitcmd.Executor, opts LegacyCleanupPlanOptions) (*LegacyCleanupPlan, error) {
	if exec == nil {
		return nil, fmt.Errorf("git executor is nil")
	}
	if err := validateOwner(opts.Owner); err != nil {
		return nil, err
	}
	source, err := resolveLegacyCleanupSource(ctx, exec, opts.RepoPath)
	if err != nil {
		return nil, err
	}
	plan, err := observeLegacyCleanupPlan(ctx, exec, opts, source)
	if err != nil {
		return nil, err
	}
	doneHash, err := hashArtifact(opts.DoneArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("done artifact: %w", err)
	}
	releasedHash, err := hashArtifact(opts.ReleasedArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("released artifact: %w", err)
	}
	plan.DoneArtifactSHA256, plan.ReleasedArtifactSHA256 = doneHash, releasedHash
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if legacyPathInside(plan.TaskWorktree, plan.DoneArtifactPath) || legacyPathInside(plan.TaskWorktree, plan.ReleasedArtifactPath) {
		return nil, fmt.Errorf("retired artifacts must be outside the task worktree")
	}
	identity, err := captureLegacyWorktreeIdentity(ctx, exec, plan)
	if err != nil {
		return nil, fmt.Errorf("capture legacy worktree identity: %w", err)
	}
	plan.WorktreeIdentity = identity
	if err := validateLegacyCleanupArtifact(opts.DoneArtifactPath, "DONE", plan); err != nil {
		return nil, fmt.Errorf("DONE artifact contract: %w", err)
	}
	if err := validateLegacyCleanupArtifact(opts.ReleasedArtifactPath, "RELEASED", plan); err != nil {
		return nil, fmt.Errorf("RELEASED artifact contract: %w", err)
	}
	plan.PlanDigest, err = legacyCleanupPlanDigest(plan)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (p *LegacyCleanupPlan) validateGitIdentity() error {
	if p.TaskBranch == "" || p.TaskOID == "" || p.TargetRef == "" || p.TargetOID == "" || p.LocalRemoteOID == "" || p.Endpoint == "" {
		return fmt.Errorf("legacy cleanup plan has incomplete identity")
	}
	if err := gitcmd.SanitizeBranchName(p.TaskBranch); err != nil {
		return fmt.Errorf("legacy cleanup plan has invalid task branch: %w", err)
	}
	targetBranch := strings.TrimPrefix(p.TargetRef, "refs/heads/")
	if !strings.HasPrefix(p.TargetRef, "refs/heads/") || targetBranch == p.TaskBranch {
		return fmt.Errorf("legacy cleanup plan has invalid target identity")
	}
	if err := gitcmd.SanitizeBranchName(targetBranch); err != nil {
		return fmt.Errorf("legacy cleanup plan has invalid target branch: %w", err)
	}
	if !repository.IsProtected(targetBranch) {
		return fmt.Errorf("legacy cleanup target is not a protected integration branch")
	}
	return nil
}

func validateLegacyCleanupEndpoint(raw string) error {
	if endpoint, err := url.Parse(raw); err == nil {
		hasPassword := false
		if endpoint.User != nil {
			_, hasPassword = endpoint.User.Password()
		}
		if endpoint.User != nil && (hasPassword || endpoint.Scheme != "ssh") {
			return fmt.Errorf("credential-bearing remote endpoint is not allowed")
		}
		for key := range endpoint.Query() {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "auth") {
				return fmt.Errorf("credential-bearing remote endpoint is not allowed")
			}
		}
	}
	return nil
}

// Validate checks the prospective plan schema and identities.
func (p *LegacyCleanupPlan) Validate() error {
	if p == nil {
		return fmt.Errorf("legacy cleanup plan is nil")
	}
	if p.SchemaVersion != legacyCleanupPlanSchema || p.Capability != legacyCleanupCapability {
		return fmt.Errorf("unsupported legacy cleanup plan")
	}
	if err := validateOwner(p.Owner); err != nil {
		return err
	}
	for name, value := range map[string]string{"repository_root": p.RepositoryRoot, "primary_root": p.PrimaryRoot, "common_dir": p.CommonDir, "git_dir": p.GitDir, "task_worktree": p.TaskWorktree, "done_artifact_path": p.DoneArtifactPath, "released_artifact_path": p.ReleasedArtifactPath} {
		if !filepath.IsAbs(value) || filepath.Clean(value) != value {
			return fmt.Errorf("%s must be a clean absolute path", name)
		}
	}
	if err := p.validateGitIdentity(); err != nil {
		return err
	}
	if p.DoneArtifactPath == p.ReleasedArtifactPath {
		return fmt.Errorf("DONE and RELEASED artifacts must be distinct")
	}
	if !validOID(p.TaskOID) || !validOID(p.TargetOID) || !validOID(p.LocalRemoteOID) {
		return fmt.Errorf("legacy cleanup plan has invalid Git OID")
	}
	if p.RetiredDisposition != legacyRetiredDisposition {
		return fmt.Errorf("legacy cleanup plan requires retired disposition")
	}
	if err := validateLegacyCleanupEndpoint(p.Endpoint); err != nil {
		return err
	}
	if !validSHA256(p.DoneArtifactSHA256) || !validSHA256(p.ReleasedArtifactSHA256) {
		return fmt.Errorf("legacy cleanup plan has invalid artifact digest")
	}
	if p.PlanDigest != "" && !validSHA256(p.PlanDigest) {
		return fmt.Errorf("legacy cleanup plan has invalid plan digest")
	}
	if p.RuntimeGuard != nil {
		if err := p.RuntimeGuard.ValidateForPlan(p); err != nil {
			return err
		}
	}
	if len(p.TaskPatterns) == 0 {
		return fmt.Errorf("legacy cleanup plan has no task pattern")
	}
	return nil
}

// WriteLegacyCleanupPlan creates a prospective plan without replacing a file.
func WriteLegacyCleanupPlan(path string, plan *LegacyCleanupPlan) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	digest, err := legacyCleanupPlanDigest(plan)
	if err != nil {
		return err
	}
	if plan.PlanDigest == "" || digest != plan.PlanDigest {
		return fmt.Errorf("legacy cleanup plan digest mismatch")
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("plan path must be a clean absolute path")
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if len(data) > legacyCleanupMaxPlanBytes {
		return fmt.Errorf("legacy cleanup plan exceeds size limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- path is caller-selected prospective output, constrained to a clean absolute path and O_EXCL.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create prospective plan: %w", err)
	}
	if _, err = f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// ReadLegacyCleanupPlan reads and verifies one immutable prospective plan.
func ReadLegacyCleanupPlan(path string) (*LegacyCleanupPlan, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("plan path must be a clean absolute path")
	}
	data, err := readBoundedRegularFile(path, legacyCleanupMaxPlanBytes, "legacy cleanup plan")
	if err != nil {
		return nil, err
	}
	var plan LegacyCleanupPlan
	if err := jsonUnmarshalStrict(data, &plan); err != nil {
		return nil, err
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if plan.WorktreeIdentity == nil {
		return nil, fmt.Errorf("legacy cleanup plan lacks physical worktree identity")
	}
	digest, err := legacyCleanupPlanDigest(&plan)
	if err != nil {
		return nil, err
	}
	if digest != plan.PlanDigest {
		return nil, fmt.Errorf("legacy cleanup plan digest mismatch")
	}
	return &plan, nil
}

func legacyCleanupPlanDigest(plan *LegacyCleanupPlan) (string, error) {
	copyPlan := *plan
	copyPlan.PlanDigest = ""
	data, err := json.Marshal(&copyPlan)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func hashArtifact(path string) (string, error) {
	path = cleanAbsolute(path)
	if path == "" {
		return "", fmt.Errorf("artifact path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("artifact is not a regular file")
	}
	if info.Size() > legacyCleanupMaxArtifact {
		return "", fmt.Errorf("artifact exceeds size limit")
	}
	// #nosec G304 -- caller-selected evidence path is checked for regular type and a bounded size before hashing.
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, legacyCleanupMaxArtifact+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validateLegacyCleanupArtifact(path, status string, plan *LegacyCleanupPlan) error {
	data, err := readBoundedRegularFile(cleanAbsolute(path), legacyCleanupMaxArtifact, "legacy cleanup artifact")
	if err != nil {
		return err
	}
	var artifact legacyCleanupArtifact
	if err := jsonUnmarshalStrict(data, &artifact); err != nil {
		return fmt.Errorf("structured artifact required: %w", err)
	}
	if artifact.SchemaVersion != 1 || artifact.Status != status || artifact.TaskSHA != plan.TaskOID || artifact.TaskBranch != plan.TaskBranch || artifact.TaskWorktree != plan.TaskWorktree || artifact.CommonDir != plan.CommonDir || !artifact.Owner.sameAs(plan.Owner) {
		return fmt.Errorf("artifact identity/status does not match plan")
	}
	return nil
}

func canonicalCommonFromRepo(ctx context.Context, g gitRepo) (string, error) {
	common, err := g.output(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve git common directory: %w", err)
	}
	return canonicalCommonDir(common)
}

func cleanAbsolute(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}

func validateOwner(owner OwnerIdentity) error {
	if err := validateOpaqueToken(owner.Actor, "owner actor"); err != nil {
		return err
	}
	if err := validateOpaqueToken(owner.Host, "owner host"); err != nil {
		return err
	}
	if owner.Kind != "agent" && owner.Kind != "human" {
		return fmt.Errorf("owner kind must be agent or human")
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validOID(value string) bool {
	if len(value) != 40 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
