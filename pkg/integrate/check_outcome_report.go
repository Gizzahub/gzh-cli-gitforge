// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gizzahub/gzh-cli-gitforge/internal/safefs"
)

const maxMakeOutcomeReportBytes = 1 << 20

// makeOutcomeReport is the version-one, target-owned report emitted by a
// repository Makefile. It deliberately has a small closed schema: a report is
// evidence for a baseline decision, not an extensible diagnostics channel.
type makeOutcomeReport struct {
	Version  int                `json:"version"`
	Target   string             `json:"target"`
	Complete bool               `json:"complete"`
	Checks   []makeCheckOutcome `json:"checks"`
}

type makeCheckOutcome struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
}

var makeOutcomeID = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,127}$`)

// readMakeOutcomeReport reads one report without trusting a pathname that may
// be replaced while it is being read. Rooted access prevents symlink escape,
// and metadata checks reject observed replacement or concurrent writes.
func readMakeOutcomeReport(path, target string, exitCode int) (*makeOutcomeReport, error) {
	if exitCode < 0 {
		return nil, fmt.Errorf("invalid make exit code %d", exitCode)
	}
	root, err := safefs.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	name := filepath.Base(path)
	before, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("lstat outcome report: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("outcome report is not a regular non-symlink file")
	}
	f, err := openMakeOutcomeReportFile(root, name)
	if err != nil {
		return nil, fmt.Errorf("open outcome report: %w", err)
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat outcome report: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("outcome report changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxMakeOutcomeReportBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read outcome report: %w", err)
	}
	if len(data) > maxMakeOutcomeReportBytes {
		return nil, fmt.Errorf("outcome report exceeds %d bytes", maxMakeOutcomeReportBytes)
	}
	after, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("final lstat outcome report: %w", err)
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(opened, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("outcome report changed while reading")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("outcome report is not valid UTF-8")
	}

	report, err := parseMakeOutcomeReport(data)
	if err != nil {
		return nil, err
	}
	if report.Target != target {
		return nil, fmt.Errorf("outcome report target %q does not match %q", report.Target, target)
	}
	failed := false
	for _, check := range report.Checks {
		failed = failed || check.Outcome == "fail"
	}
	if (exitCode == 0) != !failed {
		return nil, fmt.Errorf("outcome report conflicts with make exit code %d", exitCode)
	}
	return report, nil
}

func openMakeOutcomeReportFile(root *safefs.Root, name string) (*os.File, error) {
	f, err := root.OpenFile(name, makeOutcomeReportOpenFlags(), 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("outcome report is not a regular file")
	}
	return f, nil
}

func parseMakeOutcomeReport(data []byte) (*makeOutcomeReport, error) {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("invalid outcome report JSON: %w", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid outcome report JSON: %w", err)
	}
	if err := exactKeys(root, "version", "target", "complete", "checks"); err != nil {
		return nil, fmt.Errorf("invalid outcome report: %w", err)
	}
	report := &makeOutcomeReport{}
	if err := json.Unmarshal(root["version"], &report.Version); err != nil {
		return nil, fmt.Errorf("invalid outcome report version: %w", err)
	}
	if err := json.Unmarshal(root["target"], &report.Target); err != nil {
		return nil, fmt.Errorf("invalid outcome report target: %w", err)
	}
	if err := json.Unmarshal(root["complete"], &report.Complete); err != nil {
		return nil, fmt.Errorf("invalid outcome report complete: %w", err)
	}
	var rawChecks []json.RawMessage
	if err := json.Unmarshal(root["checks"], &rawChecks); err != nil {
		return nil, fmt.Errorf("invalid outcome report checks: %w", err)
	}
	if report.Version != 1 || !report.Complete || (report.Target != "check" && report.Target != "lint") || len(rawChecks) == 0 {
		return nil, fmt.Errorf("outcome report must be complete version 1 for check or lint with checks")
	}
	ids := make(map[string]struct{}, len(rawChecks))
	for _, raw := range rawChecks {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("invalid outcome check: %w", err)
		}
		if err := exactKeys(item, "id", "outcome"); err != nil {
			return nil, fmt.Errorf("invalid outcome check: %w", err)
		}
		var check makeCheckOutcome
		if err := json.Unmarshal(item["id"], &check.ID); err != nil {
			return nil, fmt.Errorf("invalid outcome check id: %w", err)
		}
		if err := json.Unmarshal(item["outcome"], &check.Outcome); err != nil {
			return nil, fmt.Errorf("invalid outcome check outcome: %w", err)
		}
		if !makeOutcomeID.MatchString(check.ID) || (check.Outcome != "pass" && check.Outcome != "fail") {
			return nil, fmt.Errorf("invalid outcome check %q", check.ID)
		}
		if _, exists := ids[check.ID]; exists {
			return nil, fmt.Errorf("duplicate outcome check id %q", check.ID)
		}
		ids[check.ID] = struct{}{}
		report.Checks = append(report.Checks, check)
	}
	return report, nil
}

func exactKeys(values map[string]json.RawMessage, wanted ...string) error {
	if len(values) != len(wanted) {
		return fmt.Errorf("unexpected or missing fields")
	}
	for _, key := range wanted {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("missing field %q", key)
		}
	}
	return nil
}

// rejectDuplicateJSONKeys walks the entire document before unmarshalling so
// duplicate fields cannot be hidden by encoding/json's last-value-wins rule.
func rejectDuplicateJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(dec); err != nil {
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

func scanJSONValue(dec *json.Decoder) error {
	return scanOutcomeJSONValue(dec, 0)
}

func scanOutcomeJSONValue(dec *json.Decoder, depth int) error {
	if depth > 8 {
		return fmt.Errorf("outcome report exceeds schema nesting limit")
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
			if err := scanOutcomeJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := scanOutcomeJSONValue(dec, depth+1); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

// compareMakeOutcomes implements the V1 append-only baseline contract.
func compareMakeOutcomes(base, branch *makeOutcomeReport) BaselineResult {
	if base == nil || branch == nil || base.Target != branch.Target {
		return BaselineResult{Status: BaselineFail, Reason: "outcome reports are not comparable"}
	}
	baseChecks := make(map[string]string, len(base.Checks))
	branchChecks := make(map[string]string, len(branch.Checks))
	for _, check := range base.Checks {
		baseChecks[check.ID] = check.Outcome
	}
	for _, check := range branch.Checks {
		branchChecks[check.ID] = check.Outcome
	}
	var blocks []string
	improved := 0
	for id, baseOutcome := range baseChecks {
		branchOutcome, exists := branchChecks[id]
		if !exists {
			blocks = append(blocks, "check removed: "+id)
			continue
		}
		if baseOutcome == "pass" && branchOutcome == "fail" {
			blocks = append(blocks, "check regressed: "+id)
		}
		if baseOutcome == "fail" && branchOutcome == "pass" {
			improved++
		}
	}
	for id, branchOutcome := range branchChecks {
		if _, exists := baseChecks[id]; !exists && branchOutcome == "fail" {
			blocks = append(blocks, "new failed check: "+id)
		}
	}
	if len(blocks) > 0 {
		sort.Strings(blocks)
		return BaselineResult{Status: BaselineFail, Reason: "outcome report worsened: " + strings.Join(blocks, "; ")}
	}
	if improved > 0 {
		return BaselineResult{Status: BaselinePass, Reason: fmt.Sprintf("outcome report improved %d check(s), no new failures", improved)}
	}
	return BaselineResult{Status: BaselinePass, Reason: "outcome report is non-worsening"}
}
