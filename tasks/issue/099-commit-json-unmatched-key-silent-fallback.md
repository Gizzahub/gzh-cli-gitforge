---
id: ISSUE-099
title: "gz-git commit --json: an unmatched key silently commits the auto message"
type: bug
priority: P2
severity: medium
discovered-in: "infra-ops agent session (gz-git 0.12.0, footer-bearing commit messages)"
discovered-at: 2026-10-22
created: 2026-10-22
---

## Summary

`gz-git commit --json '{"<key>":"<message>"}' --yes` commits the generated
`chore: update …` message, exits 0 and prints no warning when `<key>` matches
no repository. Callers that pass a full message to carry required trailers
(for example a `Model:` footer) lose them without any signal.

`customCommitMessageGenerator` (`cmd/gz-git/cmd/commit.go:244`) tries the
relative path, its basename and the absolute path, then returns `"", nil`
(`cmd/gz-git/cmd/commit.go:262`). `analyzeRepositoryForCommit` treats the empty message as
"no message" and falls back to `generateSimpleCommitMessage`
(`pkg/repository/bulk_commit.go:421`). Keys that matched no repository are
never reported. The same path applies to `--yaml`, piped input and
`-m repo:message`.

## Reproduction

1. Create a scratch directory with one Git repository `sbkube` holding an
   uncommitted file `a.txt`.
1. `gz-git commit -d 2 --format llm <dir>` shows `PATH: sbkube`.
1. `gz-git commit -d 2 --yes --json '{"sbkub":"msg"}' <dir>` (typo in the key).
1. `git -C <dir>/sbkube log -1 --format=%s` prints `chore: update a.txt`; the
   command exited 0 with no warning.

A key equal to the `PATH:` value or its basename commits `msg` as expected.

## Expected vs Actual

- Expected: when an explicit message map is given, a key that matches no
  dirty repository, or a dirty repository that no key covers, fails the run
  before any commit (non-zero exit, unmatched keys named), or at least warns.
- Actual: the auto message is committed, rc=0, no diagnostic.

## Scope

- Decide whether a dirty repository without a key is an error or keeps the
  auto message when other keys are present; unmatched keys should fail either
  way.
- `analyzeRepositoryForCommit` also drops a generator error (`err == nil` check before
  the fallback); surface it instead of silently using the auto message.
