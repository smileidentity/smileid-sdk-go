#!/usr/bin/env bash
# Run the release guards for a version: the tag is valid semver and matches
# the module's major version, CHANGELOG.md has a dated heading for it,
# sdkVersion in transport.go matches it, and the major version matches the
# PRs labelled breaking since the previous release.
#
# Usage: scripts/check-release.sh <tag> [<rev>]
#
# release.yml runs this on the pushed tag. quality-gates.yml runs it on
# release PRs, before the tag exists, with <rev> set to the PR's HEAD, so a
# bad release fails on the PR rather than after the tag is pushed. Run it from
# the repository root. The breaking-PR check calls gh, so it needs gh auth
# or GH_TOKEN (see check-release-major.sh).
set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
  echo "usage: $0 <tag> [<rev>]" >&2
  exit 2
fi
tag=$1
rev=${2:-$tag}
scripts_dir=$(cd "$(dirname "$0")" && pwd)

if [[ ! "$tag" =~ ^v([0-9]+)\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Tag '$tag' is not a valid semver tag (expected vX.Y.Z)"
  exit 1
fi
tag_major=${BASH_REMATCH[1]}
version=${tag#v}

module=$(awk '$1=="module"{print $2; exit}' go.mod)
if [ -z "$module" ]; then
  echo "::error::Could not find a 'module' directive in go.mod"
  exit 1
fi
# A module path only carries a /vN suffix for major version 2+; v0 and v1 have none.
module_suffix=$(echo "$module" | grep -oE '/v[0-9]+$' | grep -oE '[0-9]+' || true)
if [ -n "$module_suffix" ]; then
  if [ "$tag_major" != "$module_suffix" ]; then
    echo "::error::Tag major version v$tag_major does not match module path '$module' (expected v$module_suffix.x.y). A mismatched tag would break consumers importing that major version."
    exit 1
  fi
elif [ "$tag_major" != "0" ] && [ "$tag_major" != "1" ]; then
  echo "::error::Tag major version v$tag_major requires module path '$module' to end with /v$tag_major. A mismatched tag would break consumers importing that major version."
  exit 1
fi

if ! grep -qE "^## \[${version//./\\.}\] - [0-9]{4}-[0-9]{2}-[0-9]{2}" CHANGELOG.md; then
  echo "::error::CHANGELOG.md has no dated '## [$version] - YYYY-MM-DD' heading for tag '$tag'"
  exit 1
fi

sdk_version=$(sed -nE 's/^[[:space:]]*sdkVersion = "([^"]+)"$/\1/p' transport.go)
if [ "$sdk_version" != "$version" ]; then
  echo "::error::sdkVersion in transport.go is '$sdk_version' but the tag is '$tag'. Run scripts/bump-version.sh before tagging."
  exit 1
fi

"$scripts_dir/check-release-major.sh" "$tag" "$rev"
