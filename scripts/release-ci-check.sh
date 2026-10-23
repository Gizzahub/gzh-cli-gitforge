#!/bin/bash
# release-ci-check.sh: 릴리스가 올라설 소스 커밋의 CI 가 통과했는지 확인한다
# 용도: HEAD 와 소스 ref 의 merge-base 를 찾고, 그 커밋의 push CI 실행이 끝나서
#       success 인지 gh 로 조회한다. 실행이 없거나, 진행 중이거나, 실패했거나,
#       gh 를 쓸 수 없으면 실패한다. 빨간 master 위에 릴리스를 자르지 않기 위함이다.
# 사용법: release-ci-check.sh <source-ref> [<workflow>]

set -u

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
	echo "usage: release-ci-check.sh <source-ref> [<workflow>]" >&2
	exit 2
fi
ref=$1
workflow=${2:-CI}

if ! command -v gh >/dev/null 2>&1; then
	echo "release CI check: gh not found; it is required to read the CI result" >&2
	exit 2
fi

if ! base=$(git merge-base HEAD "$ref" 2>/dev/null); then
	echo "release CI check: no merge base between HEAD and $ref" >&2
	exit 2
fi

if ! run=$(gh run list --commit "$base" --workflow "$workflow" --event push --limit 1 \
	--json status,conclusion,url --jq '.[0] | select(.) | "\(.status)|\(.conclusion)|\(.url)"'); then
	echo "release CI check: could not query $workflow runs for $base" >&2
	exit 2
fi

if [ -z "$run" ]; then
	echo "release CI check: no $workflow push run for $base; push it and wait for CI" >&2
	exit 1
fi

# A run in progress has an empty conclusion, so the fields are not split on spaces.
IFS='|' read -r status conclusion url <<<"$run"
if [ "$status" != completed ]; then
	echo "release CI check: $workflow for $base is $status: $url" >&2
	exit 1
fi
if [ "$conclusion" != success ]; then
	echo "release CI check: $workflow for $base concluded $conclusion: $url" >&2
	exit 1
fi
echo "release CI check: $workflow passed for $base"
