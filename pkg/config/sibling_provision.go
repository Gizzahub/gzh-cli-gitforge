// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"

	"gopkg.in/yaml.v3"
)

// SiblingProvision is the closed, repository-owned declaration used to make
// a relative path dependency available in an isolated task worktree.
// Sources remain local checkouts: this contract never accepts a command, URL
// override, or a machine-specific source path.
type SiblingProvision struct {
	Version  int                 `yaml:"version" json:"version"`
	Siblings []SiblingDependency `yaml:"siblings" json:"siblings"`
}

// SiblingDependency identifies one local primary checkout and its safe
// extraction location. Revision is either source-tip or a full Git object ID.
type SiblingDependency struct {
	Name         string `yaml:"name" json:"name"`
	Remote       string `yaml:"remote" json:"remote"`
	Path         string `yaml:"path" json:"path"`
	SourceBranch string `yaml:"sourceBranch,omitempty" json:"sourceBranch,omitempty"`
	Revision     string `yaml:"revision" json:"revision"`
}

// ValidateSiblingProvision validates the closed repository-owned opt-in.
func ValidateSiblingProvision(p SiblingProvision) error {
	if p.Version != 1 || len(p.Siblings) == 0 || len(p.Siblings) > 32 {
		return fmt.Errorf("branch.siblingProvision must have version 1 and at least one sibling")
	}
	seen := map[string]bool{}
	paths := map[string]bool{}
	for _, s := range p.Siblings {
		if err := validateSiblingDependency(s, seen, paths); err != nil {
			return err
		}
	}
	return nil
}

func validateSiblingDependency(s SiblingDependency, seen, paths map[string]bool) error {
	if !validSiblingSegment(s.Name) || seen[s.Name] {
		return fmt.Errorf("branch.siblingProvision sibling name must be unique and non-empty")
	}
	seen[s.Name] = true
	if s.Remote == "" || strings.TrimSpace(s.Remote) != s.Remote || gitcmd.SanitizeURL(s.Remote) != nil {
		return fmt.Errorf("branch.siblingProvision %s remote URL is invalid", s.Name)
	}
	if err := validateSiblingRemoteCredentials(s.Remote); err != nil {
		return fmt.Errorf("branch.siblingProvision %s remote must not contain credentials", s.Name)
	}
	if !validSiblingPath(s.Path) || paths[s.Path] {
		return fmt.Errorf("branch.siblingProvision %s path must be ../<single-segment>", s.Name)
	}
	paths[s.Path] = true
	if s.Revision == "source-tip" {
		if s.SourceBranch == "" || strings.TrimSpace(s.SourceBranch) != s.SourceBranch || strings.Contains(s.SourceBranch, "..") || gitcmd.SanitizeBranchName(s.SourceBranch) != nil {
			return fmt.Errorf("branch.siblingProvision %s source-tip requires sourceBranch", s.Name)
		}
		return nil
	}
	if !fullOID(s.Revision) || s.SourceBranch != "" {
		return fmt.Errorf("branch.siblingProvision %s revision must be source-tip or a full object ID", s.Name)
	}
	return nil
}

func validateSiblingRemoteCredentials(raw string) error {
	if !strings.Contains(raw, "://") {
		return nil
	}
	remote, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if remote.User == nil {
		return nil
	}
	_, password := remote.User.Password()
	if password || remote.Scheme == "https" || remote.Scheme == "http" {
		return fmt.Errorf("credential-bearing URL")
	}
	return nil
}

func validSiblingPath(p string) bool {
	return strings.HasPrefix(p, "../") && strings.Count(p, "/") == 1 && path.Clean(p) == p && validSiblingSegment(strings.TrimPrefix(p, "../"))
}

func validSiblingSegment(s string) bool {
	if s == "" || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "-") {
		return false
	}
	for _, r := range s {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !valid {
			return false
		}
	}
	return true
}

func fullOID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		valid := r >= '0' && r <= '9' || r >= 'a' && r <= 'f'
		if !valid {
			return false
		}
	}
	return true
}

