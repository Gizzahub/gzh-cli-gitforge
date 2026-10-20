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
	// ResourceHighLoad is true only when the repository-root declaration
	// explicitly opts its integration verification into the shared host slot.
	ResourceHighLoad bool
	// LockWait is the declared branch.lockWait. LockWaitSet distinguishes an
	// omitted declaration from an explicit unbounded value.
	LockWait    time.Duration
	LockWaitSet bool
	Source      string
	Facts       []string
}

type rootBranchDecl struct {
	patterns         []string
	integration      []string
	resourceHighLoad bool
	lockWait         string
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

	lockWait, lockWaitSet, err := parseLockWait(rootDecl.lockWait, path)
	if err != nil {
		return decl, err
	}

	decl.Patterns = append([]string(nil), rootDecl.patterns...)
	decl.IntegrationBranch = append(BranchList(nil), rootDecl.integration...)
	decl.ResourceHighLoad = rootDecl.resourceHighLoad
	decl.LockWait = lockWait
	decl.LockWaitSet = lockWaitSet
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
	if err := rejectRemovedConfigKeys(data, ext == ".json", fullPath); err != nil {
		return decl, err
	}
	decl.resourceHighLoad, err = ParseResourceHighLoadDocument(data, ext == ".json")
	if err != nil {
		return decl, fmt.Errorf("parse %s: %w", path, err)
	}
	decl.lockWait, err = parseRepoRootLockWaitDocument(data, ext == ".json")
	if err != nil {
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
				decl.lockWait = fields.lockWait
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
		decl.lockWait = file.Branch.LockWait
	}
	return decl, nil
}

type jsonBranchFields struct {
	patterns, integration []string
	lockWait              string
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
	wait, err := decodeJSONLockWait(m["lockWait"], m["lockWait"] != nil)
	if err != nil {
		return jsonBranchFields{}, err
	}
	return jsonBranchFields{patterns: coerceStringList(patternsValue), integration: coerceStringList(integrationValue), lockWait: wait}, nil
}

func decodeJSONLockWait(raw json.RawMessage, present bool) (string, error) {
	if !present || string(bytes.TrimSpace(raw)) == "null" {
		if present {
			return "", fmt.Errorf("branch.lockWait must be a non-empty string")
		}
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", fmt.Errorf("branch.lockWait must be a non-empty string")
	}
	return value, nil
}

// parseRepoRootLockWaitDocument reads only the authority-bearing repository
// root declaration, rejecting duplicate, alias, and merge forms that a normal
// compatibility decoder could otherwise obscure.
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
	return decodeJSONLockWait(fields["lockWait"], fields["lockWait"] != nil)
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

func yamlRootMergeSuppliesLockWait(root *yaml.Node) (bool, error) {
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if !isYAMLMergeKey(root.Content[i]) {
			continue
		}
		hidden, err := state.mergeValueSuppliesNestedKey(root.Content[i+1], "branch", "lockWait")
		if err != nil || hidden {
			return hidden, err
		}
	}
	return false, nil
}

func decodeJSONResourceHighLoad(raw json.RawMessage, present bool) (bool, error) {
	if !present {
		return false, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return false, fmt.Errorf("branch.resourceHighLoad must be a boolean")
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("branch.resourceHighLoad must be a boolean")
	}
	return value, nil
}

// ParseResourceHighLoadDocument reads the repository-owned resource
// classification from one committed .gz-git document. An absent declaration
// is deliberately light. A present declaration must be a boolean so a dirty
// or malformed config cannot silently opt a gate out of capacity control.
func ParseResourceHighLoadDocument(data []byte, isJSON bool) (bool, error) {
	if isJSON {
		return parseJSONResourceHighLoad(data)
	}
	return parseYAMLResourceHighLoad(data)
}

func parseJSONResourceHighLoad(data []byte) (bool, error) {
	root, rootCounts, err := parseJSONObjectCounts(data)
	if err != nil {
		return false, err
	}
	if rootCounts["branch"] > 1 {
		return false, fmt.Errorf("duplicate JSON key %q", "branch")
	}
	branch, ok := root["branch"]
	if !ok {
		return false, nil
	}
	fields, counts, err := parseJSONObjectCounts(branch)
	if err != nil {
		return false, fmt.Errorf("parse branch: %w", err)
	}
	if counts["resourceHighLoad"] > 1 {
		return false, fmt.Errorf("duplicate JSON key %q", "resourceHighLoad")
	}
	return decodeJSONResourceHighLoad(fields["resourceHighLoad"], fields["resourceHighLoad"] != nil)
}

