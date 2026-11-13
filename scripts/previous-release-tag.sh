#!/usr/bin/env bash
# Print the newest stable release tag (vX.Y.Z, no pre-release suffix) that is
# <rev> itself or one of its ancestors. Prints nothing if there is none.
#
# Usage: scripts/previous-release-tag.sh <rev>
#
# For the release before a tag, pass "<tag>^". Walking the history, rather
# than taking the highest tag in the repository, means a backport tag such as
# v11.4.1 only sees the v11 line it was cut from.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 <rev>" >&2
  exit 2
fi
rev=$1

while candidate=$(git describe --tags --abbrev=0 --match 'v[0-9]*' "$rev" 2>/dev/null); do
  # Several tags can point at one commit (for example v1.0.0-rc.1 and v1.0.0),
  # and git describe returns only one of them. Check them all.
  stable=$(git tag --points-at "$candidate^{commit}" --list 'v[0-9]*' --sort=-version:refname |
    awk '/^v[0-9]+\.[0-9]+\.[0-9]+$/ && !found { print; found = 1 }')
  if [ -n "$stable" ]; then
    echo "$stable"
    exit 0
  fi
  rev="$candidate^"
done
