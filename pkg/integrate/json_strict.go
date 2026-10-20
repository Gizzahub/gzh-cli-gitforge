// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// rejectDuplicateJSONKeys walks the entire document before unmarshalling so
// duplicate fields cannot be hidden by encoding/json's last-value-wins rule.
func rejectDuplicateJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := scanStrictJSONValue(dec, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func scanStrictJSONValue(dec *json.Decoder, depth int) error {
	if depth > 8 {
		return fmt.Errorf("JSON document exceeds schema nesting limit")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]struct{}{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			keys[key] = struct{}{}
			if err := scanStrictJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := scanStrictJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}
