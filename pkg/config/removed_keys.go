// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// removedKeyReasons names configuration keys gz-git no longer reads, keyed by
// the section that held them, with the reason shown to the operator. gz-git
// does not build, execute, or lint repository code itself, so the legacy make
// gate and every declaration that tuned it are gone. A document that still
// carries one fails to load instead of being silently ignored: a stale budget
// or profile left in place would otherwise read as a gate that still runs.
var removedKeyReasons = map[string]map[string]string{
	"branch": {
		"makeTimeout":       "gz-git no longer runs make",
		"makeOutcomeReport": "gz-git no longer runs make",
		"prepareProfile":    "gz-git no longer runs make; a readiness contract selects preparation with branch.readiness.prepareProfile",
	},
	"integration": {
		"prepareProfile": "gz-git no longer runs make; a readiness contract selects preparation with branch.readiness.prepareProfile",
	},
}

// rejectRemovedConfigKeys fails when data still declares a removed key
// anywhere in the document: at the root, under workspaces, or under
// profiles. The error names the dotted key path and path, the file the key
// must be deleted from. Only the direct children of a branch or integration
// mapping are inspected, so branch.readiness.prepareProfile (a live readiness
// contract field) is never mistaken for the removed branch.prepareProfile.
// A document that does not parse is left to the caller's own decoder, which
// reports the syntax error in its usual form.
func rejectRemovedConfigKeys(data []byte, isJSON bool, path string) error {
	var doc any
	if isJSON {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil //nolint:nilerr // the caller's decoder reports the parse error
		}
	} else if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil //nolint:nilerr // the caller's decoder reports the parse error
	}
	var found []string
	collectRemovedKeys(doc, nil, &found)
	if len(found) == 0 {
		return nil
	}
	sort.Strings(found)
	key := found[0]
	section := key[:strings.LastIndex(key, ".")]
	section = section[strings.LastIndex(section, ".")+1:]
	reason := removedKeyReasons[section][key[strings.LastIndex(key, ".")+1:]]
	return fmt.Errorf("%s was removed: %s; delete it from %s", key, reason, path)
}

func collectRemovedKeys(node any, prefix []string, found *[]string) {
	m, ok := asStringMap(node)
	if !ok {
		return
	}
	section := ""
	if len(prefix) > 0 {
		section = prefix[len(prefix)-1]
	}
	for key, value := range m {
		if _, removed := removedKeyReasons[section][key]; removed {
			*found = append(*found, strings.Join(append(append([]string(nil), prefix...), key), "."))
			continue
		}
		collectRemovedKeys(value, append(append([]string(nil), prefix...), key), found)
	}
}

// asStringMap normalizes the mapping shapes encoding/json and yaml.v3 decode
// into an untyped value. A YAML mapping with non-string keys keeps only its
// string keys, which are the only ones a configuration key can be.
func asStringMap(node any) (map[string]any, bool) {
	switch m := node.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for key, value := range m {
			if name, ok := key.(string); ok {
				out[name] = value
			}
		}
		return out, true
	default:
		return nil, false
	}
}
