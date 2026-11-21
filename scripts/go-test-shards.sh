#!/bin/bash
# go-test-shards.sh: 한 패키지의 최상위 테스트를 N 개의 go test 프로세스로 나눠 동시에 돌린다
# 용도: 직렬(-parallel=1) 테스트가 많아 한 프로세스로는 임계 경로가 되는 패키지(pkg/integrate)를 단축한다.
#       샤드마다 별도 프로세스이므로 프로세스 단위 상태(TestMain 의 XDG_STATE_HOME)는 서로 격리된다.
# 사용법:
#   go-test-shards.sh run <shards> <merged-profile> <package> [go test 인자...]
#   go-test-shards.sh partition <shards> <list-file>   # 샤드별 -run 정규식을 한 줄씩 출력
#   go-test-shards.sh merge <out> <in>...              # cover 프로파일 병합
# 샤드 인자: 목록 인자는 go test 에 그대로 전달한다(-short, -timeout, -parallel 등). -coverprofile 은 스크립트가 붙인다.

set -u

die() {
	echo "go-test-shards: $*" >&2
	exit 1
}

# partition: 입력 순서대로 라운드 로빈 배정, 샤드마다 ^(A|B)$ 정규식 한 줄.
# 테스트 이름은 식별자이므로 정규식 이스케이프가 필요 없다. 이름이 모자라 빈 샤드는 출력하지 않는다.
partition() {
	local n=$1 file=$2
	awk -v n="$n" '
		NF == 0 { next }
		{ i = (c++ % n); s[i] = (i in s) ? s[i] "|" $1 : $1 }
		END { for (i = 0; i < n; i++) if (i in s) print "^(" s[i] ")$" }
	' "$file"
}

# merge: mode 줄은 한 번만 쓰고 같은 블록은 합친다. set 은 OR(최댓값), count/atomic 은 합계.
merge() {
	local out=$1
	shift
	[ "$#" -gt 0 ] || die "merge: no input profiles"
	awk '
		FNR == 1 {
			if ($0 !~ /^mode: /) { print "go-test-shards: missing mode line in " FILENAME > "/dev/stderr"; bad = 1; exit 1 }
			if (mode != "" && mode != $0) { print "go-test-shards: mixed cover modes" > "/dev/stderr"; bad = 1; exit 1 }
			mode = $0
			next
		}
		NF == 0 { next }
		{
			cnt = $NF
			key = $0
			sub(/ [0-9]+$/, "", key)
			if (!(key in seen)) { seen[key] = 1; order[++k] = key; val[key] = 0 }
			if (mode == "mode: set") { if (cnt + 0 > 0) val[key] = 1 }
			else val[key] += cnt
		}
		END {
			if (bad) exit 1
			print mode
			for (i = 1; i <= k; i++) print order[i], val[order[i]]
		}
	' "$@" >"$out.tmp" || {
		rm -f "$out.tmp"
		die "merge failed"
	}
	mv "$out.tmp" "$out"
}

run() {
	[ "$#" -ge 3 ] || die "usage: run <shards> <merged-profile> <package> [go test args...]"
	local n=$1 merged=$2 pkg=$3
	shift 3
	case "$n" in '' | *[!0-9]* | 0) die "shards must be a positive integer: $n" ;; esac

	local tmp
	tmp=$(mktemp -d "${TMPDIR:-/tmp}/gz-git-shards.XXXXXX") || die "mktemp failed"
	trap 'rm -rf "$tmp"' EXIT HUP INT TERM

	go test -list . "$pkg" >"$tmp/list.raw" 2>"$tmp/list.err" || {
		cat "$tmp/list.raw" "$tmp/list.err" >&2
		die "go test -list failed for $pkg"
	}
	# -list 출력의 마지막 "ok ..." 줄과 "no tests" 안내를 제외한 식별자만 남긴다.
	grep -E '^(Test|Example|Benchmark|Fuzz)[A-Za-z0-9_]*$' "$tmp/list.raw" | sort -u >"$tmp/list"
	local total
	total=$(wc -l <"$tmp/list" | tr -d ' ')
	[ "$total" -gt 0 ] || die "go test -list yielded no tests for $pkg"

	partition "$n" "$tmp/list" >"$tmp/regexes"
	# 합집합 검사: 샤드 정규식에서 이름을 되돌려 목록과 같은지 확인한다.
	sed -e 's/^\^(//' -e 's/)\$$//' "$tmp/regexes" | tr '|' '\n' | sort >"$tmp/union"
	cmp -s "$tmp/list" "$tmp/union" || die "shard union differs from the listed tests"
	[ "$(wc -l <"$tmp/union" | tr -d ' ')" -eq "$total" ] || die "shard union count mismatch"

	echo "go-test-shards: $total tests of $pkg in $(wc -l <"$tmp/regexes" | tr -d ' ') shards"
	local i=0 re pids=""
	while IFS= read -r re; do
		i=$((i + 1))
		go test "$@" -coverprofile="$tmp/cover.$i" -run "$re" "$pkg" >"$tmp/out.$i" 2>&1 &
		pids="$pids $!"
	done <"$tmp/regexes"

	local failed=0 pid idx=0
	for pid in $pids; do
		idx=$((idx + 1))
		if wait "$pid"; then
			echo "go-test-shards: shard $idx ok"
		else
			failed=1
			echo "go-test-shards: shard $idx FAILED" >&2
		fi
	done
	# 샤드 출력은 끝난 뒤 샤드 순서대로 한꺼번에 낸다(섞임 방지). 실패해도 전부 보인다.
	idx=0
	while [ "$idx" -lt "$i" ]; do
		idx=$((idx + 1))
		echo "=== shard $idx/$i ==="
		cat "$tmp/out.$idx"
	done
	[ "$failed" -eq 0 ] || die "one or more shards failed"

	local profiles="" j=0
	while [ "$j" -lt "$i" ]; do
		j=$((j + 1))
		[ -s "$tmp/cover.$j" ] || die "shard $j wrote no coverage profile"
		profiles="$profiles $tmp/cover.$j"
	done
	# shellcheck disable=SC2086
	merge "$merged" $profiles
}

cmd=${1:-}
[ "$#" -gt 0 ] && shift
case "$cmd" in
run) run "$@" ;;
partition) [ "$#" -eq 2 ] || die "usage: partition <shards> <list-file>"; partition "$@" ;;
merge) merge "$@" ;;
*) die "usage: $0 run|partition|merge ..." ;;
esac
