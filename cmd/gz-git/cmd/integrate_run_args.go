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
// --owner <actor>/<host>/<kind>) are folded into the fields the legacy
// flags carry. Both sets are accepted for one release (TASK-349 step 1);
// the engine and the receipt only ever see these fields.
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

// integrateRunArgsFromFlags resolves the run flags. A new argument and the
// legacy flag it replaces are mutually exclusive: there is no precedence
// rule, so two disagreeing values cannot both be accepted.
func integrateRunArgsFromFlags(cmd *cobra.Command) (integrateRunArgs, error) {
	expectTarget, err := expectedTargetFromFlag(cmd, integrateRunExpectTarget)
	if err != nil {
		return integrateRunArgs{}, err
	}
	args := integrateRunArgs{
		Target:       integrateRunTarget,
		ExpectSource: integrateRunExpectSource,
		ExpectTarget: expectTarget,
		ExecutionID:  integrateRunExecutionID,
		Generation:   integrateRunGeneration,
		OwnerActor:   integrateRunOwnerActor,
		OwnerHost:    integrateRunOwnerHost,
		OwnerKind:    integrateRunOwnerKind,
	}
	changed := cmd.Flags().Changed

	if ref, sha, pinned, err := splitPinnedTarget(integrateRunTarget); err != nil {
		return integrateRunArgs{}, err
	} else if pinned {
		if changed("expect-target") {
			return integrateRunArgs{}, fmt.Errorf("--target <ref>@<sha> and --expect-target are mutually exclusive")
		}
		args.Target, args.ExpectTarget = ref, sha
	}

	if changed("source") {
		if strings.TrimSpace(integrateRunSource) == "" {
			return integrateRunArgs{}, fmt.Errorf("--source must not be empty when supplied")
		}
		if changed("expect-source") {
			return integrateRunArgs{}, fmt.Errorf("--source and --expect-source are mutually exclusive")
		}
		args.ExpectSource = integrateRunSource
	}

	if changed("record") {
		id, generation, err := recordFromFlag(cmd, integrateRunRecord)
		if err != nil {
			return integrateRunArgs{}, err
		}
		args.ExecutionID, args.Generation = id, generation
	}

	if changed("owner") {
		if changed("owner-actor") || changed("owner-host") || changed("owner-kind") {
			return integrateRunArgs{}, fmt.Errorf("--owner and --owner-actor/--owner-host/--owner-kind are mutually exclusive")
		}
		parts := strings.Split(integrateRunOwner, "/")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return integrateRunArgs{}, fmt.Errorf("--owner must be <actor>/<host>/<kind>, got %q", integrateRunOwner)
		}
		args.OwnerActor, args.OwnerHost, args.OwnerKind = parts[0], parts[1], parts[2]
	}
	return args, nil
}

// recordFromFlag parses a supplied --record <id>@<generation> and refuses it
// next to the legacy --execution-id/--generation flags it replaces. The
// generation is whatever follows the last '@', so an id may itself contain
// one. integrate run, cleanup and receipt share it.
func recordFromFlag(cmd *cobra.Command, record string) (id, generation string, err error) {
	if cmd.Flags().Changed("execution-id") || cmd.Flags().Changed("generation") {
		return "", "", fmt.Errorf("--record and --execution-id/--generation are mutually exclusive")
	}
	id, generation, ok := cutLast(record, "@")
	if !ok || id == "" || generation == "" {
		return "", "", fmt.Errorf("--record must be <id>@<generation>, got %q", record)
	}
	return id, generation, nil
}

// executionRecordFromFlags resolves the record for the commands that address
// one existing execution (cleanup, receipt): exactly one of --record or the
// legacy --execution-id plus --generation pair, and the legacy pair complete.
func executionRecordFromFlags(cmd *cobra.Command, record, executionID, generation string) (id, gen string, err error) {
	if cmd.Flags().Changed("record") {
		return recordFromFlag(cmd, record)
	}
	if executionID == "" && generation == "" {
		return "", "", fmt.Errorf("--record <id>@<generation> (or --execution-id with --generation) is required")
	}
	if executionID == "" || generation == "" {
		return "", "", fmt.Errorf("--execution-id and --generation are supplied together and must not be empty")
	}
	return executionID, generation, nil
}

// splitPinnedTarget reads --target as <ref>@<sha> only when the text after
// the last '@' is a full commit SHA. A hex suffix of abbreviated-SHA length
// (7 or more, Git's minimum) is refused rather than taken as part of a
// branch name, because it is almost certainly a truncated SHA. Any other
// value, release@2026 included, stays the plain target ref the legacy flag
// always accepted.
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
