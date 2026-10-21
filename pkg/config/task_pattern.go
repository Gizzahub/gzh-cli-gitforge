// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
	"github.com/gizzahub/gzh-cli-gitforge/pkg/repository"
)

const factNoDeclaration = "no declaration"

// TaskPatternDecl is the repo-root declaration load result.
//
// Missing file → empty Patterns plus a reportable "no declaration" fact.
// That is not "everything is reclaimable".
type TaskPatternDecl struct {
	Patterns          []string
	IntegrationBranch BranchList
	Source            string
	Facts             []string
}

type rootBranchDecl struct {
	patterns    []string
	integration []string
}

// LoadRepoRootTaskPattern stats only <repoRoot>/.gz-git.{yaml,yml,json}.
// It does not call findConfigUpward and is not the 5-layer merger.
//
// A non-root .gz-git.yaml that declares taskPattern is ignored and its
// path is reported. A pattern that equals a literal protected name
// (main, master, develop, development) rejects the load. Overlap with
// built-in protect patterns hotfix/* and release/* is allowed.
func LoadRepoRootTaskPattern(repoRoot string) (TaskPatternDecl, error) {
	var decl TaskPatternDecl
	if strings.TrimSpace(repoRoot) == "" {
		return decl, fmt.Errorf("repo root is empty")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return decl, fmt.Errorf("resolve repo root: %w", err)
	}
	fsRoot, err := safefs.OpenRoot(root)
	if err != nil {
		return decl, fmt.Errorf("open repo root: %w", err)
	}
	defer func() { _ = fsRoot.Close() }()

	if err := reportNonRootTaskPattern(fsRoot, root, &decl); err != nil {
		return decl, err
	}

	path, err := statRepoRootConfig(fsRoot, root)
	if err != nil {
		return decl, err
	}
	if path == "" {
		decl.Facts = append(decl.Facts, factNoDeclaration)
		return decl, nil
	}

	rootDecl, err := readRootBranchDecl(fsRoot, filepath.Base(path), path)
	if err != nil {
		return decl, err
	}
	if err := rejectLiteralProtected(rootDecl.patterns, path); err != nil {
		return decl, err
	}

	decl.Patterns = append([]string(nil), rootDecl.patterns...)
	decl.IntegrationBranch = append(BranchList(nil), rootDecl.integration...)
	decl.Source = path
	if len(decl.Patterns) == 0 {
		decl.Facts = append(decl.Facts, factNoDeclaration)
	}
	return decl, nil
}

// MatchTaskPattern reports whether name falls inside a declared task-branch
// namespace. It delegates to pkg/repository, the single source of truth (see
// repository.MatchTaskPattern for why ownership sits there).
func MatchTaskPattern(name, pattern string) bool {
	return repository.MatchTaskPattern(name, pattern)
}

// MatchesAnyTaskPattern reports whether name matches any declared pattern.
func MatchesAnyTaskPattern(name string, patterns []string) bool {
	return repository.MatchesAnyTaskPattern(name, patterns)
}

func statRepoRootConfig(root *safefs.Root, rootPath string) (string, error) {
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		name := ProjectConfigFileName + ext
		st, err := root.Stat(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("stat %s: %w", filepath.Join(rootPath, name), err)
		}
		if st.IsDir() {
			continue
		}
		return filepath.Join(rootPath, name), nil
	}
	return "", nil
}

