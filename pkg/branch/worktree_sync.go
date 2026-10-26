// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package branch

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// WorktreeSyncConfig separates remote tracking from local parent propagation.
// A higher-precedence configuration replaces this entire object, including rules.
type WorktreeSyncConfig struct {
	Upstream string             `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Rules    []WorktreeSyncRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// WorktreeSyncRule declares a local parent and an explicit propagation policy.
type WorktreeSyncRule struct {
	Match    string `yaml:"match" json:"match"`
	Parent   string `yaml:"parent" json:"parent"`
	Strategy string `yaml:"strategy" json:"strategy"`
}

// ValidateWorktreeSyncConfig checks syntax; graph validation needs actual branches.
func ValidateWorktreeSyncConfig(cfg *WorktreeSyncConfig) error {
	if cfg == nil {
		return nil
	}
	if cfg.Upstream != "" && cfg.Upstream != "ff-only" {
		return fmt.Errorf("worktreeSync.upstream must be ff-only")
	}
	seen := make(map[string]bool)
	for _, rule := range cfg.Rules {
		if rule.Match == "" || strings.HasPrefix(rule.Match, "-") {
			return fmt.Errorf("worktreeSync rule requires a branch match")
		}
		if _, err := path.Match(rule.Match, ""); err != nil {
			return fmt.Errorf("invalid worktreeSync match %q: %w", rule.Match, err)
		}
		if seen[rule.Match] {
			return fmt.Errorf("duplicate worktreeSync match %q", rule.Match)
		}
		seen[rule.Match] = true
		if err := gitcmd.SanitizeBranchName(rule.Parent); err != nil {
			return fmt.Errorf("invalid worktreeSync parent: %w", err)
		}
		if rule.Strategy != "ff-only" && rule.Strategy != "rebase" {
			return fmt.Errorf("worktreeSync strategy must be ff-only or rebase")
		}
	}
	return nil
}

// WorktreeSyncOptions controls one repository's registered worktrees.
type WorktreeSyncOptions struct {
	Config  *WorktreeSyncConfig
	Include string // path.Match glob on branch names; empty selects all
	Exclude string // excluded ancestors block their dependents
	DryRun  bool   // no fetch; plan against existing remote-tracking refs
}

// WorktreeSyncStep pins both inputs so execution never follows a moving ref.
type WorktreeSyncStep struct {
	Source   string `json:"source"`
	Strategy string `json:"strategy"`
	Action   string `json:"action"`
	From     string `json:"from"`
	To       string `json:"to"`
	Status   string `json:"status"`
}

// WorktreeSyncResult is one selected worktree, including implicit ancestors.
type WorktreeSyncResult struct {
	Path     string             `json:"path"`
	Branch   string             `json:"branch"`
	Parent   string             `json:"parent,omitempty"`
	Implicit bool               `json:"implicit"`
	Status   string             `json:"status"`
	Message  string             `json:"message,omitempty"`
	Steps    []WorktreeSyncStep `json:"steps"`
}

// WorktreeSyncPlan can be inspected before execution. Its internal snapshots
// cannot be constructed by unmarshalling JSON; create it with PlanWorktreeSync.
type WorktreeSyncPlan struct {
	Repository string                `json:"repository"`
	DryRun     bool                  `json:"dry_run"`
	Fetched    bool                  `json:"fetched"`
	Results    []*WorktreeSyncResult `json:"results"`
	nodes      []*worktreeSyncNode
	git        *gitcmd.Executor
}

type worktreeSyncNode struct {
	result    *WorktreeSyncResult
	worktree  *Worktree
	parent    *worktreeSyncNode
	rule      *WorktreeSyncRule
	tip       string
	final     string
	upstream  string
	remote    string
	remoteRef string
	excluded  bool
	selected  bool
	children  int
}

// PlanWorktreeSync fetches once per required remote, then builds a complete plan.
// DryRun suppresses fetching as well as branch/worktree changes.
func PlanWorktreeSync(ctx context.Context, repoPath string, opts WorktreeSyncOptions) (*WorktreeSyncPlan, error) {
	if err := ValidateWorktreeSyncConfig(opts.Config); err != nil {
		return nil, err
	}
	for _, pattern := range []string{opts.Include, opts.Exclude} {
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid branch filter: %w", err)
		}
	}
	if err := gitcmd.SanitizePath(repoPath); err != nil {
		return nil, err
	}
	p := &WorktreeSyncPlan{Repository: repoPath, DryRun: opts.DryRun, Results: []*WorktreeSyncResult{}, git: gitcmd.NewExecutor(gitcmd.WithEnv([]string{"GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true"}))}
	if err := p.discover(ctx, opts); err != nil {
		return nil, err
	}
	p.fetch(ctx)
	for _, node := range p.nodes {
		p.planNode(ctx, node)
	}
	// V1 cannot propagate rewritten parent histories safely. Refuse the
	// parent before applying any part of its plan, then block descendants.
	for _, node := range p.nodes {
		for _, step := range node.result.Steps {
			if step.Action == "rebase" && node.children > 0 {
				blockSyncNode(node, "multi-level rebase is not supported: parent history would be rewritten")
			}
		}
		if node.parent != nil && node.parent.result.Status == "blocked" {
			blockSyncNode(node, "parent "+node.parent.result.Branch+" is blocked")
		}
	}
	return p, nil
}

func (p *WorktreeSyncPlan) discover(ctx context.Context, opts WorktreeSyncOptions) error {
	output, err := p.output(ctx, p.Repository, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return err
	}
	byBranch := make(map[string]*worktreeSyncNode)
	var nodes []*worktreeSyncNode
	for _, record := range strings.Split(output, "\x00\x00") {
		if record == "" {
			continue
		}
		wt := parseSyncWorktree(record)
		if wt.IsBare {
			continue
		}
		node := &worktreeSyncNode{
			worktree: wt, tip: wt.Ref, final: wt.Ref,
			result: &WorktreeSyncResult{Path: wt.Path, Branch: wt.Branch, Status: "planned", Steps: []WorktreeSyncStep{}},
		}
		node.excluded = syncMatches(opts.Exclude, wt.Branch)
		node.selected = !node.excluded && (opts.Include == "" || syncMatches(opts.Include, wt.Branch))
		if wt.Branch != "" {
			if byBranch[wt.Branch] != nil {
				return fmt.Errorf("branch %s is checked out more than once", wt.Branch)
			}
			byBranch[wt.Branch] = node
			if err := gitcmd.SanitizeBranchName(wt.Branch); err != nil {
				return err
			}
			if node.rule, err = syncRule(opts.Config, wt.Branch); err != nil {
				return err
			}
		}
		nodes = append(nodes, node)
	}
	for _, node := range nodes {
		if node.rule != nil {
			node.result.Parent = node.rule.Parent
			node.parent = byBranch[node.rule.Parent]
			if node.parent != nil {
				node.parent.children++
			}
		}
	}
	ordered, err := orderSyncNodes(nodes)
	if err != nil {
		return err
	}
	p.selectNodes(ctx, ordered)
	return nil
}

func parseSyncWorktree(record string) *Worktree {
	wt := &Worktree{}
	for _, field := range strings.Split(record, "\x00") {
		key, value, _ := strings.Cut(field, " ")
		switch key {
		case "worktree":
			wt.Path = value
		case "branch":
			wt.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "HEAD":
			wt.Ref = value
		case "bare":
			wt.IsBare = true
		case "detached":
			wt.IsDetached = true
		case "locked":
			wt.IsLocked = true
		case "prunable":
			wt.IsPrunable = true
		}
	}
	return wt
}

func (p *WorktreeSyncPlan) selectNodes(ctx context.Context, ordered []*worktreeSyncNode) {
	// Selecting a child includes its non-excluded ancestors explicitly.
	for i := len(ordered) - 1; i >= 0; i-- {
		node := ordered[i]
		if node.selected && node.parent != nil && !node.parent.excluded && !node.parent.selected {
			node.parent.selected = true
			node.parent.result.Implicit = true
		}
	}
	for _, node := range ordered {
		if !node.selected {
			continue
		}
		p.nodes = append(p.nodes, node)
		p.Results = append(p.Results, node.result)
		if node.rule != nil && (node.parent == nil || node.parent.excluded) {
			blockSyncNode(node, "parent "+node.rule.Parent+" has no included worktree")
		}
		if node.result.Status != "blocked" {
			p.inspectNode(ctx, node)
		}
	}
}

func syncMatches(pattern, name string) bool {
	matched, err := path.Match(pattern, name)
	return pattern != "" && err == nil && matched
}

//nolint:nilnil // No matching parent rule is a valid upstream-only configuration.
func syncRule(cfg *WorktreeSyncConfig, name string) (*WorktreeSyncRule, error) {
	if cfg == nil {
		return nil, nil
	}
	for i := range cfg.Rules {
		if cfg.Rules[i].Match == name {
			return &cfg.Rules[i], nil
		}
	}
	var match *WorktreeSyncRule
	for i := range cfg.Rules {
		if syncMatches(cfg.Rules[i].Match, name) {
			if match != nil {
				return nil, fmt.Errorf("ambiguous worktreeSync rules for %s", name)
			}
			match = &cfg.Rules[i]
		}
	}
	return match, nil
}

func orderSyncNodes(nodes []*worktreeSyncNode) ([]*worktreeSyncNode, error) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].result.Path < nodes[j].result.Path })
	state := make(map[*worktreeSyncNode]int)
	var ordered []*worktreeSyncNode
	var visit func(*worktreeSyncNode) error
	visit = func(node *worktreeSyncNode) error {
		if state[node] == 1 {
			return fmt.Errorf("cycle in worktreeSync parents at %s", node.result.Branch)
		}
		if state[node] == 2 {
			return nil
		}
		state[node] = 1
		if node.parent != nil {
			if err := visit(node.parent); err != nil {
				return err
			}
		}
		state[node] = 2
		ordered = append(ordered, node)
		return nil
	}
	for _, node := range nodes {
		if err := visit(node); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func blockSyncNode(node *worktreeSyncNode, message string) {
	if node.result.Status != "blocked" {
		node.result.Status = "blocked"
		node.result.Message = message
	}
}

func (p *WorktreeSyncPlan) output(ctx context.Context, dir string, args ...string) (string, error) {
	result, err := runGit(ctx, p.git, dir, args...)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}
