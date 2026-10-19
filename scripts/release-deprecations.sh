#!/bin/bash
# release-deprecations.sh: 릴리스 범위 커밋의 Deprecated: trailer 를 릴리스 노트 꼬리말로 모은다
# 용도: GoReleaser changelog 그룹은 커밋 제목만 보므로 본문의 폐기 예고를 고를 수 없다.
#       직전 v*.*.* 태그부터 <tag> 까지 커밋의 `Deprecated:` trailer 를 읽어
#       "## Deprecated" 목록을 표준출력에 쓴다. 해당 커밋이 없으면 아무것도 쓰지 않는다.
#       release.yml 이 이 출력을 GoReleaser --release-footer 로 넘긴다.
# 사용법: release-deprecations.sh <tag>

set -euo pipefail

TAG=${1:-}
if [ -z "$TAG" ]; then
	echo "usage: release-deprecations.sh <tag>" >&2
	exit 2
fi

git rev-parse -q --verify "refs/tags/$TAG^{commit}" >/dev/null || {
	echo "release-deprecations: tag not found: $TAG" >&2
	exit 1
}

# 첫 릴리스는 직전 태그가 없으므로 태그까지의 전체 이력을 본다.
# v0.2.0-rc.1 같은 프리릴리스 태그는 범위의 시작이 아니다(release.yml 의 latest 판정과 같다).
if prev=$(git describe --tags --abbrev=0 --match 'v*.*.*' --exclude 'v*-*' "$TAG^" 2>/dev/null); then
	range="$prev..$TAG"
else
	range=$TAG
fi

# unfold 은 들여쓴 연속 줄을 한 항목으로 합친다.
notices=$(git log --reverse --format='%(trailers:key=Deprecated,valueonly,unfold,separator=%x0A)' "$range" |
	sed -e 's/^[[:space:]]*//' -e '/^$/d')

# 들여쓰지 않은 연속 줄은 대개 trailer 블록 전체를 무효로 만들어 조용히 빠진다.
# (Signed-off-by 같은 git 생성 trailer 옆이면 블록은 남고 항목이 첫 줄에서 잘린다.)
# 본문에서 키로 시작하는 줄이 모인 항목보다 많으면 경고만 한다. git 처럼
# 키의 대소문자와 콜론 앞 공백을 가리지 않는다.
lines=$(git log --format='%B' "$range" | grep -Eci '^deprecated[[:space:]]*:' || true)
collected=$(printf '%s' "$notices" | grep -c '' || true)
if [ "$lines" -gt "$collected" ]; then
	echo "::warning::release-deprecations: $((lines - collected)) 'Deprecated:' line(s) in $range are not trailers and were not collected" >&2
fi
[ -n "$notices" ] || exit 0

printf '## Deprecated\n\n'
printf '%s\n' "$notices" | sed 's/^/- /'
