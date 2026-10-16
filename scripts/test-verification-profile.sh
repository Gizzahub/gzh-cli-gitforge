#!/bin/bash
set -euo pipefail

assertion_failed() {
	printf 'verification selector assertion failed at line %s\n' "$1" >&2
	exit 1
}

root=$(cd "$(dirname "$0")/.." && pwd)
selector="$root/scripts/verification-profile.sh"
mkdir -p "$root/tmp"
fixture=$(mktemp -d "$root/tmp/verification-profile-fixture.XXXXXX")
trap 'rm -rf "$fixture"' EXIT HUP INT TERM

cd "$fixture"
git init -q
git config user.email test@example.invalid
git config user.name verification-profile-test
cat >go.mod <<'EOF'
module example.invalid/profile
go 1.26

require example.invalid/foreign v0.0.0
replace example.invalid/foreign => ./foreign
EOF
mkdir -p dep consumer testconsumer disconnected foreign
printf 'module example.invalid/foreign\ngo 1.26\n' >foreign/go.mod
printf 'package foreign\nfunc Value() int { return 1 }\n' >foreign/foreign.go
printf 'package disconnected\nimport ("fmt"; "example.invalid/foreign")\nfunc Value() string { return fmt.Sprint(foreign.Value()) }\n' >disconnected/disconnected.go
printf 'package dep\nfunc Value() int { return 1 }\n' >dep/dep.go
printf 'package consumer\nimport "example.invalid/profile/dep"\nfunc Value() int { return dep.Value() }\n' >consumer/consumer.go
printf 'package testconsumer\n' >testconsumer/testconsumer.go
printf 'package testconsumer\nimport ("testing"; "example.invalid/profile/dep")\nfunc TestValue(t *testing.T) { _ = dep.Value() }\n' >testconsumer/testconsumer_test.go
printf 'fixture=true\n' >config.txt
git add .
git commit -qm base
base=$(git rev-parse HEAD)

mkdir committed && printf 'package committed\n' >committed/committed.go && printf 'package committed\nimport "testing"\nfunc TestCommitted(t *testing.T) {}\n' >committed/committed_test.go && git add committed && git commit -qm committed-change
affected=$(/bin/bash "$selector" select --base "$base")
[[ "$affected" == $'affected\texample.invalid/profile/committed' ]] || assertion_failed "$LINENO"
printf '\n// changed\n' >>dep/dep.go
affected=$(/bin/bash "$selector" select --base "$base")
expected_closure=$'affected\texample.invalid/profile/committed\texample.invalid/profile/consumer\texample.invalid/profile/dep\texample.invalid/profile/testconsumer'
[[ "$affected" == "$expected_closure" ]] || assertion_failed "$LINENO"
git add dep/dep.go
printf '\n// unstaged\n' >>dep/dep.go
[[ $(/bin/bash "$selector" select --base "$base") == "$expected_closure" ]] || assertion_failed "$LINENO"

git reset --hard -q "$base"
mkdir no_test && printf 'package no_test\n' >no_test/code.go
[[ $(/bin/bash "$selector" select --base "$base") == $'full\taffected closure has no Go test files; refusing a no-test pass' ]] || assertion_failed "$LINENO"
rm no_test/code.go && rmdir no_test

mkdir newpkg && printf 'package newpkg\n' >newpkg/new.go && printf 'package newpkg\nimport "testing"\nfunc TestNew(t *testing.T) {}\n' >newpkg/new_test.go
[[ $(/bin/bash "$selector" select --base "$base") == $'affected\texample.invalid/profile/newpkg' ]] || assertion_failed "$LINENO"
rm newpkg/new.go newpkg/new_test.go && rmdir newpkg
git mv dep/dep.go dep/renamed.go
[[ $(/bin/bash "$selector" select --base "$base") == full$'\t'* ]] || assertion_failed "$LINENO"
git reset --hard -q "$base"
git rm -q dep/dep.go
[[ $(/bin/bash "$selector" select --base "$base") == full$'\t'* ]] || assertion_failed "$LINENO"
git reset --hard -q "$base"

