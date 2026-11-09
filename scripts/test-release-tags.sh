#!/usr/bin/env bash
# Tests for scripts/previous-release-tag.sh and scripts/check-release-major.sh,
# run in a throwaway git repo.
#
# Usage: scripts/test-release-tags.sh
set -euo pipefail

scripts_dir="$(cd "$(dirname "$0")" && pwd)"
repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

g() { git -C "$repo" -c user.name=test -c user.email=test@example.com "$@"; }
commit() {
  g commit -q --allow-empty -m "$1"
  g rev-parse HEAD
}
prev_tag() { (cd "$repo" && "$scripts_dir/previous-release-tag.sh" "$1"); }
expect_prev() {
  local got
  got=$(prev_tag "$1")
  [ "$got" = "$2" ] || fail "previous-release-tag $1 = '$got', want '$2'"
}

# History on main:
#   c0 v0.9.0 - c1 (breaking #1) v1.0.0 - c2 v11.0.0 - c3 (breaking #3) v12.0.0
#   - c4 v12.1.0-rc.1 and v12.1.0 - c5 (breaking #5) v13.0.0-rc.1 - c6 (no tag)
# Branch release-11 from c2: c7 (fix #7) v11.4.1
g init -q -b main
c0=$(commit c0) && g tag v0.9.0
c1=$(commit "breaking 1") && g tag v1.0.0
c2=$(commit c2) && g tag v11.0.0
c3=$(commit "breaking 3") && g tag -a v12.0.0 -m v12.0.0
c4=$(commit c4) && g tag v12.1.0-rc.1 && g tag v12.1.0
c5=$(commit "breaking 5") && g tag v13.0.0-rc.1
c6=$(commit c6)
g checkout -q -b release-11 "$c2"
c7=$(commit "fix 7") && g tag v11.4.1
g checkout -q main
: "$c0" "$c6" "$c7"

expect_prev v0.9.0 v0.9.0
expect_prev 'v0.9.0^' ""
expect_prev 'v1.0.0^' v0.9.0
expect_prev 'v12.1.0^' v12.0.0
expect_prev main v12.1.0 # skips v13.0.0-rc.1
expect_prev "$c4" v12.1.0 # the rc on the same commit does not hide v12.1.0
expect_prev 'v11.4.1^' v11.0.0
expect_prev release-11 v11.4.1

# Breaking PRs, as gh would list them: one per labelled merge commit, plus
# one whose commit is not in this clone.
prs="$repo/.git/prs.txt"
cat >"$prs" <<EOF
$c1 #1 Breaking change one
$c3 #3 Breaking change three
$c5 #5 Breaking change five
0000000000000000000000000000000000000000 #9 Merged somewhere else
EOF

check() {
  (cd "$repo" && BREAKING_PRS_FILE="$prs" "$scripts_dir/check-release-major.sh" "$1" >/dev/null 2>&1)
}
expect_pass() { check "$1" || fail "check-release-major $1 failed, want pass"; }
expect_fail() { check "$1" && fail "check-release-major $1 passed, want fail"; return 0; }

expect_pass v0.9.0    # no earlier release
expect_pass v1.0.0    # 0.x to 1.0.0 with breaking #1
expect_fail v11.0.0   # major bump, no breaking PR in v1.0.0..v11.0.0
expect_pass v12.0.0   # major bump with breaking #3
expect_pass v12.1.0   # minor, #3 is already in v12.0.0
expect_pass v11.4.1   # backport only sees its own commit, not #3 or #5

# A minor release that contains breaking #5 must fail.
g tag v12.2.0 "$c5"
expect_fail v12.2.0
g tag v13.0.0 "$c5"
expect_pass v13.0.0

# 0.x to 1.0.0 needs no breaking PR.
: >"$repo/.git/none.txt"
(cd "$repo" && BREAKING_PRS_FILE="$repo/.git/none.txt" "$scripts_dir/check-release-major.sh" v1.0.0 >/dev/null 2>&1) ||
  fail "0.x to 1.0.0 without a breaking PR failed"
# But 11.x to 12.0.0 does.
(cd "$repo" && BREAKING_PRS_FILE="$repo/.git/none.txt" "$scripts_dir/check-release-major.sh" v12.0.0 >/dev/null 2>&1) &&
  fail "11.x to 12.0.0 without a breaking PR passed"

expect_fail v12.1.0-rc.1 # not a stable tag

if [ "$failures" -ne 0 ]; then
  echo "$failures release-tag test(s) failed" >&2
  exit 1
fi
echo "release-tag tests passed"
