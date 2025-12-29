# gz-git bulk commit 기능 요청

## 배경

### 문제 상황

Multi-repo 환경(devbox, monorepo with nested repos)에서 여러 저장소를 한번에 커밋해야 하는 상황이 자주 발생합니다.

예시 구조:

```
agent-mesh-devbox/
├── .git/                         ← 루트 저장소
├── agent-mesh-cli/.git/          ← 독립 저장소
├── agent-mesh-connectors/.git/
├── agent-mesh-engine/.git/
├── agent-mesh-flow-dsl/.git/
├── agent-mesh-saas-backend/.git/
└── agent-mesh-webui/.git/
```

현재 각 저장소에 변경사항이 있을 때:

- 각 디렉토리로 이동해서 개별 커밋해야 함
- 10개 저장소 × 수동 커밋 = 매우 번거로움

### 현재 gz-git 기능

| 명령어               | 기능                         | 한계          |
| -------------------- | ---------------------------- | ------------- |
| `gz-git status`      | 여러 저장소 상태 한번에 확인 | 조회만 가능   |
| `gz-git commit auto` | 자동 커밋 메시지 생성 + 커밋 | 단일 저장소만 |

`gz-git status`로 dirty 저장소를 한번에 확인할 수 있지만, 커밋은 각각 해야 합니다.

## 요청 기능

`gz-git commit bulk` 명령 추가

### 핵심 설계 원칙

**배치 처리**: 한번에 수집 → 한번에 확인 → 한번에 실행

이유:

- **토큰/비용 절감**: LLM 호출 시 배치로 처리
- **속도 향상**: 병렬 처리로 빠른 실행
- **UX 개선**: 한 화면에서 전체 확인, 한번에 입력

❌ 안 좋은 방식 (하나씩 순차 처리):

```
저장소1 diff 확인 → 메시지 입력 → 커밋
저장소2 diff 확인 → 메시지 입력 → 커밋
저장소3 diff 확인 → 메시지 입력 → 커밋
...
```

✅ 좋은 방식 (배치 처리):

```
Phase 1: 모든 저장소 diff 한번에 수집 (병렬)
Phase 2: 전체 목록 + 제안 메시지 한번에 출력
Phase 3: 사용자가 한번에 검토/수정
Phase 4: 모든 저장소 한번에 커밋 (병렬)
```

## 상세 동작

### Phase 1: 정보 수집 (병렬)

모든 dirty 저장소의 정보를 병렬로 수집:

- 변경 파일 목록
- diff 요약 (추가/삭제 라인 수)
- 제안 커밋 메시지 (기존 `commit auto` 로직 활용)

### Phase 2: 한번에 출력

```
$ gz-git commit bulk

Scanning repositories (depth: 1)...
Found 6 dirty repositories

=== Bulk Commit Preview ===

# | Repository              | Branch | Files | +/-      | Message (suggested)
--|-------------------------|--------|-------|----------|------------------------------------
1 | agent-mesh-cli          | master |     1 |   +10/-5 | fix(cli): update main entry
2 | agent-mesh-connectors   | master |     4 |   +45/-20| refactor(connectors): update impls
3 | agent-mesh-engine       | master |     3 |   +30/-15| fix(engine): scheduler updates
4 | agent-mesh-flow-dsl     | master |     2 |   +20/-8 | refactor(dsl): parser improvements
5 | agent-mesh-saas-backend | master |     7 |  +120/-40| feat(api): add flow endpoints
6 | agent-mesh-webui        | master |    26 |  +250/-80| feat(ui): runs page updates

Total: 6 repositories, 43 files, +475/-168 lines

Proceed? [Y/n/e]
  Y - commit all with suggested messages (default)
  n - cancel
  e - edit messages in editor
```

### Phase 3: 메시지 입력 방식

**방식 A: 자동 승인 (Y)**

- 제안된 메시지 그대로 사용
- 빠른 처리

**방식 B: 에디터에서 일괄 수정 (e)**
`e` 입력 시 `$EDITOR` (또는 vim)로 임시 파일 열기:

```yaml
# Bulk Commit Messages
# Edit messages below. Lines starting with # are ignored.
# Format: repository: commit message
# Save and close to proceed. Delete all lines to cancel.

agent-mesh-cli: fix(cli): update main entry
agent-mesh-connectors: refactor(connectors): update impls
agent-mesh-engine: fix(engine): scheduler updates
agent-mesh-flow-dsl: refactor(dsl): parser improvements
agent-mesh-saas-backend: feat(api): add flow endpoints
agent-mesh-webui: feat(ui): runs page updates
```

저장 후 닫으면 수정된 메시지로 커밋 진행.

**방식 C: 공통 메시지 (CLI 플래그)**

```bash
gz-git commit bulk -m "chore: sync for release v1.2.0"
```

모든 저장소에 동일한 메시지 적용.

### Phase 4: 커밋 실행 (병렬)

```
=== Committing (6 repositories) ===
[1/6] agent-mesh-cli ................ ✓ abc1234
[2/6] agent-mesh-connectors ......... ✓ def5678
[3/6] agent-mesh-engine ............. ✓ ghi9012
[4/6] agent-mesh-flow-dsl ........... ✓ jkl3456
[5/6] agent-mesh-saas-backend ....... ✓ mno7890
[6/6] agent-mesh-webui .............. ✓ pqr1234

=== Summary ===
✓ 6/6 commits successful
  Total: 43 files, +475/-168 lines
```

## CLI 인터페이스

### 기본 사용법

