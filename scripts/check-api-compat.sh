#!/usr/bin/env bash
# Compare the public API with the latest v* release tag using gorelease.
#
# Usage: scripts/check-api-compat.sh [<pr-base-ref>]
#
# Without an argument, fails if HEAD has any incompatible change since the
# latest release tag.
#
# With <pr-base-ref> (for example origin/main), fails only on incompatible
# changes that HEAD adds on top of that ref. Breaks that are already on the
# base branch are listed as warnings: they were reviewed when they merged,
# and the release workflow decides whether they need a major version.
#
# Exits 0 with a notice when there is no release tag yet, 1 on a break and
# 2 when gorelease fails.
#
# Needs gorelease on PATH:
#   go install golang.org/x/exp/cmd/gorelease@<version>
set -euo pipefail

cd "$(dirname "$0")/.."
pr_base=${1:-}

tag=$(git tag --list 'v*' --sort=-version:refname | head -n 1)
if [ -z "$tag" ]; then
  echo "::notice::No v* release tag yet, so there is no public API to compare with."
  exit 0
fi
echo "Comparing the public API with $tag"

work=$(mktemp -d)
cleanup() {
  if [ -d "$work/base" ]; then git worktree remove --force "$work/base" >/dev/null 2>&1 || true; fi
  rm -rf "$work"
}
trap cleanup EXIT

# incompatible <dir> <output file>: write one "package: change" line per
# incompatible change gorelease reports for the module in <dir>.
incompatible() {
  local dir=$1 out=$2 report
  report=$(cd "$dir" && gorelease -base="$tag" 2>&1) || true
  if ! grep -q '^# summary' <<<"$report"; then
    echo "$report" >&2
    echo "::error::gorelease did not produce a report for $dir" >&2
    exit 2
  fi
  awk '
    /^# summary/ { exit }
    /^# / { pkg = substr($0, 3); section = ""; next }
    /^## / { section = substr($0, 4); next }
    section == "incompatible changes" && NF { print pkg ": " $0 }
  ' <<<"$report" | sort -u >"$out"
  echo "$report"
}

echo "::group::gorelease report for HEAD"
incompatible . "$work/head.txt"
echo "::endgroup::"

if [ -n "$pr_base" ]; then
  git worktree add --detach --quiet "$work/base" "$pr_base"
  echo "::group::gorelease report for $pr_base"
  incompatible "$work/base" "$work/base.txt"
  echo "::endgroup::"
  comm -12 "$work/head.txt" "$work/base.txt" >"$work/inherited.txt"
  comm -23 "$work/head.txt" "$work/base.txt" >"$work/new.txt"
  if [ -s "$work/inherited.txt" ]; then
    echo "Incompatible changes since $tag that are already on $pr_base:"
    sed 's/^/  /' "$work/inherited.txt"
    while IFS= read -r line; do
      echo "::warning::Already on $pr_base, not yet released: $line"
    done <"$work/inherited.txt"
  fi
else
  cp "$work/head.txt" "$work/new.txt"
fi

if [ -s "$work/new.txt" ]; then
  echo "Incompatible public API changes since $tag:"
  sed 's/^/  /' "$work/new.txt"
  exit 1
fi
echo "No new incompatible public API changes since $tag."
