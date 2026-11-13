# Plan: staying in sync with provider updates

IBKR ships connector releases often. Run this routine on every connector release (watch https://ibkrguides.com/releasenotes/ and `ibkr whats-new`), and at least monthly.

## Routine

1. `ibkr connect` if needed (a human does this), then `ibkr version` to see the recorded compatibility.
2. `ibkr whats-new` and read the release notes (`https://ibkrguides.com/releasenotes/connector-vX.Y.Z.htm`).
3. `ibkr tools --refresh --diff`: lists added (`+`), removed (`-`) and changed (`~`) tools against `specs/provider/tools-snapshot.json`.
4. For each added tool: use it immediately via `ibkr call` / `ibkr describe`; decide whether it earns a curated command (frequent, read-only or clearly gated, stable output). Classification: check `ibkr tools` shows the right `kind`; if a harmless read tool is classed write, add its annotation expectation to `specs/contracts/cli.md` notes or extend the name heuristic in `internal/mcp/safety.go` with a test.
5. For each removed or changed tool: fix the curated command that maps to it (`internal/cli/curated.go`, `orders.go`), the tests in `internal/cli/cli_test.go` and the fake in the same file.
6. Spot-check output shape: `ibkr <cmd> --raw` for each curated command; adjust the `View` column lists in `internal/cli`.
7. `ibkr tools --snapshot specs/provider/tools-snapshot.json`; review the git diff.
8. Update `skills/ibkr/SKILL.md` / `references/commands.md`, `specs/contracts/cli.md`, `wiki/Commands.md`.
9. Update `specs/provider/compatibility.md` and `internal/version/version.go` (provider version and date).
10. Add CHANGELOG entries; bump per `release-process.md` (new tool -> minor, output change -> minor, fix -> patch); `scripts/release.sh X.Y.Z`; push main and the tag.

## Triggers for an out-of-cycle release

- Authorization changes (new scope, endpoint moves): patch or minor, update `specs/contracts/auth.md`.
- A tool the curated commands use is renamed or removed: patch.
- Security-relevant change in order flows: minor or major, update `wiki/Safety.md`.

## Automation ideas (not built)

A scheduled job holding a read-only `IBKR_ACCESS_TOKEN` could run `ibkr tools --diff` weekly and open an issue when it is not empty.
