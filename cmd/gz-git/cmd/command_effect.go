// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// A command's effect is a fact about what it can change, not a statement of
// who may run it. The CLI cannot tell a person from an agent, so the policy
// that maps effects to "allowed" lives with the caller (the agent hook reads
// the declarations from `gz-git help --all --format json`). Declaring the
// policy here too would give it two owners that drift apart.
//
// A declaration is the upper bound across all flags: a command that mutates
// only when a flag is set is still declared mutating. Read-only means the
// command changes no repository, remote, forge, config, credential or run
// state. Printing, writing only a file the caller names (for example
// --output), temporary directories that are always removed, and gz-git's own
// empty config directories created on first use stay read-only. A fetch
// updates remote-tracking refs and is declared (tracking-refs), so a consumer
// allows it deliberately rather than by omission.
//
// Git keeps a branch's upstream in .git/config (branch.<name>.*), so a command
// that can create a tracking branch or delete a local branch in an existing
// repository declares config as well as refs. The config of a repository the
// command itself creates is part of that new directory (filesystem).

const (
	effectReadOnly   = "read-only"
	effectMutating   = "mutating"
	effectUndeclared = "undeclared"
)

// Mutation targets. Consumers match these strings, so they are a contract:
// add new ones, never rename.
const (
	mutatesWorktree          = "worktree"           // working-tree files or index of an existing repository
	mutatesRefs              = "refs"               // local branches, tags, HEAD, stash, commits
	mutatesTrackingRefs      = "tracking-refs"      // remote-tracking refs only
	mutatesRemote            = "remote"             // pushes to or deletes on a git remote
	mutatesForge             = "forge"              // objects created or changed through a forge API
	mutatesFilesystem        = "filesystem"         // directories created or removed (clones, git worktrees)
	mutatesConfig            = "config"             // gz-git config, profiles, or git config
	mutatesCredentials       = "credentials"        // OS keychain tokens
	mutatesRunState          = "run-state"          // task run records
	mutatesReadinessContract = "readiness-contract" // target-owned readiness contract
	mutatesArbitrary         = "arbitrary"          // a user-supplied command runs
)

var knownMutationTargets = map[string]bool{
	mutatesWorktree: true, mutatesRefs: true, mutatesTrackingRefs: true, mutatesRemote: true,
	mutatesForge: true, mutatesFilesystem: true, mutatesConfig: true, mutatesCredentials: true,
	mutatesRunState: true, mutatesReadinessContract: true, mutatesArbitrary: true,
}

