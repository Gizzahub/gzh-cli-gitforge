// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	// PrepareProfileFamilybookEntV1 prepares Ent generated code.
	PrepareProfileFamilybookEntV1 = "familybook-ent-v1"
	// PrepareProfileFlowTaskchainLocalSubprojectsV1 prepares local taskchain subprojects.
	PrepareProfileFlowTaskchainLocalSubprojectsV1 = "flow-taskchain-local-subprojects-v1"
)

var validPrepareProfiles = map[string]struct{}{
	PrepareProfileFamilybookEntV1:                 {},
	PrepareProfileFlowTaskchainLocalSubprojectsV1: {},
}

// ValidatePrepareProfile validates the closed set of repository preparation
// profiles. An empty value is valid only as an absent declaration; callers
// that parse a present declaration reject it before reaching this function.
func ValidatePrepareProfile(profile string) error {
	if _, ok := validPrepareProfiles[profile]; !ok {
		return fmt.Errorf("branch.prepareProfile has unsupported profile %q", profile)
	}
	return nil
}

// ParsePrepareProfileDocument extracts branch.prepareProfile from a
// repository-root config. It deliberately examines the source document rather
// than a merged config so duplicate keys and YAML multi-document payloads
// cannot be hidden by a permissive general-purpose decoder.
func ParsePrepareProfileDocument(data []byte, isJSON bool) (profile string, present bool, err error) {
	if isJSON {
		return parsePrepareProfileJSON(data)
	}
	return parsePrepareProfileYAML(data)
}

func parsePrepareProfileJSON(data []byte) (profile string, present bool, err error) {
	root, rootCounts, err := parseJSONObjectCounts(data)
	if err != nil {
		return "", false, fmt.Errorf("parse config: %w", err)
	}
	if rootCounts["branch"] > 1 {
		return "", false, fmt.Errorf("duplicate JSON key %q", "branch")
	}
	branch, ok := root["branch"]
	if !ok {
		return "", false, nil
	}
	if trimmed := bytes.TrimSpace(branch); len(trimmed) == 0 || trimmed[0] != '{' {
		return "", false, nil
	}
	fields, counts, err := parseJSONObjectCounts(branch)
	if err != nil {
		return "", false, fmt.Errorf("parse branch: %w", err)
	}
	if counts["prepareProfile"] > 1 {
		return "", false, fmt.Errorf("duplicate JSON key %q", "prepareProfile")
	}
	raw, ok := fields["prepareProfile"]
	if !ok {
		return "", false, nil
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", false, fmt.Errorf("branch.prepareProfile must be a string")
	}
	if err := ValidatePrepareProfile(profile); err != nil {
		return "", false, err
	}
	return profile, true, nil
}

func parsePrepareProfileYAML(data []byte) (profile string, present bool, err error) {
	if err := RejectMultiDocumentYAML(data); err != nil {
		return "", false, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", false, fmt.Errorf("parse config: %w", err)
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return "", false, nil
	}
	if mappingKeyCount(root.Content[0], "branch") > 1 {
		return "", false, fmt.Errorf("duplicate YAML key %q", "branch")
	}
	branch := mappingValue(root.Content[0], "branch")
	if branch == nil || branch.Kind != yaml.MappingNode {
		return "", false, nil
	}
	if mappingKeyCount(branch, "prepareProfile") > 1 {
		return "", false, fmt.Errorf("duplicate YAML key %q", "prepareProfile")
	}
	node := mappingValue(branch, "prepareProfile")
	if node == nil {
		return "", false, nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", false, fmt.Errorf("branch.prepareProfile must be a string")
	}
	if err := ValidatePrepareProfile(node.Value); err != nil {
		return "", false, err
	}
	return node.Value, true, nil
}
