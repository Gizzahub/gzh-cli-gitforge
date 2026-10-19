#!/bin/bash
# test-release-deprecations.sh: release-deprecations.sh 가 릴리스 범위의 Deprecated: trailer 만 모으는지 검증한다
# 용도: 임시 저장소에 태그와 커밋을 만들고 범위·trailer 판정·빈 출력·첫 릴리스·없는 태그를 잰다.
#       본문 중간의 "Deprecated:" 는 trailer 가 아니므로 모이면 안 된다.
# 사용법: test-release-deprecations.sh

set -u

here=$(cd "$(dirname "$0")" && pwd -P)
collect="$here/release-deprecations.sh"
tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/gz-git-deprecations.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT

failures=0
repo="$tmpdir/repo"

g() {
	git -C "$repo" -c user.name=t -c user.email=t@t -c commit.gpgsign=false -c tag.gpgsign=false "$@"
}

commit() {
	g commit -q --allow-empty -m "$1"
}

check() {
	name=$1 want_rc=$2 want_out=$3 tag=$4
	got_out=$(cd "$repo" && "$collect" "$tag" 2>/dev/null)
	got_rc=$?
	if [ "$got_rc" = "$want_rc" ] && [ "$got_out" = "$want_out" ]; then
		echo "ok   [$name]"
	else
		echo "FAIL [$name] want rc=$want_rc out='$want_out' got rc=$got_rc out='$got_out'"
		failures=$((failures + 1))
	fi
}

mkdir -p "$repo"
git -C "$repo" init -q
commit "$(printf 'feat: first\n\nDeprecated: OldFirst; use NewFirst')"
g tag v0.1.0
commit "$(printf 'fix: prose only\n\nDeprecated: is mentioned here as prose.\n\nModel: m-1')"
commit "$(printf 'feat: retire\n\nWhy it changes.\n\nDeprecated: Client.Open; use Client.OpenContext\nModel: m-1')"
commit "$(printf 'refactor: two\n\nDeprecated: --old-flag; use --new-flag\nDeprecated: sync run; use sync from-config')"
g tag v0.2.0
commit 'fix: nothing deprecated'
g tag v0.2.1

want_range='## Deprecated

- Client.Open; use Client.OpenContext
- --old-flag; use --new-flag
- sync run; use sync from-config'

check range 0 "$want_range" v0.2.0
check none 0 '' v0.2.1
check first 0 "$(printf '## Deprecated\n\n- OldFirst; use NewFirst')" v0.1.0
check missing-tag 1 '' v9.9.9

if [ "$failures" -ne 0 ]; then
	echo "test-release-deprecations: $failures failure(s)"
	exit 1
fi
echo "test-release-deprecations: all passed"
