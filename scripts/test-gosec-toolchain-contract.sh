#!/bin/bash
# test-gosec-toolchain-contract.sh: gosec 핀의 Go 툴체인 계약을 검증한다
# 용도: (기본 모드) .make/tools.mk 의 GOSEC 핀과 bin/tools 바이너리의 module pin
#       일치, 설치된 고정 컴파일러(go1.26.7, go1.27.0) 각각으로 재빌드한 identity,
#       pkg/gitsettings 최소 스캔을 검증한다. (--full-security-no-regression 모드)
#       현재 핀과 기준 핀 v2.22.10 을 go1.26.7 로 각각 private tmp 에 빌드해 같은
#       소스에서 make security-code GOSEC=<binary> 와 gosec -fmt=json ./... 를
#       실행하고 JSON Issues 의 rule_id/file/line/code 집합을 비교한다. 새 finding,
#       scanner/runtime/export-data 오류, finding 외 원인의 exit 1, JSON 누락은
#       거절한다. 고정 컴파일러가 없으면 건너뛰지 않고 실패한다. full 모드는
#       tasks/issue/40-gosec-go127-export-data.md 에 비밀 없는 실행 영수증을
#       남긴다(같은 소스에서 재실행하면 같은 마커 안에 덮어쓴다).
#       library issue 40 / 중앙 보드 TASK-270 참조.
# 사용법: bash scripts/test-gosec-toolchain-contract.sh [--full-security-no-regression]

set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$repo"

GO_FLOOR_VER="1.26.7"
GO_NEXT_VER="1.27.0"
BASELINE_PIN="v2.22.10"
ISSUE_CARD="tasks/issue/40-gosec-go127-export-data.md"
RECEIPT_START="<!-- TASK-270 receipt:start -->"
RECEIPT_END="<!-- TASK-270 receipt:end -->"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

info() {
	echo "[gosec-contract] $*"
}

# 공통 전제: 검증 대상 makefile 과 최소 스캔 대상 패키지가 있어야 한다.
[ -f .make/tools.mk ] || fail ".make/tools.mk not found under $repo (run from the repository root)"
[ -d pkg/gitsettings ] || fail "pkg/gitsettings not found under $repo (run from the repository root)"

pin="$(sed -n 's/^GOSEC_VERSION ?= //p' .make/tools.mk | head -1)"
if [ -z "$pin" ] || ! printf '%s' "$pin" | grep -q '^v[0-9]'; then
	fail "cannot parse GOSEC_VERSION pin from .make/tools.mk (got: '${pin:-empty}')"
fi
gosec_module="$(sed -n 's/^GOSEC_MODULE := //p' .make/tools.mk | head -1)"
[ -n "$gosec_module" ] || fail "cannot parse GOSEC_MODULE from .make/tools.mk"

# mise 설치 고정 컴파일러를 찾는다. 없으면 skip 성공이 아니라 실패가 계약이다.
find_go() { # $1: Go 버전(예: 1.26.7) -> go 바이너리 경로 출력
	local ver="$1" dir
	if command -v mise >/dev/null 2>&1 && dir="$(mise where go "$ver" 2>/dev/null)" && [ -x "$dir/bin/go" ]; then
		echo "$dir/bin/go"
		return 0
	fi
	dir="$HOME/.local/share/mise/installs/go/$ver"
	if [ -x "$dir/bin/go" ]; then
		echo "$dir/bin/go"
		return 0
	fi
	return 1
}

resolve_go() { # $1: Go 버전 -> GO_BIN/GO_ROOT 설정, 실제 버전 검증
	local ver="$1" got
	GO_BIN="$(find_go "$ver")" || fail "fixed compiler go${ver} is not installed (mise install go@${ver}); 이 계약은 컴파일러 누락 시 skip 성공을 허용하지 않는다"
	GO_ROOT="$(cd "$(dirname "$GO_BIN")/.." && pwd -P)"
	got="$("$GO_BIN" version | awk '{print $3}')"
	[ "$got" = "go${ver}" ] || fail "go binary at $GO_BIN reports $got, expected go${ver}"
	info "compiler go${ver}: $GO_BIN"
}

tmp="$(mktemp -d "${TMPDIR:-/tmp}/gosec-contract.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

resolve_go "$GO_FLOOR_VER"
GO_FLOOR_BIN="$GO_BIN"
GO_FLOOR_ROOT="$GO_ROOT"
GOEXE="$("$GO_FLOOR_BIN" env GOEXE)"

