// Copyright (c) 2025 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/doctor"
)

func resetDoctorFlags(t *testing.T) {
	t.Helper()
	prevSkipForge, prevSkipRepo := doctorSkipForge, doctorSkipRepo
	prevDepth, prevFormat := doctorScanDepth, doctorFormat
	prevVerbose := verbose
	t.Cleanup(func() {
		doctorSkipForge, doctorSkipRepo = prevSkipForge, prevSkipRepo
		doctorScanDepth, doctorFormat = prevDepth, prevFormat
		verbose = prevVerbose
	})
	doctorSkipForge = true
	doctorSkipRepo = true
	doctorScanDepth = 1
	doctorFormat = ""
	verbose = false
}

func TestStatusSymbolAndCategoryTitle(t *testing.T) {
	cases := []struct {
		s    doctor.Status
		want string
	}{
		{doctor.StatusOK, "✓"},
		{doctor.StatusWarning, "⚠"},
		{doctor.StatusError, "✗"},
		{doctor.StatusUnreachable, "⊘"},
		{doctor.StatusSkipped, "-"},
		{doctor.Status("unknown"), "?"},
	}
	for _, tc := range cases {
		got := statusSymbol(tc.s)
		if !strings.Contains(got, tc.want) {
			t.Errorf("statusSymbol(%v)=%q want contain %q", tc.s, got, tc.want)
		}
	}

	titles := map[doctor.Category]string{
		doctor.CategorySystem: "System",
		doctor.CategoryConfig: "Configuration",
		doctor.CategoryAuth:   "Authentication",
		doctor.CategoryForge:  "Forge Connectivity",
		doctor.CategoryRepo:   "Repositories",
		doctor.Category("x"):  "x",
	}
	for c, want := range titles {
		if got := categoryTitle(c); got != want {
			t.Errorf("categoryTitle(%v)=%q want %q", c, got, want)
		}
	}
}

func TestPrintDoctorReportAndJSON(t *testing.T) {
	resetDoctorFlags(t)
	report := &doctor.Report{
		Checks: []doctor.CheckResult{
			{Category: doctor.CategorySystem, Status: doctor.StatusOK, Message: "git ok", Detail: "2.40"},
			{Category: doctor.CategoryConfig, Status: doctor.StatusWarning, Message: "no profile"},
			{Category: doctor.CategoryAuth, Status: doctor.StatusError, Message: "no token"},
			{Category: doctor.CategoryForge, Status: doctor.StatusSkipped, Message: "skipped"},
			{Category: doctor.CategoryRepo, Status: doctor.StatusUnreachable, Message: "down"},
		},
		Summary:  doctor.Summary{Total: 5, OK: 1, Warning: 1, Error: 1, Unreachable: 1},
		Duration: time.Millisecond * 12,
	}

	out := captureStdout(t, func() { printDoctorReport(report) })
	if !strings.Contains(out, "gz-git doctor") {
		t.Errorf("report header missing: %q", out)
	}
	if !strings.Contains(out, "System") || !strings.Contains(out, "git ok") {
		t.Errorf("category/message missing: %q", out)
	}

	out = captureStdout(t, func() {
		if err := printDoctorJSON(report); err != nil {
			t.Fatalf("printDoctorJSON: %v", err)
		}
	})
	if !json.Valid([]byte(strings.TrimSpace(out))) {
		t.Errorf("invalid json: %q", out)
	}
}

func TestRunDoctor_SkipForgeAndRepo(t *testing.T) {
	resetDoctorFlags(t)
	doctorSkipForge = true
	doctorSkipRepo = true
	doctorFormat = ""
	out := captureStdout(t, func() {
		if err := runDoctor(doctorCmd, nil); err != nil {
			t.Fatalf("runDoctor: %v", err)
		}
	})
	if !strings.Contains(out, "doctor") && !strings.Contains(out, "Checks") && out == "" {
		t.Logf("doctor output empty-ish: %q", out)
	}

	doctorFormat = "json"
	out = captureStdout(t, func() {
		if err := runDoctor(doctorCmd, nil); err != nil {
			t.Fatalf("runDoctor json: %v", err)
		}
	})
	if !json.Valid([]byte(strings.TrimSpace(out))) {
		// may be pretty-printed multi-line
		var v any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &v); err != nil {
			t.Errorf("doctor json: %v\n%s", err, out)
		}
	}
}
