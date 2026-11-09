#!/usr/bin/env bash
# Tests for scripts/bump-version.sh. Each case runs in a throwaway git repo
# with a small fake of the files the script edits.
#
# Usage: scripts/test-bump-version.sh
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/bump-version.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

# make_repo <dir> <version> <module> <unreleased entries>
make_repo() {
  local dir=$1 version=$2 module=$3 entries=$4
  mkdir -p "$dir/scripts" "$dir/example" "$dir/generated/operations"
  cp "$script" "$dir/scripts/bump-version.sh"
  cat >"$dir/go.mod" <<EOF
module $module

go 1.21
EOF
  cat >"$dir/transport.go" <<EOF
package usesmileid

import "$module/generated/operations"

const (
	sdkName    = "go"
	sdkVersion = "$version"
)
EOF
  cat >"$dir/generated/operations/operations.go" <<EOF
package operations

import "$module/generated/models"
EOF
  cat >"$dir/example/go.mod" <<EOF
module github.com/smileidentity/smileid-example-go

require $module v$version

replace $module => ../
EOF
  cat >"$dir/README.md" <<EOF
go get $module

The import path ends in \`/v12\` because the module is at major version 12.
EOF
  cat >"$dir/CHANGELOG.md" <<EOF
# Changelog

## [Unreleased]
$entries
## [$version] - 2026-01-01

### Added

- Old entry for $module.
EOF
  git -C "$dir" init -q
  git -C "$dir" add -A
}

run_bump() {
  (cd "$1" && BUMP_DATE=2026-11-09 scripts/bump-version.sh "$2")
}

entries='
### Fixed

- A fix.
'

# --- minor bump ---
repo=$work/minor
make_repo "$repo" 12.2.0 github.com/smileidentity/smileid-sdk-go/v12 "$entries"
out=$(run_bump "$repo" minor) || fail "minor bump exited non-zero"
[ "$out" = "12.3.0" ] || fail "minor bump printed '$out', want 12.3.0"
grep -q 'sdkVersion = "12.3.0"' "$repo/transport.go" || fail "minor: transport.go not updated"
grep -q '^require github.com/smileidentity/smileid-sdk-go/v12 v12.3.0$' "$repo/example/go.mod" || fail "minor: example/go.mod not updated"
expected='# Changelog

## [Unreleased]

## [12.3.0] - 2026-11-09

### Fixed

- A fix.

## [12.2.0] - 2026-01-01

### Added

- Old entry for github.com/smileidentity/smileid-sdk-go/v12.'
[ "$(cat "$repo/CHANGELOG.md")" = "$expected" ] || {
  fail "minor: unexpected CHANGELOG.md"
  diff <(echo "$expected") "$repo/CHANGELOG.md" >&2 || true
}

# Nothing unreleased now, so a second run fails and changes nothing.
before=$(cat "$repo/CHANGELOG.md" "$repo/transport.go")
if run_bump "$repo" minor 2>/dev/null; then
  fail "second bump with an empty Unreleased section succeeded"
fi
[ "$(cat "$repo/CHANGELOG.md" "$repo/transport.go")" = "$before" ] || fail "failed bump changed files"

# --- patch bump ---
repo=$work/patch
make_repo "$repo" 12.2.0 github.com/smileidentity/smileid-sdk-go/v12 "$entries"
out=$(run_bump "$repo" patch) || fail "patch bump exited non-zero"
[ "$out" = "12.2.1" ] || fail "patch bump printed '$out', want 12.2.1"
grep -q '^## \[12.2.1\] - 2026-11-09$' "$repo/CHANGELOG.md" || fail "patch: changelog heading missing"

# --- major bump moves the module path ---
repo=$work/major
make_repo "$repo" 12.2.0 github.com/smileidentity/smileid-sdk-go/v12 "$entries"
out=$(run_bump "$repo" major) || fail "major bump exited non-zero"
[ "$out" = "13.0.0" ] || fail "major bump printed '$out', want 13.0.0"
grep -q '^module github.com/smileidentity/smileid-sdk-go/v13$' "$repo/go.mod" || fail "major: go.mod module path not moved"
grep -q '"github.com/smileidentity/smileid-sdk-go/v13/generated/operations"' "$repo/transport.go" || fail "major: root import not moved"
grep -q '"github.com/smileidentity/smileid-sdk-go/v13/generated/models"' "$repo/generated/operations/operations.go" || fail "major: generated import not moved"
grep -q '^require github.com/smileidentity/smileid-sdk-go/v13 v13.0.0$' "$repo/example/go.mod" || fail "major: example require not moved"
grep -q '^replace github.com/smileidentity/smileid-sdk-go/v13 => ../$' "$repo/example/go.mod" || fail "major: example replace not moved"
# shellcheck disable=SC2016 # literal backticks
grep -q 'ends in `/v13` because the module is at major version 13' "$repo/README.md" || fail "major: README not updated"
grep -q 'Old entry for github.com/smileidentity/smileid-sdk-go/v12\.' "$repo/CHANGELOG.md" || fail "major: changelog history was rewritten"

# --- bad input and bad shapes fail without changes ---
repo=$work/bad
make_repo "$repo" 12.2.0 github.com/smileidentity/smileid-sdk-go/v12 "$entries"
run_bump "$repo" huge 2>/dev/null && fail "unknown part succeeded"
(cd "$repo" && scripts/bump-version.sh 2>/dev/null) && fail "missing argument succeeded"

repo=$work/empty
make_repo "$repo" 12.2.0 github.com/smileidentity/smileid-sdk-go/v12 ""
run_bump "$repo" patch 2>/dev/null && fail "bump with nothing unreleased succeeded"

repo=$work/mismatch
make_repo "$repo" 11.0.0 github.com/smileidentity/smileid-sdk-go/v12 "$entries"
run_bump "$repo" patch 2>/dev/null && fail "bump with module/version mismatch succeeded"
grep -q 'sdkVersion = "11.0.0"' "$repo/transport.go" || fail "mismatch: transport.go changed"

repo=$work/noversion
make_repo "$repo" 12.2.0 github.com/smileidentity/smileid-sdk-go/v12 "$entries"
sed -i.bak 's/sdkVersion = .*/sdkVersion = version()/' "$repo/transport.go"
run_bump "$repo" patch 2>/dev/null && fail "bump without a literal sdkVersion succeeded"
if grep -q '12.2.1' "$repo/CHANGELOG.md"; then
  fail "noversion: changelog changed"
fi

if [ "$failures" -ne 0 ]; then
  echo "$failures bump-version test(s) failed" >&2
  exit 1
fi
echo "bump-version tests passed"
