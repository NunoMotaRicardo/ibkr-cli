# ibkr-cli: Interactive Brokers CLI + Agent Skill

A Go CLI (`ibkr`) that is an MCP client of Interactive Brokers' public remote connector and exposes its tools as token-lean shell commands, plus an Agent Skill (`skills/ibkr/`). Part of the `nmr-plugins` marketplace (https://github.com/NunoMotaRicardo/nmr-plugins).

`CLAUDE.md` is a symlink to this file. Edit `AGENTS.md` only.

## Principles
- Anything that changes state needs `--yes`; live orders are never retried after an ambiguous outcome. Do not weaken the gates in `internal/cli/app.go` or the classification in `internal/mcp/safety.go` without updating `specs/contracts/cli.md` and the tests.
- Never commit tokens, credentials, account ids or personal data. Never run `ibkr connect` in tests or CI (it needs a human browser login); tests use `internal/testfake`.
- Provider fields change: rendering must tolerate unknown or missing fields. Prefer schema-driven behaviour over hardcoded parameter names.
- Skills are Agent Skills folders, tool-neutral; flag tool-specific features inside the skill.

## Layout
```
cmd/ibkr/            main
internal/cli/        cobra commands, safety gates, rendering
internal/mcp/        MCP client, tool cache, arg coercion, classification, snapshot diff
internal/auth/       OAuth discovery, DCR, PKCE, refresh, revoke
internal/secrets/    keyring + 0600 file + env
internal/config/, internal/version/, internal/testfake/
specs/               contracts/{cli,auth,output}.md, layering-rules.md, provider/ (snapshot, compatibility)
docs/                planning/design: requirements, ADRs, code overview, plans (provider-sync, release-process)
wiki/                user docs mirrored to the GitHub wiki
skills/ibkr/         SKILL.md + references/commands.md
scripts/             release.sh, check-versions.sh
.sdlc/               SDLC config for the adlc skills
```
Dependency rule: `cmd -> cli -> mcp -> auth -> secrets/config` (see `specs/layering-rules.md`).

## Commands
- Gates (same as CI): `go test ./...`, `go vet ./...`, `test -z "$(gofmt -l .)"`, `go build ./...`
- Sanity: `go run ./cmd/ibkr --help`; `go run ./cmd/ibkr tools` must exit 3 when not connected.

## Conventions
- Every user-visible change adds a `CHANGELOG.md` entry under Unreleased and bumps the version at release time. Never hand-edit versions: run `scripts/release.sh X.Y.Z` (updates all manifests and the CHANGELOG, tags, does not push). Rules: new provider tool -> minor; curated output change -> minor; fix -> patch; breaking flag/exit-code change -> major (minor while 0.x).
- Provider update routine: `docs/plans/provider-sync.md` (`ibkr whats-new`, `ibkr tools --diff`, adjust, update `specs/provider/compatibility.md`, release).
- Behaviour changes update `specs/` and `wiki/` in the same change. Key decisions are ADRs in `docs/architecture/adr/`.
- Manifests: `plugin.json`, `.claude-plugin/plugin.json`, `.cursor-plugin/plugin.json`, `gemini-extension.json` (name `ibkr-cli`, one shared version; no MCP server registration).
- Skill folder name == frontmatter `name`; keep `SKILL.md` under 100 lines.
- Commits: conventional style, branch `main`.

## Wiki
- `wiki/` is the source of truth; `Home.md` is the landing page. Do not edit the GitHub wiki directly: the `wiki-sync` workflow overwrites it on push to `main`.
- First-time setup: enable Wikis in repo settings and create one page by hand so the wiki repo exists.
