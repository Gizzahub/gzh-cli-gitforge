// Doctor-side Worktrunk contract tests. Doctor is where an unsupported
// Worktrunk version is refused, before any inventory is trusted: the version
// report runs through the command-runner seam so each refusal names the exact
// report the runtime rejected.

package runtask

import (
	"errors"
	"strings"
	"testing"
)

// TestDoctorRejectsUnsupportedWorktrunkVersion proves the version gate in
// Doctor fails closed: every report that does not carry the contracted
// Worktrunk version -- the retired 0.74.0 this contract was upgraded forward
// from, neighbors on other lines, a report with no version at all, and a
// version command that failed -- answers BLOCKED with the required version
// named and the wt --version evidence recorded.
func TestDoctorRejectsUnsupportedWorktrunkVersion(t *testing.T) {
	cases := []struct {
		name   string
		answer worktrunkStubAnswer
	}{
		{"retired 0.74.0", worktrunkStubAnswer{stdout: "wt 0.74.0\n"}},
		{"older neighboring minor", worktrunkStubAnswer{stdout: "wt 0.79.2\n"}},
		{"short form 0.8.0", worktrunkStubAnswer{stdout: "wt 0.8.0\n"}},
		{"newer unrelated line", worktrunkStubAnswer{stdout: "wt 0.90.0\n"}},
		{"no version in report", worktrunkStubAnswer{stdout: "wt\n"}},
		{"version command failed", worktrunkStubAnswer{exitCode: 1, stderr: "wt: broken install\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := newWorktrunkStubRunner()
			runner.version = tc.answer
			service, _ := newWorktrunkContractRepo(t, runner)
			res := service.Doctor(t.Context())
			if res.Status != TaskExecutionStatusBlocked {
				t.Fatalf("doctor status = %s, want BLOCKED for version report %+v", res.Status, tc.answer)
			}
			if want := "Worktrunk " + worktrunkVersion + " is required"; res.Reason != want {
				t.Fatalf("doctor reason = %q, want %q", res.Reason, want)
			}
			if strings.TrimSpace(res.NextAction) == "" {
				t.Fatal("blocked doctor answer carries no next action")
			}
			seen := false
			for _, d := range res.Diagnostics {
				if d.Command == "wt" && len(d.Args) > 0 && d.Args[0] == "--version" {
					seen = true
				}
			}
			if !seen {
				t.Fatalf("blocked doctor answer did not record the wt --version evidence: %+v", res.Diagnostics)
			}
		})
	}
}

// TestDoctorRejectsVersionsEmbeddingTheContract proves the version gate is an
// exact-report match, not substring containment: every report below embeds
// the contracted version somewhere in its output -- a prerelease, build
// metadata, a longer version on another line, a padded patch, the report on
// the wrong stream -- and containment would have waved every one of them
// through as 0.80.0. None of them is the installed report, so each answers
// BLOCKED naming the required release.
func TestDoctorRejectsVersionsEmbeddingTheContract(t *testing.T) {
	cases := []struct {
		name   string
		answer worktrunkStubAnswer
	}{
		{"prerelease 0.80.0-rc1", worktrunkStubAnswer{stdout: "wt 0.80.0-rc1\n"}},
		{"build metadata 0.80.0+build.7", worktrunkStubAnswer{stdout: "wt 0.80.0+build.7\n"}},
		{"longer version 10.80.0", worktrunkStubAnswer{stdout: "wt 10.80.0\n"}},
		{"padded patch 0.80.05", worktrunkStubAnswer{stdout: "wt 0.80.05\n"}},
		{"report on stderr only", worktrunkStubAnswer{stderr: "wt 0.80.0\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := newWorktrunkStubRunner()
			runner.version = tc.answer
			service, _ := newWorktrunkContractRepo(t, runner)
			res := service.Doctor(t.Context())
			if res.Status != TaskExecutionStatusBlocked {
				t.Fatalf("doctor status = %s, want BLOCKED for version report %+v", res.Status, tc.answer)
			}
			if want := "Worktrunk " + worktrunkVersion + " is required"; res.Reason != want {
				t.Fatalf("doctor reason = %q, want %q", res.Reason, want)
			}
			if strings.TrimSpace(res.NextAction) == "" {
				t.Fatal("blocked doctor answer carries no next action")
			}
		})
	}
}

// TestDoctorRecordsNeverStartedVersionCommand proves a `wt --version` that
// never starts fails closed with the same evidence every other doctor refusal
// carries: the recorded command diagnostic -- not started, exit 127, with the
// start error -- and a next action that names repairing the install instead
// of leaving the operator a bare error string.
func TestDoctorRecordsNeverStartedVersionCommand(t *testing.T) {
	runner := newWorktrunkStubRunner()
	runner.version.err = errors.New("resolve command wt: exec: wt: not found")
	service, _ := newWorktrunkContractRepo(t, runner)
	res := service.Doctor(t.Context())
	if res.Status != TaskExecutionStatusBlocked {
		t.Fatalf("doctor status = %s, want BLOCKED when wt --version never starts", res.Status)
	}
	if !strings.Contains(res.Reason, "wt") {
		t.Fatalf("doctor reason = %q, want the start failure naming the wt command", res.Reason)
	}
	if !strings.Contains(res.NextAction, "Worktrunk") || !strings.Contains(res.NextAction, worktrunkVersion) {
		t.Fatalf("doctor next action = %q, want it to name repairing Worktrunk %s", res.NextAction, worktrunkVersion)
	}
	var evidence *CommandDiagnostics
	for i := range res.Diagnostics {
		if d := res.Diagnostics[i]; d.Command == "wt" && len(d.Args) > 0 && d.Args[0] == "--version" {
			evidence = &res.Diagnostics[i]
		}
	}
	if evidence == nil {
		t.Fatalf("blocked doctor answer did not record the wt --version evidence: %+v", res.Diagnostics)
	}
	if !evidence.NotStarted || evidence.ExitCode != 127 || evidence.Error == "" {
		t.Fatalf("recorded evidence = not-started %t, exit %d, error %q; want the never-started 127 report with its start error", evidence.NotStarted, evidence.ExitCode, evidence.Error)
	}
}
