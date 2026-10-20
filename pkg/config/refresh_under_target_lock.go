// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RefreshUnderTargetLockKey is the branch key that opts a repository into
// ADR-0003: a stale task branch is rebased onto the target while the
// integration still holds the target lock.
const RefreshUnderTargetLockKey = "refreshUnderTargetLock"

// ParseRefreshUnderTargetLockDocument reads branch.refreshUnderTargetLock
// from one committed .gz-git document. Absent means off. A present
// declaration must be a plain boolean: the opt-in lets the integration
// rewrite task history, so a malformed, merged, or aliased value must fail
// instead of guessing.
func ParseRefreshUnderTargetLockDocument(data []byte, isJSON bool) (bool, error) {
	return parseStrictBranchBool(data, isJSON, RefreshUnderTargetLockKey)
}

// parseStrictBranchBool reads branch.<key> with the same rules as
// branch.prepareHighLoad: one document, no duplicate keys, no YAML merge or
// alias that supplies the key, and a literal boolean value.
func parseStrictBranchBool(data []byte, isJSON bool, key string) (bool, error) {
	if isJSON {
		return parseJSONStrictBranchBool(data, key)
	}
	return parseYAMLStrictBranchBool(data, key)
}

func parseJSONStrictBranchBool(data []byte, key string) (bool, error) {
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
	if counts[key] > 1 {
		return false, fmt.Errorf("duplicate JSON key %q", key)
	}
	raw, present := fields[key]
	if !present {
		return false, nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return false, fmt.Errorf("branch.%s must be a boolean", key)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("branch.%s must be a boolean", key)
	}
	return value, nil
}

func parseYAMLStrictBranchBool(data []byte, key string) (bool, error) {
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
	doc := root.Content[0]
	if mappingKeyCount(doc, "branch") > 1 {
		return false, fmt.Errorf("duplicate YAML key %q", "branch")
	}
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if !isYAMLMergeKey(doc.Content[i]) {
			continue
		}
		hidden, err := state.mergeValueSuppliesNestedKey(doc.Content[i+1], "branch", key)
		if err != nil {
			return false, err
		}
		if hidden {
			return false, fmt.Errorf("branch.%s does not allow a root YAML merge that supplies %s", key, key)
		}
	}
	branch := mappingValue(doc, "branch")
	if branch == nil {
		return false, nil
	}
	if branch.Kind == yaml.AliasNode {
		hidden, err := yamlNodeSuppliesKey(branch, key)
		if err != nil {
			return false, err
		}
		if hidden {
			return false, fmt.Errorf("branch.%s does not allow an aliased branch declaration", key)
		}
		return false, nil
	}
	if branch.Kind != yaml.MappingNode {
		return false, nil
	}
	if mappingKeyCount(branch, key) > 1 {
		return false, fmt.Errorf("duplicate YAML key %q", key)
	}
	hidden, err := yamlMergeSuppliesKey(branch, key)
	if err != nil {
		return false, err
	}
	if hidden {
		return false, fmt.Errorf("branch.%s does not allow a YAML merge that supplies %s", key, key)
	}
	node := mappingValue(branch, key)
	if node == nil {
		return false, nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
		return false, fmt.Errorf("branch.%s must be a boolean", key)
	}
	var value bool
	if err := node.Decode(&value); err != nil {
		return false, fmt.Errorf("branch.%s must be a boolean: %w", key, err)
	}
	return value, nil
}
