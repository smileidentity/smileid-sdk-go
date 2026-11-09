#!/usr/bin/env bash
# Fail if the lines added since <base-ref> exclude code from coverage.
#
# Usage: scripts/check-coverage-exclusions.sh <base-ref>
#
# Scans the lines added in `git diff <base-ref>...HEAD` for coverage exclusion
# markers used across the Smile ID SDKs:
#   nocov, pragma: no cover, c8 ignore, istanbul ignore, @codeCoverageIgnore,
#   #[coverage(off)], a JaCoCo @Generated annotation outside generated code,
#   and new exclude entries in a JaCoCo build config.
# Prints each hit as file:line: text and exits 1 if there are any.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 <base-ref>" >&2
  exit 2
fi
base=$1

# This script and its test name the markers on purpose.
self_excludes=(
  ':(exclude)scripts/check-coverage-exclusions.sh'
  ':(exclude)scripts/test-check-coverage-exclusions.sh'
)

git diff --unified=0 --no-color --no-ext-diff "$base...HEAD" -- . "${self_excludes[@]}" | awk '
  # File headers ("--- a/x" then "+++ b/x") only appear between "diff --git"
  # and the first hunk. Inside a hunk, an added line whose text starts with
  # "++ " also begins with "+++ ", so it must not switch the file.
  /^diff --git / { in_hunk = 0; after_minus = 0; next }
  !in_hunk && /^--- / { after_minus = 1; next }
  !in_hunk && after_minus && /^\+\+\+ / {
    file = substr($0, 5)
    sub(/^b\//, "", file)
    after_minus = 0
    next
  }
  /^@@ / {
    # @@ -a,b +c,d @@  ->  next added line is number c
    in_hunk = 1
    after_minus = 0
    split($3, parts, ",")
    line = substr(parts[1], 2) + 0
    next
  }
  !in_hunk { next }
  /^\+/ {
    text = substr($0, 2)
    hit = 0
    if (text ~ /nocov|pragma:[[:space:]]*no cover|c8[[:space:]]+ignore|istanbul[[:space:]]+ignore|@codeCoverageIgnore|#\[coverage\(off\)\]/) hit = 1
    # JaCoCo skips classes annotated @Generated; only generated code may use it.
    if (text ~ /@([A-Za-z_.]+\.)?Generated([^A-Za-z_]|$)/ && file !~ /(^|\/)generated\//) hit = 1
    # New exclude entries in a Maven or Gradle build config.
    if (file ~ /(^|\/)(pom\.xml|[^\/]*\.gradle(\.kts)?)$/ && text ~ /<excludes?>|excludes?[[:space:]]*(\+?=|\(|\{)/) hit = 1
    if (hit) {
      printf "%s:%d: %s\n", file, line, text
      found = 1
    }
    line++
    next
  }
  END { exit found ? 1 : 0 }
' && status=0 || status=$?

if [ "$status" -eq 1 ]; then
  echo "Coverage exclusions were added since $base. Write tests for the code instead." >&2
fi
exit "$status"
