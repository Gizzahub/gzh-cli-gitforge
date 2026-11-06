// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// The inbound schema's deepest container is plan.allowed_paths (or
// protected_paths): root object, begin object, plan object, then array.
const maxInboundJSONDepth = 4

// checkInboundJSON rejects ambiguous object keys before typed decoding.
// Protocol field names are lower-case ASCII with optional digits/underscores;
// accepting case variants would let encoding/json map distinct keys to one field.
func checkInboundJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := checkJSONValue(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing data")
	}
	return nil
}

func checkJSONValue(decoder *json.Decoder, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	if depth >= maxInboundJSONDepth {
		return errors.New("JSON nesting exceeds the inbound schema")
	}
	switch delim {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || !validProtocolKey(key) {
				return errors.New("invalid JSON field name")
			}
			if _, exists := keys[key]; exists {
				return errors.New("duplicate JSON field")
			}
			keys[key] = struct{}{}
			if err := checkJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := checkJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

func validProtocolKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
