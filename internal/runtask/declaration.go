// Decoding the repository-root `.gz-git.yaml` that declares where finished
// work lands and which branches reclaim may act on.

package runtask

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StringList accepts either a scalar or a sequence.
//
// The documented shape is a sequence, but a hand-written `integrationBranch:
// master` parses as a scalar and would otherwise fail the whole file — turning
// a repository that has declared its target into one reported as having
// declared nothing, which is the opposite of the truth.
type StringList []string

// UnmarshalYAML implements the scalar-or-sequence tolerance. A mapping is not
// a shape this field has; it reads as no declaration rather than failing the
// whole file, because the other keys in it are still true.
func (l *StringList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var single string
		if err := value.Decode(&single); err != nil {
			return err
		}
		if single != "" {
			*l = StringList{single}
		}
		return nil
	}
	if value.Kind == yaml.SequenceNode {
		var many []string
		if err := value.Decode(&many); err != nil {
			return err
		}
		*l = many
	}
	return nil
}

// First returns the first non-empty entry, or "" when the field declares
// nothing.
//
// Several entries take the first rather than being refused, because that is
// the answer the worktree audit has always given for this field and a second
// answer here would put the disagreement back. An empty string is a
// declaration of nothing, not a declaration of "": a branch with no name
// resolves to no ref, and failing later with an unresolvable name hides which
// of the two happened.
func (l StringList) First() string {
	for _, entry := range l {
		if entry != "" {
			return entry
		}
	}
	return ""
}

// declarationFile is the subset of `.gz-git.yaml` the run lifecycle reads.
type declarationFile struct {
	Branch struct {
		IntegrationBranch StringList `yaml:"integrationBranch"`
		TaskPattern       StringList `yaml:"taskPattern"`
	} `yaml:"branch"`
}

// parseDeclaration decodes the declaration bytes. A malformed file is an
// error: a declaration that cannot be read must not silently read as one that
// declares nothing.
func parseDeclaration(raw []byte) (declarationFile, error) {
	var parsed declarationFile
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return declarationFile{}, fmt.Errorf("parse .gz-git.yaml: %w", err)
	}
	return parsed, nil
}
