package runtask

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const worktrunkVersion = "0.74.0"

// Service carries one repository root and its collaborators for the run
// lifecycle verbs. It holds no per-run state: all durable state lives in the
// store under the git common dir.
type Service struct {
	root   string
	runner CommandRunner
	// engine integrates the task branch in-process; gz-git is the declared
	// integration-provider, so finish never shells out to another tool to do
	// what this binary already owns.
	engine integrateEngine
	now    func() time.Time
}

// NewService builds a Service for root, using runner for external commands
// and engine for the in-process integration boundary.
func NewService(root string, runner CommandRunner, engine integrateEngine) *Service {
	return &Service{root: root, runner: runner, engine: engine, now: time.Now}
}

func response(status, task, reason, next string, actions ...string) TaskResponse {
	return TaskResponse{SchemaVersion: 1, Status: status, Task: task, AllowedActions: actions, Reason: reason, NextAction: next}
}
func diagnostic(result CommandResult) CommandDiagnostics {
	return CommandDiagnostics{Command: result.Command, Executable: result.Executable, Args: result.Args, Cwd: result.WorkDir, ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr, Error: result.Error, NotStarted: result.NotStarted}
}

type identityError struct {
	result CommandResult
	err    error
}

func (e *identityError) Error() string { return e.err.Error() }
func (e *identityError) Unwrap() error { return e.err }

func appendIdentityDiagnostic(res *TaskResponse, err error) {
	var identityErr *identityError
	if errors.As(err, &identityErr) {
		res.Diagnostics = append(res.Diagnostics, diagnostic(identityErr.result))
	}
}

func identityNextAction(err error) string {
	var resolution *hostResolutionError
	if errors.As(err, &resolution) {
		return resolution.nextAction
	}
	return "resolve the device and actor identity for this machine"
}
func (s *Service) command(ctx context.Context, dir, name string, args ...string) (CommandResult, error) {
	return s.runner.Run(ctx, CommandRequest{Command: name, Args: args, WorkDir: dir})
}
func (s *Service) store(ctx context.Context) (stateStore, error) {
	r, err := s.command(ctx, s.root, "git", "rev-parse", "--git-common-dir")
	if err != nil {
		return stateStore{}, err
	}
	if r.ExitCode != 0 {
		return stateStore{}, fmt.Errorf("resolve git common dir: %s", strings.TrimSpace(r.Stderr))
	}
	path := strings.TrimSpace(r.Stdout)
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.root, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return stateStore{}, err
	}
	return newStateStore(path), nil
}

func (s *Service) config() (TaskRuntimeConfig, error) {
	var cfg TaskRuntimeConfig
	b, err := os.ReadFile(filepath.Join(s.root, TaskRuntimeConfigFile))
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", TaskRuntimeConfigFile, err)
	}
	var keys map[string]yaml.Node
	if err := yaml.Unmarshal(b, &keys); err != nil {
		return cfg, fmt.Errorf("decode %s: %w", TaskRuntimeConfigFile, err)
	}
	if _, legacy := keys["worktree-root"]; legacy {
		return cfg, fmt.Errorf("unsupported task runtime config key \"worktree-root\"; use host-keyed \"worktree-roots.<GIT_WORK_HOST>\"")
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("decode %s: %w", TaskRuntimeConfigFile, err)
	}
	if cfg.SchemaVersion < 0 || cfg.SchemaVersion > 2 {
		return cfg, fmt.Errorf("unsupported task runtime schema-version %d", cfg.SchemaVersion)
	}
	if cfg.SchemaVersion < 2 {
		if _, declared := keys["integration-network-policy"]; declared {
			return cfg, fmt.Errorf("integration-network-policy requires task runtime schema-version: 2")
		}
		cfg.IntegrationNetworkPolicy = IntegrationNetworkPolicyAllow
	} else {
		allowed := map[string]bool{
			"schema-version": true, "repository-id": true, "worktree-roots": true,
			"integration-provider": true, "integration-network-policy": true,
		}
		unknown := make([]string, 0)
		for key := range keys {
			if !allowed[key] {
				unknown = append(unknown, key)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return cfg, fmt.Errorf("unsupported task runtime config key %q for schema-version 2", unknown[0])
		}
		policy, declared := keys["integration-network-policy"]
		if !declared || policy.Kind != yaml.ScalarNode || policy.Tag == "!!null" || strings.TrimSpace(policy.Value) == "" {
			return cfg, fmt.Errorf("schema-version 2 requires integration-network-policy: allow or no-fetch")
		}
	}
	if cfg.RepositoryID == "" || len(cfg.WorktreeRoots) == 0 || cfg.IntegrationProvider == "" {
		return cfg, fmt.Errorf("repository-id, worktree-roots, and integration-provider are required")
	}
	if normalize(cfg.RepositoryID) != cfg.RepositoryID {
		return cfg, fmt.Errorf("repository-id %q must be one lowercase kebab-case path segment", cfg.RepositoryID)
	}
	if cfg.IntegrationProvider != integrationProviderGZGit {
		return cfg, fmt.Errorf("unsupported integration-provider %q", cfg.IntegrationProvider)
	}
	if cfg.IntegrationNetworkPolicy != IntegrationNetworkPolicyAllow && cfg.IntegrationNetworkPolicy != IntegrationNetworkPolicyNoFetch {
		return cfg, fmt.Errorf("unsupported integration-network-policy %q (valid: allow, no-fetch)", cfg.IntegrationNetworkPolicy)
	}
	return cfg, nil
}