printf '\n// mixed regular Go edit\n' >>dep/dep.go
rm config.txt
ln -s dep/dep.go config.txt
git add config.txt
[[ $(/bin/bash "$selector" select --base "$base") == $'full\tmoved, deleted, type-changed, or unmerged path changed' ]] || assertion_failed "$LINENO"
git reset --hard -q "$base"

ln -s dep/dep.go link.go
[[ $(/bin/bash "$selector" select --base "$base") == full$'\t'* ]] || assertion_failed "$LINENO"
rm link.go
printf 'notes\n' >notes.txt
[[ $(/bin/bash "$selector" select --base "$base") == full$'\t'* ]] || assertion_failed "$LINENO"
rm notes.txt
if invalid_base_output=$(/bin/bash "$selector" select --base does-not-exist 2>&1); then echo 'invalid base unexpectedly succeeded' >&2; exit 1; fi
[[ "$invalid_base_output" == *'not a valid commit'* ]] || assertion_failed "$LINENO"

printf '\n// run mode\n' >>dep/dep.go
run_output=$(/bin/bash "$selector" run --base "$base" -- printenv VERIFY_SELECTED_PACKAGES)
[[ "$run_output" == $'example.invalid/profile/consumer\texample.invalid/profile/dep\texample.invalid/profile/testconsumer' ]] || assertion_failed "$LINENO"

fake_git="$fixture/fake-git"
printf '#!/bin/sh\n[ "$1" = diff ] && exit 2\nexec git "$@"\n' >"$fake_git"
chmod +x "$fake_git"
if diff_failure_output=$(VERIFY_GIT_BIN="$fake_git" /bin/bash "$selector" select --base "$base" 2>&1); then echo 'git diff failure unexpectedly succeeded' >&2; exit 1; fi
[[ "$diff_failure_output" == *'git diff --quiet'* ]] || assertion_failed "$LINENO"
printf '#!/bin/sh\n[ "$1" = ls-files ] && exit 2\nexec git "$@"\n' >"$fake_git"
if ls_failure_output=$(VERIFY_GIT_BIN="$fake_git" /bin/bash "$selector" select --base "$base" 2>&1); then echo 'git ls-files failure unexpectedly succeeded' >&2; exit 1; fi
[[ "$ls_failure_output" == *'git ls-files'* ]] || assertion_failed "$LINENO"

# An unexpected file proves that successful selection cannot conceal failed
# cleanup. Remove only this fixture-owned residual after asserting its path.
printf '#!/bin/sh\nif [ "$1" = ls-files ]; then for d in tmp/verification-profile-select.*; do [ -d "$d" ] && : >"$d/unexpected"; done; fi\nexec git "$@"\n' >"$fake_git"
if cleanup_failure_output=$(VERIFY_GIT_BIN="$fake_git" /bin/bash "$selector" select --base "$base" 2>&1); then echo 'cleanup refusal unexpectedly succeeded' >&2; exit 1; fi
cleanup_residual=$(find tmp -maxdepth 1 -type d -name 'verification-profile-select.*' -print)
[[ -n "$cleanup_residual" ]] || assertion_failed "$LINENO"
[[ "$cleanup_failure_output" == *"selector temporary directory remains: $cleanup_residual"* ]] || assertion_failed "$LINENO"
rm "$cleanup_residual/unexpected"
rmdir "$cleanup_residual"

# A TERM delivered while Git is blocked must still run the selector cleanup.
printf '#!/bin/sh\nif [ "$1" = ls-files ]; then kill -TERM "$PPID"; sleep 1; fi\nexec git "$@"\n' >"$fake_git"
VERIFY_GIT_BIN="$fake_git" /bin/bash "$selector" select --base "$base" >"$fixture/term.out" 2>&1 &
selector_pid=$!
if wait "$selector_pid"; then echo 'TERM interruption unexpectedly succeeded' >&2; exit 1; fi
[ -z "$(find tmp -maxdepth 1 -type d -name 'verification-profile-select.*' -print)" ]

echo 'verification profile selector tests passed'
