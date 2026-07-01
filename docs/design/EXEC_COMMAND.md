# Exec Command Design

## Overview

**Feature**: 전체 repo에 임의 명령을 병렬 실행하는 `gz-git exec`
**Priority**: P2
**Phase**: 후속 릴리스 (0.8.x 후보)
**Status**: Design (2026-07-02 완성도 점검에서 도출 — `reports/2026-07-02-completeness-gap-analysis.md`)

## Problem Statement

gita / mu-repo / myrepos / git-xargs 등 동종 다중 repo 도구는 모두 "모든 repo에서
임의 명령 실행"을 핵심 기능으로 제공한다. gz-git은 git 하위 작업별 커맨드(fetch,
pull, commit 등)는 풍부하지만, 그 목록에 없는 작업(`git gc`, `go mod tidy`,
스크립트 실행 등)을 일괄 수행할 방법이 없다.

## CLI Shape

```bash
gz-git exec [directory] -- <command> [args...]

# 예시
gz-git exec -- git gc --aggressive
gz-git exec ~/mydevbox -d 2 -- go mod tidy
gz-git exec --include "gzh-.*" --format json -- git rev-parse --short HEAD
```

- `--` 구분자 필수: gz-git 플래그와 실행 명령을 명확히 분리
- 기존 bulk 공통 플래그 재사용: `-d/--scan-depth`, `-j/--parallel`,
  `--include/--exclude`, `-f/--format` (default|compact|json|llm), `-n/--dry-run`
- 추가 플래그: `--fail-fast` (첫 실패 시 중단), `--timeout <dur>` (repo당 제한)

## Security Design (CRITICAL)

프로젝트 보안 원칙(`sh -c` 금지)을 그대로 유지한다:

```go
// argv 직접 실행 — 셸 해석 없음
cmd := exec.CommandContext(ctx, args[0], args[1:]...)
cmd.Dir = repoPath
```

- 파이프/글롭/변수 확장은 **의도적으로 미지원**. 필요하면 사용자가 스크립트
  파일을 만들어 `gz-git exec -- ./script.sh`로 실행
- 실행 명령은 사용자가 명시적으로 입력한 것이므로 sanitization 대상이 아니나,
  로그 출력 시 credential 마스킹 규칙은 동일 적용

## Execution Model

1. scanner로 repo 발견 (기존 `pkg/scanner` 재사용)
2. worker pool 병렬 실행 (기존 bulk 실행기 패턴, 기본 `DefaultLocalParallel`)
3. repo별 결과 수집: exit code, stdout/stderr(테일 제한), 소요 시간
4. 포맷터로 출력 — 실패 repo 요약을 마지막에 재출력

## Output (default format)

```text
✓ gzh-cli-core        (120ms)
✗ gzh-cli-gitforge    exit 1: go: module lookup failed...
Summary: 10 repos, 9 ok, 1 failed
```

## Open Questions

1. 비-git 디렉토리도 대상에 포함하는 옵션(`--any-dir`)이 필요한가?
2. 환경변수 주입(`GZ_REPO_NAME`, `GZ_REPO_PATH`)을 제공할 것인가? (git-xargs 스타일)
3. exit code 정책: 하나라도 실패하면 non-zero? (`--fail-fast`와 별개로 기본값 결정 필요)

## Non-Goals

- 셸 문법 지원 (보안 원칙 위배)
- 원격(forge API) 실행 — 로컬 워킹카피 대상 한정
