#!/bin/bash
# Bash 3.2-compatible impact selector. It deliberately has no implicit base.
set -euo pipefail

git_bin=${VERIFY_GIT_BIN:-git}
go_bin=${VERIFY_GO_BIN:-go}

usage() { printf 'usage: %s (select|run) --base <source-ref> [-- command ...]\n' "$0" >&2; exit 2; }
cleanup() {
	[ -n "${selector_tmp:-}" ] || return
	rm -f "$selector_tmp/branch" "$selector_tmp/cached" "$selector_tmp/worktree" "$selector_tmp/untracked" "$selector_tmp/dirs" "$selector_tmp/sorted-dirs" "$selector_tmp/selected" "$selector_tmp/metadata" "$selector_tmp/reverse"
	if ! rmdir "$selector_tmp"; then
		printf 'verification profile: selector temporary directory remains: %s\n' "$selector_tmp" >&2
		return 1
	fi
}
finish() {
	op_status=$?
	trap - EXIT
	cleanup_status=0
	cleanup || cleanup_status=$?
	[ "$op_status" -ne 0 ] && exit "$op_status"
	exit "$cleanup_status"
}
fail() { printf 'verification profile: %s\n' "$*" >&2; exit 2; }
require_git_output() { file=$1; shift; "$git_bin" "$@" >"$file" || fail "git $* failed"; }
structural_change() {
	if "$git_bin" diff --quiet --diff-filter=DRTU "$@"; then
		return 0
	else
		rc=$?
	fi
	[ "$rc" -eq 1 ] || fail "git diff --quiet --diff-filter=DRTU $* failed"
	printf 'full\tmoved, deleted, type-changed, or unmerged path changed\n'
	exit 0
}

