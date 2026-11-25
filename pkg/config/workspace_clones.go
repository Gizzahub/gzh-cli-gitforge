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
// Two declarations clone into ignored directories and are therefore the
// allow-list reclaim uses:
//   - top-level `workspaces:` entries of type git with a url, at the entry's
//     path, or its key when the path is omitted;
//   - flat `repositories:` entries with a url, at the entry's path, or its
//     name when the path is omitted. That is where `gz-git workspace sync`
//     puts them. An entry with enabled: false is not listed. A name and a
//     path that are both empty is not listed: deriving the directory from
//     the URL belongs to the sync loader, and an allow-list must not guess it.
//
// Entries whose path is absolute, home-relative, or leaves repoRoot are not
// listed, nor are forge, config, or nested workspace entries. Two entries
// that resolve to the same relative path with different URLs are an error,
// so the allow-list cannot vouch for the wrong origin. A missing config
// yields an empty list. A repositories entry without a url is an error from
// the shared flat parser, and then nothing is listed.
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
	if err := RejectRemovedKeys(data, path); err != nil {
		return nil, err
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
		if err := admitDeclaredClone(clones, rel, ws.URL); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	flat, err := ParseFlatRepositories(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range flat.Repositories {
		repo := flat.Repositories[i]
		if repo.Enabled != nil && !*repo.Enabled {
			continue
		}
		rel := repo.Path
		if rel == "" {
			rel = repo.Name
		}
		if rel == "" {
			continue
		}
		if err := admitDeclaredClone(clones, rel, repo.URL); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	return clones, nil
}

// admitDeclaredClone records one clone whose path stays inside the repo.
// An absolute path, a home-relative path, or a path that leaves the repo is
// omitted. The same path with the same URL is one clone. The same path with
// a different URL is an error.
func admitDeclaredClone(clones map[string]string, rel, url string) error {
	if url == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "~/") {
		return nil
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	rel = filepath.ToSlash(rel)
	if prev, ok := clones[rel]; ok && prev != url {
		return fmt.Errorf("declared clones %s and %s share path %s", prev, url, rel)
	}
	clones[rel] = url
	return nil
}
