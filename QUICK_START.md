# Quick Start

5분 안에 gz-git 시작하기.

## 요구사항

- Go 1.25.1+
- Git 2.30+

## 설치

```bash
go install github.com/gizzahub/gzh-cli-gitforge/cmd/gz-git@latest
```

설치 확인:

```bash
gz-git --version
```

> **PATH 오류 시**: `echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc && source ~/.bashrc`

______________________________________________________________________

## 기본 사용법

### 저장소 상태 확인

```bash
gz-git status
gz-git info
```

### 저장소 복제

```bash
# 기본: 현재 디렉토리에 clone
gz-git clone --url https://github.com/user/repo.git

# 특정 디렉토리에 clone
gz-git clone ~/projects --url https://github.com/user/repo.git

# 특정 브랜치
gz-git clone -b develop --url https://github.com/user/repo.git

# 얕은 복제
gz-git clone --depth 1 --url https://github.com/user/repo.git

# URL 목록 파일(한 줄에 하나)로 clone
gz-git clone --file repos.txt
```

______________________________________________________________________

## 핵심 기능: 다중 저장소 관리

gz-git의 가장 강력한 기능입니다.

### 일괄 Fetch/Pull/Push

```bash
# 현재 디렉토리의 모든 저장소 fetch
gz-git fetch -d 1

# 2단계 깊이까지 스캔하여 pull
gz-git pull -d 2 ~/projects

# rebase 전략으로 pull
gz-git pull -s rebase -d 2 ~/projects

# 병렬 10개로 실행
gz-git fetch -j 10 ~/workspace

# 미리보기 (실행 안함)
gz-git pull -n ~/projects
```

### 단축 플래그

| 플래그         | 축약 | 설명         |
| -------------- | ---- | ------------ |
| `--scan-depth` | `-d` | 스캔 깊이    |
| `--parallel`   | `-j` | 병렬 처리 수 |
| `--dry-run`    | `-n` | 미리보기     |
| `--strategy`   | `-s` | pull 전략    |

______________________________________________________________________

## 추가 기능

```bash
gz-git commit                              # 다중 저장소 커밋 (v0.4.0+)
gz-git commit --yes                        # 자동 확인으로 커밋
gz-git branch list --all                   # 브랜치 목록
gz-git history stats --since "1 month ago" # 커밋 통계
gz-git merge detect feature/new main       # 머지 충돌 감지
```

______________________________________________________________________

## 다음 단계

- [명령어 레퍼런스](docs/commands/README.md)
- [FAQ](docs/user/guides/faq.md)
- [Go 라이브러리](docs/user/getting-started/library-usage.md)
