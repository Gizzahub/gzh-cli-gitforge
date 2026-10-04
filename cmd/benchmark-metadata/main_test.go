// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gizzahub/gzh-cli-gitforge/internal/benchmarkreport"
)

func TestMetadataEscapesStrings(t *testing.T) {
	args := []string{"abc", "go version", "git version", "linux", "amd64", "workload", "2026-10-05T00:00:00Z", "GOWORK=off go test", "quote \" slash \\ newline\n tab\t"}
	var output bytes.Buffer
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	var got benchmarkreport.Metadata
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Note != args[8] || got.MeasurementCommand != args[7] || got.SourceCommit != args[0] {
		t.Fatalf("metadata changed during encoding: %+v", got)
	}
}

func TestMetadataRequiresAllArguments(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("missing arguments accepted")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestMetadataWriteFailure(t *testing.T) {
	if err := run(make([]string, 9), failingWriter{}); err == nil {
		t.Fatal("write failure swallowed")
	}
}
