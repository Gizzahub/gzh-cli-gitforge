// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// FlatRepositories is the neutral representation of the flat repositories
// YAML schema (the `repositories` array format) shared by the workspace and
// reposync CLI loaders. It records only what the document says: optional
// fields use pointers so an omitted value stays distinguishable from an
// explicit zero, and no CLI defaults, inheritance, or provider resolution
// are applied here. Consumers keep their own compatibility policy.
type FlatRepositories struct {
	// Meta information.
	Version  int        `yaml:"version,omitempty"`
	Kind     ConfigKind `yaml:"kind,omitempty"`
	Metadata *Metadata  `yaml:"metadata,omitempty"`

	// Sync settings.
	Strategy             string     `yaml:"strategy"`
	Parallel             *int       `yaml:"parallel"`
	MaxRetries           *int       `yaml:"maxRetries"`
	Resume               bool       `yaml:"resume"`
	DryRun               bool       `yaml:"dryRun"`
	CleanupOrphans       *bool      `yaml:"cleanupOrphans"`
	StrictBranchCheckout bool       `yaml:"strictBranchCheckout"` // default: false (lenient)
	Branch               FlexBranch `yaml:"branch"`
	Roots                []string   `yaml:"roots"`
	Repositories         []FlatRepo `yaml:"repositories"`
}

// FlatRepo is a single entry of the flat repositories list.
type FlatRepo struct {
	Name                 string            `yaml:"name"`
	Description          string            `yaml:"description"` // optional: human-readable description
	Provider             string            `yaml:"provider"`
	URL                  string            `yaml:"url"`
	AdditionalRemotes    map[string]string `yaml:"additionalRemotes"` // additional git remotes (name: url)
	Path                 string            `yaml:"path"`
	Branch               FlexBranch        `yaml:"branch"`               // optional: branch to checkout after clone/update
	StrictBranchCheckout *bool             `yaml:"strictBranchCheckout"` // optional: override global setting (nil = use global)
	Strategy             string            `yaml:"strategy"`
	Enabled              *bool             `yaml:"enabled"`       // optional: if false, exclude from sync (default: true)
	AssumePresent        bool              `yaml:"assumePresent"` // if true, skip clone check (assume already exists)
}

// ParseFlatRepositories decodes data as a flat repositories document and
// performs the format validation shared by every consumer of the schema:
// YAML well-formedness and per-entry URL presence. It applies no defaults
// and resolves no inheritance, so callers decide what an omitted or explicit
// zero value means.
func ParseFlatRepositories(data []byte) (FlatRepositories, error) {
	var doc FlatRepositories
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return FlatRepositories{}, fmt.Errorf("parse flat repositories YAML: %w", err)
	}

	for i := range doc.Repositories {
		if doc.Repositories[i].URL == "" {
			return FlatRepositories{}, fmt.Errorf("repository[%d]: missing URL", i)
		}
	}

	return doc, nil
}
