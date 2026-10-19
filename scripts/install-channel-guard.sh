#!/bin/bash
# install-channel-guard.sh: BINDIR 없는 make install 이 Homebrew 관리본을 조용히 가리지 못하게 막는다
# 용도: 전역 채널은 Homebrew(cask/formula)다. BINDIR 를 주지 않은 설치는 GOBIN 이나
#       GOPATH/bin 으로 가는데, 그 디렉터리가 PATH 에 있으면 brew 사본과 경쟁한다. 앞서면
#       cask 를 가리고 뒤에 있으면 설치본이 실행되지 않는다. 어느 쪽이든 기기마다 다른
#       손빌드가 남으므로 publish 전에 거부한다. 개발 빌드는 BINDIR 를 명시해서 설치한다.
#       PATH 에 없는 디렉터리(make test-install 의 임시 GOBIN 등)는 경쟁하지 않으므로 통과한다.
# 사용법: install-channel-guard.sh <binary-name> <install-bindir> <explicit-bindir>
#         <explicit-bindir> 가 비어 있지 않으면 사용자가 위치를 고른 것이므로 검사하지 않는다.

set -u

BINARY=${1:-}
BINDIR_TARGET=${2:-}
EXPLICIT=${3:-}

if [ -z "$BINARY" ] || [ -z "$BINDIR_TARGET" ]; then
	echo "usage: install-channel-guard.sh <binary-name> <install-bindir> <explicit-bindir>" >&2
	exit 2
fi

[ -z "$EXPLICIT" ] || exit 0
command -v brew >/dev/null 2>&1 || exit 0

# 접두사는 install-path-audit.sh 와 같이 `brew --prefix` 가 답한 것만 믿는다.
_bp=$(brew --prefix 2>/dev/null) || exit 0
[ -n "$_bp" ] || exit 0
prefix_abs=$(cd "$_bp" 2>/dev/null && pwd -P) || exit 0

managed=""
for _m in "$prefix_abs/Caskroom/$BINARY" "$prefix_abs/Cellar/$BINARY"; do
	if [ -d "$_m" ]; then
		managed=$_m
		break
	fi
done
[ -n "$managed" ] || exit 0

target_abs=$(cd "$BINDIR_TARGET" 2>/dev/null && pwd -P) || target_abs=$BINDIR_TARGET
on_path=0
_saved_ifs=$IFS
IFS=:
set -f
for _d in $PATH; do
	[ -n "$_d" ] || continue
	_abs=$(cd "$_d" 2>/dev/null && pwd -P) || continue
	if [ "$_abs" = "$target_abs" ]; then
		on_path=1
		break
	fi
done
set +f
IFS=$_saved_ifs
[ "$on_path" -eq 1 ] || exit 0

echo "install-channel-guard: REFUSED" >&2
echo "  Homebrew 가 '$BINARY' 를 관리한다: $managed" >&2
echo "  BINDIR 없이 설치하면 PATH 의 $target_abs 에 들어가 그 사본과 경쟁한다." >&2
echo "  해소 방법:" >&2
echo "    - 전역 사용: brew upgrade $BINARY" >&2
echo "    - 개발 빌드: make install BINDIR=<디렉터리>  (예: DevBox 의 tmp/bin)" >&2
exit 1
