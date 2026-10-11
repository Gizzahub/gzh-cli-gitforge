// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"bytes"
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
	// LockWait is the declared branch.lockWait, already parsed. LockWaitSet
	// distinguishes an absent key from an explicit unbounded value.
	LockWait    time.Duration
	LockWaitSet bool
	Source      string
	Facts       []string
}

type rootBranchDecl struct {
	patterns      []string
	integration   []string
	makeTimeout   string
	lockWait      string
	outcomeReport *MakeOutcomeReport
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

	rootDecl, err := readRootBranchDecl(fsRoot, filepath.Base(path))
	if err != nil {
		return decl, err
	}
	if err := rejectLiteralProtected(rootDecl.patterns, path); err != nil {
		return decl, err
	}

	budget, err := parseMakeTimeout(rootDecl.makeTimeout, path)
	if err != nil {
		return decl, err
	}
	lockBudget, lockSet, err := parseLockWait(rootDecl.lockWait, path)
	if err != nil {
		return decl, err
	}

	decl.Patterns = append([]string(nil), rootDecl.patterns...)
	decl.IntegrationBranch = append(BranchList(nil), rootDecl.integration...)
	decl.MakeTimeout = budget
	decl.LockWait = lockBudget
	decl.LockWaitSet = lockSet
	decl.MakeOutcomeReport = rootDecl.outcomeReport
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

func readRootBranchDecl(root *safefs.Root, path string) (rootBranchDecl, error) {
	var decl rootBranchDecl
	data, err := root.ReadFile(path)
	if err != nil {
		return decl, fmt.Errorf("read %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	decl.lockWait, err = parseRepoRootLockWaitDocument(data, ext == ".json")
	if err != nil {
		return decl, fmt.Errorf("parse %s: %w", path, err)
	}
	parsedOutcome, err := ParseMakeOutcomeReportDocument(data, ext == ".json")
	if err != nil {
		return decl, fmt.Errorf("parse %s: %w", path, err)
	}
	if ext == ".json" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return decl, fmt.Errorf("parse %s: %w", path, err)
		}
		if b, ok := raw["branch"]; ok {
			decl.patterns, decl.integration, decl.makeTimeout, _, err = decodeJSONBranchFields(b)
			if err != nil {
				return decl, err
			}
		}
		decl.outcomeReport = parsedOutcome
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
		decl.makeTimeout = file.Branch.MakeTimeout
	}
	decl.outcomeReport = parsedOutcome
	return decl, nil
}

// parseRepoRootLockWaitDocument reads only the authority-bearing lockWait
// declaration from the raw repository-root document. General config decoding
// is intentionally permissive for compatibility; it cannot be allowed to
// hide duplicate or null lockWait keys.
func parseRepoRootLockWaitDocument(data []byte, isJSON bool) (string, error) {
	if isJSON {
		return parseRepoRootLockWaitJSON(data)
	}
	return parseRepoRootLockWaitYAML(data)
}

func parseRepoRootLockWaitJSON(data []byte) (string, error) {
	root, rootCounts, err := parseJSONObjectCounts(data)
	if err != nil {
		return "", err
	}
	if rootCounts["branch"] > 1 {
		return "", fmt.Errorf("duplicate JSON key %q", "branch")
	}
	branch, ok := root["branch"]
	if !ok || len(bytes.TrimSpace(branch)) == 0 || bytes.TrimSpace(branch)[0] != '{' {
		return "", nil
	}
	fields, counts, err := parseJSONObjectCounts(branch)
	if err != nil {
		return "", fmt.Errorf("parse branch: %w", err)
	}
	if counts["lockWait"] > 1 {
		return "", fmt.Errorf("duplicate JSON key %q", "lockWait")
	}
	raw, ok := fields["lockWait"]
	if !ok {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", fmt.Errorf("branch.lockWait must be a non-empty string")
	}
	return value, nil
}

func parseRepoRootLockWaitYAML(data []byte) (string, error) {
	if err := RejectMultiDocumentYAML(data); err != nil {
		return "", err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", err
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return "", nil
	}
	config := root.Content[0]
	if mappingKeyCount(config, "branch") > 1 {
		return "", fmt.Errorf("duplicate YAML key %q", "branch")
	}
	hidden, err := yamlRootMergeSuppliesLockWait(config)
	if err != nil {
		return "", err
	}
	if hidden {
		return "", fmt.Errorf("branch.lockWait does not allow a root YAML merge that supplies lockWait")
	}
	branch := mappingValue(config, "branch")
	if branch == nil {
		return "", nil
	}
	if branch.Kind == yaml.AliasNode {
		hidden, err := yamlNodeSuppliesKey(branch, "lockWait")
		if err != nil {
			return "", err
		}
		if hidden {
			return "", fmt.Errorf("branch.lockWait does not allow an aliased branch declaration")
		}
		return "", nil
	}
	if branch.Kind != yaml.MappingNode {
		return "", nil
	}
	if mappingKeyCount(branch, "lockWait") > 1 {
		return "", fmt.Errorf("duplicate YAML key %q", "lockWait")
	}
	hidden, err = yamlMergeSuppliesKey(branch, "lockWait")
	if err != nil {
		return "", err
	}
	if hidden {
		return "", fmt.Errorf("branch.lockWait does not allow a YAML merge that supplies lockWait")
	}
	node := mappingValue(branch, "lockWait")
	if node == nil {
		return "", nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" || node.Value == "" {
		return "", fmt.Errorf("branch.lockWait must be a non-empty string")
	}
	return node.Value, nil
}

func decodeJSONBranchFields(raw json.RawMessage) (patterns, integration []string, makeTimeout, lockWait string, err error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, "", "", err
	}
	timeout, err := decodeJSONMakeTimeout(m["makeTimeout"])
	if err != nil {
		return nil, nil, "", "", err
	}
	wait, err := decodeJSONLockWait(m["lockWait"])
	if err != nil {
		return nil, nil, "", "", err
	}
	return coerceStringList(m["taskPattern"]), coerceStringList(m["integrationBranch"]), timeout, wait, nil
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

func decodeJSONLockWait(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", nil
	case string:
		return t, nil
	default:
		return "", fmt.Errorf("branch.lockWait must be a string")
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

// parseLockWait accepts an explicit unbounded wait as either "0" or
// "unbounded". Unlike makeTimeout, zero is meaningful here.
func parseLockWait(raw, path string) (time.Duration, bool, error) {
	if raw == "" {
		return 0, false, nil
	}
	if strings.TrimSpace(raw) != raw {
		return 0, false, fmt.Errorf("%s: branch.lockWait %q must not have surrounding whitespace", path, raw)
	}
	if raw == "unbounded" {
		return 0, true, nil
	}
	wait, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s: branch.lockWait %q: %w", path, raw, err)
	}
	if wait < 0 {
		return 0, false, fmt.Errorf("%s: branch.lockWait %q must not be negative", path, raw)
	}
	return wait, true, nil
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
		if nestedDecl, err := readRootBranchDecl(root, entryRel); err == nil && len(nestedDecl.patterns) > 0 {
			decl.Facts = append(decl.Facts, "ignored non-root taskPattern: "+entryPath)
		}
	}

	return nil
}
