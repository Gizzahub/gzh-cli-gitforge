// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// maxYAMLKeyTraversalNodes bounds how many mappings one alias/merge walk may
// enter, so a hostile document cannot make key discovery unbounded.
const maxYAMLKeyTraversalNodes = 1024

// yamlNodeSuppliesKey and yamlMergeSuppliesKey report whether a mapping, or a
// YAML merge inside it, supplies an authority-bearing key. Direct keys remain
// inspectable; a merged key could otherwise be accepted by the permissive
// project decoder without passing a closed-contract parser's checks.
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
		return fmt.Errorf("YAML alias or merge cycle")
	}
	s.visited++
	if s.visited > maxYAMLKeyTraversalNodes {
		return fmt.Errorf("YAML alias or merge traversal exceeds %d nodes", maxYAMLKeyTraversalNodes)
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
