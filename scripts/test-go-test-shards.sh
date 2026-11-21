#!/bin/bash
# test-go-test-shards.sh: go-test-shards.sh 의 분할·병합·실패 전파를 검증한다
# 용도: 라운드 로빈 분할이 서로소이고 합집합이 목록과 같은지, set/count 병합이 맞는지,
#       가짜 go 로 샤드 하나가 실패하면 전체가 실패하는지와 목록 실패가 빠르게 실패하는지 잰다.
# 사용법: test-go-test-shards.sh

set -u

here=$(cd "$(dirname "$0")" && pwd -P)
tool="$here/go-test-shards.sh"
tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/gz-git-shards-test.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT
failures=0

check() { # check <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then echo "ok   $1"; else
		echo "FAIL $1: want [$2] got [$3]" >&2
		failures=$((failures + 1))
	fi
}

printf 'TestA\nTestB\nTestC\nTestD\nTestE\n' >"$tmpdir/list"
check "partition round robin" '^(TestA|TestD)$
^(TestB|TestE)$
^(TestC)$' "$("$tool" partition 3 "$tmpdir/list")"
check "partition fewer tests than shards" '^(TestA)$
^(TestB)$' "$(printf 'TestA\nTestB\n' >"$tmpdir/l2"; "$tool" partition 4 "$tmpdir/l2")"

printf 'mode: set\nf.go:1.1,2.2 1 1\nf.go:3.1,4.2 1 0\n' >"$tmpdir/p1"
printf 'mode: set\nf.go:3.1,4.2 1 1\nf.go:5.1,6.2 1 0\n' >"$tmpdir/p2"
"$tool" merge "$tmpdir/m" "$tmpdir/p1" "$tmpdir/p2"
check "merge set" 'mode: set
f.go:1.1,2.2 1 1
f.go:3.1,4.2 1 1
f.go:5.1,6.2 1 0' "$(cat "$tmpdir/m")"
printf 'mode: count\nf.go:1.1,2.2 1 2\n' >"$tmpdir/c1"
printf 'mode: count\nf.go:1.1,2.2 1 3\n' >"$tmpdir/c2"
"$tool" merge "$tmpdir/mc" "$tmpdir/c1" "$tmpdir/c2"
check "merge count" 'mode: count
f.go:1.1,2.2 1 5' "$(cat "$tmpdir/mc")"
"$tool" merge "$tmpdir/mx" "$tmpdir/p1" "$tmpdir/c1" 2>/dev/null
check "merge mixed modes fails" 1 "$([ -e "$tmpdir/mx" ] && echo 0 || echo 1)"

# 가짜 go: -list 는 3 개 테스트를 내고, 실행은 FAKE_FAIL 정규식이 -run 에 있으면 실패한다.
mkdir "$tmpdir/bin"
cat >"$tmpdir/bin/go" <<'FAKE'
#!/bin/bash
if [ "$1" = test ] && [ "$2" = -list ]; then
	[ -z "${FAKE_LIST_FAIL:-}" ] || { echo boom >&2; exit 1; }
	[ -z "${FAKE_LIST_EMPTY:-}" ] || { echo "ok  pkg 0.1s"; exit 0; }
	printf 'TestA\nTestB\nTestC\nok  \tpkg\t0.1s\n'; exit 0
fi
prof=""; run=""
for a in "$@"; do case "$a" in -coverprofile=*) prof=${a#-coverprofile=} ;; esac; done
while [ "$#" -gt 0 ]; do [ "$1" = -run ] && run=$2; shift; done
printf 'mode: set\nf.go:1.1,2.2 1 1\n' >"$prof"
echo "ran $run"
case "$run" in *"${FAKE_FAIL:-NOPE_NEVER}"*) echo "--- FAIL"; exit 1 ;; esac
FAKE
chmod +x "$tmpdir/bin/go"

out=$(PATH="$tmpdir/bin:$PATH" "$tool" run 2 "$tmpdir/out.cov" ./pkg/x -short 2>&1); rc=$?
check "run ok exit" 0 "$rc"
check "run profile written" 'mode: set
f.go:1.1,2.2 1 1' "$(cat "$tmpdir/out.cov")"
case "$out" in *"3 tests of ./pkg/x in 2 shards"*) check "run summary" 1 1 ;; *) check "run summary" 1 0 ;; esac

out=$(FAKE_FAIL=TestB PATH="$tmpdir/bin:$PATH" "$tool" run 2 "$tmpdir/fail.cov" ./pkg/x 2>&1); rc=$?
check "shard failure fails" 1 "$rc"
case "$out" in *"--- FAIL"*) check "failure output visible" 1 1 ;; *) check "failure output visible" 1 0 ;; esac
check "no profile on failure" 1 "$([ -e "$tmpdir/fail.cov" ] && echo 0 || echo 1)"

FAKE_LIST_FAIL=1 PATH="$tmpdir/bin:$PATH" "$tool" run 2 "$tmpdir/x.cov" ./pkg/x >/dev/null 2>&1
check "list failure fails" 1 "$?"
FAKE_LIST_EMPTY=1 PATH="$tmpdir/bin:$PATH" "$tool" run 2 "$tmpdir/x.cov" ./pkg/x >/dev/null 2>&1
check "empty list fails" 1 "$?"

[ "$failures" -eq 0 ] || { echo "$failures failure(s)" >&2; exit 1; }
echo "all passed"