module_pin_of() { # $1: 바이너리 -> go version -m 의 module pin (빌드 툴체인은 보지 않는다)
	"$GO_FLOOR_BIN" version -m "$1" 2>/dev/null | awk -v m="$gosec_module" '$1 == "mod" && $2 == m {print $3; exit}'
}

check_identity() { # $1: 바이너리  $2: 기대 컴파일러(1.26.7 등)  $3: 기대 핀
	"$GO_FLOOR_BIN" version -m "$1" 2>/dev/null | awk -v want="go$2" -v m="$gosec_module" -v v="$3" '
		NR == 1 { built = $NF }
		$1 == "mod" && $2 == m && $3 == v { found = 1 }
		END { exit !(found && built == want) }' \
		|| fail "$1 is not $gosec_module $3 built with go$2"
	info "identity: $gosec_module $3 built with go$2 OK"
}

build_gosec() { # $1: go 바이너리  $2: GOROOT  $3: GOBIN dir  $4: 버전 -> 바이너리 경로 출력
	local gobin="$1" root="$2" bindir="$3" ver="$4" exe
	mkdir -p "$bindir"
	exe="$("$gobin" env GOEXE)"
	GOWORK=off GOTOOLCHAIN=local GOROOT="$root" GOBIN="$bindir" "$gobin" install "${gosec_module}/cmd/gosec@${ver}" \
		|| fail "go install ${gosec_module}/cmd/gosec@${ver} with $gobin failed"
	[ -x "$bindir/gosec${exe}" ] || fail "built gosec binary missing at $bindir/gosec${exe}"
	echo "$bindir/gosec${exe}"
}

# 스캔 시점(패키지를 적재하는) go 툴체인을 고정해 최소 스캔을 실행한다.
# export-data 파손은 gosec 를 빌드한 컴파일러가 아니라 이 go 가 결정한다(issue 40 재실측).
run_minimal() { # $1: gosec 바이너리  $2: GOROOT  $3: 결과 prefix; RC_ 전역 설정
	RC_=0
	(
		export GOWORK=off GOTOOLCHAIN=local GOROOT="$2"
		export PATH="$2/bin:$PATH"
		"$1" -fmt=json ./pkg/gitsettings
	) >"$3.json" 2>"$3.stderr" || RC_=$?
}

# 한 스캔 결과를 판정한다. exit 1 은 '유효 JSON + 기존 finding' 일 때만 허용하고,
# export-data 사인, type/적재 오류, JSON 누락은 모두 거절한다.
validate_scan() { # $1: cell 이름  $2: exit  $3: json 파일  $4: stderr 파일
	local cell="$1" rc="$2" jsonf="$3" errf="$4" issues gerr bytes
	if grep -q 'without types was imported' "$errf" 2>/dev/null; then
		fail "$cell: export-data failure signature in stderr (this pin cannot read that toolchain's export data)"
	fi
	if grep -q 'internal error: package' "$errf" 2>/dev/null; then
		fail "$cell: go/types internal error in stderr"
	fi
	bytes="$(wc -c <"$jsonf" 2>/dev/null | tr -d ' ')"
	if ! jq -e . "$jsonf" >/dev/null 2>&1; then
		fail "$cell: exit=$rc but gosec produced no valid JSON (${bytes:-0} bytes)"
	fi
	gerr="$(jq -r '.["Golang errors"] | length' "$jsonf")"
	[ "$gerr" = "0" ] || fail "$cell: gosec reported $gerr Golang error group(s) — type/load errors are not findings"
	issues="$(jq -r '.Issues | length' "$jsonf")"
	if [ "$rc" -eq 0 ]; then
		info "$cell: exit=0 issues=$issues"
	elif [ "$rc" -eq 1 ]; then
		[ "$issues" -gt 0 ] || fail "$cell: exit=1 with 0 findings — the exit code does not come from valid findings"
		info "$cell: exit=1 issues=$issues (pre-existing findings)"
	else
		fail "$cell: unexpected gosec exit=$rc"
	fi
}

mode="${1:-default}"

