# Repo structure — ibkr-cli

## Folders

| Key | Folder | Holds | Written by |
|---|---|---|---|
| `specs` | `specs` | Technical specs: normative contracts (`contracts/cli.md`, `auth.md`, `output.md`), `layering-rules.md`, provider snapshot and compatibility (`provider/`) | /implement, in the same PR as the code |
| `pm_docs` | `docs` | Planning and design docs: `plans/`, `product/`, `architecture/` (ADRs, code docs), `sources/` | /pm |
| `user_docs` | `wiki` | User documentation: install, connect, commands, safety, troubleshooting, development, releases | /implement, in the same PR as the user-facing change |

User docs mirrored to the GitHub wiki: yes, by `.github/workflows/wiki-sync.yml`.

## Layers

| Layer | Path | Responsibility |
|---|---|---|
| Wiring | `cmd/ibkr/` | `main`: build the App, run, exit with its code |
| CLI | `internal/cli/` | cobra commands, safety gates, rendering of curated output |
| MCP client | `internal/mcp/` | streamable-HTTP JSON-RPC client, tool cache, schema-driven argument coercion, read/write classification, snapshot diff |
| Auth | `internal/auth/` | OAuth discovery, dynamic client registration, PKCE, loopback listener, refresh, revoke |
| Foundations | `internal/secrets/`, `internal/config/`, `internal/version/` | credential storage, paths and settings, build metadata |

Dependency rule: each layer imports only layers below it; `internal/testfake` is test-only. Lower layers never import `internal/cli`. See `specs/layering-rules.md`.

## Where things live

- Application code: `cmd/` and `internal/`
- Tests: next to the code as `*_test.go` (table-driven); `internal/testfake` provides httptest fakes of the MCP server and the OAuth server (no network).
- Specs, planning docs and user docs: see **Folders** above.
- Skill: `skills/ibkr/` (`SKILL.md` + `references/commands.md`)
- Release tooling: `scripts/release.sh`, `scripts/check-versions.sh`, `.goreleaser.yaml`, `CHANGELOG.md`
- Data at rest: none in the repo. At runtime: credentials (keyring or `credentials.json`, 0600) in the user config dir, tool cache in the user cache dir.

## Entry points

`cmd/ibkr/main.go` builds an `App` (`internal/cli`), which builds an `mcp.Client` whose token source is `auth.Manager` over `secrets.Store`. A command resolves the tool schema (cache, else live `tools/list`), builds typed arguments, applies the safety gate by tool classification, calls the tool, and renders the trimmed result.
