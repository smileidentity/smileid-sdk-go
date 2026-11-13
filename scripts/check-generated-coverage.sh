#!/usr/bin/env bash
# Require 100% coverage of generated/:
#   - statements, from the merged profile written by scripts/coverage.sh
#   - conditions and branches, measured by gobco on each generated package
#
# Go has no native branch coverage. gobco instruments every condition and
# branch and reports which outcomes the package's own tests never reached, so
# generated/ needs tests in its own packages, not only in the root package.
#
# Usage: scripts/check-generated-coverage.sh [coverage.out]
#
# Needs gobco on PATH:
#   go install github.com/rillig/gobco@v1.3.4
set -euo pipefail

cd "$(dirname "$0")/.."
profile=${1:-coverage.out}
failed=0

# Statements. Profile lines look like:
#   github.com/.../generated/models/binary.go:23.42,25.2 1 7
read -r covered total < <(awk '
  NR > 1 && $1 ~ /\/generated\// {
    total += $2
    if ($3 > 0) covered += $2
    else print "not covered: " $1 > "/dev/stderr"
  }
  END { print covered + 0, total + 0 }
' "$profile")
echo "generated/ statement coverage: $covered/$total"
if [ "$total" -eq 0 ] || [ "$covered" -ne "$total" ]; then
  echo "::error::generated/ statement coverage must be 100% ($covered/$total)"
  failed=1
fi

# Conditions and branches, package by package (gobco takes one at a time).
for pkg in $(go list ./generated/...); do
  dir=./$(go list -f '{{.Dir}}' "$pkg" | sed "s|^$PWD/||")
  for mode in condition branch; do
    flags=()
    if [ "$mode" = branch ]; then flags=(-branch); fi
    out=$(gobco ${flags[@]+"${flags[@]}"} "$dir" 2>&1) || { echo "$out"; failed=1; continue; }
    summary=$(echo "$out" | grep -iE '^(condition|branch) coverage: [0-9]+/[0-9]+' || true)
    echo "$dir $summary"
    if [ -z "$summary" ]; then
      echo "$out"
      echo "::error::could not read gobco $mode coverage for $dir"
      failed=1
      continue
    fi
    ratio=${summary##* }
    if [ "${ratio%/*}" != "${ratio#*/}" ]; then
      echo "$out" | grep -E '^generated/' || true
      echo "::error::$dir $mode coverage must be 100% ($ratio)"
      failed=1
    fi
  done
done

exit "$failed"
