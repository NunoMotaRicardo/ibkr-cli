# ADR 0009: One SemVer version, scripted releases

Status: accepted (2026-11-12)

## Context

Four plugin manifests and the binary must never disagree, and releases should be a repeatable procedure.

## Decision

SemVer, tags `vX.Y.Z`, starting at 0.1.0. One version across the binary (`-ldflags` into `internal/version`) and all manifests. `scripts/release.sh X.Y.Z` updates manifests and CHANGELOG, commits and tags locally without pushing; `scripts/check-versions.sh` runs in CI and on tag builds; goreleaser publishes archives, checksums and the CHANGELOG section as release notes. `ibkr version` prints build metadata and the provider compatibility line; `version --check` looks for a newer release. Bump rules: new provider tool support -> minor; curated output change -> minor; fix -> patch; breaking CLI flag or exit-code change -> major (minor while 0.x, noted in the CHANGELOG).

## Consequences

- Versions are never hand-edited.
- See `docs/plans/release-process.md`.
