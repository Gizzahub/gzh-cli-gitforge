// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"errors"
	"strings"
)

// Run exit codes, shared by every caller that surfaces an integrate run as a
// process-shaped answer. Plain ints, by contract: the values are the CLI's
// published exit codes (pkg/cliutil's ExitReclaimIncomplete and
// ExitSourceMismatch among them), so the classifier can stay engine-side
// without pkg/integrate importing the CLI package that also embeds it.
const (
	RunExitOK                = 0
	RunExitToolError         = 1
	RunExitInterrupted       = 2
	RunExitReclaimIncomplete = 3
	RunExitSourceMismatch    = 4
)

// RunExitCode classifies one integrate run outcome into the exit code the
// `gz-git integrate run` command would answer with. Every caller of the
// engine -- the CLI command and the in-process engine adapter behind the run
// lifecycle -- must classify through this function: two classifiers
// answerable for the same operation is how the engine path and the CLI path
// drift apart. The reclaim-incomplete run (integrated but provider cleanup
// unfinished) is exit 3 whether it arrived as an error-wrapped report or as
// a completed report, because recovery is keyed on that code.
func RunExitCode(report *RunReport, err error) int {
	if err == nil {
		return RunExitOK
	}
	switch {
	case report != nil && report.Integrated:
		return RunExitReclaimIncomplete
	case errors.Is(err, ErrSourceMismatch), errors.Is(err, ErrTargetMismatch):
		return RunExitSourceMismatch
	case errors.Is(err, ErrInterrupted):
		return RunExitInterrupted
	case errors.Is(err, ErrImplicitSourceIsTarget) || strings.Contains(err.Error(), "not ready") || strings.Contains(err.Error(), "--target") || strings.Contains(err.Error(), "integration branch"):
		return RunExitToolError
	default:
		return RunExitInterrupted
	}
}