// readRootBranchDecl reads the declaration at path relative to root;
// fullPath names it in errors. A declaration that still carries a removed key
// fails the load and names the file the key must be deleted from.
func readRootBranchDecl(root *safefs.Root, path, fullPath string) (rootBranchDecl, error) {
	var decl rootBranchDecl
	data, err := root.ReadFile(path)
	if err != nil {
		return decl, fmt.Errorf("read %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if err := RejectRemovedKeys(data, fullPath); err != nil {
		return decl, err
	}
	if err := validateRepoRootDocument(data, ext == ".json"); err != nil {
		return decl, fmt.Errorf("parse %s: %w", path, err)
	}
	if ext == ".json" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return decl, fmt.Errorf("parse %s: %w", path, err)
		}
		if b, ok := raw["branch"]; ok {
			fields, decodeErr := decodeJSONBranchFields(b)
			if decodeErr != nil {
				err = decodeErr
			} else {
				decl.patterns = fields.patterns
				decl.integration = fields.integration
			}
			if err != nil {
				return decl, err
			}
		}
		return decl, nil
	}

	var file struct {
		Branch *BranchConfig `yaml:"branch"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return decl, fmt.Errorf("parse %s: %w", path, err)
	}
	if file.Branch != nil {
		decl.patterns = append([]string(nil), file.Branch.TaskPattern...)
		decl.integration = append([]string(nil), file.Branch.IntegrationBranch...)
	}
	return decl, nil
}

type jsonBranchFields struct {
	patterns, integration []string
}

func decodeJSONBranchFields(raw json.RawMessage) (jsonBranchFields, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return jsonBranchFields{}, err
	}
	var patternsValue, integrationValue any
	if field, ok := m["taskPattern"]; ok {
		if err := json.Unmarshal(field, &patternsValue); err != nil {
			return jsonBranchFields{}, err
		}
	}
	if field, ok := m["integrationBranch"]; ok {
		if err := json.Unmarshal(field, &integrationValue); err != nil {
			return jsonBranchFields{}, err
		}
	}
	return jsonBranchFields{patterns: coerceStringList(patternsValue), integration: coerceStringList(integrationValue)}, nil
}

// validateRepoRootDocument rejects the document shapes the authority-bearing
// repository root declaration must not take: several YAML documents and a
// duplicated branch key, which a compatibility decoder would silently resolve.
func validateRepoRootDocument(data []byte, isJSON bool) error {
	if isJSON {
		_, counts, err := parseJSONObjectCounts(data)
		if err != nil {
			return err
		}
		if counts["branch"] > 1 {
			return fmt.Errorf("duplicate JSON key %q", "branch")
		}
		return nil
	}
	if err := RejectMultiDocumentYAML(data); err != nil {
		return err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	if len(root.Content) == 1 && root.Content[0].Kind == yaml.MappingNode && mappingKeyCount(root.Content[0], "branch") > 1 {
		return fmt.Errorf("duplicate YAML key %q", "branch")
	}
	return nil
}

func coerceStringList(v any) []string {
	switch t := v.(type) {
	case string:
		t = strings.TrimSpace(t)
		if t == "" {
			return nil
		}
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func rejectLiteralProtected(patterns []string, path string) error {
	literals := literalProtectedNames()
	for _, pattern := range patterns {
		if i := strings.IndexByte(pattern, '*'); i == 0 {
			return fmt.Errorf("%s: taskPattern %q matches every name", path, pattern)
		}
		for _, lit := range literals {
			if pattern == lit {
				return fmt.Errorf("%s: taskPattern %q equals protected name %q", path, pattern, lit)
			}
		}
	}
	return nil
}

func literalProtectedNames() []string {
	var out []string
	for _, p := range repository.ProtectedBranches {
		if p != "" && !strings.Contains(p, "*") {
			out = append(out, p)
		}
	}
	return out
}

func reportNonRootTaskPattern(root *safefs.Root, rootPath string, decl *TaskPatternDecl) error {
	return reportNonRootTaskPatternAt(root, rootPath, ".", decl)
}

func reportNonRootTaskPatternAt(root *safefs.Root, rootPath, rel string, decl *TaskPatternDecl) error {
	entries, err := root.ReadDir(rel)
	if err != nil {
		return fmt.Errorf("scan %s: %w", filepath.Join(rootPath, rel), err)
	}

	for _, entry := range entries {
		name := entry.Name()
		entryRel := filepath.Join(rel, name)
		entryPath := filepath.Join(rootPath, entryRel)
		if entry.IsDir() {
			if name == ".git" || name == "vendor" || name == "node_modules" {
				continue
			}
			if err := reportNonRootTaskPatternAt(root, rootPath, entryRel, decl); err != nil {
				return err
			}
			continue
		}

		if name != ProjectConfigFileName+".yaml" &&
			name != ProjectConfigFileName+".yml" &&
			name != ProjectConfigFileName+".json" {
			continue
		}
		if filepath.Dir(entryPath) == rootPath {
			continue
		}
		if nested, err := readRootBranchDecl(root, entryRel, entryPath); err == nil && len(nested.patterns) > 0 {
			decl.Facts = append(decl.Facts, "ignored non-root taskPattern: "+entryPath)
		}
	}

	return nil
}
