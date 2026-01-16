# Breaking Changes - v2.0

## sync from-forge 명령어 재설계

### 개요

`gz-git sync from-forge` 명령어를 Breaking Change로 재설계했습니다.
기존의 혼란스러운 `--ssh` boolean 플래그와 `--base-url`의 이중 역할을 제거하고,
명확한 역할 분리와 확장 가능한 설계를 적용했습니다.

### 변경 사항

#### 1. 제거된 플래그

- ❌ `--ssh` (boolean) - Deprecated 경고 없이 완전 제거
- ❌ `UseSSH` field in `ForgePlannerConfig`

#### 2. 새로운 플래그

- ✅ `--clone-proto` (string) - Clone 프로토콜 선택: `ssh` | `https` (기본값: `ssh`)
- ✅ `--ssh-port` (int) - 커스텀 SSH 포트 명시적 지정 (기본값: 0 = port 22)

#### 3. 의미 명확화

**Before (혼란스러움)**:
```bash
--base-url ssh://git@gitlab.polypia.net:2224  # API endpoint인데 SSH 정보 포함?
--ssh  # boolean이라 https 외의 프로토콜 확장 불가
```

**After (명확함)**:
```bash
--base-url https://gitlab.polypia.net  # API endpoint만 (http/https)
--clone-proto ssh                      # Clone 프로토콜 (확장 가능)
--ssh-port 2224                        # SSH 포트 명시적 지정
```

### 마이그레이션 가이드

#### Case 1: 표준 GitLab (port 22)

**Before**:
```bash
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN \
  --ssh  # Deprecated
```

**After**:
```bash
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN
  # --clone-proto ssh (기본값이므로 생략 가능)
```

#### Case 2: Self-hosted GitLab (커스텀 SSH 포트)

**Before**:
```bash
gz-git sync from-forge \
  --provider gitlab \
  --org devbox \
  --target ~/.mydevbox \
  --base-url ssh://git@gitlab.polypia.net:2224 \  # 혼란스러움
  --token $GITLAB_TOKEN \
  --ssh
```

**After**:
```bash
gz-git sync from-forge \
  --provider gitlab \
  --org devbox \
  --target ~/.mydevbox \
  --base-url https://gitlab.polypia.net \  # API endpoint (명확함)
  --token $GITLAB_TOKEN \
  --clone-proto ssh \
  --ssh-port 2224  # 명시적
```

#### Case 3: HTTPS Clone

**Before**:
```bash
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN
  # --ssh 없으면 HTTPS (암묵적)
```

**After**:
```bash
gz-git sync from-forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN \
  --clone-proto https  # 명시적
```

### 영향받는 코드

#### 1. Command Line Interface

**File**: `pkg/reposynccli/forge_command.go`

**Changes**:
- `ForgeCommandOptions.UseSSH` 제거
- `ForgeCommandOptions.CloneProto` 추가 (string)
- `ForgeCommandOptions.SSHPort` 추가 (int)
- `--ssh` 플래그 완전 제거
- Deprecated 처리 로직 제거

#### 2. Planner Configuration

**File**: `pkg/reposync/planner_forge.go`

**Changes**:
- `ForgePlannerConfig.UseSSH` 제거
- `ForgePlannerConfig.CloneProto` 추가 (string)
- `ForgePlannerConfig.SSHPort` 추가 (int)
- `toRepoSpec()` 메서드 로직 변경

#### 3. GitLab Provider

**File**: `pkg/gitlab/provider.go`

**Changes**:
- `ProviderOptions` 추가 (struct)
- `NewProviderWithOptions()` 추가 (생성자)
- `extractSSHInfo()` → `extractHostFromURL()` (함수명 변경, 역할 명확화)
- SSH 포트 로직 간소화

#### 4. Tests

**File**: `pkg/reposync/planner_forge_test.go`

**Changes**:
- `UseSSH: true` → `CloneProto: "ssh"`
- Test cases 업데이트

### 설계 원칙

#### 1. 역할 분리 (Separation of Concerns)

| 항목 | 역할 | 형식 |
|------|------|------|
| `--base-url` | GitLab API endpoint | `https://gitlab.polypia.net` |
| `--clone-proto` | Clone 프로토콜 선택 | `ssh` \| `https` |
| `--ssh-port` | SSH 포트 지정 | `2224` (int) |

#### 2. 명시성 (Explicitness over Implicitness)

- ❌ Before: `--base-url`에서 SSH 정보 추출 (암묵적)
- ✅ After: `--ssh-port`로 명시적 지정

#### 3. 확장성 (Extensibility)

- ❌ Before: `--ssh` boolean → `git://` 프로토콜 추가 불가
- ✅ After: `--clone-proto` string → 향후 `git://` 추가 가능

### 추가 개선 사항

#### 1. Help 메시지 개선

```
Flags:
  --base-url string      Base URL for self-hosted instances (API endpoint)
  --clone-proto string   Clone protocol: ssh, https (default "ssh")
  --ssh-port int         Custom SSH port (0 = default 22)
```

#### 2. 예제 명확화

```bash
# Sync from self-hosted GitLab with custom SSH port
gz-git sync from-forge --provider gitlab --org mygroup --target ./repos \
  --base-url https://gitlab.company.com --token $GITLAB_TOKEN \
  --clone-proto ssh --ssh-port 2224
```

#### 3. 에러 메시지 개선

