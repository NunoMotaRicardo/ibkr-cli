# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/). While the version is 0.x, a breaking
CLI change (flag or exit-code) bumps the minor version and is called out here.

## [Unreleased]

### Fixed
- Profiles documentation: distinct per-profile client names do **not** stop IBKR from replacing a consent when two profiles use the **same IBKR login**. Observed: after connecting a second account of one login, the first profile failed with `invalid_grant`, while a profile on a separate (paper) login kept working. Profiles are for separate logins; two accounts under one login need one profile that you reconnect to switch. `connect` now warns about this when other profiles exist.

## [0.2.1] - 2026-11-21

### Fixed
- `version` now reports the real version, and `version --check` works, for binaries installed with `go install ...@latest`: the version, commit and date fall back to the build info embedded by the Go toolchain when the release ldflags are absent. Previously such installs reported `dev`.

## [0.2.0] - 2026-11-21

### Added
- Credential profiles for several IBKR logins (for example live and paper): `ibkr --profile <name> connect`, `--profile` / `IBKR_PROFILE` on every command, and `ibkr profiles` to list them. Each profile has its own keyring entry or `0600` file and its own tool cache, and registers with IBKR as `ibkr-cli (<name>)` so consents do not replace each other.
- Once more than one profile exists, every command that touches credentials needs `--profile` or `IBKR_PROFILE` (usage error, exit 2), so a command can never silently act on the wrong account. Existing single-login setups are unchanged: the original login is the `default` profile and keeps its storage location.

## [0.1.0] - 2026-11-13

### Added
- `ibkr` CLI: an MCP client for Interactive Brokers' public remote connector (streamable HTTP).
- Guided `ibkr connect` wizard (OAuth discovery, dynamic client registration, PKCE, loopback or manual callback), `status`, `logout`.
- Dynamic commands: `tools`, `describe`, `call` (arguments typed from the live JSON schema, 24h tool cache).
- Curated commands: `search`, `quote`, `history`, `account`, `balances`, `positions`, `orders`, `trades`, `watchlists`/`watchlist`, `alerts`/`alert`, `options`, `futures`, `screener`, `earnings`, `themes`, `topics`, `perf`, `flow`, `instruction`, `order`, `whats-new`.
- Safety gates: write tools need `--yes` (exit 4); live-order tools are previewed interactively and never retried after an ambiguous outcome (exit 5).
- Secrets in the OS keyring with a 0600 file fallback; `IBKR_ACCESS_TOKEN` override for CI.
- `ibkr tools --diff` / `--snapshot` against `specs/provider/tools-snapshot.json`; `ibkr version --check`.
- Agent Skill `skills/ibkr` and plugin manifests for Claude Code, Cursor, Gemini CLI and Codex-compatible tools.

[Unreleased]: https://github.com/NunoMotaRicardo/ibkr-cli/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/NunoMotaRicardo/ibkr-cli/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/NunoMotaRicardo/ibkr-cli/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/NunoMotaRicardo/ibkr-cli/releases/tag/v0.1.0
