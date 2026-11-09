#!/usr/bin/env bash
# Run the unit tests with coverage and write:
#   coverage.out  Go cover profile, merged across test packages
#   coverage.xml  the same data in Cobertura XML (for diff-cover)
#
# Usage: scripts/coverage.sh
#
# Needs gocover-cobertura on PATH:
#   go install github.com/boumenot/gocover-cobertura@v1.5.0
set -euo pipefail

cd "$(dirname "$0")/.."

raw=$(mktemp)
trap 'rm -f "$raw"' EXIT

# -coverpkg=./... counts code in every package, whichever package's tests run
# it. Each test binary writes its own copy of every block, so merge them.
go test -covermode=atomic -coverpkg=./... -coverprofile="$raw" ./...

awk '
  NR == 1 { print; next }
  {
    key = $1 " " $2
    if (!(key in hits)) order[++n] = key
    hits[key] += $3
  }
  END { for (i = 1; i <= n; i++) print order[i], hits[order[i]] }
' "$raw" > coverage.out

gocover-cobertura < coverage.out > coverage.xml

go tool cover -func=coverage.out | tail -n 1