if [ "$mode" = "default" ]; then
	# --- 기본 모드: bin/tools 핀 일치 + 양쪽 컴파일러 재빌드 identity + 최소 스캔 4셀 ---
	info "mode: default (pin=$gosec_module $pin)"
	resolve_go "$GO_NEXT_VER"
	GO_NEXT_BIN="$GO_BIN"
	GO_NEXT_ROOT="$GO_ROOT"

	bin_gosec="$repo/bin/tools/gosec${GOEXE}"
	if [ ! -x "$bin_gosec" ] || [ "$(module_pin_of "$bin_gosec")" != "$pin" ]; then
		info "bin/tools gosec is missing or not pinned to $pin; provisioning via make install-gosec"
		make install-gosec >/dev/null || fail "make install-gosec failed while provisioning bin/tools gosec"
	fi
	got_pin="$(module_pin_of "$bin_gosec")"
	[ "$got_pin" = "$pin" ] || fail "bin/tools gosec at $bin_gosec is ${got_pin:-unknown}, tools.mk pins $pin"

	b126="$(build_gosec "$GO_FLOOR_BIN" "$GO_FLOOR_ROOT" "$tmp/default-go126" "$pin")"
	b127="$(build_gosec "$GO_NEXT_BIN" "$GO_NEXT_ROOT" "$tmp/default-go127" "$pin")"
	check_identity "$b126" "$GO_FLOOR_VER" "$pin"
	check_identity "$b127" "$GO_NEXT_VER" "$pin"

	run_minimal "$b126" "$GO_FLOOR_ROOT" "$tmp/min-126-126"
	validate_scan "minimal: gosec(1.26.7-built) x scan-go(1.26.7)" "$RC_" "$tmp/min-126-126.json" "$tmp/min-126-126.stderr"
	run_minimal "$b126" "$GO_NEXT_ROOT" "$tmp/min-126-127"
	validate_scan "minimal: gosec(1.26.7-built) x scan-go(1.27.0)" "$RC_" "$tmp/min-126-127.json" "$tmp/min-126-127.stderr"
	run_minimal "$b127" "$GO_FLOOR_ROOT" "$tmp/min-127-126"
	validate_scan "minimal: gosec(1.27.0-built) x scan-go(1.26.7)" "$RC_" "$tmp/min-127-126.json" "$tmp/min-127-126.stderr"
	run_minimal "$b127" "$GO_NEXT_ROOT" "$tmp/min-127-127"
	validate_scan "minimal: gosec(1.27.0-built) x scan-go(1.27.0)" "$RC_" "$tmp/min-127-127.json" "$tmp/min-127-127.stderr"

	info "summary: pin=$gosec_module $pin; bin/tools pin OK; identities OK (go1.26.7, go1.27.0); 4 minimal scan cells OK"
	info "PASS: default mode"
	exit 0
fi

