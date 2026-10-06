#!/bin/bash
# test-install-path-audit.sh: install-path-audit.sh 가 실패해야 할 때 실제로 실패하는지 검증한다
# 용도: 감사 스크립트를 합성 PATH 위에서 돌린다. 실환경은 대개 깨끗하므로 "오늘 통과했다" 는
#       그 감사가 살아 있다는 증거가 되지 못한다. 그림자·중복·회수·신원미확인·판정불가를
#       각각 만들어 놓고 판정과 종료코드를 잰다.
# 사용법: test-install-path-audit.sh

set -u

here=$(cd "$(dirname "$0")" && pwd -P)
audit="$here/install-path-audit.sh"
tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/gz-git-audit.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT

failures=0
BASE_PATH=/usr/bin:/bin:/usr/sbin:/sbin
AUDIT_BASH_ENV=/dev/null
AUDIT_ENV=/dev/null

# 자기를 'gz-git' 이라고 밝히는 가짜. 감사는 절대경로로 --version 을 물어 신원을 확인한다.
make_fake() {
	mkdir -p "$1"
	cat >"$1/gz-git" <<FAKE
#!/bin/bash
echo "gz-git version $2"
FAKE
	chmod +x "$1/gz-git"
}

# 같은 버전 문자열이라도 실제 내용이 다른 사본을 만든다. audit은 버전 비교가 아니라
# command -v 결과를 설치 경로와 대조해야 이 그림자를 잡을 수 있다.
make_changed_same_version_fake() {
	make_fake "$1" "$2"
	printf '# changed build marker\n' >>"$1/gz-git"
}

# 이름만 같고 자기를 밝히지 않는 남의 파일.
make_impostor() {
	mkdir -p "$1"
	cat >"$1/gz-git" <<'IMPOSTOR'
#!/bin/bash
echo "some other tool 9.9"
IMPOSTOR
	chmod +x "$1/gz-git"
}

# mise 흉내. `which <bin>` 에는 정본 경로를 답하고(빈 경로면 실패로), 디스패처로 실행되면
# mise 다운 자기 소개를 한다 — 'gz-git version' 이 아니므로 감사의 신원 확인은 통과하지 못한다.
make_fake_mise() {
	mkdir -p "$1"
	{
		printf '#!/bin/bash\n'
		printf 'if [ "$1" = "which" ]; then\n'
		if [ -n "$2" ]; then
			printf 'echo %q\n' "$2"
			printf 'exit 0\n'
		fi
		printf 'exit 1\n'
		printf 'fi\n'
		printf 'echo "mise 2026.9.15 fake"\n'
	} >"$1/mise"
	chmod +x "$1/mise"
}

# brew 흉내. `brew --prefix` 에 접두사를 답한다.
make_fake_brew() {
	mkdir -p "$1"
	{
		printf '#!/bin/bash\n'
		printf 'if [ "$1" = "--prefix" ]; then echo %q; exit 0; fi\n' "$2"
		printf 'exit 1\n'
	} >"$1/brew"
	chmod +x "$1/brew"
}

check() {
	name=$1 want_rc=$2 want_text=$3 got_rc=$4 got_out=$5
	if [ "$got_rc" != "$want_rc" ]; then
		echo "FAIL [$name] rc: want=$want_rc got=$got_rc" >&2
		printf '%s\n' "$got_out" | sed 's/^/       /' >&2
		failures=$((failures + 1))
		return
	fi
	if [ -n "$want_text" ] && ! printf '%s' "$got_out" | grep -Fq "$want_text"; then
		echo "FAIL [$name] output missing: $want_text" >&2
		printf '%s\n' "$got_out" | sed 's/^/       /' >&2
		failures=$((failures + 1))
		return
	fi
	echo "ok   [$name]"
}

# 1) 깨끗한 환경 — 설치본만 PATH 에 있다.
good="$tmpdir/good bin"
make_fake "$good" 0.7.0

