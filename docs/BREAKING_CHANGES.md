# Breaking Changes - v2.0

## sync forge 명령어 재설계

### 개요

`gz-git sync forge` 명령어를 Breaking Change로 재설계했습니다.
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
gz-git sync forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN \
  --ssh  # Deprecated
```

**After**:
```bash
gz-git sync forge \
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
gz-git sync forge \
  --provider gitlab \
  --org devbox \
  --target ~/.mydevbox \
  --base-url ssh://git@gitlab.polypia.net:2224 \  # 혼란스러움
  --token $GITLAB_TOKEN \
  --ssh
```

**After**:
```bash
gz-git sync forge \
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
gz-git sync forge \
  --provider gitlab \
  --org mygroup \
  --target ~/repos \
  --base-url https://gitlab.com \
  --token $GITLAB_TOKEN
  # --ssh 없으면 HTTPS (암묵적)
```

**After**:
```bash
gz-git sync forge \
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
gz-git sync forge --provider gitlab --org mygroup --target ./repos \
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

**버전**: 2.0.0
**날짜**: 2026-01-16
**영향**: Breaking Change - CLI 플래그 변경
