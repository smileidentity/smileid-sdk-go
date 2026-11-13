#!/usr/bin/env bash
# Bump the SDK version and cut a changelog release.
#
# Usage: scripts/bump-version.sh <major|minor|patch>
#
# - Sets sdkVersion in transport.go (release.yml checks it against the tag).
# - Moves the entries under "## [Unreleased]" in CHANGELOG.md to a new
#   "## [x.y.z] - YYYY-MM-DD" heading and leaves an empty Unreleased section.
# - Updates the example module's require line.
# - On a major bump, moves the module path to the new /vN suffix in go.mod and
#   every import, as Go modules require from v2 onwards.
#
# Prints the new version. Fails without changing anything if there is nothing
# unreleased or a file is not in the expected shape.
#
# Set BUMP_DATE=YYYY-MM-DD to override today's date (UTC).
set -euo pipefail

die() {
  echo "bump-version: $*" >&2
  exit 1
}

[ $# -eq 1 ] || die "usage: $0 <major|minor|patch>"
part=$1
case "$part" in
  major | minor | patch) ;;
  *) die "unknown part '$part' (expected major, minor or patch)" ;;
esac

cd "$(dirname "$0")/.."

version_file=transport.go
changelog=CHANGELOG.md
example_mod=example/go.mod
date=${BUMP_DATE:-$(date -u +%Y-%m-%d)}
[[ "$date" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "BUMP_DATE '$date' is not YYYY-MM-DD"

# --- Read and check the current state before changing anything. ---

[ "$(grep -cE '^[[:space:]]*sdkVersion = "[0-9]+\.[0-9]+\.[0-9]+"$' "$version_file")" -eq 1 ] ||
  die "$version_file must contain exactly one line: sdkVersion = \"x.y.z\""
current=$(sed -nE 's/^[[:space:]]*sdkVersion = "([0-9]+\.[0-9]+\.[0-9]+)"$/\1/p' "$version_file")
IFS=. read -r major minor patch <<<"$current"

module=$(awk '$1 == "module" { print $2; exit }' go.mod)
[ -n "$module" ] || die "go.mod has no module directive"
module_major=$(echo "$module" | sed -nE 's|.*/v([0-9]+)$|\1|p')
if [ -n "$module_major" ]; then
  [ "$module_major" = "$major" ] || die "go.mod module path $module does not match sdkVersion $current"
elif [ "$major" -ge 2 ]; then
  die "sdkVersion $current needs a /v$major suffix on the module path $module"
fi

[ "$(grep -c '^## \[Unreleased\]$' "$changelog")" -eq 1 ] ||
  die "$changelog must contain exactly one '## [Unreleased]' heading"
# The entries between "## [Unreleased]" and the next "## " heading.
unreleased=$(awk '
  /^## \[Unreleased\]$/ { inside = 1; next }
  inside && /^## / { exit }
  inside { print }
' "$changelog")
[ -n "$(echo "$unreleased" | tr -d '[:space:]')" ] ||
  die "$changelog has nothing under '## [Unreleased]' to release"

case "$part" in
  major) next="$((major + 1)).0.0" ;;
  minor) next="$major.$((minor + 1)).0" ;;
  patch) next="$major.$minor.$((patch + 1))" ;;
esac
grep -qE "^## \[${next//./\\.}\]" "$changelog" && die "$changelog already has a $next section"

grep -qE "^require ${module//./\\.} v$current$" "$example_mod" ||
  die "$example_mod must require $module v$current"

# --- Write the changes. ---

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

sed -E "s/^([[:space:]]*sdkVersion = )\"$current\"$/\1\"$next\"/" "$version_file" >"$tmp"
cat "$tmp" >"$version_file"

awk -v heading="## [$next] - $date" '
  /^## \[Unreleased\]$/ { print; print ""; print heading; next }
  { print }
' "$changelog" >"$tmp"
cat "$tmp" >"$changelog"

new_module=$module
if [ "$part" = major ]; then
  next_major=$((major + 1))
  if [ -n "$module_major" ]; then
    new_module="${module%/v*}/v$next_major"
  elif [ "$next_major" -ge 2 ]; then
    new_module="$module/v$next_major"
  fi
fi

sed -E "s|^require ${module//./\\.} v$current$|require $new_module v$next|" "$example_mod" >"$tmp"
cat "$tmp" >"$example_mod"

if [ "$new_module" != "$module" ]; then
  # Rewrite the import path everywhere except the changelog, whose history
  # must keep the old path.
  git ls-files -z -- '*.go' '*.mod' '*.md' ':!CHANGELOG.md' |
    while IFS= read -r -d '' f; do
      grep -q "$module" "$f" || continue
      sed -E "s#${module//./\\.}([^0-9]|$)#$new_module\1#g" "$f" >"$tmp"
      cat "$tmp" >"$f"
    done
  if [ -n "$module_major" ] && [ -f README.md ]; then
    sed -E "s#\`/v$major\`#\`/v$next_major\`#g; s#major version $major([^0-9]|$)#major version $next_major\1#g" README.md >"$tmp"
    cat "$tmp" >README.md
  fi
fi

echo "$next"
