// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"strings"

	"github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd"
)

// porcelainPushRecord is one machine-readable ref-status record printed by
// git push --porcelain: "<flag>\t<from>:<to>\t<summary>" optionally followed
// by " (<reason>)". The grammar and its tab separators are the ones a real
// Git prints for "!	<sha>:refs/heads/<branch>	[rejected] (stale info)".
type porcelainPushRecord struct {
	flag    byte
	from    string
	to      string
	summary string
	reason  string
}

// porcelainPushStatus is the parsed skeleton of one git push --porcelain
// report: its ref-status records plus the counts of the surrounding "To
// <destination>" header and "Done" footer lines.
type porcelainPushStatus struct {
	records []porcelainPushRecord
	headers int
	footers int
}

// parsePorcelainPushStatus parses stdout of git push --porcelain in the exact
// grammar a real Git prints: a final-newline-terminated report whose first
// line is the "To <destination>" header, then ref-status records, then "Done"
// as the last line. ok is false for anything else — partial or malformed
// output, a blank or smuggled line, a header without a destination, or a
// reordered skeleton — because half-trusted output is ambiguous, not proof.
func parsePorcelainPushStatus(stdout string) (porcelainPushStatus, bool) {
	var status porcelainPushStatus
	if stdout == "" {
		return status, true
	}
	if !strings.HasSuffix(stdout, "\n") {
		// A real report ends with a newline; a capture that lost it may have
		// lost a record's tail too.
		return status, false
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	sawRecord, sawFooter := false, false
	for i, line := range lines {
		if line == "" {
			return status, false
		}
		if porcelainToHeader(line) {
			if sawRecord || sawFooter {
				return status, false
			}
			status.headers++
			continue
		}
		if line == "Done" {
			if sawFooter || i != len(lines)-1 {
				return status, false
			}
			sawFooter = true
			status.footers++
			continue
		}
		record, ok := parsePorcelainPushRecord(line)
		if !ok || sawFooter {
			return status, false
		}
		sawRecord = true
		status.records = append(status.records, record)
	}
	return status, true
}

// porcelainToHeader recognizes the "To <destination>" header line. The
// destination is a non-empty URL or path without padding or embedded tabs;
// an empty, padded, or record-smuggling "To " line is not a header a real Git
// prints and leaves the whole report malformed.
func porcelainToHeader(line string) bool {
	const prefix = "To "
	if !strings.HasPrefix(line, prefix) {
		return false
	}
	destination := line[len(prefix):]
	return destination != "" && strings.TrimSpace(destination) == destination && !strings.Contains(destination, "\t")
}

// parsePorcelainPushRecord parses exactly one porcelain record line: flag,
// tab, "<from>:<to>", tab, summary with an optional " (<reason>)" suffix. A
// missing field separator, a superfluous tab, or a ref pair that is not two
// colon-separated parts is malformed.
func parsePorcelainPushRecord(line string) (porcelainPushRecord, bool) {
	var record porcelainPushRecord
	if len(line) < 4 || line[1] != '\t' {
		return record, false
	}
	record.flag = line[0]
	fields := strings.Split(line[2:], "\t")
	if len(fields) != 2 {
		return record, false
	}
	refs := strings.Split(fields[0], ":")
	if len(refs) != 2 {
		return record, false
	}
	record.from, record.to = refs[0], refs[1]
	record.summary = fields[1]
	if strings.HasSuffix(record.summary, ")") {
		if open := strings.LastIndex(record.summary, " ("); open >= 0 {
			record.reason = record.summary[open+2 : len(record.summary)-1]
			record.summary = record.summary[:open]
		}
	}
	return record, true
}

// isSolePorcelainStaleLeaseRejection reports whether a failed push printed
// exactly one complete porcelain ref-status record and that record is the
// stale-lease rejection of precisely sourceSHA:targetRef. Only stdout is
// consulted: stderr never proves a retryable classification, and any second
// record (another ref, an accepted or mixed update), an unrecognized or
// truncated line, a missing header or footer, or no record at all keeps the
// failure generic.
func isSolePorcelainStaleLeaseRejection(push *gitcmd.Result, sourceSHA, targetRef string) bool {
	if push == nil {
		return false
	}
	status, ok := parsePorcelainPushStatus(push.Stdout)
	if !ok || len(status.records) != 1 || status.headers != 1 || status.footers != 1 {
		return false
	}
	rejection := status.records[0]
	return rejection.flag == '!' &&
		rejection.from == sourceSHA &&
		rejection.to == targetRef &&
		rejection.summary == "[rejected]" &&
		rejection.reason == "stale info"
}
