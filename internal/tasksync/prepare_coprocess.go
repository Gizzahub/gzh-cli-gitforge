// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package tasksync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// RunPrepareCoprocess is a provisional local-only preparation adapter.
// Its wire format is not yet accepted; it never publishes or applies a candidate.
func RunPrepareCoprocess(ctx context.Context, in io.Reader, out io.Writer) error {
	return runPrepareCoprocess(ctx, in, out, PrepareClean)
}

func runPrepareCoprocess(ctx context.Context, in io.Reader, out io.Writer, prepare func(context.Context, Request) Result) error {
	rawInput, err := io.ReadAll(io.LimitReader(in, coprocessMaxLine+1))
	if err != nil || len(rawInput) > coprocessMaxLine {
		return fmt.Errorf("%w: prepare request cannot be read within the size limit", ErrCoprocessBegin)
	}
	var request Request
	decoder := json.NewDecoder(bytes.NewReader(rawInput))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return fmt.Errorf("%w: prepare request is not valid", ErrCoprocessBegin)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("%w: prepare request has trailing data", ErrCoprocessBegin)
	}
	result := prepare(ctx, request)
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("%w: encode prepare result", ErrCoprocessResult)
	}
	if _, err := io.Copy(out, bytes.NewReader(append(raw, '\n'))); err != nil {
		return fmt.Errorf("%w: write prepare result", ErrCoprocessResult)
	}
	return nil
}
