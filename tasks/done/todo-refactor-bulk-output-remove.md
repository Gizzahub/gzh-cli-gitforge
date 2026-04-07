---
id: todo-refactor-bulk-output-remove
title: Remove duplicated JSON/LLM output logic in bulk commands
type: cleanup
priority: P2
effort: M
parent: P2-refactor-bulk-cmd-json-llm-output-builder
created-at: 2026-04-07T10:44:19+09:00
depends-on: [todo-refactor-bulk-output-create]
---

# Remove duplicated JSON/LLM output logic in bulk commands

Use the newly created `writeBulkOutput` helper to replace the duplicated JSON and LLM formatting functions in all bulk commands and remove the old legacy functions.

## Goal
Consolidate the JSON/LLM format generation code in the 10 bulk commands by pointing them to `writeBulkOutput` and deleting the redundant `displayXxxResultsJSON/LLM` logic.

## Requirements
- Update `fetch`, `pull`, `push`, `status`, `clean`, `commit`, `diff`, `switch`, `update`, `tag` command display layers.
- For each command, replace duplicate functions with `writeBulkOutput`.
- Remove legacy files/functions containing the duplication.
- Verify `make build && make test` passes successfully.
