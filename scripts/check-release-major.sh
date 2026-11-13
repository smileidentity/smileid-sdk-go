#!/usr/bin/env bash
# Check that a release tag's major version matches the PRs labelled breaking
# that it contains.
#
# Usage: scripts/check-release-major.sh <tag>
#
# The release contains the commits in <previous stable tag>..<tag>, where the
# previous tag is the newest stable tag that is an ancestor of <tag>. A PR
# labelled breaking counts if its merge commit is in that range.
#   - Breaking PRs and no major bump: fail.
#   - A major bump and no breaking PRs: fail, except 0.x to 1.0.0, which
#     stabilises the API rather than breaking it.
#
# PRs come from `gh pr list` (needs GH_TOKEN and GITHUB_REPOSITORY). Tests set
# BREAKING_PRS_FILE to a file of "<merge commit sha> #<number> <title>" lines.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 <tag>" >&2
  exit 2
fi
tag=$1
scripts_dir=$(cd "$(dirname "$0")" && pwd)

if [[ ! "$tag" =~ ^v([0-9]+)\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::$tag is not a stable vX.Y.Z tag" >&2
  exit 1
fi
tag_major=${BASH_REMATCH[1]}

prev=$("$scripts_dir/previous-release-tag.sh" "$tag^")
if [ -z "$prev" ]; then
  echo "::notice::$tag has no earlier release tag in its history, so there are no breaking PRs to check."
  exit 0
fi
prev_major=$(echo "${prev#v}" | cut -d. -f1)
echo "Previous release: $prev"

if [ -n "${BREAKING_PRS_FILE:-}" ]; then
  candidates=$(cat "$BREAKING_PRS_FILE")
else
  candidates=$(gh pr list --repo "$GITHUB_REPOSITORY" --state merged --label breaking --limit 1000 \
    --json number,title,mergeCommit --jq '.[] | "\(.mergeCommit.oid) #\(.number) \(.title)"')
fi

breaking=""
while read -r oid rest; do
  [ -n "$oid" ] || continue
  if git merge-base --is-ancestor "$oid" "$tag" 2>/dev/null &&
    ! git merge-base --is-ancestor "$oid" "$prev" 2>/dev/null; then
    breaking+="$rest"$'\n'
  fi
done <<<"$candidates"

if [ -n "$breaking" ]; then
  echo "PRs labelled breaking in $prev..$tag:"
  printf '%s' "$breaking"
fi

if [ -n "$breaking" ] && [ "$tag_major" -le "$prev_major" ]; then
  echo "::error::$tag contains PRs labelled breaking, so it must be a new major version (after $prev)."
  exit 1
fi
if [ -z "$breaking" ] && [ "$tag_major" -gt "$prev_major" ]; then
  if [ "$prev_major" -eq 0 ] && [ "$tag_major" -eq 1 ]; then
    echo "$tag stabilises the 0.x API, so it needs no breaking PR."
    exit 0
  fi
  echo "::error::$tag is a major release, but it contains no PR labelled breaking since $prev. Label the PR that needs it, or release a minor or patch version."
  exit 1
fi
echo "$tag matches the breaking PRs since $prev."
