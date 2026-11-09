#!/usr/bin/env bash
# Tests for scripts/check-coverage-exclusions.sh, run in a throwaway git repo.
#
# Usage: scripts/test-check-coverage-exclusions.sh
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/check-coverage-exclusions.sh"
repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

git_q() { git -C "$repo" -c user.name=test -c user.email=test@example.com "$@" >/dev/null; }

git_q init -q -b main
printf 'package a\n\n// nocov in the base commit is not reported\nfunc A() {}\n' >"$repo/a.go"
git_q add -A
git_q commit -q -m base

# Clean change: passes.
git_q checkout -q -b clean
printf 'package a\n\n// nocov in the base commit is not reported\nfunc A() {}\n\nfunc B() {}\n' >"$repo/a.go"
git_q commit -q -am clean
if ! out=$(cd "$repo" && "$script" main 2>&1); then
  fail "clean diff reported hits: $out"
fi

# Every marker on an added line is reported with its file and line.
git_q checkout -q -b dirty
cat >"$repo/b.go" <<'EOF'
package a

func C() {} // nocov
// pragma: no cover
/* c8 ignore next */
/* istanbul ignore next */
/** @codeCoverageIgnore */
#[coverage(off)]
@Generated
@javax.annotation.processing.Generated("x")
EOF
mkdir -p "$repo/generated"
echo '@Generated("generator")' >"$repo/generated/Model.java"
cat >"$repo/pom.xml" <<'EOF'
<excludes>
  <exclude>**/Foo*</exclude>
</excludes>
EOF
echo 'jacocoTestReport { excludes = ["**/Bar*"] }' >"$repo/build.gradle"
git_q add -A
git_q commit -q -m dirty

status=0
out=$(cd "$repo" && "$script" main 2>/dev/null) || status=$?
[ "$status" -eq 1 ] || fail "dirty diff exited $status, want 1"
for want in \
  'b.go:3: func C() {} // nocov' \
  'b.go:4: // pragma: no cover' \
  'b.go:5: /* c8 ignore next */' \
  'b.go:6: /* istanbul ignore next */' \
  'b.go:7: /** @codeCoverageIgnore */' \
  'b.go:8: #[coverage(off)]' \
  'b.go:9: @Generated' \
  'b.go:10: @javax.annotation.processing.Generated("x")' \
  'pom.xml:1: <excludes>' \
  'pom.xml:2:   <exclude>**/Foo*</exclude>' \
  'build.gradle:1: jacocoTestReport { excludes = ["**/Bar*"] }'; do
  grep -qxF "$want" <<<"$out" || fail "missing hit: $want"
done
grep -q 'generated/Model.java' <<<"$out" && fail "@Generated in generated/ was reported"
grep -q 'base commit' <<<"$out" && fail "unchanged line was reported"

# Wrong usage.
(cd "$repo" && "$script" 2>/dev/null) && fail "missing argument succeeded"

if [ "$failures" -ne 0 ]; then
  echo "$failures coverage-exclusion test(s) failed" >&2
  exit 1
fi
echo "coverage-exclusion tests passed"
