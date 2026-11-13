# ibkr-cli

`ibkr` is a Go command-line client for Interactive Brokers' public remote MCP connector, plus an Agent Skill. It gives AI agents (Claude Code, Codex, Copilot CLI, Gemini CLI, Antigravity, Cursor) short shell commands with trimmed output instead of 49+ MCP tool schemas in the context window.

- Dynamic: `ibkr tools`, `ibkr describe <tool>`, `ibkr call <tool> key=value` always match the connector's current tools.
- Curated: `ibkr positions`, `quote`, `history`, `options chain`, `alerts`, `watchlists`, `screener`, ...
- Safe: writes need `--yes`; live orders only with an explicit scope opt-in, previewed, never retried after an ambiguous result.
- Guided login: `ibkr connect` (OAuth with PKCE in your browser; credentials in the OS keyring).

## Install

```bash
go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@latest
```

or download an archive from the [releases](https://github.com/NunoMotaRicardo/ibkr-cli/releases) page (linux, macOS, windows; amd64, arm64).

## Quick start

```bash
ibkr connect          # guided login in your browser (run it yourself)
ibkr status
ibkr search AAPL
ibkr quote 265598
ibkr positions
ibkr tools            # everything the connector offers
```

Exit codes: 0 ok, 1 error, 2 usage, 3 not connected (run `ibkr connect`), 4 confirmation required (`--yes`), 5 ambiguous order outcome.

## Use with agents

The skill lives in `skills/ibkr/`. As a plugin it is published through the [nmr-plugins](https://github.com/NunoMotaRicardo/nmr-plugins) marketplace. The agent needs the `ibkr` binary on its PATH and a one-time human `ibkr connect`.

## Docs

User docs: [wiki](wiki/Home.md). Specs: [specs/](specs/). Design: [docs/](docs/Home.md). Changes: [CHANGELOG](CHANGELOG.md).

Not affiliated with Interactive Brokers. This is not investment advice; you are responsible for every order.

MIT licensed.
