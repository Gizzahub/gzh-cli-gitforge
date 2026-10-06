// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOutcomeReport(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "outcome.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadMakeOutcomeReportRejectsMalformedInputs(t *testing.T) {
	valid := `{"version":1,"target":"check","complete":true,"checks":[{"id":"unit","outcome":"pass"}]}`
	for _, test := range []struct {
		name    string
		content string
		exit    int
	}{
		{"duplicate root", `{"version":1,"version":1,"target":"check","complete":true,"checks":[{"id":"unit","outcome":"pass"}]}`, 0},
		{"duplicate nested", `{"version":1,"target":"check","complete":true,"checks":[{"id":"unit","id":"other","outcome":"pass"}]}`, 0},
		{"missing field", `{"version":1,"target":"check","complete":true}`, 0},
		{"unknown field", `{"version":1,"target":"check","complete":true,"checks":[{"id":"unit","outcome":"pass"}],"extra":true}`, 0},
		{"trailing value", valid + ` true`, 0},
		{"target mismatch", valid, 0},
		{"exit mismatch pass", valid, 1},
		{"exit mismatch fail", `{"version":1,"target":"check","complete":true,"checks":[{"id":"unit","outcome":"fail"}]}`, 0},
		{"invalid id", `{"version":1,"target":"check","complete":true,"checks":[{"id":"Upper","outcome":"pass"}]}`, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := "check"
			if test.name == "target mismatch" {
				target = "lint"
			}
			if _, err := readMakeOutcomeReport(writeOutcomeReport(t, test.content), target, test.exit); err == nil {
				t.Fatal("readMakeOutcomeReport succeeded, want error")
			}
		})
	}
}

func TestReadMakeOutcomeReportRejectsInvalidUTF8OversizeAndSymlink(t *testing.T) {
	t.Run("invalid UTF-8", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "outcome.json")
		if err := os.WriteFile(path, []byte{0xff}, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readMakeOutcomeReport(path, "check", 0); err == nil {
			t.Fatal("readMakeOutcomeReport succeeded, want error")
		}
	})
	t.Run("oversize", func(t *testing.T) {
		path := writeOutcomeReport(t, strings.Repeat(" ", maxMakeOutcomeReportBytes+1))
		if _, err := readMakeOutcomeReport(path, "check", 0); err == nil {
			t.Fatal("readMakeOutcomeReport succeeded, want error")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "real.json")
		if err := os.WriteFile(target, []byte(`{"version":1,"target":"check","complete":true,"checks":[{"id":"unit","outcome":"pass"}]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link.json")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := readMakeOutcomeReport(link, "check", 0); err == nil {
			t.Fatal("readMakeOutcomeReport succeeded, want error")
		}
	})
}

func report(checks ...makeCheckOutcome) *makeOutcomeReport {
	return &makeOutcomeReport{Version: 1, Target: "check", Complete: true, Checks: checks}
}

func TestCompareMakeOutcomes(t *testing.T) {
	pass := makeCheckOutcome{ID: "unit", Outcome: "pass"}
	fail := makeCheckOutcome{ID: "unit", Outcome: "fail"}
	for _, test := range []struct {
		name   string
		base   *makeOutcomeReport
		branch *makeOutcomeReport
		want   BaselineStatus
	}{
		{"unchanged failure allowed", report(fail), report(fail), BaselinePass},
		{"failure improved", report(fail), report(pass), BaselinePass},
		{"regression blocks", report(pass), report(fail), BaselineFail},
		{"retired ID blocks", report(pass), report(makeCheckOutcome{ID: "other", Outcome: "pass"}), BaselineFail},
		{"new pass allowed", report(pass), report(pass, makeCheckOutcome{ID: "new", Outcome: "pass"}), BaselinePass},
		{"new failure blocks", report(pass), report(pass, makeCheckOutcome{ID: "new", Outcome: "fail"}), BaselineFail},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := compareMakeOutcomes(test.base, test.branch)
			if got.Status != test.want {
				t.Fatalf("status = %v, want %v (%s)", got.Status, test.want, got.Reason)
			}
		})
	}
}

func TestReadMakeOutcomeReportAcceptsFailureWithNonzeroExit(t *testing.T) {
	content := `{"version":1,"target":"lint","complete":true,"checks":[{"id":"unit","outcome":"pass"},{"id":"lint","outcome":"fail"}]}`
	got, err := readMakeOutcomeReport(writeOutcomeReport(t, content), "lint", 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "lint" || len(got.Checks) != 2 {
		t.Fatalf("report = %+v", got)
	}
	if _, err := readMakeOutcomeReport(writeOutcomeReport(t, content), "lint", -1); err == nil {
		t.Fatal("negative exit code succeeded")
	}
}