select_profile() {
	base_ref=$1
	base_commit=$("$git_bin" rev-parse --verify "${base_ref}^{commit}") || fail "VERIFY_BASE $base_ref is not a valid commit"
	merge_base=$("$git_bin" merge-base HEAD "$base_commit") || fail "cannot find a merge base for HEAD and $base_ref"
	mkdir -p tmp
	selector_tmp=$(mktemp -d "tmp/verification-profile-select.XXXXXX") || fail "cannot create selector temporary directory"
	trap finish EXIT
	trap 'exit 129' HUP
	trap 'exit 130' INT
	trap 'exit 143' TERM

	structural_change "$merge_base...HEAD"
	structural_change --cached
	structural_change
	require_git_output "$selector_tmp/branch" diff --name-only -z --diff-filter=ACMRD "$merge_base...HEAD"
	require_git_output "$selector_tmp/cached" diff --name-only -z --diff-filter=ACMRD --cached
	require_git_output "$selector_tmp/worktree" diff --name-only -z --diff-filter=ACMRD
	require_git_output "$selector_tmp/untracked" ls-files --others --exclude-standard -z

	path_count=0
	: >"$selector_tmp/dirs"
	for path_file in "$selector_tmp/branch" "$selector_tmp/cached" "$selector_tmp/worktree" "$selector_tmp/untracked"; do
		while IFS= read -r -d '' path; do
			# Ignore only the selector's own just-created temporary files. All
			# other untracked directories, symlinks, and files still widen.
			if [ "$path" = "$selector_tmp" ] || [ "${path#"$selector_tmp"/}" != "$path" ]; then
				continue
			fi
			path_count=$((path_count + 1))
			[ ! -L "$path" ] || { printf 'full\tsymlink input changed: %s\n' "$path"; exit 0; }
			case "$path" in
				go.mod|go.sum|go.work|go.work.sum|Makefile|.gz-git.yaml|.gz-git.yml|.golangci.yml|.gosec.yaml|.ci/*|.make/*|scripts/*|tools/*)
					printf 'full\tshared verification input changed: %s\n' "$path"; exit 0 ;;
				*.go)
					[ -e "$path" ] || { printf 'full\tdeleted Go source changed: %s\n' "$path"; exit 0; }
					printf '%s\n' "$(dirname "$path")" >>"$selector_tmp/dirs" ;;
				*) printf 'full\tnon-Go or unknown input changed: %s\n' "$path"; exit 0 ;;
			esac
		done <"$path_file"
	done
	[ "$path_count" -gt 0 ] || { printf 'full\tno changed paths; refusing a zero-test pass\n'; exit 0; }

	LC_ALL=C sort -u "$selector_tmp/dirs" >"$selector_tmp/sorted-dirs" || fail "cannot sort affected directories"
	: >"$selector_tmp/selected"
	while IFS= read -r dir; do
		[ -n "$dir" ] || continue
		package=$(GOWORK=off "$go_bin" list -f '{{.ImportPath}}' "./$dir") || { printf 'full\tcannot resolve changed Go package: %s\n' "$dir"; exit 0; }
		printf '%s\n' "$package" >>"$selector_tmp/selected"
	done <"$selector_tmp/sorted-dirs"
	GOWORK=off "$go_bin" list -f '{{.ImportPath}}|{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... >"$selector_tmp/metadata" || fail "go list for reverse test dependencies failed"
	awk -F'|' '
FILENAME == ARGV[1] { chosen[$1] = 1; next }
{ pkg = $1; packages[++n] = pkg; deps[pkg] = $2 }
END {
  changed = 1
  while (changed) {
    changed = 0
    for (i = 1; i <= n; i++) {
      pkg = packages[i]; if (chosen[pkg]) continue
      count = split(deps[pkg], imports, " ")
      for (j = 1; j <= count; j++) if (chosen[imports[j]]) { chosen[pkg] = 1; changed = 1; break }
    }
  }
  # Array lookups above also create false entries; emit only the true closure.
  for (pkg in chosen) if (chosen[pkg]) print pkg
}' "$selector_tmp/selected" "$selector_tmp/metadata" | LC_ALL=C sort >"$selector_tmp/reverse" || fail "reverse dependency selection failed"
	[ -s "$selector_tmp/reverse" ] || { printf 'full\tno affected Go test packages; refusing a zero-test pass\n'; exit 0; }
	# Integration fixtures exceed the affected package budget even on the
	# unchanged source. Preserve the suite and use the existing full profile.
	module_path=$(GOWORK=off "$go_bin" list -m -f '{{.Path}}') || fail "go list for current module failed"
	while IFS= read -r package; do
		if [ "$package" = "$module_path/pkg/integrate" ]; then
			printf 'full\taffected closure includes slow package: %s\n' "$package"
			exit 0
		fi
	done <"$selector_tmp/reverse"
	closure_has_test=0
	while IFS= read -r package; do
		test_counts=$(GOWORK=off "$go_bin" list -f '{{len .TestGoFiles}} {{len .XTestGoFiles}}' "$package") || fail "go list for selected package test files failed: $package"
		case "$test_counts" in
			'0 0') ;;
			*) closure_has_test=1; break ;;
		esac
	done <"$selector_tmp/reverse"
	[ "$closure_has_test" -eq 1 ] || { printf 'full\taffected closure has no Go test files; refusing a no-test pass\n'; exit 0; }
	printf 'affected'
	while IFS= read -r package; do printf '\t%s' "$package"; done <"$selector_tmp/reverse"
	printf '\n'
}

action=${1:-}
[ "$action" = select ] || [ "$action" = run ] || usage
[ "${2:-}" = --base ] && [ -n "${3:-}" ] || usage
base_ref=$3
shift 3
if [ "$action" = select ]; then [ "$#" -eq 0 ] || usage; select_profile "$base_ref"; exit 0; fi
[ "${1:-}" = -- ] || usage
shift
[ "$#" -gt 0 ] || usage
selection=$(select_profile "$base_ref")
tab=$(printf '\t')
mode=${selection%%"$tab"*}
case "$mode" in
	affected) packages=${selection#*"$tab"}; VERIFY_SELECTED_PACKAGES="$packages" exec "$@" ;;
	full) printf '%s\n' "$selection" >&2; exit 3 ;;
	*) fail "invalid selector result: $selection" ;;
esac
