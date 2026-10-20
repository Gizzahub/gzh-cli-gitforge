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
	"path"
)

const siblingInstructionFileLimit int64 = 1 << 20

type siblingInstruction struct {
	data []byte
	mode int64
}

// Agent instruction aliases are data, not build dependencies. Materialize root
// aliases to another regular root instruction file and nested CLAUDE.md or
// GEMINI.md aliases to their directory's regular AGENTS.md. All other symlinks
// remain subject to the archive extractor's rejection policy.
func materializeSiblingAgentAliases(ctx context.Context, data []byte) ([]byte, error) {
	regular, err := collectSiblingRootInstructions(ctx, data)
	if err != nil {
		return nil, err
	}
	return rewriteSiblingArchive(ctx, data, regular)
}

func collectSiblingRootInstructions(ctx context.Context, data []byte) (map[string]siblingInstruction, error) {
	targets, err := siblingInstructionTargets(ctx, data)
	if err != nil {
		return nil, err
	}
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
		if entries > siblingArchiveEntryLimit {
			return nil, fmt.Errorf("sibling archive has too many entries")
		}
		if err := validateSiblingArchiveEntry(header); err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if !siblingInstructionName(header.Name) {
			continue
		}
		canonical, err := safeArchivePath(header.Name, false)
		if err != nil {
			return nil, err
		}
		if !targets[canonical] {
			continue
		}
		if header.Size > siblingInstructionFileLimit {
			return nil, fmt.Errorf("sibling instruction file exceeds size limit")
		}
		payload, err := io.ReadAll(io.LimitReader(reader, header.Size))
		if err != nil {
			return nil, err
		}
		regular[canonical] = siblingInstruction{data: payload, mode: header.Mode}
	}
}

// siblingInstructionTargets validates instruction entry identity before reading
// payloads. Root instruction files retain the historical size cap. A nested
// instruction file is read and capped only when an accepted nested alias names
// it as its exact AGENTS.md target; unrelated nested files stay on the generic
// archive policy.
func siblingInstructionTargets(ctx context.Context, data []byte) (map[string]bool, error) {
	targets := map[string]bool{"AGENTS.md": true, "CLAUDE.md": true, "GEMINI.md": true}
	seen := map[string]bool{}
	reader := tar.NewReader(bytes.NewReader(data))
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return targets, nil
		}
		if err != nil {
			return nil, err
		}
		entries++
		if entries > siblingArchiveEntryLimit {
			return nil, fmt.Errorf("sibling archive has too many entries")
		}
		if err := validateSiblingArchiveEntry(header); err != nil {
			return nil, err
		}
		if !siblingInstructionName(header.Name) {
			continue
		}
		canonical, err := safeArchivePath(header.Name, false)
		if err != nil {
			return nil, err
		}
		if seen[canonical] {
			return nil, fmt.Errorf("sibling archive shadows instruction entry %q", header.Name)
		}
		seen[canonical] = true
		if header.Typeflag != tar.TypeSymlink {
			continue
		}
		target, alias, err := siblingInstructionAliasTarget(header)
		if err != nil {
			return nil, err
		}
		if alias {
			targets[target] = true
		}
	}
}

func rewriteSiblingArchive(ctx context.Context, data []byte, regular map[string]siblingInstruction) ([]byte, error) {
	return rewriteSiblingArchiveWithLimit(ctx, data, regular, siblingArchiveLimit)
}

func rewriteSiblingArchiveWithLimit(ctx context.Context, data []byte, regular map[string]siblingInstruction, limit int64) ([]byte, error) {
	var output bytes.Buffer
	writer := tar.NewWriter(&limitedSiblingArchiveWriter{writer: &output, remaining: limit})
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
	if header.Typeflag == tar.TypeSymlink && siblingInstructionName(header.Name) {
		targetPath, alias, err := siblingInstructionAliasTarget(header)
		if err != nil {
			return err
		}
		if alias {
			return writeSiblingInstructionAlias(writer, header, targetPath, regular)
		}
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := io.CopyN(writer, reader, header.Size)
	return err
}

func writeSiblingInstructionAlias(writer *tar.Writer, header *tar.Header, targetPath string, regular map[string]siblingInstruction) error {
	target, exists := regular[targetPath]
	if !exists {
		return fmt.Errorf("sibling instruction alias must target a regular instruction file")
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
	if header.Size < 0 || header.Size > siblingArchiveLimit {
		return fmt.Errorf("sibling archive entry %q has invalid size", header.Name)
	}
	return nil
}

func rootInstructionName(name string) bool {
	return name == "AGENTS.md" || name == "CLAUDE.md" || name == "GEMINI.md"
}

func siblingInstructionName(name string) bool {
	return rootInstructionName(path.Base(name))
}

// siblingInstructionAliasTarget applies the narrow sibling-only compatibility
// contract. Root aliases retain their existing target flexibility. Nested
// aliases must use the exact relative AGENTS.md target in the same directory.
func siblingInstructionAliasTarget(header *tar.Header) (targetPath string, isAlias bool, err error) {
	aliasPath, err := safeArchivePath(header.Name, false)
	if err != nil {
		return "", false, err
	}
	dir, base := path.Dir(aliasPath), path.Base(aliasPath)
	if dir == "." {
		if !rootInstructionName(base) || !rootInstructionName(header.Linkname) {
			return "", false, fmt.Errorf("sibling root instruction alias %q has invalid target %q", header.Name, header.Linkname)
		}
		target, err := safeArchivePath(header.Linkname, false)
		if err != nil || target != header.Linkname {
			return "", false, fmt.Errorf("sibling root instruction alias %q has unsafe target %q", header.Name, header.Linkname)
		}
		return target, true, nil
	}
	if (base != "CLAUDE.md" && base != "GEMINI.md") || header.Linkname != "AGENTS.md" {
		return "", false, fmt.Errorf("sibling nested instruction alias %q must target AGENTS.md in the same directory", header.Name)
	}
	target := path.Join(dir, "AGENTS.md")
	canonicalTarget, err := safeArchivePath(target, false)
	if err != nil || canonicalTarget != target {
		return "", false, fmt.Errorf("sibling nested instruction alias %q has unsafe target %q", header.Name, target)
	}
	return canonicalTarget, true, nil
}

type limitedSiblingArchiveWriter struct {
	writer    io.Writer
	remaining int64
}

func (writer *limitedSiblingArchiveWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > writer.remaining {
		return 0, fmt.Errorf("materialized sibling archive exceeds size limit")
	}
	n, err := writer.writer.Write(data)
	writer.remaining -= int64(n)
	return n, err
}
