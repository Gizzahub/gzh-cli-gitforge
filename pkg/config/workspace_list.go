// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// DeclaredWorkspace is one top-level `workspaces:` entry of a config file, as
// `gz-git workspace sync` would read it.
type DeclaredWorkspace struct {
	// Name is the entry's key in the workspaces map.
	Name string `json:"name"`
	// Type is the effective type (ClassifyWorkspace), not only the written one.
	Type WorkspaceType `json:"type"`
	// Path is absolute: the entry's path, or its name when omitted, resolved
	// against the config file's directory.
	Path string `json:"path"`
	// URL is the url the entry declares, whatever its type; empty when none.
	URL string `json:"url,omitempty"`
	// DefaultBranch is the declared branch list (`branch: master` shorthand
	// or branch.defaultBranch); empty when the entry declares none.
	DefaultBranch []string `json:"defaultBranch,omitempty"`
	// Access is the effective access; an omitted access is read-write.
	Access WorkspaceAccess `json:"access"`
}

// ListDeclaredWorkspaces returns the top-level workspaces the config file at
// configPath declares, sorted by name.
//
// It reads only that file and rejects removed keys; url, path, branch and
// access are reported as the entry declares them, so a branch inherited from
// a parent config is not shown. It does not read parent or nested configs,
// auto-discover directories, or look at checkouts and remotes, so the list
// is the declaration, not the state on disk, and it includes entries sync
// would skip (for example a git entry without a URL).
//
// A file whose kind is not a workspace kind (`workspace validate`'s set:
// empty, workspace, workspaces, repositories, repository) is refused. Within
// those kinds a `workspaces:` key is listed even beside flat repositories,
// because sync acts on both. Without one, a repositories config (by kind or
// by a non-empty `repositories:` list) is refused, since an empty list would
// misreport it as declaring none; any other file yields an empty list.
func ListDeclaredWorkspaces(configPath string) ([]DeclaredWorkspace, error) {
	if strings.TrimSpace(configPath) == "" {
		return nil, fmt.Errorf("config path is empty")
	}
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve config path %s: %w", configPath, err)
	}
	raw, err := ReadConfigFile(absPath)
	if err != nil {
		return nil, err
	}
	if err := checkWorkspaceListShape(raw, absPath); err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", absPath, err)
	}

	result := make([]DeclaredWorkspace, 0, len(cfg.Workspaces))
	for name, ws := range cfg.Workspaces {
		if ws == nil {
			return nil, fmt.Errorf("workspace %q in %s is empty", name, absPath)
		}
		kind, err := ws.Kind()
		if err != nil {
			return nil, fmt.Errorf("workspace %q in %s: %w", name, absPath, err)
		}
		// ClassifyWorkspace passes an unknown type through; sync then plans
		// nothing for it, so listing it would claim a workspace sync ignores.
		if !kind.IsValid() {
			return nil, fmt.Errorf("workspace %q in %s has unknown type %q", name, absPath, kind)
		}
		if !ws.Access.IsValid() {
			return nil, fmt.Errorf("workspace %q in %s has invalid access %q", name, absPath, ws.Access)
		}
		path, err := resolvePath(filepath.Dir(absPath), WorkspacePathOrName(ws.Path, name))
		if err != nil {
			return nil, fmt.Errorf("workspace %q in %s: %w", name, absPath, err)
		}
		access := ws.Access
		if access == "" {
			access = WorkspaceAccessReadWrite
		}
		entry := DeclaredWorkspace{
			Name:   name,
			Type:   kind,
			Path:   filepath.Clean(path),
			URL:    ws.URL,
			Access: access,
		}
		if ws.Branch != nil && len(ws.Branch.DefaultBranch) > 0 {
			entry.DefaultBranch = slices.Clone([]string(ws.Branch.DefaultBranch))
		}
		result = append(result, entry)
	}
	slices.SortFunc(result, func(a, b DeclaredWorkspace) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

// checkWorkspaceListShape refuses a file whose workspace list would be
// misleadingly empty. The kinds accepted are the ones `workspace validate`
// treats as a workspace config.
func checkWorkspaceListShape(raw []byte, absPath string) error {
	var shape struct {
		Kind         string    `yaml:"kind"`
		Workspaces   yaml.Node `yaml:"workspaces"`
		Repositories yaml.Node `yaml:"repositories"`
	}
	if err := yaml.Unmarshal(raw, &shape); err != nil {
		return fmt.Errorf("failed to parse config %s: %w", absPath, err)
	}
	isRepositoriesKind := false
	switch shape.Kind {
	case "", string(KindWorkspace), "workspaces":
	case string(KindRepositories), "repository":
		isRepositoriesKind = true
	default:
		return fmt.Errorf("config %s has kind %q, which is not a workspace config", absPath, shape.Kind)
	}
	if !shape.Workspaces.IsZero() {
		return nil
	}
	if isRepositoriesKind {
		return fmt.Errorf("config %s is kind %q: it lists repositories, not workspaces", absPath, shape.Kind)
	}
	if shape.Repositories.Kind == yaml.SequenceNode && len(shape.Repositories.Content) > 0 {
		return fmt.Errorf("config %s has a repositories list and no workspaces key: it lists repositories, not workspaces", absPath)
	}
	return nil
}
