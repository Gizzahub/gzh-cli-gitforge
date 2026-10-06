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
	maxMakeOutcomeReportDocumentBytes = 64 * 1024
	maxMakeOutcomeReportYAMLNodes     = 1024
)

// MakeOutcomeReport is the closed declaration that permits a legacy Make
// integration gate to report a known target outcome. V1 deliberately admits
// only the two fixed Make targets; it provides no command, argument, or path
// execution surface.
type MakeOutcomeReport struct {
	Version int      `yaml:"version" json:"version"`
	Targets []string `yaml:"targets" json:"targets"`
}

// Includes reports whether target is one of the declaration's exact targets.
// A nil report never includes a target.
func (r *MakeOutcomeReport) Includes(target string) bool {
	if r == nil {
		return false
	}
	for _, declared := range r.Targets {
		if declared == target {
			return true
		}
	}
	return false
}

// ValidateMakeOutcomeReport validates the closed V1 declaration.
func ValidateMakeOutcomeReport(r MakeOutcomeReport) error {
	if r.Version != 1 {
		return fmt.Errorf("branch.makeOutcomeReport.version must be 1")
	}
	if len(r.Targets) == 0 {
		return fmt.Errorf("branch.makeOutcomeReport.targets must not be empty")
	}
	seen := make(map[string]struct{}, len(r.Targets))
	for _, target := range r.Targets {
		if target != "check" && target != "lint" {
			return fmt.Errorf("branch.makeOutcomeReport.targets contains unsupported target %q", target)
		}
		if _, duplicate := seen[target]; duplicate {
			return fmt.Errorf("branch.makeOutcomeReport.targets contains duplicate target %q", target)
		}
		seen[target] = struct{}{}
	}
	return nil
}

// ParseMakeOutcomeReportDocument extracts branch.makeOutcomeReport from a
// repository-root document. It inspects raw syntax rather than a decoded
// ProjectConfig so duplicate keys, aliases, merges, or extra contract fields
// cannot be hidden by the permissive general configuration decoder.
//
// A nil result means the declaration is absent. Other root and branch fields
// remain outside this closed contract and are deliberately accepted.
func ParseMakeOutcomeReportDocument(data []byte, isJSON bool) (*MakeOutcomeReport, error) {
	if isJSON {
		return parseMakeOutcomeReportJSON(data)
	}
	return parseMakeOutcomeReportYAML(data)
}

//nolint:nilnil // A missing declaration is the documented optional result.
func parseMakeOutcomeReportJSON(data []byte) (*MakeOutcomeReport, error) {
	root, rootCounts, err := parseJSONObjectCounts(data)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if rootCounts["branch"] > 1 {
		return nil, fmt.Errorf("duplicate JSON key %q", "branch")
	}
	branch, ok := root["branch"]
	if !ok {
		return nil, nil
	}
	trimmed := bytes.TrimSpace(branch)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, nil
	}
	fields, branchCounts, err := parseJSONObjectCounts(branch)
	if err != nil {
		return nil, fmt.Errorf("parse branch: %w", err)
	}
	if branchCounts["makeOutcomeReport"] > 1 {
		return nil, fmt.Errorf("duplicate JSON key %q", "makeOutcomeReport")
	}
	raw, ok := fields["makeOutcomeReport"]
	if !ok {
		return nil, nil
	}
	if len(data) > maxMakeOutcomeReportDocumentBytes {
		return nil, fmt.Errorf("branch.makeOutcomeReport document exceeds %d bytes", maxMakeOutcomeReportDocumentBytes)
	}
	reportFields, err := parseJSONObjectUnique(raw)
	if err != nil {
		return nil, fmt.Errorf("branch.makeOutcomeReport must be an object")
	}
	if len(reportFields) != 2 || reportFields["version"] == nil || reportFields["targets"] == nil {
		return nil, fmt.Errorf("branch.makeOutcomeReport has unknown or missing fields")
	}
	var report MakeOutcomeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, fmt.Errorf("parse branch.makeOutcomeReport: %w", err)
	}
	if err := ValidateMakeOutcomeReport(report); err != nil {
		return nil, err
	}
	return &report, nil
}