if [ "$mode" = "--full-security-no-regression" ]; then
	# --- full 모드: 기준 핀과의 전체 security-code 무회귀 비교 (native go1.26.7) ---
	command -v jq >/dev/null 2>&1 || fail "jq is required for --full-security-no-regression"
	command -v make >/dev/null 2>&1 || fail "make is required for --full-security-no-regression"
	[ "$pin" != "$BASELINE_PIN" ] || info "note: current pin equals baseline pin $BASELINE_PIN"

	info "mode: --full-security-no-regression (scan compiler go1.26.7; baseline $BASELINE_PIN vs current $pin)"

	b_base="$(build_gosec "$GO_FLOOR_BIN" "$GO_FLOOR_ROOT" "$tmp/full-baseline" "$BASELINE_PIN")"
	b_curr="$(build_gosec "$GO_FLOOR_BIN" "$GO_FLOOR_ROOT" "$tmp/full-current" "$pin")"
	check_identity "$b_base" "$GO_FLOOR_VER" "$BASELINE_PIN"
	check_identity "$b_curr" "$GO_FLOOR_VER" "$pin"

	src_sha="$(git -C "$repo" rev-parse HEAD)"
	src_branch="$(git -C "$repo" branch --show-current)"

	scan_with() { # $1: label  $2: gosec 바이너리; MAKE_RC_/JSON_RC_ 전역 설정
		local label="$1" bin="$2"
		MAKE_RC_=0
		env GOWORK=off GOTOOLCHAIN=local GOROOT="$GO_FLOOR_ROOT" PATH="$GO_FLOOR_ROOT/bin:$PATH" \
			make security-code "GOSEC=$bin" >"$tmp/$label.make.log" 2>&1 || MAKE_RC_=$?
		JSON_RC_=0
		env GOWORK=off GOTOOLCHAIN=local GOROOT="$GO_FLOOR_ROOT" PATH="$GO_FLOOR_ROOT/bin:$PATH" \
			"$bin" -fmt=json ./... >"$tmp/$label.json" 2>"$tmp/$label.stderr" || JSON_RC_=$?

		if grep -q 'without types was imported\|internal error: package' "$tmp/$label.make.log"; then
			fail "full $label: make security-code hit the export-data/type-error signature (see $tmp/$label.make.log)"
		fi
		[ "$MAKE_RC_" -eq 0 ] || [ "$MAKE_RC_" -eq 1 ] || fail "full $label: make security-code exit=$MAKE_RC_ (expected 0 or findings-1)"
		validate_scan "full $label: gosec -fmt=json ./..." "$JSON_RC_" "$tmp/$label.json" "$tmp/$label.stderr"
		jq -r '.Issues[]? | [.rule_id, .file, .line, .code] | @tsv' "$tmp/$label.json" | LC_ALL=C sort >"$tmp/$label.findings"
		info "full $label: make exit=$MAKE_RC_ json exit=$JSON_RC_ findings=$(wc -l <"$tmp/$label.findings" | tr -d ' ')"
	}

	scan_with baseline "$b_base"
	MAKE_RC_BASE="$MAKE_RC_"
	JSON_RC_BASE="$JSON_RC_"
	scan_with current "$b_curr"
	MAKE_RC_CURR="$MAKE_RC_"
	JSON_RC_CURR="$JSON_RC_"

	new_findings="$(comm -13 "$tmp/baseline.findings" "$tmp/current.findings")"
	resolved_count="$(comm -23 "$tmp/baseline.findings" "$tmp/current.findings" | wc -l | tr -d ' ')"
	base_count="$(wc -l <"$tmp/baseline.findings" | tr -d ' ')"
	curr_count="$(wc -l <"$tmp/current.findings" | tr -d ' ')"
	new_count="$(printf '%s\n' "$new_findings" | grep -c . || true)"

	verdict="PASS — no new findings; no scanner/runtime/export-data errors"
	if [ "$new_count" -gt 0 ]; then
		verdict="FAIL — new findings vs $BASELINE_PIN"
	fi

	# 실행 영수증: 비밀 없는 항목만, 같은 소스에서 재실행하면 동일한 내용으로 덮어쓴다.
	h1_of() { "$GO_FLOOR_BIN" version -m "$1" | awk -v m="$gosec_module" '$1 == "mod" && $2 == m {print $4; exit}'; }
	{
		printf 'mode: --full-security-no-regression (scripts/test-gosec-toolchain-contract.sh)\n'
		printf 'source: %s (%s)\n' "$src_sha" "$src_branch"
		printf 'scan compiler: go%s at %s\n' "$GO_FLOOR_VER" "$GO_FLOOR_ROOT"
		printf 'baseline pin: %s (%s)\n' "$BASELINE_PIN" "$(h1_of "$b_base")"
		printf 'current pin: %s (%s)\n' "$pin" "$(h1_of "$b_curr")"
		printf 'make security-code exits: baseline=%s current=%s\n' "$MAKE_RC_BASE" "$MAKE_RC_CURR"
		printf 'gosec -fmt=json ./... exits: baseline=%s current=%s\n' "$JSON_RC_BASE" "$JSON_RC_CURR"
		printf 'findings (rule_id/file/line/code): baseline=%s current=%s new=%s resolved-by-current=%s\n' \
			"$base_count" "$curr_count" "$new_count" "$resolved_count"
		printf 'verdict: %s\n' "$verdict"
		printf 'private builds/scans ran in an isolated mktemp dir, removed on exit in all outcomes\n'
	} >"$tmp/receipt-body"

	[ -f "$ISSUE_CARD" ] || fail "issue card not found at $ISSUE_CARD (receipt cannot be recorded)"
	awk -v body="$tmp/receipt-body" -v start="$RECEIPT_START" -v end="$RECEIPT_END" '
		index($0, start) == 1 {
			print; while ((getline line < body) > 0) print line; close(body)
			infed = 1; seen = 1; next
		}
		index($0, end) == 1 { infed = 0; print; next }
		!infed { print }
		END {
			if (!seen) {
				print ""; print start
				while ((getline line < body) > 0) print line; close(body)
				print end
			}
		}
	' "$ISSUE_CARD" >"$tmp/issue.new" || fail "failed to rewrite receipt in $ISSUE_CARD"
	mv "$tmp/issue.new" "$ISSUE_CARD"
	info "receipt written into $ISSUE_CARD (between TASK-270 receipt markers)"

	if [ "$new_count" -gt 0 ]; then
		echo "FAIL: new findings introduced by $pin vs $BASELINE_PIN:" >&2
		printf '%s\n' "$new_findings" >&2
		exit 1
	fi
	info "summary: baseline=$BASELINE_PIN findings=$base_count; current=$pin findings=$curr_count; new=0 resolved=$resolved_count"
	info "PASS: --full-security-no-regression"
	exit 0
fi

fail "unknown mode '$mode' (expected: default or --full-security-no-regression)"
