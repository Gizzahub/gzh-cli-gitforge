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
if prev=$(git describe --tags --abbrev=0 --match 'v*.*.*' "$TAG^" 2>/dev/null); then
	range="$prev..$TAG"
else
	range=$TAG
fi

notices=$(git log --reverse --format='%(trailers:key=Deprecated,valueonly,separator=%x0A)' "$range" |
	sed -e 's/^[[:space:]]*//' -e '/^$/d')
[ -n "$notices" ] || exit 0

printf '## Deprecated\n\n'
printf '%s\n' "$notices" | sed 's/^/- /'
