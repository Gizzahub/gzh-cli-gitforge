// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// integrateRunArgs is the run invocation after the ADR-0161 arguments
// (--source, --target <ref>@<sha>, --record <id>@<generation>,
// --owner <actor>/<host>/<kind>) are folded into the fields the engine and
// the receipt consume. The engine and the receipt only ever see these fields.
type integrateRunArgs struct {
	Target       string
	ExpectSource string
	ExpectTarget string
	ExecutionID  string
	Generation   string
	OwnerActor   string
	OwnerHost    string
	OwnerKind    string
}

// integrateRunArgsFromFlags resolves the run flags into the engine fields.
func integrateRunArgsFromFlags(cmd *cobra.Command) (integrateRunArgs, error) {
	args := integrateRunArgs{
		Target: integrateRunTarget,
	}

	if ref, sha, pinned, err := splitPinnedTarget(integrateRunTarget); err != nil {
		return integrateRunArgs{}, err
	} else if pinned {
		args.Target, args.ExpectTarget = ref, sha
	}

	if cmd.Flags().Changed("source") {
		if strings.TrimSpace(integrateRunSource) == "" {
			return integrateRunArgs{}, fmt.Errorf("--source must not be empty when supplied")
		}
		args.ExpectSource = integrateRunSource
	}

	if cmd.Flags().Changed("record") {
		id, generation, err := recordFromFlag(integrateRunRecord)
		if err != nil {
			return integrateRunArgs{}, err
		}
		args.ExecutionID, args.Generation = id, generation
	}

	if cmd.Flags().Changed("owner") {
		parts := strings.Split(integrateRunOwner, "/")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return integrateRunArgs{}, fmt.Errorf("--owner must be <actor>/<host>/<kind>, got %q", integrateRunOwner)
		}
		args.OwnerActor, args.OwnerHost, args.OwnerKind = parts[0], parts[1], parts[2]
	}
	return args, nil
}

// recordFromFlag parses a supplied --record <id>@<generation>. The generation
// is whatever follows the last '@', so an id may itself contain one.
// integrate run, cleanup and receipt share it.
func recordFromFlag(record string) (id, generation string, err error) {
	id, generation, ok := cutLast(record, "@")
	if !ok || id == "" || generation == "" {
		return "", "", fmt.Errorf("--record must be <id>@<generation>, got %q", record)
	}
	return id, generation, nil
}

// executionRecordFromFlags resolves the record for the commands that address
// one existing execution (cleanup, receipt): --record <id>@<generation> is
// required.
func executionRecordFromFlags(record string) (id, gen string, err error) {
	if strings.TrimSpace(record) == "" {
		return "", "", fmt.Errorf("--record <id>@<generation> is required")
	}
	return recordFromFlag(record)
}

// splitPinnedTarget reads --target as <ref>@<sha> only when the text after
// the last '@' is a full commit SHA. A hex suffix of abbreviated-SHA length
// (7 or more, Git's minimum) is refused rather than taken as part of a
// branch name, because it is almost certainly a truncated SHA. Any other
// value, release@2026 included, stays the plain target ref.
func splitPinnedTarget(value string) (ref, sha string, pinned bool, err error) {
	ref, sha, ok := cutLast(value, "@")
	if !ok || len(sha) < minAbbrevSHA || !isHex(sha) {
		return "", "", false, nil
	}
	if len(sha) != 40 && len(sha) != 64 {
		return "", "", false, fmt.Errorf("--target %q: the commit after '@' must be a full SHA", value)
	}
	if ref == "" {
		return "", "", false, fmt.Errorf("--target %q: the ref before '@' must not be empty", value)
	}
	return ref, sha, true, nil
}

const minAbbrevSHA = 7

func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
