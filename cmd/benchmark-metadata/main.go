// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

// Command benchmark-metadata emits the JSON sidecar used by make benchmark-record.
// Its nine arguments are source commit, Go version, Git version, OS, architecture,
// workload, observed timestamp, measurement command, and note, in that order.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/gizzahub/gzh-cli-gitforge/internal/benchmarkreport"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) != 9 {
		return fmt.Errorf("expected nine metadata arguments, got %d", len(args))
	}
	metadata := benchmarkreport.Metadata{
		SourceCommit: args[0], GoVersion: args[1], GitVersion: args[2],
		OS: args[3], Arch: args[4], Workload: args[5], ObservedAt: args[6],
		MeasurementCommand: args[7], Note: args[8],
	}
	if err := json.NewEncoder(output).Encode(metadata); err != nil {
		return fmt.Errorf("encoding metadata: %w", err)
	}
	return nil
}
