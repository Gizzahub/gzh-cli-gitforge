// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"fmt"
)

// ErrWorkspaceTypeConflict marks a workspace that sets a forge source
// together with a type other than forge. Loaders return it so sync and
// status refuse the config instead of each picking a different meaning.
var ErrWorkspaceTypeConflict = errors.New("workspace type conflicts with its source")

// ClassifyWorkspace returns the effective type of a workspace entry. It is
// the one rule `workspace sync` and the status loader share, so they cannot
// classify the same entry differently (ISSUE-104). Callers decoding their own
// YAML structs pass the fields directly; *Workspace callers use Kind.
//
//   - A source makes the workspace a forge workspace. A source together with
//     an explicit type other than forge is refused (ErrWorkspaceTypeConflict).
//   - An explicit forge or config type is taken as written. An unknown type
//     is returned unchanged, so no planner matches it; Validator reports it.
//   - Otherwise (no type, or type git) a workspace without a URL and with
//     sync.recursive is a config workspace: it has no repository of its own
//     to clone, so its child config lists the repositories. Every other
//     entry is a git workspace.
//
// Whether a config workspace takes part in sync is a separate gate:
// SyncsConfigWorkspace.
func ClassifyWorkspace(declared WorkspaceType, hasSource bool, url string, recursive bool) (WorkspaceType, error) {
	if hasSource {
		if declared != "" && declared != WorkspaceTypeForge {
			return "", fmt.Errorf("%w: type %q with a source; a source requires type forge or no type", ErrWorkspaceTypeConflict, declared)
		}
		return WorkspaceTypeForge, nil
	}
	if declared != "" && declared != WorkspaceTypeGit {
		return declared, nil
	}
	if url == "" && recursive {
		return WorkspaceTypeConfig, nil
	}
	return WorkspaceTypeGit, nil
}

// SyncsConfigWorkspace reports whether a workspace of the given effective
// type is a config workspace that sync loads: only with sync.recursive.
func SyncsConfigWorkspace(effective WorkspaceType, recursive bool) bool {
	return effective == WorkspaceTypeConfig && recursive
}

// WorkspacePathOrName returns the configured workspace path, or the
// workspace name when the path is omitted (compact config convention).
func WorkspacePathOrName(path, name string) string {
	if path == "" {
		return name
	}
	return path
}

// Kind classifies ws with ClassifyWorkspace.
func (ws *Workspace) Kind() (WorkspaceType, error) {
	return ClassifyWorkspace(ws.Type, ws.Source != nil, ws.URL, ws.recursive())
}

func (ws *Workspace) recursive() bool {
	return ws.Sync != nil && ws.Sync.Recursive
}
