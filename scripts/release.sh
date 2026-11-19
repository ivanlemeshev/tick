#!/usr/bin/env bash
# Tag and push the next release: mise run release [patch|minor|major]
#
# The version lives only in git tags of the form vMAJOR.MINOR.PATCH. The next
# version bumps the highest such tag (v0.0.0 if there is none). Releases are
# made from main, from a clean tree, of the commit origin/main already has.
# Running it again on a released commit pushes that tag and changes nothing else.
# It always asks before tagging or pushing; anything but y or yes cancels,
# and so does having no input to answer from.
set -euo pipefail

level=${1:-patch}
case $level in
  patch | minor | major) ;;
  *)
    echo "usage: mise run release [patch|minor|major]" >&2
    exit 2
    ;;
esac

fail() {
  echo "release: $*" >&2
  exit 1
}

# confirm asks a yes/no question and exits unless the answer is yes.
confirm() {
  printf '%s [y/N] ' "$1" >&2
  local answer=""
  read -r answer || true
  case $(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]') in
    y | yes) ;;
    *)
      echo >&2
      echo "release canceled" >&2
      exit 1
      ;;
  esac
}

semver='^v[0-9]+\.[0-9]+\.[0-9]+$'

[ -z "$(git status --porcelain)" ] || fail "working tree has uncommitted changes"
[ "$(git branch --show-current)" = main ] || fail "releases are made from main"
git remote get-url origin >/dev/null 2>&1 || fail "no origin remote to push the release to"
git fetch -q --tags origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] ||
  fail "HEAD is not origin/main; push main first, or pull if it is behind"

if
  current=$(git tag --points-at HEAD | grep -E "$semver" | sort -V | tail -n 1)
  [ -n "$current" ]
then
  echo "already released as $current"
  confirm "Push the existing tag $current to origin?"
  git push -q origin "refs/tags/$current"
  exit 0
fi

latest=$(git tag --list 'v*' | grep -E "$semver" | sort -V | tail -n 1 || true)
latest=${latest:-v0.0.0}
IFS=. read -r major minor patch <<<"${latest#v}"
case $level in
  major) next="v$((major + 1)).0.0" ;;
  minor) next="v$major.$((minor + 1)).0" ;;
  patch) next="v$major.$minor.$((patch + 1))" ;;
esac

confirm "Release $next (previous: $latest) and push it to origin?"
git tag -a "$next" -m "Release $next"
git push -q origin "refs/tags/$next"
echo "released $next (previous: $latest)"
