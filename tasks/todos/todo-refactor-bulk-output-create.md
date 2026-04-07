---
id: todo-refactor-bulk-output-create
title: Build shared writeBulkOutput helper for JSON/LLM formats
type: refactor
priority: P2
effort: S
parent: P2-refactor-bulk-cmd-json-llm-output-builder
created-at: 2026-04-07T10:44:19+09:00
depends-on: []
---

# Build shared writeBulkOutput helper for JSON/LLM formats

Build a new `writeBulkOutput` helper function in `cmd/gz-git/cmd/bulk_common.go` to handle the generic JSON and LLM output formatting for bulk commands.

## Goal
Implement the central rendering logic that will later replace duplicated `displayXxxResultsJSON()` and `displayXxxResultsLLM()` calls across all bulk commands.

## Requirements
- Create `writeBulkOutput(format string, output any)` in `cmd/gz-git/cmd/bulk_common.go` (or equivalent file).
- Implement `json` formatting dispatch.
- Implement `llm` formatting dispatch.
