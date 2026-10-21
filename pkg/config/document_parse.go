// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// RejectMultiDocumentYAML reports an error when data carries more than one YAML
// document.
//
// yaml.Unmarshal decodes the first document and discards the rest without a
// word. Every shape check written on top of it — the repository-root field
// readers in this package — then inspects document 1 while documents 2..n
// travel with the commit unread. A config whose first document is an
// unremarkable, passing declaration can carry a second document declaring
// anything at all, and the checks that exist to keep it off a protected branch
// never see it.
//
// A leading "---" is not a second document. A trailing one is, because it opens
// an empty document after the content; both are rejected the same way. The
// declaration is exactly one document, and no .gz-git.yaml in this family uses
// a document marker at all.
func RejectMultiDocumentYAML(data []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var first yaml.Node
	if err := dec.Decode(&first); err != nil {
		if errors.Is(err, io.EOF) {
			// Empty input holds no second document. Callers keep their own
			// handling for it rather than inheriting a wrong error here.
			return nil
		}
		return fmt.Errorf("parse config: %w", err)
	}
	var extra yaml.Node
	err := dec.Decode(&extra)
	switch {
	case err == nil:
		return fmt.Errorf("config must contain exactly one YAML document")
	case errors.Is(err, io.EOF):
		return nil
	default:
		return fmt.Errorf("parse config: %w", err)
	}
}

func parseJSONObjectUnique(data []byte) (map[string]json.RawMessage, error) {
	out, counts, err := parseJSONObjectCounts(data)
	if err != nil {
		return nil, err
	}
	for _, count := range counts {
		if count > 1 {
			return nil, fmt.Errorf("duplicate or invalid JSON key")
		}
	}
	return out, nil
}

func parseJSONObjectCounts(data []byte) (values map[string]json.RawMessage, counts map[string]int, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, nil, fmt.Errorf("expected JSON object")
	}
	values = map[string]json.RawMessage{}
	counts = map[string]int{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("invalid JSON key")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, nil, err
		}
		values[key] = value
		counts[key]++
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return nil, nil, fmt.Errorf("trailing JSON data")
	}
	return values, counts, nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func mappingKeyCount(node *yaml.Node, key string) int {
	count := 0
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			count++
		}
	}
	return count
}
