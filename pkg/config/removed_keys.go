// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// noRepositoryCode is the reason shared by every key that configured the
// verification gz-git used to run itself.
const noRepositoryCode = "gz-git no longer builds, runs, or lints repository code; " +
	"verification belongs to the task runtime (CE `ce task gate` and the repository's own checks), " +
	"which must pass before gz-git is called"

const noGateHostSlot = "it bounded only the removed gate host slot; " +
	"git mutation locks wait until the lock is free or the command is interrupted"

// removedKeyReasons names configuration keys gz-git no longer reads, keyed by
// the section that held them, with the reason shown to the operator. gz-git
// does not build, execute, or lint repository code itself, so the legacy make
// gate, the readiness contract, and every declaration that tuned either are
// gone. A document that still carries one fails to load instead of being
// silently ignored: a stale contract, budget, or profile left in place would
// otherwise read as a gate that still runs.
var removedKeyReasons = map[string]map[string]string{
	"branch": {
		"makeTimeout":       noRepositoryCode,
		"makeOutcomeReport": noRepositoryCode,
		"prepareProfile":    noRepositoryCode,
		"readiness":         noRepositoryCode,
		"prepareHighLoad":   noRepositoryCode,
		"resourceHighLoad":  noRepositoryCode,
		"lockWait":          noGateHostSlot,
	},
	"integration": {
		"prepareProfile":    noRepositoryCode,
		"readinessApproval": noRepositoryCode,
	},
}

// ReadConfigFile reads a gz-git configuration file and rejects it when it
// still declares a removed key. Every loader of .gz-git.yaml, a workspace
// config, or the global config reads through this function (or passes bytes
// it read another way to RejectRemovedKeys), so no loader can accept a
// removed key that another one rejects. The read error is returned unwrapped
// so callers keep their os.IsNotExist checks and their own wording.
func ReadConfigFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- callers pass the config path they already selected.
	if err != nil {
		return nil, err
	}
	if err := RejectRemovedKeys(data, path); err != nil {
		return nil, err
	}
	return data, nil
}

// RejectRemovedKeys applies the removed-key guard to configuration bytes a
// loader read itself, for example through an os.Root or from stdin. path
// names the source in the error and selects JSON decoding for a .json file;
// any other name is decoded as YAML.
func RejectRemovedKeys(data []byte, path string) error {
	return rejectRemovedConfigKeys(data, strings.EqualFold(filepath.Ext(path), ".json"), path)
}

// rejectRemovedConfigKeys fails when data still declares a removed key
// anywhere in the document: at the root, under workspaces, or under
// profiles. The error names the dotted key path and path, the file the key
// must be deleted from. Only the direct children of a branch or integration
// mapping are inspected, and a removed key is reported whatever it holds:
// branch.readiness is rejected as a whole, not field by field.
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