func (s *Service) worktreeRoot(cfg TaskRuntimeConfig, host string) (string, error) {
	path := cfg.WorktreeRoots[host]
	if path == "" {
		return "", fmt.Errorf("worktree root is not declared for host %q", host)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("worktree root for host %q must be absolute", host)
	}
	root, err := canonicalizePotentialPath(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve repository path for worktree safety: %w", err)
	}
	path, err = canonicalizePotentialPath(path)
	if err != nil {
		return "", fmt.Errorf("resolve worktree root for host %q: %w", host, err)
	}
	if filepath.Dir(path) == path {
		return "", fmt.Errorf("worktree root for host %q cannot be a filesystem root", host)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for worktree safety: %w", err)
	}
	home, err = canonicalizePotentialPath(home)
	if err != nil {
		return "", fmt.Errorf("resolve user home for worktree safety: %w", err)
	}
	sameHome, err := sameExistingPath(path, home)
	if err != nil {
		return "", fmt.Errorf("compare worktree root with user home: %w", err)
	}
	if path == home || sameHome {
		return "", fmt.Errorf("worktree root for host %q cannot be the user home directory", host)
	}
	if filepath.Base(path) != cfg.RepositoryID {
		return "", fmt.Errorf("worktree root for host %q must end with repository-id %q", host, cfg.RepositoryID)
	}
	insideRepository, err := withinFilesystem(path, root)
	if err != nil {
		return "", fmt.Errorf("compare worktree root with repository: %w", err)
	}
	besideRepository, err := sameExistingPath(filepath.Dir(root), filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("compare worktree and repository parents: %w", err)
	}
	if within(path, root) || insideRepository || filepath.Dir(root) == filepath.Dir(path) || besideRepository {
		return "", fmt.Errorf("worktree root must be outside and not beside the repository")
	}
	return path, nil
}

func canonicalizePotentialPath(path string) (string, error) {
	current, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	missing := make([]string, 0)
	for {
		if _, statErr := os.Lstat(current); statErr == nil {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return "", resolveErr
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing ancestor for %q", path)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func sameExistingPath(left, right string) (bool, error) {
	leftInfo, leftErr := os.Stat(left)
	if errors.Is(leftErr, os.ErrNotExist) {
		return false, nil
	}
	if leftErr != nil {
		return false, leftErr
	}
	rightInfo, rightErr := os.Stat(right)
	if errors.Is(rightErr, os.ErrNotExist) {
		return false, nil
	}
	if rightErr != nil {
		return false, rightErr
	}
	return os.SameFile(leftInfo, rightInfo), nil
}

func withinFilesystem(path, root string) (bool, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	current := path
	for {
		info, statErr := os.Stat(current)
		if statErr == nil {
			for {
				if os.SameFile(info, rootInfo) {
					return true, nil
				}
				parent := filepath.Dir(current)
				if parent == current {
					return false, nil
				}
				current = parent
				info, statErr = os.Stat(current)
				if statErr != nil {
					return false, statErr
				}
			}
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return false, statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}

func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func expectedWorktreePath(root, branch string) string {
	return filepath.Join(root, strings.ReplaceAll(strings.TrimPrefix(branch, "dev/"), "/", "__"))
}

func sameOwner(existing, current TaskOwner) bool {
	return existing.Actor == current.Actor && existing.Host == current.Host &&
		(existing.Kind == "" || existing.Kind == current.Kind)
}

func (s *Service) owner(ctx context.Context) (TaskOwner, error) {
	return s.resolveOwner(ctx)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func normalize(value string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
}

func (s *Service) sourceBranch(ctx context.Context) (string, error) {
	// Read through the shared decoder rather than a local struct: this used to
	// decode into []string and require exactly one entry, so a hand-written
	// scalar and a two-entry list both fell through to origin/HEAD without
	// saying so -- a repository that had declared its target behaving like one
	// that had not.
	//
	// The fallback belongs to an ABSENT declaration and to nothing else. A file
	// that exists but cannot be read or parsed is a third state, and folding it
	// into absence reproduces that same silence one layer down: the repository
	// declared its target, the declaration is unreadable, and the caller is
	// told it declared nothing.
	declaration := filepath.Join(s.root, ".gz-git.yaml")
	switch b, readErr := os.ReadFile(declaration); { //nolint:gosec // the path is this repository's own declaration file
	case readErr == nil:
		doc, parseErr := parseDeclaration(b)
		if parseErr != nil {
			return "", fmt.Errorf("%s cannot be parsed, and a declaration that cannot be read is not one that declares nothing: %w", declaration, parseErr)
		}
		if declared := doc.Branch.IntegrationBranch.First(); declared != "" {
			return declared, nil
		}
	case !os.IsNotExist(readErr):
		return "", fmt.Errorf("%s exists but cannot be read: %w", declaration, readErr)
	}
	r, runErr := s.command(ctx, s.root, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if runErr == nil && r.ExitCode == 0 {
		parts := strings.Split(strings.TrimSpace(r.Stdout), "/")
		return parts[len(parts)-1], nil
	}
	return "", fmt.Errorf("source branch is not declared and origin/HEAD is unresolved")
}