//nolint:nilnil // A missing declaration is the documented optional result.
func parseMakeOutcomeReportYAML(data []byte) (*MakeOutcomeReport, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	node, err := findMakeOutcomeReportYAMLNode(&root)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, nil
	}
	if len(data) > maxMakeOutcomeReportDocumentBytes {
		return nil, fmt.Errorf("branch.makeOutcomeReport document exceeds %d bytes", maxMakeOutcomeReportDocumentBytes)
	}
	if err := RejectMultiDocumentYAML(data); err != nil {
		return nil, err
	}
	if err := rejectMakeOutcomeReportYAMLAliases(node); err != nil {
		return nil, err
	}
	if err := validateMakeOutcomeReportYAMLNode(node); err != nil {
		return nil, err
	}
	var report MakeOutcomeReport
	if err := node.Decode(&report); err != nil {
		return nil, fmt.Errorf("parse branch.makeOutcomeReport: %w", err)
	}
	if err := ValidateMakeOutcomeReport(report); err != nil {
		return nil, err
	}
	return &report, nil
}

//nolint:nilnil // A missing declaration is the documented optional result.
func findMakeOutcomeReportYAMLNode(root *yaml.Node) (*yaml.Node, error) {
	if root == nil || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, nil
	}
	config := root.Content[0]
	if mappingKeyCount(config, "branch") > 1 {
		return nil, fmt.Errorf("duplicate YAML key %q", "branch")
	}
	hidden, err := yamlRootMergeSuppliesMakeOutcomeReport(config)
	if err != nil {
		return nil, err
	}
	if hidden {
		return nil, fmt.Errorf("branch.makeOutcomeReport does not allow a root YAML merge that supplies makeOutcomeReport")
	}
	branch := mappingValue(config, "branch")
	if branch == nil {
		return nil, nil
	}
	if branch.Kind == yaml.AliasNode {
		hidden, err := yamlNodeSuppliesKey(branch, "makeOutcomeReport")
		if err != nil {
			return nil, err
		}
		if hidden {
			return nil, fmt.Errorf("branch.makeOutcomeReport does not allow an aliased branch declaration that supplies makeOutcomeReport")
		}
		return nil, nil
	}
	if branch.Kind != yaml.MappingNode {
		return nil, nil
	}
	if mappingKeyCount(branch, "makeOutcomeReport") > 1 {
		return nil, fmt.Errorf("duplicate YAML key %q", "makeOutcomeReport")
	}
	hidden, err = yamlMergeSuppliesKey(branch, "makeOutcomeReport")
	if err != nil {
		return nil, err
	}
	if hidden {
		return nil, fmt.Errorf("branch.makeOutcomeReport does not allow a branch YAML merge that supplies makeOutcomeReport")
	}
	return mappingValue(branch, "makeOutcomeReport"), nil
}

func validateMakeOutcomeReportYAMLNode(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode || len(node.Content) != 4 || mappingValue(node, "version") == nil || mappingValue(node, "targets") == nil {
		return fmt.Errorf("branch.makeOutcomeReport has unknown or missing fields")
	}
	if err := rejectDuplicateYAMLKeys(node); err != nil {
		return err
	}
	version := mappingValue(node, "version")
	targets := mappingValue(node, "targets")
	if version.Kind != yaml.ScalarNode || version.Tag != "!!int" {
		return fmt.Errorf("branch.makeOutcomeReport.version must be an integer")
	}
	if targets.Kind != yaml.SequenceNode {
		return fmt.Errorf("branch.makeOutcomeReport.targets must be a list of strings")
	}
	for _, target := range targets.Content {
		if target.Kind != yaml.ScalarNode || target.Tag != "!!str" {
			return fmt.Errorf("branch.makeOutcomeReport.targets must be a list of strings")
		}
	}
	return nil
}

func rejectMakeOutcomeReportYAMLAliases(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf("branch.makeOutcomeReport does not allow YAML aliases or anchors")
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "<<" {
				return fmt.Errorf("branch.makeOutcomeReport does not allow YAML merges")
			}
		}
	}
	for _, child := range node.Content {
		if err := rejectMakeOutcomeReportYAMLAliases(child); err != nil {
			return err
		}
	}
	return nil
}

// yamlMergeSuppliesKey reports whether a mapping's YAML merge supplies a
// particular authority-bearing key. Direct keys remain inspectable; a merged
// branch or makeOutcomeReport could otherwise be accepted by the permissive
// project decoder without passing this parser's closed-contract checks.
func yamlRootMergeSuppliesMakeOutcomeReport(root *yaml.Node) (bool, error) {
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	return state.rootMergeSuppliesMakeOutcomeReport(root)
}

