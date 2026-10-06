// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// MakeOutcomeReport is the strictly parsed repo-root declaration. Nil
	// means the key is absent; consumers must not discover it through merged
	// configuration.
	MakeOutcomeReport *MakeOutcomeReport
	// MakeTimeout is the declared branch.makeTimeout, already parsed. Zero
	// means the key is absent and the consumer applies its built-in default;
	// a present-but-invalid value never gets here because the load fails.
	MakeTimeout time.Duration
	Source      string
	Facts       []string
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

	patterns, integration, makeTimeout, outcomeReport, err := readRootBranchDecl(fsRoot, filepath.Base(path))
	if err != nil {
		return decl, err
	}
	if err := rejectLiteralProtected(patterns, path); err != nil {
		return decl, err
	}

	budget, err := parseMakeTimeout(makeTimeout, path)
	if err != nil {
		return decl, err
	}

	decl.Patterns = append([]string(nil), patterns...)
	decl.IntegrationBranch = append(BranchList(nil), integration...)
	decl.MakeTimeout = budget
	decl.MakeOutcomeReport = outcomeReport
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

func readRootBranchDecl(root *safefs.Root, path string) (patterns, integration []string, makeTimeout string, outcomeReport *MakeOutcomeReport, err error) {
	data, err := root.ReadFile(path)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("read %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	parsedOutcome, err := ParseMakeOutcomeReportDocument(data, ext == ".json")
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ext == ".json" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, nil, "", nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if b, ok := raw["branch"]; ok {
			patterns, integration, makeTimeout, err = decodeJSONBranchFields(b)
			if err != nil {
				return nil, nil, "", nil, err
			}
		}
		return patterns, integration, makeTimeout, parsedOutcome, nil
	}

	var file struct {
		Branch *BranchConfig `yaml:"branch"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, "", nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if file.Branch != nil {
		patterns = append([]string(nil), file.Branch.TaskPattern...)
		integration = append([]string(nil), file.Branch.IntegrationBranch...)
		makeTimeout = file.Branch.MakeTimeout
	}
	return patterns, integration, makeTimeout, parsedOutcome, nil
}

func decodeJSONBranchFields(raw json.RawMessage) (patterns, integration []string, makeTimeout string, err error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, "", err
	}
	timeout, err := decodeJSONMakeTimeout(m["makeTimeout"])
	if err != nil {
		return nil, nil, "", err
	}
	return coerceStringList(m["taskPattern"]), coerceStringList(m["integrationBranch"]), timeout, nil
}

// decodeJSONMakeTimeout reads the declared branch.makeTimeout from a decoded
// JSON branch object. Absent means the default applies; anything present must
// be a string, so a numeric duration cannot slip past the parser that the
// YAML path would have rejected.
func decodeJSONMakeTimeout(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	default:
		return "", fmt.Errorf("branch.makeTimeout must be a string")
	}
}

// parseMakeTimeout turns the declared branch.makeTimeout string into the
// budget the integrate package consumes. Absent means zero — the caller
// applies its built-in default — while anything present must be a positive
// Go duration. A value that cannot be honored fails the load instead of
// silently falling back to the default, because a repository that declares a
// budget its gate cannot meet would otherwise run under the very ceiling the
// declaration was written to lift.
func parseMakeTimeout(raw, path string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	if strings.TrimSpace(raw) != raw {
		return 0, fmt.Errorf("%s: branch.makeTimeout %q must not have surrounding whitespace", path, raw)
	}
	budget, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: branch.makeTimeout %q: %w", path, raw, err)
	}
	if budget <= 0 {
		return 0, fmt.Errorf("%s: branch.makeTimeout %q must be positive", path, raw)
	}
	return budget, nil
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
		if patterns, _, _, _, err := readRootBranchDecl(root, entryRel); err == nil && len(patterns) > 0 {
			decl.Facts = append(decl.Facts, "ignored non-root taskPattern: "+entryPath)
		}
	}

	return nil
}