func parseYAMLResourceHighLoad(data []byte) (bool, error) {
	if err := RejectMultiDocumentYAML(data); err != nil {
		return false, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false, err
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	config := root.Content[0]
	if mappingKeyCount(config, "branch") > 1 {
		return false, fmt.Errorf("duplicate YAML key %q", "branch")
	}
	if err := rejectRootResourceHighLoadMerge(config); err != nil {
		return false, err
	}
	branch := mappingValue(config, "branch")
	if branch == nil {
		return false, nil
	}
	if branch.Kind == yaml.AliasNode {
		return rejectAliasedResourceHighLoad(branch)
	}
	if branch.Kind != yaml.MappingNode {
		return false, nil
	}
	if mappingKeyCount(branch, "resourceHighLoad") > 1 {
		return false, fmt.Errorf("duplicate YAML key %q", "resourceHighLoad")
	}
	hidden, err := yamlMergeSuppliesKey(branch, "resourceHighLoad")
	if err != nil {
		return false, err
	}
	if hidden {
		return false, fmt.Errorf("branch.resourceHighLoad does not allow a YAML merge that supplies resourceHighLoad")
	}
	node := mappingValue(branch, "resourceHighLoad")
	if node == nil {
		return false, nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
		return false, fmt.Errorf("branch.resourceHighLoad must be a boolean")
	}
	var value bool
	if err := node.Decode(&value); err != nil {
		return false, fmt.Errorf("branch.resourceHighLoad must be a boolean: %w", err)
	}
	return value, nil
}

func rejectRootResourceHighLoadMerge(config *yaml.Node) error {
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	for i := 0; i+1 < len(config.Content); i += 2 {
		if !isYAMLMergeKey(config.Content[i]) {
			continue
		}
		hidden, err := state.mergeValueSuppliesNestedKey(config.Content[i+1], "branch", "resourceHighLoad")
		if err != nil {
			return err
		}
		if hidden {
			return fmt.Errorf("branch.resourceHighLoad does not allow a root YAML merge that supplies resourceHighLoad")
		}
	}
	return nil
}

func rejectAliasedResourceHighLoad(branch *yaml.Node) (bool, error) {
	hidden, err := yamlNodeSuppliesKey(branch, "resourceHighLoad")
	if err != nil {
		return false, err
	}
	if hidden {
		return false, fmt.Errorf("branch.resourceHighLoad does not allow an aliased branch declaration")
	}
	return false, nil
}

// decodeJSONPrepareHighLoad validates a present JSON boolean declaration.
func decodeJSONPrepareHighLoad(raw json.RawMessage, present bool) (bool, error) {
	if !present {
		return false, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return false, fmt.Errorf("branch.prepareHighLoad must be a boolean")
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("branch.prepareHighLoad must be a boolean")
	}
	return value, nil
}

// ParsePrepareHighLoadDocument reads the repository-owned resource
// classification from one committed .gz-git document. An absent declaration
// is deliberately light. A present declaration must be a boolean so a dirty
// or malformed config cannot silently opt a gate out of capacity control.
func ParsePrepareHighLoadDocument(data []byte, isJSON bool) (bool, error) {
	if isJSON {
		return parseJSONPrepareHighLoad(data)
	}
	return parseYAMLPrepareHighLoad(data)
}

func parseJSONPrepareHighLoad(data []byte) (bool, error) {
	root, rootCounts, err := parseJSONObjectCounts(data)
	if err != nil {
		return false, err
	}
	if rootCounts["branch"] > 1 {
		return false, fmt.Errorf("duplicate JSON key %q", "branch")
	}
	branch, ok := root["branch"]
	if !ok {
		return false, nil
	}
	fields, counts, err := parseJSONObjectCounts(branch)
	if err != nil {
		return false, fmt.Errorf("parse branch: %w", err)
	}
	if counts["prepareHighLoad"] > 1 {
		return false, fmt.Errorf("duplicate JSON key %q", "prepareHighLoad")
	}
	return decodeJSONPrepareHighLoad(fields["prepareHighLoad"], fields["prepareHighLoad"] != nil)
}

func parseYAMLPrepareHighLoad(data []byte) (bool, error) {
	if err := RejectMultiDocumentYAML(data); err != nil {
		return false, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false, err
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	config := root.Content[0]
	if mappingKeyCount(config, "branch") > 1 {
		return false, fmt.Errorf("duplicate YAML key %q", "branch")
	}
	if err := rejectRootPrepareHighLoadMerge(config); err != nil {
		return false, err
	}
	branch := mappingValue(config, "branch")
	if branch == nil {
		return false, nil
	}
	if branch.Kind == yaml.AliasNode {
		return rejectAliasedPrepareHighLoad(branch)
	}
	if branch.Kind != yaml.MappingNode {
		return false, nil
	}
	if mappingKeyCount(branch, "prepareHighLoad") > 1 {
		return false, fmt.Errorf("duplicate YAML key %q", "prepareHighLoad")
	}
	hidden, err := yamlMergeSuppliesKey(branch, "prepareHighLoad")
	if err != nil {
		return false, err
	}
	if hidden {
		return false, fmt.Errorf("branch.prepareHighLoad does not allow a YAML merge that supplies prepareHighLoad")
	}
	node := mappingValue(branch, "prepareHighLoad")
	if node == nil {
		return false, nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
		return false, fmt.Errorf("branch.prepareHighLoad must be a boolean")
	}
	var value bool
	if err := node.Decode(&value); err != nil {
		return false, fmt.Errorf("branch.prepareHighLoad must be a boolean: %w", err)
	}
	return value, nil
}

func rejectRootPrepareHighLoadMerge(config *yaml.Node) error {
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	for i := 0; i+1 < len(config.Content); i += 2 {
		if !isYAMLMergeKey(config.Content[i]) {
			continue
		}
		hidden, err := state.mergeValueSuppliesNestedKey(config.Content[i+1], "branch", "prepareHighLoad")
		if err != nil {
			return err
		}
		if hidden {
			return fmt.Errorf("branch.prepareHighLoad does not allow a root YAML merge that supplies prepareHighLoad")
		}
	}
	return nil
}

func rejectAliasedPrepareHighLoad(branch *yaml.Node) (bool, error) {
	hidden, err := yamlNodeSuppliesKey(branch, "prepareHighLoad")
	if err != nil {
		return false, err
	}
	if hidden {
		return false, fmt.Errorf("branch.prepareHighLoad does not allow an aliased branch declaration")
	}
	return false, nil
}

// parseLockWait accepts "0" and "unbounded" as explicit unbounded waits.
// The boolean keeps that choice distinct from an omitted declaration.
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
		if nested, err := readRootBranchDecl(root, entryRel, entryPath); err == nil && len(nested.patterns) > 0 {
			decl.Facts = append(decl.Facts, "ignored non-root taskPattern: "+entryPath)
		}
	}

	return nil
}
