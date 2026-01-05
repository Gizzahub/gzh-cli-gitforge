# Requirements (Current)

This repository keeps detailed, per-feature requirements in `specs/` and product intent in `docs/00-product/`.
This file is a short index so requirements stay discoverable without duplicating specs.

## Compatibility

- Go: `1.25.1+` (see `go.mod`)
- Git: `2.30+`

## Safety / Security

- Never execute via shell (`sh -c`); always pass arguments as a slice.
- Sanitize Git arguments before execution (see `internal/gitcmd/` and `docs/.claude-context/security-guide.md`).
- Avoid logging credentials (strip tokens from URLs before logging).

## Canonical Sources

- Feature specifications: `specs/00-overview.md`
- Product docs: `docs/00-product/`
- Architecture overview: `ARCHITECTURE.md`

## Archived

- Historical TRD (v1.0, 2025-11-27): `docs/_deprecated/2026-01/REQUIREMENTS_v1.0_2025-11-27.md`

