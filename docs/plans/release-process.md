# Plan: release process

## Versioning

SemVer, tags `vX.Y.Z`, first release 0.1.0. One version for the binary and all manifests (`plugin.json`, `.claude-plugin/plugin.json`, `.cursor-plugin/plugin.json`, `gemini-extension.json`). Never hand-edit versions: use `scripts/release.sh`.

### Bump rules

| Change | Bump |
|---|---|
| New provider tool supported by a curated command or by classification | minor |
| Curated command output change (columns, format) | minor |
| Bug fix, docs, internal change | patch |
| Breaking CLI change (flag removed/renamed, exit code meaning changed) | major (minor while 0.x; call it out in the CHANGELOG) |

## Procedure

1. Every user-visible change adds a line under `## [Unreleased]` in `CHANGELOG.md` in the same PR.
2. Run the provider-sync routine when the connector changed (`provider-sync.md`).
3. On `main`, clean tree: `scripts/release.sh X.Y.Z`. It checks main/clean/SemVer greater than the last tag, updates the four manifests, moves Unreleased into `## [X.Y.Z] - <today>`, updates the compare links, commits `chore(release): vX.Y.Z` and creates the annotated tag. It does not push.
4. Review (`git show vX.Y.Z`), then `git push origin main && git push origin vX.Y.Z`.
5. CI (`release.yml`) runs `scripts/check-versions.sh` (tag equals manifest version, CHANGELOG section exists), gofmt/vet/test, then goreleaser: archives for linux/darwin/windows x amd64/arm64, `checksums.txt`, release notes from the CHANGELOG section, ldflags for version/commit/date.
6. Verify: `ibkr version --check` from an older install reports the upgrade; the README install line works.

## Tie-in with provider sync

Every provider-sync run ends in a release or an explicit "no change" note: update `specs/provider/compatibility.md` with the new verification date even if nothing else changed (commit as `docs:`; no release needed).

## Pre-seeded first release

The repository ships with `0.1.0` already in the manifests and a dated `## [0.1.0]` CHANGELOG section. `scripts/release.sh 0.1.0` stamps today's date on that section, validates the manifests and creates the tag.