# A caller's non-interactive shell startup must not rewrite the synthetic PATH.
# Without the per-invocation BASH_ENV/ENV isolation below, a workstation-level
# BASH_ENV can expose and even let a RECLAIM case delete a real installed binary.
ambient="$tmpdir/ambient bin"
make_fake "$ambient" 9.9.0
host_bash_env="$tmpdir/host-bash-env"
printf 'export PATH=%q:$PATH\n' "$ambient" >"$host_bash_env"
export BASH_ENV="$host_bash_env"
export ENV="$host_bash_env"

out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$good:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check clean 0 "install-path-audit: OK" "$?" "$out"

# 2) 그림자 — 설치본보다 PATH 앞에 다른 파일이 있다. 설치는 성공했지만 실행되지 않는다.
#    앞선 파일의 버전을 설치본과 **같게** 둔다. 다르게 두면 중복 판정이 같은 실패를
#    만들어내서 그림자 검사를 무력화해도 이 케이스가 통과해 버린다(돌연변이로 확인됨).
same_ver_shadow="$tmpdir/same version shadow bin"
make_changed_same_version_fake "$same_ver_shadow" 0.7.0
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$same_ver_shadow:$good:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check shadow 1 "방금 설치한 파일이 실행되지 않는다" "$?" "$out"

shadow="$tmpdir/shadow bin"
make_fake "$shadow" 0.6.1

# 3) 중복 — 다른 버전이 뒤에 있다. 지금은 무해하지만 PATH 순서가 바뀌면 2번이 된다.
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$good:$shadow:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check duplicate 1 "다른 버전의 'gz-git' 가 PATH 에 있다" "$?" "$out"

# 4) WARN_ONLY — 같은 결함을 보고하되 실패시키지 않는다.
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" INSTALL_AUDIT_WARN_ONLY=1 PATH="$good:$shadow:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check warn-only 0 "install-path-audit: FAIL" "$?" "$out"

# 5) 신원 미확인 — 이름만 같은 남의 파일은 RECLAIM 이 켜져 있어도 삭제하지 않는다.
impostor="$tmpdir/impostor bin"
make_impostor "$impostor"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$impostor:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
rc=$?
check impostor-not-deleted 0 "신원이 확인되지 않는 파일이다" "$rc" "$out"
if [ ! -f "$impostor/gz-git" ]; then
	echo "FAIL [impostor-not-deleted] 남의 파일이 삭제됐다" >&2
	failures=$((failures + 1))
fi

# 6) 회수 — 신원이 확인된 여러 중복만 삭제하고 통과로 돌아선다. 한 개를 회수해도
#    남은 실패를 잊지 않도록 audit은 divergent를 집계한다.
reclaim="$tmpdir/reclaim bin"
reclaim_second="$tmpdir/reclaim second bin"
make_fake "$reclaim" 0.6.1
make_fake "$reclaim_second" 0.5.2
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$reclaim:$reclaim_second:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
rc=$?
check reclaim 0 "회수함: 삭제 완료" "$rc" "$out"
if [ -f "$reclaim/gz-git" ]; then
	echo "FAIL [reclaim] 중복본이 남아 있다" >&2
	failures=$((failures + 1))
fi
if [ -f "$reclaim_second/gz-git" ]; then
	echo "FAIL [reclaim] 두 번째 중복본이 남아 있다" >&2
	failures=$((failures + 1))
fi
if [ ! -f "$ambient/gz-git" ]; then
	echo "FAIL [ambient-not-deleted] BASH_ENV 가 노출한 외부 바이너리가 삭제됐다" >&2
	failures=$((failures + 1))
fi

# 7) 판정 불가 — 설치 위치가 PATH 에 없다. 통과가 아니라 SKIP 이어야 한다.
#    make test-install 이 임시 BINDIR 로 install 을 재귀 호출하는 경로가 여기다.
off="$tmpdir/off path bin"
make_fake "$off" 0.7.0
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$BASE_PATH" "$audit" gz-git "$off/gz-git" 2>&1)
check off-path 0 "install-path-audit: SKIP" "$?" "$out"

