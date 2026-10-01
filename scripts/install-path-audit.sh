#!/bin/bash
# install-path-audit.sh: 설치본이 실제로 실행되는 바이너리인지 검사하고 PATH 상의 중복본을 보고/회수한다
# 용도: `make install` 직후에 세 가지를 판정한다.
#         (1) 그림자 — 방금 설치한 파일과 셸이 실제로 고르는 파일이 다른가.
#         (2) 중복 — PATH 어딘가에 다른 버전의 동명 바이너리가 있어, PATH 순서가 바뀌면 (1)이 되는가.
#         (3) mise 공존 — mise 가 고른 설치본은 보호된 fallback 사본이다. 다만 그 사본이
#             방금 설치한 파일보다 먼저 선택되면 설치본 그림자이므로 반드시 실패한다.
#       두 판정 모두 이 셸이 수행한다. 설치된 바이너리에게 "네가 맞느냐"고 묻지 않으므로,
#       낡은 바이너리가 자기 자신을 무죄로 판정하는 순환이 없다. 낡은 바이너리에게 요구하는
#       협조는 `--version` 으로 자기 이름과 버전을 말하는 것뿐이고, 그것은 모든 버전이 이미 한다.
# 사용법: install-path-audit.sh <binary-name> <installed-path>
#         RECLAIM=1                  신원이 확인된 중복본을 삭제한다 (기본: 보고만).
#                                    mise 정본 설치본과 mise 디스패처 심은 대상에서 제외한다.
#         INSTALL_AUDIT_WARN_ONLY=1  판정을 보고만 하고 실패시키지 않는다

set -u

BINARY=${1:-}
INSTALLED=${2:-}

if [ -z "$BINARY" ] || [ -z "$INSTALLED" ]; then
	echo "usage: install-path-audit.sh <binary-name> <installed-path>" >&2
	exit 2
fi

RECLAIM=${RECLAIM:-0}
WARN_ONLY=${INSTALL_AUDIT_WARN_ONLY:-0}

