// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
)

// DeclaredWorkspaceClones lists the single-repository clones the repo-root
// .gz-git.yaml declares inside repoRoot, as cleaned slash-separated
// root-relative path -> declared clone URL.
//
// Only top-level `workspaces:` entries of type git with a url are listed, at
// the path `gz-git sync` clones them to: the entry's path, or its key when
// the path is omitted, resolved against the config directory. Entries whose
// path is absolute, home-relative, or leaves repoRoot are not listed, nor are
// forge, config, or nested entries, so a caller that treats this list as an
// allow-list stays conservative. A missing config yields an empty list.
func DeclaredWorkspaceClones(repoRoot string) (map[string]string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("repo root is empty")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	fsRoot, err := safefs.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open repo root: %w", err)
	}
	defer func() { _ = fsRoot.Close() }()

	path, err := statRepoRootConfig(fsRoot, root)
	if err != nil {
		return nil, err
	}
	clones := map[string]string{}
	if path == "" {
		return clones, nil
	}
	data, err := fsRoot.ReadFile(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg struct {
		Workspaces map[string]*Workspace `yaml:"workspaces"`
	}
	// JSON is a YAML subset, so one decoder reads every accepted extension.
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for name, ws := range cfg.Workspaces {
		if ws == nil || ws.URL == "" || ws.Type.Resolve(ws.Source != nil) != WorkspaceTypeGit {
			continue
		}
		rel := ws.Path
		if rel == "" {
			rel = name
		}
		if filepath.IsAbs(rel) || strings.HasPrefix(rel, "~/") {
			continue
		}
		rel = filepath.Clean(rel)
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		clones[filepath.ToSlash(rel)] = ws.URL
	}
	return clones, nil
}
