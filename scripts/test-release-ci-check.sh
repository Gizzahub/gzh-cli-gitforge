#!/bin/bash
# test-release-ci-check.sh: release-ci-check.sh 가 소스 커밋의 CI 결과로만 통과시키는지 검증한다
# 용도: 가짜 gh 로 성공·진행 중·실패·실행 없음·조회 실패·gh 없음을 재고,
#       조회 대상이 HEAD 가 아니라 소스 ref 와의 merge-base 인지 확인한다.
# 사용법: test-release-ci-check.sh

set -u

here=$(cd "$(dirname "$0")" && pwd -P)
check_script="$here/release-ci-check.sh"
tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/gz-git-release-ci.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT

failures=0
repo="$tmpdir/repo"
fakebin="$tmpdir/fakebin"
onlygit="$tmpdir/onlygit"

g() {
	git -C "$repo" -c user.name=t -c user.email=t@t -c commit.gpgsign=false "$@"
}

mkdir -p "$repo" "$fakebin" "$onlygit"
git -C "$repo" init -q
g commit -q --allow-empty -m 'source tip'
g branch -q source
source_sha=$(g rev-parse HEAD)
g commit -q --allow-empty -m 'release bump'

# The fake gh records its arguments and prints FAKE_GH_OUT with FAKE_GH_RC.
cat >"$fakebin/gh" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >"$FAKE_GH_ARGS"
[ -n "$FAKE_GH_OUT" ] && printf '%s\n' "$FAKE_GH_OUT"
exit "$FAKE_GH_RC"
EOF
chmod +x "$fakebin/gh"
ln -s "$(command -v git)" "$onlygit/git"

# run <name> <want_rc> <gh_out> <gh_rc> [<path>] [<ref>]
# BASH_ENV is dropped because a host BASH_ENV can restore PATH.
run() {
	name=$1 want_rc=$2 gh_out=$3 gh_rc=$4 path=${5:-$fakebin:$PATH} ref=${6-source}
	: >"$tmpdir/args"
	(cd "$repo" && env -u BASH_ENV PATH="$path" FAKE_GH_ARGS="$tmpdir/args" \
		FAKE_GH_OUT="$gh_out" FAKE_GH_RC="$gh_rc" /bin/bash "$check_script" "$ref") \
		>"$tmpdir/out" 2>&1
	got_rc=$?
	if [ "$got_rc" = "$want_rc" ]; then
		echo "ok   [$name]"
	else
		echo "FAIL [$name] want rc=$want_rc got rc=$got_rc: $(cat "$tmpdir/out")"
		failures=$((failures + 1))
	fi
}

run 'passed run' 0 'completed|success|https://ci/1' 0
if grep -q -- "--commit $source_sha --workflow CI --event push --limit 1 " "$tmpdir/args"; then
	echo "ok   [queries the merge base, not HEAD]"
else
	echo "FAIL [queries the merge base, not HEAD] args: $(cat "$tmpdir/args")"
	failures=$((failures + 1))
fi
run 'run in progress' 1 'in_progress||https://ci/2' 0
if grep -q 'is in_progress: https://ci/2' "$tmpdir/out"; then
	echo "ok   [in-progress message keeps the url]"
else
	echo "FAIL [in-progress message keeps the url] out: $(cat "$tmpdir/out")"
	failures=$((failures + 1))
fi
run 'failed run' 1 'completed|failure|https://ci/3' 0
run 'cancelled run' 1 'completed|cancelled|https://ci/4' 0
run 'no run' 1 '' 0
run 'query fails' 2 '' 1
run 'gh missing' 2 '' 0 "$onlygit"
run 'unknown ref' 2 'completed|success|https://ci/5' 0 "$fakebin:$PATH" no-such-ref
run 'empty ref' 2 'completed|success|https://ci/6' 0 "$fakebin:$PATH" ''
if (cd "$repo" && /bin/bash "$check_script") >/dev/null 2>&1; then
	echo "FAIL [no arguments] want rc=2 got rc=0"
	failures=$((failures + 1))
elif [ "$?" = 2 ]; then
	echo "ok   [no arguments]"
else
	echo "FAIL [no arguments] want rc=2"
	failures=$((failures + 1))
fi

if [ "$failures" -ne 0 ]; then
	echo "release CI check tests: $failures failed" >&2
	exit 1
fi
echo "release CI check tests passed"