# 심링크를 따라가고 디렉터리를 물리 경로로 정규화한다. macOS 는 /var -> /private/var 처럼
# 상위 디렉터리 자체가 심링크라 문자열 비교만으로는 같은 파일을 다르다고 판정한다.
resolve() {
	_p=$1
	_n=0
	while [ -L "$_p" ] && [ "$_n" -lt 16 ]; do
		_t=$(readlink "$_p") || break
		case $_t in
		/*) _p=$_t ;;
		*) _p=$(dirname "$_p")/$_t ;;
		esac
		_n=$((_n + 1))
	done
	_d=$(cd "$(dirname "$_p")" 2>/dev/null && pwd -P) || {
		printf '%s\n' "$_p"
		return 0
	}
	printf '%s/%s\n' "$_d" "$(basename "$_p")"
}

# 후보를 절대경로로 직접 실행한다. PATH 해석을 거치지 않으므로 "누가 답했는가" 가 확정된다.
# 자기 이름을 정확히 밝힌 것만 우리 것으로 인정한다 — 이름만 같은 남의 파일은 건드리지 않는다.
identify() {
	_out=$("$1" --version 2>/dev/null) || return 1
	case $_out in
	"$BINARY version "*) printf '%s\n' "${_out#"$BINARY version "}" ;;
	*) return 1 ;;
	esac
}

installed_abs=$(resolve "$INSTALLED")
installed_dir=$(dirname "$installed_abs")

# PATH 항목을 물리 경로로 한 번만 수집한다. 공백이 든 경로가 깨지지 않도록 IFS 는 ':' 로만
# 자르고 글로빙은 끈다.
path_dirs=""
_saved_ifs=$IFS
IFS=:
set -f
for _d in $PATH; do
	[ -n "$_d" ] || continue
	_abs=$(cd "$_d" 2>/dev/null && pwd -P) || continue
	path_dirs="$path_dirs$_abs
"
done
set +f
IFS=$_saved_ifs

# 설치 위치가 PATH 에 없으면 "셸이 무엇을 고르는가" 라는 질문 자체가 성립하지 않는다.
# 판정 불가는 통과가 아니라 SKIP 으로 표시한다. make test-install 이 임시 BINDIR 로
# install 을 재귀 호출하는 경로가 여기에 해당한다.
if ! printf '%s' "$path_dirs" | grep -Fxq "$installed_dir"; then
	echo "install-path-audit: SKIP"
	echo "  설치 위치가 PATH 에 없어 그림자/중복 판정을 하지 않았다: $installed_dir"
	echo "  이 셸에서 '$BINARY' 를 실행하면 방금 설치한 것이 아닌 다른 것이 실행된다."
	exit 0
fi

# mise 정본 — `mise which` 는 mise 가 이 컨텍스트에서 내려는 실제 경로(심이 아닌 설치본)를
# 돌려준다. 이 경로와 mise 디스패처 심은 RECLAIM 대상에서 보호한다. 보호는 PATH 우선순위를
# 승인하지 않는다. 설치본보다 먼저 선택되면 아래 그림자 판정은 항상 FAIL 이다.
mise_bin_abs=""
canonical_abs=""
if command -v mise >/dev/null 2>&1; then
	mise_bin_abs=$(resolve "$(command -v mise)")
	_w=$(mise which "$BINARY" 2>/dev/null) || _w=""
	[ -n "$_w" ] && canonical_abs=$(resolve "$_w")
fi

shadowed=0
divergent=0

# (1) 그림자 — make 가 아는 사실(방금 쓴 경로)과 셸이 고르는 것을 대조한다.
resolved=$(command -v "$BINARY" 2>/dev/null || true)
if [ -z "$resolved" ]; then
	echo "  WARN: PATH 에 설치했는데도 '$BINARY' 가 해석되지 않는다"
	shadowed=1
else
	resolved_abs=$(resolve "$resolved")
	if [ "$resolved_abs" = "$installed_abs" ]; then
		echo "  OK: 셸이 고르는 '$BINARY' 가 방금 설치한 파일이다"
	else
		shadowed=1
		echo "  FAIL: 방금 설치한 파일이 실행되지 않는다"
		echo "        설치함: $installed_abs"
		echo "        실행됨: $resolved_abs ($(identify "$resolved_abs" || echo '버전 확인 불가'))"
		if [ -n "$canonical_abs" ] && [ "$resolved_abs" = "$canonical_abs" ]; then
			echo "        mise 설치본이 앞서 있다. $installed_dir 를 mise shim/설치 경로보다 PATH 앞에 둬라."
		elif [ -n "$mise_bin_abs" ] && [ "$resolved_abs" = "$mise_bin_abs" ]; then
			echo "        mise 디스패처 심이 앞서 있다. $installed_dir 를 mise shim 경로보다 PATH 앞에 둬라."
		fi
	fi
fi

# (2) 중복 — PATH 순서가 바뀌면 (1)이 될 파일들. 지금 당장은 무해하지만 잠복한 지뢰다.
installed_version=$(identify "$installed_abs" || true)
seen=""
while IFS= read -r dir; do
	[ -n "$dir" ] || continue
	cand="$dir/$BINARY"
	[ -f "$cand" ] && [ -x "$cand" ] || continue
	cand_abs=$(resolve "$cand")
	[ "$cand_abs" = "$installed_abs" ] && continue
	case "$seen" in *"[$cand_abs]"*) continue ;; esac
	seen="$seen[$cand_abs]"

	# mise 디스패처 심 — 이름만 같은 파일이 mise 바이너리 자체를 가리키는 것은 mise 의
	# 정상 구조다. 남의 파일이 아니므로 WARN 도 아니고 RECLAIM 대상도 아니다.
	if [ -n "$mise_bin_abs" ] && [ "$cand_abs" = "$mise_bin_abs" ]; then
		echo "  INFO: mise 디스패처 심이다 (삭제 대상이 아니다): $cand_abs"
		continue
	fi
	# mise 정본 설치본 — fallback 사본은 중복 판정과 RECLAIM 대상에서 제외한다.
	if [ -n "$canonical_abs" ] && [ "$cand_abs" = "$canonical_abs" ]; then
		echo "  INFO: mise 정본 설치본이다: $cand_abs ($(identify "$cand_abs" || echo '버전 확인 불가'))"
		continue
	fi

	if cand_version=$(identify "$cand_abs"); then
		if [ -n "$installed_version" ] && [ "$cand_version" = "$installed_version" ]; then
			echo "  INFO: 같은 버전의 사본이 PATH 에 있다: $cand_abs ($cand_version)"
			continue
		fi
		# 여러 다른 버전이 있을 수 있다. RECLAIM 이 하나를 지웠다고 나머지 실패를
		# 잊지 않도록 개수로 집계한다.
		divergent=$((divergent + 1))
		echo "  FAIL: 다른 버전의 '$BINARY' 가 PATH 에 있다"
		echo "        $cand_abs ($cand_version) — 설치본은 ${installed_version:-확인불가}"
		if [ "$RECLAIM" = "1" ]; then
			if rm -f "$cand_abs"; then
				echo "        회수함: 삭제 완료"
				divergent=$((divergent - 1))
			else
				echo "        회수 실패: 권한을 확인하라" >&2
			fi
		fi
	else
		# 이름은 같지만 자기를 '$BINARY' 라고 밝히지 않았다. 남의 파일일 수 있으므로
		# RECLAIM 이 켜져 있어도 삭제하지 않는다.
		echo "  WARN: 이름만 같고 신원이 확인되지 않는 파일이다 (삭제하지 않음): $cand_abs"
	fi
done <<EOF
$path_dirs
EOF

if [ "$shadowed" -eq 0 ] && [ "$divergent" -eq 0 ]; then
	echo "install-path-audit: OK"
	exit 0
fi

echo "install-path-audit: FAIL"
echo "  해소 방법:"
echo "    - $installed_dir 를 mise shim 및 다른 '$BINARY' 경로보다 PATH 앞에 둬라"
echo "    - RECLAIM=1 make install  (mise 보호 경로를 제외한 신원이 확인된 다른 버전만 회수한다)"
echo "    - INSTALL_AUDIT_WARN_ONLY=1 로 이 판정을 보고만 받게 할 수 있다"

[ "$WARN_ONLY" = "1" ] && exit 0
exit 1
