#!/usr/bin/env bash
# Verifies that every manifest carries the same version. On a tag build (CI sets
# GITHUB_REF_TYPE=tag, or pass --tag vX.Y.Z) the tag must equal that version and
# CHANGELOG.md must have a "## [X.Y.Z]" section.
set -euo pipefail
cd "$(dirname "$0")/.."

MANIFESTS=(plugin.json .claude-plugin/plugin.json .cursor-plugin/plugin.json gemini-extension.json)

tag=""
if [[ "${1:-}" == "--tag" ]]; then
  tag="${2:?--tag needs a value}"
elif [[ "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
  tag="${GITHUB_REF_NAME:-}"
fi

version_of() {
  sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -n1
}

ref=""
status=0
for m in "${MANIFESTS[@]}"; do
  v="$(version_of "$m")"
  if [[ -z "$v" ]]; then
    echo "ERROR: no version found in $m" >&2
    status=1
    continue
  fi
  echo "$m: $v"
  if [[ -z "$ref" ]]; then ref="$v"; elif [[ "$v" != "$ref" ]]; then
    echo "ERROR: $m has $v but expected $ref" >&2
    status=1
  fi
done
[[ $status -eq 0 ]] || exit 1

if [[ ! "$ref" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "ERROR: '$ref' is not a plain SemVer X.Y.Z" >&2
  exit 1
fi

if [[ -n "$tag" ]]; then
  if [[ "$tag" != "v$ref" ]]; then
    echo "ERROR: tag $tag does not match manifest version v$ref" >&2
    exit 1
  fi
  if ! grep -qE "^## \[$ref\]" CHANGELOG.md; then
    echo "ERROR: CHANGELOG.md has no '## [$ref]' section" >&2
    exit 1
  fi
  echo "tag $tag OK"
fi
echo "versions OK ($ref)"