# 8) mise 설치본 그림자 — mise 가 고른 설치본도 방금 설치한 파일을 가리면 실패한다.
#    mise 설치본 자체는 fallback으로 허용되며 RECLAIM 보호 대상이다.
mise_bin="$tmpdir/mise bin"
mise_install="$tmpdir/mise installs dir"
make_fake "$mise_install" 0.8.0
make_fake_mise "$mise_bin" "$mise_install/gz-git"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$mise_install:$good:$mise_bin:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check mise-canonical-shadow 1 "mise 설치본이 앞서 있다" "$?" "$out"

# 9) mise 디스패처 그림자 — 심이 mise 바이너리 자체를 가리키는 표준 mise 배치도 설치본을
#    가리면 실패한다. 심 자체는 RECLAIM 보호 대상이며 INFO 로 분류된다.
shims="$tmpdir/shims dir"
mkdir -p "$shims"
ln -s "$mise_bin/mise" "$shims/gz-git"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$shims:$good:$mise_bin:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
rc=$?
check mise-dispatcher-shadow 1 "mise 디스패처 심이 앞서 있다" "$rc" "$out"
check mise-dispatcher-shim-info 1 "mise 디스패처 심이다" "$rc" "$out"

# 10) mise 가 이 이름을 관리하지 않으면 — 정본 인정 없이 기존 FAIL 을 유지한다(fail-closed).
mise_unmanaged="$tmpdir/mise unmanaged bin"
unmanaged_shadow="$tmpdir/unmanaged shadow bin"
make_fake "$unmanaged_shadow" 0.5.0
make_fake_mise "$mise_unmanaged" ""
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$unmanaged_shadow:$good:$mise_unmanaged:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check mise-unmanaged-shadow 1 "방금 설치한 파일이 실행되지 않는다" "$?" "$out"

# 11) 설치본이 먼저 선택되는 자리에서는 mise fallback 사본이 다른 버전이어도 공존 가능하다.
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$good:$mise_install:$mise_bin:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check mise-canonical-duplicate 0 "mise 정본 설치본이다:" "$?" "$out"

# Homebrew 흉내 prefix. 케이스마다 새로 만들어, 한 케이스가 파일을 지워도 다음 케이스의
# 실패 원인이 흐려지지 않게 한다. $3 이 caskroom 이면 cask 배치(bin -> Caskroom),
# cellar 면 formula 배치(bin -> Cellar), plain 이면 bin 에 직접 놓인 파일이다.
make_brew_prefix() {
	_pfx=$1 _ver=$2 _kind=$3
	mkdir -p "$_pfx/bin"
	case $_kind in
	caskroom)
		make_fake "$_pfx/Caskroom/gz-git/$_ver" "$_ver"
		ln -s "../Caskroom/gz-git/$_ver/gz-git" "$_pfx/bin/gz-git"
		;;
	cellar)
		make_fake "$_pfx/Cellar/gz-git/$_ver/bin" "$_ver"
		ln -s "../Cellar/gz-git/$_ver/bin/gz-git" "$_pfx/bin/gz-git"
		;;
	plain) make_fake "$_pfx/bin" "$_ver" ;;
	esac
}
expect_file() {
	if [ ! -f "$2" ]; then
		echo "FAIL [$1] 파일이 삭제됐다: $2" >&2
		failures=$((failures + 1))
	fi
}

# 12) Homebrew cask 관리본 — 다른 버전이어도 중복 FAIL 이 아니고 RECLAIM 으로도 지우지 않는다.
#     brew 소유 파일을 지우면 brew 의 설치 기록과 실제 파일이 어긋난다.
p12="$tmpdir/brew12 prefix"
b12="$tmpdir/brew12 cmd"
make_brew_prefix "$p12" 0.9.0 caskroom
make_fake_brew "$b12" "$p12"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$p12/bin:$b12:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-owned-duplicate 0 "Homebrew 관리본이다" "$?" "$out"
expect_file brew-owned-duplicate "$p12/Caskroom/gz-git/0.9.0/gz-git"

# 13) Homebrew formula 관리본(Cellar)도 같은 규칙이다.
p13="$tmpdir/brew13 prefix"
b13="$tmpdir/brew13 cmd"
make_brew_prefix "$p13" 0.9.0 cellar
make_fake_brew "$b13" "$p13"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$p13/bin:$b13:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-cellar-duplicate 0 "Homebrew 관리본이다" "$?" "$out"
expect_file brew-cellar-duplicate "$p13/Cellar/gz-git/0.9.0/bin/gz-git"

