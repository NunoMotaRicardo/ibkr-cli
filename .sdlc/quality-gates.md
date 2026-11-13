# Quality gates — ibkr-cli

## Gates

- Commands: see `quality_gates.*` in `sdlc-config.yml` (test, lint, format_check, type_check).
- CI: `.github/workflows/ci.yml` runs the same commands on ubuntu, macos and windows, plus `scripts/check-versions.sh` (all manifests carry one version). `release.yml` re-runs them on a `v*` tag before goreleaser.
- Security scanning (SAST/dependency audit): none configured yet; tests never use the network and no secret is ever committed. Add `govulncheck` to CI when desired.

## Merge requirements

- CI green on all three OSes.
- Every user-visible change adds a `CHANGELOG.md` entry under Unreleased; versions are only changed by `scripts/release.sh`.
- Behaviour changes update `specs/` and `wiki/` in the same PR.
