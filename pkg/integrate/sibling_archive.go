// Copyright (c) 2026 Gizzahub
// SPDX-License-Identifier: MIT

package integrate

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

const siblingInstructionFileLimit int64 = 1 << 20

type siblingInstruction struct {
	data []byte
	mode int64
}

// Agent instruction aliases are data, not build dependencies. Materialize only
// root aliases to another regular root instruction file; all other symlinks
// remain subject to the archive extractor's rejection policy.
func materializeSiblingAgentAliases(ctx context.Context, data []byte) ([]byte, error) {
	regular, err := collectSiblingRootInstructions(ctx, data)
	if err != nil {
		return nil, err
	}
	return rewriteSiblingArchive(ctx, data, regular)
}

func collectSiblingRootInstructions(ctx context.Context, data []byte) (map[string]siblingInstruction, error) {
	regular := map[string]siblingInstruction{}
	reader := tar.NewReader(bytes.NewReader(data))
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return regular, nil
		}
		if err != nil {
			return nil, err
		}
		entries++
		if entries > flowTaskchainArchiveEntryLimit {
			return nil, fmt.Errorf("sibling archive has too many entries")
		}
		if err := validateSiblingArchiveEntry(header); err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg || !rootInstructionName(header.Name) {
			continue
		}
		if header.Size > siblingInstructionFileLimit {
			return nil, fmt.Errorf("sibling instruction file exceeds size limit")
		}
		payload, err := io.ReadAll(io.LimitReader(reader, header.Size))
		if err != nil {
			return nil, err
		}
		regular[header.Name] = siblingInstruction{data: payload, mode: header.Mode}
	}
}

func rewriteSiblingArchive(ctx context.Context, data []byte, regular map[string]siblingInstruction) ([]byte, error) {
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	reader := tar.NewReader(bytes.NewReader(data))
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := writeSiblingArchiveEntry(writer, reader, header, regular); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeSiblingArchiveEntry(writer *tar.Writer, reader io.Reader, header *tar.Header, regular map[string]siblingInstruction) error {
	if err := validateSiblingArchiveEntry(header); err != nil {
		return err
	}
	if header.Typeflag == tar.TypeSymlink && rootInstructionName(header.Name) {
		return writeSiblingInstructionAlias(writer, header, regular)
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := io.CopyN(writer, reader, header.Size)
	return err
}

func writeSiblingInstructionAlias(writer *tar.Writer, header *tar.Header, regular map[string]siblingInstruction) error {
	target, exists := regular[header.Linkname]
	if !exists || !rootInstructionName(header.Linkname) {
		return fmt.Errorf("sibling instruction alias must target a regular root instruction file")
	}
	copyHeader := *header
	copyHeader.Typeflag, copyHeader.Linkname, copyHeader.Mode = tar.TypeReg, "", target.mode
	copyHeader.Size = int64(len(target.data))
	if err := writer.WriteHeader(&copyHeader); err != nil {
		return err
	}
	_, err := writer.Write(target.data)
	return err
}

func validateSiblingArchiveEntry(header *tar.Header) error {
	if header.Size < 0 || header.Size > flowTaskchainArchiveLimit {
		return fmt.Errorf("sibling archive entry %q has invalid size", header.Name)
	}
	return nil
}

func rootInstructionName(name string) bool {
	return name == "AGENTS.md" || name == "CLAUDE.md" || name == "GEMINI.md"
}