func yamlNodeSuppliesKey(node *yaml.Node, key string) (bool, error) {
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	return state.nodeSuppliesKey(node, key)
}

func yamlMergeSuppliesKey(node *yaml.Node, key string) (bool, error) {
	state := yamlKeyTraversal{visiting: make(map[*yaml.Node]bool)}
	return state.mergeSuppliesKey(node, key)
}

type yamlKeyTraversal struct {
	visiting map[*yaml.Node]bool
	visited  int
}

func (s *yamlKeyTraversal) rootMergeSuppliesMakeOutcomeReport(root *yaml.Node) (bool, error) {
	if root == nil || root.Kind != yaml.MappingNode {
		return false, nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if !isYAMLMergeKey(root.Content[i]) {
			continue
		}
		hidden, err := s.mergeValueSuppliesNestedKey(root.Content[i+1], "branch", "makeOutcomeReport")
		if err != nil || hidden {
			return hidden, err
		}
	}
	return false, nil
}

func (s *yamlKeyTraversal) mergeSuppliesKey(node *yaml.Node, key string) (bool, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return false, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if !isYAMLMergeKey(node.Content[i]) {
			continue
		}
		hidden, err := s.mergeValueSuppliesKey(node.Content[i+1], key)
		if err != nil || hidden {
			return hidden, err
		}
	}
	return false, nil
}

func (s *yamlKeyTraversal) mergeValueSuppliesNestedKey(node *yaml.Node, outer, inner string) (bool, error) {
	if node == nil {
		return false, nil
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			hidden, err := s.mergeValueSuppliesNestedKey(child, outer, inner)
			if err != nil || hidden {
				return hidden, err
			}
		}
		return false, nil
	}
	if node.Kind == yaml.AliasNode {
		outerNode, found, err := s.mappingValue(node.Alias, outer)
		if err != nil || !found {
			return false, err
		}
		return s.nodeSuppliesKey(outerNode, inner)
	}
	outerNode, found, err := s.mappingValue(node, outer)
	if err != nil || !found {
		return false, err
	}
	return s.nodeSuppliesKey(outerNode, inner)
}

func (s *yamlKeyTraversal) mergeValueSuppliesKey(node *yaml.Node, key string) (bool, error) {
	if node == nil {
		return false, nil
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			hidden, err := s.mergeValueSuppliesKey(child, key)
			if err != nil || hidden {
				return hidden, err
			}
		}
		return false, nil
	}
	return s.nodeSuppliesKey(node, key)
}

func (s *yamlKeyTraversal) nodeSuppliesKey(node *yaml.Node, key string) (bool, error) {
	if node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	_, found, err := s.mappingValue(node, key)
	return found, err
}

func (s *yamlKeyTraversal) mappingValue(node *yaml.Node, key string) (*yaml.Node, bool, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false, nil
	}
	if err := s.enter(node); err != nil {
		return nil, false, err
	}
	defer s.leave(node)
	if value := mappingValue(node, key); value != nil {
		return value, true, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if !isYAMLMergeKey(node.Content[i]) {
			continue
		}
		value, found, err := s.mergeValueForKey(node.Content[i+1], key)
		if err != nil || found {
			return value, found, err
		}
	}
	return nil, false, nil
}

func (s *yamlKeyTraversal) mergeValueForKey(node *yaml.Node, key string) (*yaml.Node, bool, error) {
	if node == nil {
		return nil, false, nil
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			value, found, err := s.mergeValueForKey(child, key)
			if err != nil || found {
				return value, found, err
			}
		}
		return nil, false, nil
	}
	if node.Kind == yaml.AliasNode {
		return s.mappingValue(node.Alias, key)
	}
	return s.mappingValue(node, key)
}

func (s *yamlKeyTraversal) enter(node *yaml.Node) error {
	if s.visiting[node] {
		return fmt.Errorf("branch.makeOutcomeReport YAML alias or merge cycle")
	}
	s.visited++
	if s.visited > maxMakeOutcomeReportYAMLNodes {
		return fmt.Errorf("branch.makeOutcomeReport YAML alias or merge traversal exceeds %d nodes", maxMakeOutcomeReportYAMLNodes)
	}
	s.visiting[node] = true
	return nil
}

func (s *yamlKeyTraversal) leave(node *yaml.Node) {
	delete(s.visiting, node)
}

func isYAMLMergeKey(node *yaml.Node) bool {
	return node != nil && (node.Value == "<<" || node.Tag == "!!merge")
}
