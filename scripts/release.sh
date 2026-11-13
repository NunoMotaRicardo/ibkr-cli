#!/usr/bin/env bash
# Usage: scripts/release.sh X.Y.Z
# Prepares a release locally: updates every manifest version, moves the
# CHANGELOG "Unreleased" entries into a dated X.Y.Z section, commits
# "chore(release): vX.Y.Z" and creates an annotated tag. It does NOT push.
set -euo pipefail
cd "$(dirname "$0")/.."

REPO_URL="https://github.com/NunoMotaRicardo/ibkr-cli"
MANIFESTS=(plugin.json .claude-plugin/plugin.json .cursor-plugin/plugin.json gemini-extension.json)

new="${1:-}"
if [[ ! "$new" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: scripts/release.sh X.Y.Z (plain SemVer, no leading v)" >&2
  exit 2
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" != "main" ]]; then
  echo "ERROR: must be on main (on '$branch')" >&2
  exit 1
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "ERROR: working tree is not clean" >&2
  exit 1
fi
if git rev-parse -q --verify "refs/tags/v$new" >/dev/null; then
  echo "ERROR: tag v$new already exists" >&2
  exit 1
fi

semver_gt() { # $1 > $2 ?
  local IFS=.; local -a a=($1) b=($2)
  for i in 0 1 2; do
    if ((a[i] > b[i])); then return 0; fi
    if ((a[i] < b[i])); then return 1; fi
  done
  return 1
}

prev="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n1)"
if [[ -n "$prev" ]] && ! semver_gt "$new" "$prev"; then
  echo "ERROR: $new is not greater than the last tag v$prev" >&2
  exit 1
fi

today="$(date +%Y-%m-%d)"

# --- CHANGELOG ---------------------------------------------------------------
if ! grep -qE '^## \[Unreleased\]' CHANGELOG.md; then
  echo "ERROR: CHANGELOG.md has no '## [Unreleased]' section" >&2
  exit 1
fi
if grep -qE "^## \[$new\]" CHANGELOG.md; then
  # Pre-seeded section (e.g. the first release): just stamp the date.
  sed -i.bak -E "s/^## \[$new\] - .*/## [$new] - $today/" CHANGELOG.md
else
  body="$(awk '/^## \[Unreleased\]/{f=1;next} /^## \[/{f=0} f' CHANGELOG.md | grep -v '^[[:space:]]*$' || true)"
  if [[ -z "$body" ]]; then
    echo "ERROR: the Unreleased section of CHANGELOG.md is empty; nothing to release" >&2
    exit 1
  fi
  sed -i.bak -E "s/^## \[Unreleased\].*/## [Unreleased]\n\n## [$new] - $today/" CHANGELOG.md
fi
# link references at the bottom
if grep -qE '^\[Unreleased\]:' CHANGELOG.md; then
  sed -i.bak -E "s#^\[Unreleased\]:.*#[Unreleased]: $REPO_URL/compare/v$new...HEAD#" CHANGELOG.md
  if ! grep -qE "^\[$new\]:" CHANGELOG.md; then
    if [[ -n "$prev" ]]; then link="$REPO_URL/compare/v$prev...v$new"; else link="$REPO_URL/releases/tag/v$new"; fi
    sed -i.bak -E "/^\[Unreleased\]:/a\\
[$new]: $link" CHANGELOG.md
  fi
fi
rm -f CHANGELOG.md.bak

# --- manifests -----------------------------------------------------------------
for m in "${MANIFESTS[@]}"; do
  sed -i.bak -E "s/^([[:space:]]*\"version\"[[:space:]]*:[[:space:]]*\")[^\"]*(\".*)$/\1$new\2/" "$m"
  rm -f "$m.bak"
done

scripts/check-versions.sh >/dev/null

git add CHANGELOG.md "${MANIFESTS[@]}"
if git diff --cached --quiet; then
  echo "note: nothing changed on disk; tagging the current commit"
else
  git commit -q -m "chore(release): v$new"
fi
git tag -a "v$new" -m "v$new"

echo "Release v$new prepared (not pushed). Review with: git show v$new"
echo "To publish:  git push origin main && git push origin v$new"
