// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Naming the provider's own refusal from the output the receipt captured.

package runtask

import "strings"

// failingCheckReason names the check that made gz-git refuse the finish and
// its first diagnostic line, taken only from the provider output already in the
// receipt. It returns "" when the output carries nothing to name, so the caller
// keeps the generic reason instead of an invented one.
func failingCheckReason(result CommandResult) string {
	// A FAIL line beats any banner; stderr's first line beats stdout's, since
	// stdout is where progress banners live.
	line := firstMatch(result.Stdout, true)
	if line == "" {
		line = firstMatch(result.Stderr, true)
	}
	if line == "" {
		line = firstMatch(result.Stderr, false)
	}
	if line == "" {
		line = firstMatch(result.Stdout, false)
	}
	if line == "" {
		return ""
	}
	return "integrate-run failed: " + line
}

// provenancePrefixes start the lines gz-git prints before any diagnostic: the
// run provenance line (`integrate: run device=<label> gz-git=<version>`), the
// hint that follows it when the device is undeclared or invalid, and the wait
// notices and non-fatal warnings gz-git prints while a lock or admission is
// busy. They name the device, build or wait, never the failure, so they cannot
// be a finish reason.
var provenancePrefixes = []string{
	"integrate: run device=",
	"integrate: declare the device ",
	"integrate: the declared device name is invalid",
	"integrate: waiting ",
	"integrate: warning: ",
}

func isProvenanceLine(line string) bool {
	for _, prefix := range provenancePrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// firstMatch returns the first "FAIL <check> ..." line without its marker when
// failOnly is set, otherwise the first non-empty line that is not gz-git run
// provenance.
func firstMatch(output string, failOnly bool) string {
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || isProvenanceLine(line) {
			continue
		}
		rest, ok := strings.CutPrefix(line, "FAIL ")
		if ok {
			return strings.TrimSpace(rest)
		}
		if !failOnly {
			return line
		}
	}
	return ""
}

// siblingChangedMarker is gz-git's refusal to release a provisioned sibling
// whose contents no longer match the manifest it was provisioned from.
const siblingChangedMarker = "sibling destination changed"

// siblingReleaseNext names what changed when reclaim refused a sibling
// release, instead of the generic "resolve gz-git diagnostics". The lease is
// intact in that case; the usual cause is build output a run wrote into the
// sibling, such as Cargo's target/. It returns "" for any other provider
// output so the caller keeps its own next step.
func siblingReleaseNext(result CommandResult) string {
	if !strings.Contains(result.Stdout, siblingChangedMarker) && !strings.Contains(result.Stderr, siblingChangedMarker) {
		return ""
	}
	return "a provisioned sibling's contents changed since provisioning (often build output such as target/); in the task worktree run 'gz-git integrate provision siblings inspect', remove only what the run created in the sibling until inspect reports RESUMABLE, then retry run-finish"
}