// commandEffects declares every runnable command, keyed by its path below
// the root ("" is the root itself). TestEveryRunnableCommandDeclaresEffect
// fails when a runnable command is missing here or an entry names no command.
var commandEffects = map[string][]string{
	"":                      nil,
	"help":                  nil,
	"gen-docs":              {mutatesFilesystem},
	"version":               nil,
	"capability":            nil,
	"schema":                nil,
	"completion bash":       nil,
	"completion fish":       nil,
	"completion powershell": nil,
	"completion zsh":        nil,

	"branch list": nil,
	"branch name": nil,
	"clean":       {mutatesWorktree},
	"clone":       {mutatesFilesystem, mutatesWorktree, mutatesRefs, mutatesTrackingRefs, mutatesArbitrary},
	"commit":      {mutatesWorktree, mutatesRefs},
	"diff":        nil,
	"doctor":      nil,
	"exec":        {mutatesArbitrary},
	"fetch":       {mutatesRefs, mutatesTrackingRefs},
	"info":        nil,
	"observe":     {mutatesArbitrary},
	"pull":        {mutatesWorktree, mutatesRefs, mutatesTrackingRefs},
	"push":        {mutatesTrackingRefs, mutatesRemote, mutatesConfig},
	"status":      {mutatesTrackingRefs},
	"switch":      {mutatesWorktree, mutatesRefs, mutatesConfig},
	"sync":        {mutatesFilesystem, mutatesWorktree, mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesConfig, mutatesArbitrary},
	"update":      {mutatesWorktree, mutatesRefs, mutatesTrackingRefs},
	"watch":       nil,

	"cleanup branch": {mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesConfig},
	"cleanup wizard": {mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesConfig},

	"config hierarchy":      nil,
	"config show":           nil,
	"config init":           {mutatesConfig},
	"config recommended":    {mutatesConfig},
	"config profile list":   nil,
	"config profile show":   nil,
	"config profile create": {mutatesConfig},
	"config profile delete": {mutatesConfig},
	"config profile use":    {mutatesConfig},
	"config token get":      nil,
	"config token set":      {mutatesCredentials},
	"config token delete":   {mutatesCredentials},

	"conflict detect": nil,

	"forge config generate": {mutatesConfig},
	"forge from":            {mutatesFilesystem, mutatesWorktree, mutatesRefs, mutatesTrackingRefs},
	"forge setup":           {mutatesFilesystem, mutatesWorktree, mutatesRefs, mutatesTrackingRefs, mutatesConfig},
	"forge status":          {mutatesTrackingRefs, mutatesFilesystem},

	"handoff check": nil,
	"handoff start": {mutatesWorktree, mutatesRefs, mutatesTrackingRefs},
	"handoff end":   {mutatesWorktree, mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesConfig},

	"history blame":        nil,
	"history contributors": nil,
	"history file":         nil,
	"history stats":        nil,

	// Plans fetch the target unconditionally; tracking refs are their whole
	// footprint. Applies change the target-owned readiness contract. run also
	// fast-forwards a checked-out target and reclaims the task worktree and
	// branch.
	"integrate queue":                  {mutatesTrackingRefs},
	"integrate check":                  {mutatesTrackingRefs, mutatesArbitrary},
	"integrate run":                    {mutatesWorktree, mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesFilesystem, mutatesConfig, mutatesArbitrary},
	"integrate bootstrap plan":         {mutatesTrackingRefs},
	"integrate bootstrap apply":        {mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesReadinessContract},
	"integrate readiness update plan":  {mutatesTrackingRefs},
	"integrate readiness update apply": {mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesReadinessContract},

	"pr create": {mutatesForge},

	// Commands that list worktrees run `wt list`, and Worktrunk caches the
	// default branch in .git/config (worktrunk.default-branch).
	"run doctor":      {mutatesConfig},
	"run list":        {mutatesConfig, mutatesRunState},
	"run status":      {mutatesConfig, mutatesRunState},
	"run recover":     {mutatesRunState},
	"run import-ce":   {mutatesRunState},
	"run abort":       {mutatesRunState},
	"run start":       {mutatesFilesystem, mutatesRefs, mutatesConfig, mutatesRunState},
	"run discard":     {mutatesFilesystem, mutatesRefs, mutatesConfig, mutatesRunState},
	"run finish":      {mutatesWorktree, mutatesFilesystem, mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesConfig, mutatesRunState, mutatesArbitrary},
	"stash list":      nil,
	"stash apply":     {mutatesWorktree},
	"stash pop":       {mutatesWorktree, mutatesRefs},
	"stash save":      {mutatesWorktree, mutatesRefs},
	"tag list":        nil,
	"tag status":      nil,
	"tag auto":        {mutatesRefs},
	"tag create":      {mutatesRefs},
	"tag push":        {mutatesRemote},
	"worktree list":   nil,
	"worktree add":    {mutatesFilesystem, mutatesRefs, mutatesConfig},
	"worktree remove": {mutatesFilesystem},

	"workspace validate":        nil,
	"workspace status":          {mutatesTrackingRefs, mutatesFilesystem},
	"workspace add":             {mutatesConfig},
	"workspace init":            {mutatesConfig},
	"workspace generate-config": {mutatesConfig},
	"workspace sync":            {mutatesFilesystem, mutatesWorktree, mutatesRefs, mutatesTrackingRefs, mutatesRemote, mutatesConfig, mutatesArbitrary},
}

// declaredEffect reports the effect and mutation targets for cmd. ok is false
// when cmd has no declaration.
func declaredEffect(cmd *cobra.Command) (effect string, mutates []string, ok bool) {
	targets, ok := commandEffects[commandKey(cmd)]
	if !ok {
		return effectUndeclared, nil, false
	}
	if len(targets) == 0 {
		return effectReadOnly, nil, true
	}
	return effectMutating, targets, true
}

// commandKey is the command path without the root name.
func commandKey(cmd *cobra.Command) string {
	path := cmd.CommandPath()
	if i := strings.IndexByte(path, ' '); i >= 0 {
		return path[i+1:]
	}
	return ""
}

// effectSummary is the one-line form shown in help: "read-only",
// "mutating (refs, remote)", or "undeclared".
func effectSummary(cmd *cobra.Command) string {
	effect, mutates, _ := declaredEffect(cmd)
	if effect != effectMutating {
		return effect
	}
	return effect + " (" + strings.Join(mutates, ", ") + ")"
}
