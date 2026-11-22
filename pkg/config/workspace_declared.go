// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// DeclaredRepositories is the set of repositories a project config declares
// below a scan directory: the universe a `--select listed` bulk run draws
// from.
type DeclaredRepositories struct {
	// ConfigPath is the config file the set was read from, for messages.
	ConfigPath string

	// Paths are git workspaces, each exactly one repository, relative to the
	// scan directory and slash separated. "." is the scan directory itself,
	// declared by selfSync.enabled when the config lives there.
	Paths []string

	// Prefixes are forge and config workspaces: directories that hold any
	// number of repositories, relative to the scan directory. "." means the
	// scan directory lies inside such a workspace.
	Prefixes []string

	// DeclaresAny reports whether the config declares any repository at all
	// (a workspace or selfSync), even outside the scan directory. It tells
	// "declares nothing here" apart from "declares nothing".
	DeclaresAny bool
}

// Empty reports whether the config declares nothing below the scan directory.
func (d DeclaredRepositories) Empty() bool {
	return len(d.Paths) == 0 && len(d.Prefixes) == 0
}

// Declares reports whether rel (a normalized, scan-relative path) is a
// declared repository, or lies under a declared forge or config workspace.
func (d DeclaredRepositories) Declares(rel string) bool {
	for _, p := range d.Paths {
		if p == rel {
			return true
		}
	}
	for _, p := range d.Prefixes {
		if p == "." || rel == p || (len(rel) > len(p) && rel[:len(p)] == p && rel[len(p)] == '/') {
			return true
		}
	}
	return false
}

// FindDeclaredRepositories reads the nearest project config at or above
// scanDir and returns the repositories its workspaces declare below scanDir.
// found is false when there is no project config.
//
// Membership is the same rule workspace access uses (findWorkspaceAccess): a
// workspace path is resolved against the config directory, nested workspaces
// against their parent workspace. A forge or config workspace that contains
// scanDir declares "." (everything the scan finds); other entries outside
// scanDir are dropped, since a scan of scanDir can never reach them, and
// DeclaresAny still records that they exist. The parent config chain is not
// consulted: a parent supplies defaults, not repositories, matching how
// LoadConfigRecursive loads parents without their workspaces.
func FindDeclaredRepositories(scanDir string) (DeclaredRepositories, bool, error) {
	root, err := canonicalWorkspacePath(scanDir)
	if err != nil {
		return DeclaredRepositories{}, false, err
	}
	_, configPath, err := findConfigUpward(root, projectConfigNames())
	if err != nil {
		return DeclaredRepositories{}, false, err
	}
	if configPath == "" {
		return DeclaredRepositories{}, false, nil
	}
	canonicalConfig, err := canonicalWorkspacePath(configPath)
	if err != nil {
		return DeclaredRepositories{}, false, err
	}
	data, err := ReadConfigFile(canonicalConfig)
	if err != nil {
		return DeclaredRepositories{}, true, fmt.Errorf("read %s: %w", configPath, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DeclaredRepositories{}, true, fmt.Errorf("parse %s: %w", configPath, err)
	}

	declared := DeclaredRepositories{ConfigPath: canonicalConfig}
	configDir := filepath.Dir(canonicalConfig)
	if cfg.SelfSync != nil && cfg.SelfSync.Enabled {
		declared.DeclaresAny = true
		if configDir == root {
			declared.Paths = append(declared.Paths, ".")
		}
	}
	if err := collectDeclared(root, configDir, cfg.Workspaces, &declared); err != nil {
		return DeclaredRepositories{}, true, fmt.Errorf("%s: %w", configPath, err)
	}
	sort.Strings(declared.Paths)
	sort.Strings(declared.Prefixes)
	return declared, true, nil
}

func collectDeclared(root, base string, workspaces map[string]*Workspace, out *DeclaredRepositories) error {
	for name, ws := range workspaces {
		if ws == nil {
			continue
		}
		kind, err := ws.Kind()
		if err != nil {
			return fmt.Errorf("workspace %q: %w", name, err)
		}
		out.DeclaresAny = true
		path, err := canonicalDeclaredPath(resolveWorkspaceAccessPath(base, WorkspacePathOrName(ws.Path, name)))
		if err != nil {
			return err
		}
		switch {
		case pathContains(root, path):
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("workspace %q: %w", name, err)
			}
			rel = filepath.ToSlash(rel)
			if kind == WorkspaceTypeGit {
				out.Paths = append(out.Paths, rel)
			} else {
				out.Prefixes = append(out.Prefixes, rel)
			}
		case kind != WorkspaceTypeGit && pathContains(path, root):
			// Scanning inside a forge or config workspace: everything the scan
			// finds is under it.
			out.Prefixes = append(out.Prefixes, ".")
		}
		if err := collectDeclared(root, path, ws.Workspaces, out); err != nil {
			return err
		}
	}
	return nil
}

// canonicalDeclaredPath resolves symlinks like canonicalWorkspacePath, but
// also for a workspace that is not cloned yet: it resolves the deepest
// existing ancestor and re-appends the missing tail. Without that, an
// uncloned /tmp/ws/api stays /tmp/ws/api on macOS while the scan root is
// /private/tmp/ws, the workspace falls outside the scan and is silently left
// out of the listed universe instead of failing as not on disk.
func canonicalDeclaredPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path %s: %w", path, err)
	}
	abs = filepath.Clean(abs)
	var tail []string
	for dir := abs; ; dir = filepath.Dir(dir) {
		if resolved, evalErr := filepath.EvalSymlinks(dir); evalErr == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return filepath.Clean(resolved), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs, nil
		}
		tail = append(tail, filepath.Base(dir))
	}
}