```bash
# 기본: 미리보기 후 확인
gz-git commit bulk

# dry-run: 미리보기만 (커밋 안 함)
gz-git commit bulk --dry-run

# 공통 메시지
gz-git commit bulk -m "chore: sync all repos"

# 확인 없이 자동 실행
gz-git commit bulk -y

# 에디터로 메시지 수정
gz-git commit bulk -e
```

### 전체 플래그

| 플래그            | 단축 | 설명                      | 기본값     |
| ----------------- | ---- | ------------------------- | ---------- |
| `--dry-run`       |      | 미리보기만, 커밋 안 함    | false      |
| `--message`       | `-m` | 모든 repo에 공통 메시지   | (자동생성) |
| `--edit`          | `-e` | 에디터로 메시지 일괄 수정 | false      |
| `--yes`           | `-y` | 확인 없이 자동 승인       | false      |
| `--depth`         | `-d` | 스캔 깊이                 | 1          |
| `--include`       |      | 포함할 저장소 패턴 (glob) | \*         |
| `--exclude`       |      | 제외할 저장소 패턴 (glob) |            |
| `--parallel`      | `-p` | 병렬 처리 수              | (CPU 수)   |
| `--format`        | `-f` | 출력 포맷 (text/json)     | text       |
| `--messages-file` |      | 메시지 JSON 파일 경로     |            |

### 필터링 예시

```bash
# 특정 패턴만 커밋
gz-git commit bulk --include "agent-mesh-*"

# 특정 저장소 제외
gz-git commit bulk --exclude "*-docs-*"

# 복합 조건
gz-git commit bulk --include "agent-mesh-*" --exclude "*-test*"
```

## JSON 출력 (자동화/CI 연동)

### 미리보기 JSON

```bash
gz-git commit bulk --dry-run --format json
```

```json
{
  "scan_depth": 1,
  "total_repositories": 6,
  "total_files": 43,
  "total_additions": 475,
  "total_deletions": 168,
  "repositories": [
    {
      "path": "agent-mesh-cli",
      "branch": "master",
      "status": "dirty",
      "files": [
        "src/agent_mesh_cli/main.py"
      ],
      "additions": 10,
      "deletions": 5,
      "suggested_message": "fix(cli): update main entry"
    },
    {
      "path": "agent-mesh-connectors",
      "branch": "master",
      "status": "dirty",
      "files": [
        "src/agent_mesh_connectors/base.py",
        "src/agent_mesh_connectors/claude.py",
        "src/agent_mesh_connectors/codex.py",
        "src/agent_mesh_connectors/ollama.py"
      ],
      "additions": 45,
      "deletions": 20,
      "suggested_message": "refactor(connectors): update impls"
    }
  ]
}
```

### 메시지 파일로 입력

외부 도구에서 메시지를 생성/수정한 후 파일로 전달:

```bash
gz-git commit bulk --messages-file /tmp/messages.json
```

`/tmp/messages.json`:

```json
{
  "agent-mesh-cli": "fix(cli): update main entry point",
  "agent-mesh-connectors": "refactor(connectors): improve base class"
}
```

### 결과 JSON

```bash
gz-git commit bulk -y --format json
```

```json
{
  "success": true,
  "total": 6,
  "committed": 6,
  "failed": 0,
  "results": [
    {
      "path": "agent-mesh-cli",
      "status": "success",
      "commit_hash": "abc1234",
      "message": "fix(cli): update main entry"
    },
    {
      "path": "agent-mesh-connectors",
      "status": "success",
      "commit_hash": "def5678",
      "message": "refactor(connectors): update impls"
    }
  ]
}
```

## 에러 처리

### 부분 실패 시

```
=== Committing (6 repositories) ===
[1/6] agent-mesh-cli ................ ✓ abc1234
[2/6] agent-mesh-connectors ......... ✓ def5678
[3/6] agent-mesh-engine ............. ✗ error: merge conflict
[4/6] agent-mesh-flow-dsl ........... ✓ jkl3456
[5/6] agent-mesh-saas-backend ....... ✓ mno7890
[6/6] agent-mesh-webui .............. ✓ pqr1234

=== Summary ===
✓ 5/6 commits successful
✗ 1/6 commits failed

Failed repositories:
  - agent-mesh-engine: error: merge conflict in scheduler.py
```

- 한 저장소 실패해도 나머지는 계속 진행
- 최종 결과에 실패 목록 표시
- exit code: 부분 실패 시 1, 전체 성공 시 0

## 구현 참고

### 기존 코드 재사용

1. **저장소 스캔**: `gz-git status` 로직 재사용
1. **메시지 생성**: `gz-git commit auto` 로직 재사용
1. **병렬 처리**: `gz-git status`의 병렬 스캔 패턴 재사용

### 새로 구현 필요

1. 배치 커밋 실행 로직
1. 에디터 연동 (임시 파일 생성 → $EDITOR 호출 → 파싱)
1. 메시지 파일 입력 파싱
1. 결과 집계 및 출력

## 우선순위

### MVP (Phase 1)

- [ ] `gz-git commit bulk` 기본 동작
- [ ] 테이블 형식 출력
- [ ] Y/n 확인
- [ ] 병렬 커밋 실행

### Phase 2

- [ ] `--dry-run`
- [ ] `-m` 공통 메시지
- [ ] `-y` 자동 승인
- [ ] `--include/exclude` 필터

### Phase 3

- [ ] `-e` 에디터 연동
- [ ] `--format json`
- [ ] `--messages-file` 입력
- [ ] `--depth` 깊이 조절