```
Error: invalid --clone-proto: xxx (must be ssh or https)
```

### 테스트

모든 테스트 통과 ✅

```bash
ok  	github.com/gizzahub/gzh-cli-gitforge/pkg/gitlab	0.003s
ok  	github.com/gizzahub/gzh-cli-gitforge/pkg/reposync	0.004s
```

### 문서

- ✅ [CLAUDE.md](../CLAUDE.md) - 업데이트 완료
- ✅ [sync-gitlab-custom-ssh-v2.md](sync-gitlab-custom-ssh-v2.md) - 신규 가이드
- ✅ Help messages - 업데이트 완료

### 관련 이슈

- GitLab 커스텀 SSH 포트 지원
- `--base-url`과 `--ssh` 플래그의 혼란스러운 역할

### 향후 계획

- [ ] GitHub SSH 포트 지원 (현재는 GitLab만)
- [ ] `git://` 프로토콜 지원
- [ ] Per-repository 프로토콜 오버라이드
- [ ] Clone URL 템플릿 지원

---

## sync 명령어 구조 재설계 (v2.1)

### 개요

**Breaking Change**: `sync forge` → `sync from-forge`, `sync run` → `sync from-config`

Source-centric 명령어 구조로 전면 재설계하여 명령어 일관성과 확장성을 개선했습니다.

### 변경 사항

#### 1. 명령어 이름 변경 (Breaking!)

| Old (v2.0) | New (v2.1) | 설명 |
|------------|------------|------|
| `sync forge` | `sync from-forge` | Git forge (API) 기반 동기화 |
| `sync run` | `sync from-config` | YAML config 파일 기반 동기화 |
| N/A | `sync config` | Config 관리 명령어 그룹 |

#### 2. 새로운 config 관리 명령어

```bash
# Sample config 생성
gz-git sync config init -o sync.yaml

# 로컬 디렉토리 스캔 → config 생성 (NEW!)
gz-git sync config scan ~/mydevbox --strategy unified -o sync.yaml
gz-git sync config scan ~/mydevbox --strategy per-directory --depth 3

# Forge API → config 생성 (NEW!)
gz-git sync config generate --provider gitlab --org devbox --token $TOKEN -o sync.yaml

# Config 검증
gz-git sync config validate -c sync.yaml

# Config 병합 (Placeholder)
gz-git sync config merge --provider gitlab --org another-group --into sync.yaml
```

#### 3. 로컬 스캔 기능 (sync config scan)

**기능**: 로컬 디렉토리를 재귀적으로 스캔하여 git repo 발견 → YAML config 생성

**전략**:
- **unified**: 단일 config 파일에 모든 repo 등록
- **per-directory**: 각 디렉토리 레벨마다 config 파일 생성

**특징**:
- `.gitignore` 패턴 자동 존중 (disable 가능: `--no-gitignore`)
- 중첩 repo 지원 (상위 repo + 하위 repo 모두 포함)
- Multiple remote URL 처리 (`url` vs `urls` field)
- 패턴 기반 제외/포함 (`--exclude`, `--include`)

**예제**:
```bash
# ~/mydevbox 스캔 (19개 repo 발견)
gz-git sync config scan ~/mydevbox --strategy unified -o sync.yaml

# .gitignore 무시하고 스캔
gz-git sync config scan ~/mydevbox --no-gitignore --depth 2

# 특정 패턴 제외
gz-git sync config scan ~/mydevbox --exclude "vendor,node_modules,tmp/*"
```

#### 4. GitLab Subgroup 지원 (NEW!)

GitLab 하위 그룹을 포함하여 동기화:

```bash
# Flat mode: parent-group/subgroup/repo → parent-group-subgroup-repo
gz-git sync from-forge --provider gitlab --org parent-group \
  --include-subgroups --subgroup-mode flat --target ~/repos

# Nested mode: parent-group/subgroup/repo (directory hierarchy)
gz-git sync from-forge --provider gitlab --org parent-group \
  --include-subgroups --subgroup-mode nested --target ~/repos
```

### 마이그레이션 가이드

#### Case 1: Forge 동기화

**Before (v2.0)**:
```bash
gz-git sync forge --provider gitlab --org mygroup --target ~/repos --token $TOKEN
```

**After (v2.1)**:
```bash
gz-git sync from-forge --provider gitlab --org mygroup --target ~/repos --token $TOKEN
```

#### Case 2: Config 파일 동기화

**Before (v2.0)**:
```bash
gz-git sync run -c sync.yaml
```

**After (v2.1)**:
```bash
gz-git sync from-config -c sync.yaml
```

### 신규 파일

- `pkg/scanner/git_scanner.go` - 로컬 git repo 스캐너
- `pkg/reposynccli/config_scan_command.go` - Scan 명령어
- `pkg/reposynccli/config_generate_command.go` - Generate 명령어
- `pkg/reposynccli/from_forge_command.go` - Renamed from `forge_command.go`
- `pkg/reposynccli/from_config_command.go` - Renamed from `run_command.go`

### 문서

- ✅ [CLAUDE.md](../CLAUDE.md) - sync 섹션 전면 개편
- ✅ [sync-command-redesign.md](sync-command-redesign.md) - 설계 문서
- ✅ Help messages - 모든 명령어 업데이트

---

**버전**: 2.1.0
**날짜**: 2026-01-16
**영향**: Breaking Change - 명령어 이름 변경, 신규 기능 추가
