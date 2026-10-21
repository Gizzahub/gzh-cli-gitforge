---
id: ISSUE-94
title: "Docs say the global config lives in ~/.config/gz-git; macOS reads ~/Library/Application Support/gz-git"
type: docs
priority: P2
severity: medium
discovered-in: "devenv readinessApproval seeding"
discovered-at: 2026-10-20
created: 2026-10-20
---

## Summary

`pkg/config/paths.go` `NewPaths` builds the config directory from Go
`os.UserConfigDir()`. That gives:

- macOS: `$HOME/Library/Application Support/gz-git` (`XDG_CONFIG_HOME` is
  ignored)
- Linux: `${XDG_CONFIG_HOME:-$HOME/.config}/gz-git` (a relative
  `XDG_CONFIG_HOME` is an error)

Several docs still say `~/.config/gz-git`:

- `CLAUDE.md`
- the config guide
- the `paths.go` comments (`ConfigDir`, `ProfilesDir`, `StateDir`, and "It uses
  XDG_CONFIG_HOME if set, otherwise falls back to ~/.config")

## Evidence

On 2026-10-20 a `~/.config/gz-git/config.yaml` with
`integration.readinessApproval: automatic` had no effect on macOS. The setting
took effect only after the file was moved to
`~/Library/Application Support/gz-git/config.yaml`.

## Scope

- Correct the path in these docs and comments, per OS.
- Note that `config.yaml`, `config.yml` and `config.json` are looked up in that
  order and the first one found wins (`supportedExtensions`).

## Resolution

Resolved on 2026-10-22 by master 40b8782e ("docs(config): state the per-OS
global config directory, not ~/.config/gz-git"). The per-OS rule is stated
once in `docs/.claude-context/config-guide.md`; other docs, Go comments, the
`config init --global` help text and the profile template comment use
`<config-dir>` = `os.UserConfigDir()/gz-git`. Historical docs
(`docs/MIGRATION_V2_TO_V3.md`, `reports/`) were left unchanged.
