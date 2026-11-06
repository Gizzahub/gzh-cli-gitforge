// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/pkg/cliutil"
)

type observedReader struct{ called bool }

func (r *observedReader) Read([]byte) (int, error) {
	r.called = true
	return 0, errors.New("unexpected read")
}

func TestProvisionalPrepareRefusesBeforeReading(t *testing.T) {
	if supportsCapability(taskSyncPrepareV1Capability) {
		t.Fatal("provisional prepare capability is advertised")
	}
	reader := &observedReader{}
	var out bytes.Buffer
	command := newTaskSyncPrepareCommand()
	command.SetIn(reader)
	command.SetOut(&out)
	err := command.RunE(command, nil)
	if cliutil.ExitCodeForError(err) != 2 || reader.called || out.Len() != 0 {
		t.Fatalf("exit=%d reader-called=%v output=%q", cliutil.ExitCodeForError(err), reader.called, out.String())
	}
}