# 14) 보호 경계 — prefix 아래라도 Caskroom/Cellar 밖(bin 에 직접 놓인 파일)은 brew 소유가 아니다.
#     다른 버전 중복이고 RECLAIM 이 회수한다.
p14="$tmpdir/brew14 prefix"
b14="$tmpdir/brew14 cmd"
make_brew_prefix "$p14" 0.9.0 plain
make_fake_brew "$b14" "$p14"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$p14/bin:$b14:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-prefix-plain-reclaimed 0 "회수함: 삭제 완료" "$?" "$out"
if [ -f "$p14/bin/gz-git" ]; then
	echo "FAIL [brew-prefix-plain-reclaimed] prefix/bin 의 비관리 사본이 남아 있다" >&2
	failures=$((failures + 1))
fi

# 15) symlink prefix — brew 가 심링크 경로로 prefix 를 답해도 물리 경로로 맞춰 보호한다.
#     TMPDIR 이 물리 경로인 CI 에서도 이 분기를 잴 수 있도록 심링크를 명시적으로 만든다.
p15="$tmpdir/brew15 real prefix"
l15="$tmpdir/brew15 link prefix"
b15="$tmpdir/brew15 cmd"
make_brew_prefix "$p15" 0.9.0 caskroom
ln -s "$p15" "$l15"
make_fake_brew "$b15" "$l15"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$l15/bin:$b15:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-symlinked-prefix 0 "Homebrew 관리본이다" "$?" "$out"
expect_file brew-symlinked-prefix "$p15/Caskroom/gz-git/0.9.0/gz-git"

# 16) Homebrew 관리본 그림자 — 보호는 PATH 우선순위를 승인하지 않는다.
p16="$tmpdir/brew16 prefix"
b16="$tmpdir/brew16 cmd"
make_brew_prefix "$p16" 0.9.0 caskroom
make_fake_brew "$b16" "$p16"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" PATH="$p16/bin:$good:$b16:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-owned-shadow 1 "Homebrew 설치본이 앞서 있다" "$?" "$out"

# 17) brew 가 없으면 경로 모양만으로 통과시키지 않는다(fail-closed). RECLAIM 이 켜져 있어도
#     Homebrew 모양 경로는 지우지 않고 FAIL 로 남긴다 — 다른 prefix 나 --prefix 실패도 같은 길이다.
p17="$tmpdir/brew17 prefix"
make_brew_prefix "$p17" 0.9.0 caskroom
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$p17/bin:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-absent-fail-closed 1 "회수 보류: Homebrew 경로로 보이나" "$?" "$out"
expect_file brew-absent-fail-closed "$p17/Caskroom/gz-git/0.9.0/gz-git"

# 18) 다른 prefix — brew 가 답한 prefix 와 다른 Homebrew 의 Cellar(formula) 사본도 지우지
#     않는다. 17 이 Caskroom 을 재므로 보류 패턴의 두 갈래가 모두 고정된다.
p18a="$tmpdir/brew18 answered prefix"
p18b="$tmpdir/brew18 other prefix"
b18="$tmpdir/brew18 cmd"
mkdir -p "$p18a"
make_brew_prefix "$p18b" 0.9.0 cellar
make_fake_brew "$b18" "$p18a"
out=$(BASH_ENV="$AUDIT_BASH_ENV" ENV="$AUDIT_ENV" RECLAIM=1 PATH="$good:$p18b/bin:$b18:$BASE_PATH" "$audit" gz-git "$good/gz-git" 2>&1)
check brew-other-prefix-kept 1 "회수 보류: Homebrew 경로로 보이나" "$?" "$out"
expect_file brew-other-prefix-kept "$p18b/Cellar/gz-git/0.9.0/bin/gz-git"

if [ "$failures" -ne 0 ]; then
	echo "install-path-audit tests: $failures failed" >&2
	exit 1
fi
echo "install-path-audit tests: all passed"