// ParseSiblingProvisionDocument extracts the strict, root-only declaration.
func ParseSiblingProvisionDocument(data []byte, isJSON bool) (SiblingProvision, bool, error) {
	if isJSON {
		root, counts, err := parseJSONObjectCounts(data)
		if err != nil {
			return SiblingProvision{}, false, err
		}
		if counts["branch"] > 1 {
			return SiblingProvision{}, false, fmt.Errorf("duplicate JSON key %q", "branch")
		}
		branch, ok := root["branch"]
		if !ok || len(bytes.TrimSpace(branch)) == 0 {
			return SiblingProvision{}, false, nil
		}
		fields, err := parseJSONObjectUnique(branch)
		if err != nil {
			return SiblingProvision{}, false, fmt.Errorf("branch must be an object")
		}
		raw, ok := fields["siblingProvision"]
		if !ok {
			return SiblingProvision{}, false, nil
		}
		if err := validateSiblingProvisionJSON(raw); err != nil {
			return SiblingProvision{}, false, err
		}
		var p SiblingProvision
		if err := json.Unmarshal(raw, &p); err != nil {
			return p, false, fmt.Errorf("parse branch.siblingProvision: %w", err)
		}
		return p, true, ValidateSiblingProvision(p)
	}
	if err := RejectMultiDocumentYAML(data); err != nil {
		return SiblingProvision{}, false, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return SiblingProvision{}, false, err
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return SiblingProvision{}, false, nil
	}
	if mappingKeyCount(root.Content[0], "branch") > 1 {
		return SiblingProvision{}, false, fmt.Errorf("duplicate YAML key branch")
	}
	branch := mappingValue(root.Content[0], "branch")
	if branch == nil || branch.Kind != yaml.MappingNode {
		return SiblingProvision{}, false, nil
	}
	if mappingKeyCount(branch, "siblingProvision") > 1 {
		return SiblingProvision{}, false, fmt.Errorf("duplicate YAML key %q", "siblingProvision")
	}
	node := mappingValue(branch, "siblingProvision")
	if node == nil {
		return SiblingProvision{}, false, nil
	}
	if err := validateSiblingProvisionYAML(node); err != nil {
		return SiblingProvision{}, false, err
	}
	var p SiblingProvision
	if err := node.Decode(&p); err != nil {
		return p, false, fmt.Errorf("parse branch.siblingProvision: %w", err)
	}
	return p, true, ValidateSiblingProvision(p)
}

func validateSiblingProvisionJSON(raw []byte) error {
	fields, err := parseJSONObjectUnique(raw)
	if err != nil {
		return fmt.Errorf("branch.siblingProvision must be an object")
	}
	if len(fields) != 2 || fields["version"] == nil || fields["siblings"] == nil {
		return fmt.Errorf("branch.siblingProvision has unknown or missing fields")
	}
	var siblings []json.RawMessage
	if err := json.Unmarshal(fields["siblings"], &siblings); err != nil {
		return fmt.Errorf("branch.siblingProvision.siblings must be an array")
	}
	if len(siblings) > 32 {
		return fmt.Errorf("branch.siblingProvision supports at most 32 siblings")
	}
	for _, raw := range siblings {
		f, err := parseJSONObjectUnique(raw)
		if err != nil {
			return fmt.Errorf("sibling must be an object")
		}
		for key := range f {
			if key != "name" && key != "remote" && key != "path" && key != "sourceBranch" && key != "revision" {
				return fmt.Errorf("sibling has unknown field %q", key)
			}
		}
		if f["name"] == nil || f["remote"] == nil || f["path"] == nil || f["revision"] == nil {
			return fmt.Errorf("sibling has missing fields")
		}
	}
	return nil
}

func validateSiblingProvisionYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode || mappingKeyCount(node, "version") != 1 || mappingKeyCount(node, "siblings") != 1 || len(node.Content) != 4 {
		return fmt.Errorf("branch.siblingProvision has unknown or missing fields")
	}
	siblings := mappingValue(node, "siblings")
	if siblings == nil || siblings.Kind != yaml.SequenceNode || len(siblings.Content) > 32 {
		return fmt.Errorf("branch.siblingProvision.siblings must contain at most 32 entries")
	}
	for _, entry := range siblings.Content {
		if entry.Kind != yaml.MappingNode {
			return fmt.Errorf("sibling must be an object")
		}
		allowed := map[string]bool{"name": true, "remote": true, "path": true, "sourceBranch": true, "revision": true}
		for i := 0; i < len(entry.Content); i += 2 {
			key := entry.Content[i].Value
			if !allowed[key] || mappingKeyCount(entry, key) > 1 {
				return fmt.Errorf("sibling has unknown or duplicate field %q", key)
			}
		}
		if mappingValue(entry, "name") == nil || mappingValue(entry, "remote") == nil || mappingValue(entry, "path") == nil || mappingValue(entry, "revision") == nil {
			return fmt.Errorf("sibling has missing fields")
		}
	}
	return nil
}
