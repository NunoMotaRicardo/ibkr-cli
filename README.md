# ibkr-cli

`ibkr` is a Go command-line client for Interactive Brokers' public remote MCP connector, plus an Agent Skill. It gives AI agents (Claude Code, Codex, Copilot CLI, Gemini CLI, Antigravity, Cursor) short shell commands with trimmed output instead of 49+ MCP tool schemas in the context window.

- **Dynamic**: `ibkr tools`, `ibkr describe <tool>`, `ibkr call <tool> key=value` always match the connector's current tools.
- **Curated**: `ibkr positions`, `quote`, `history`, `options chain`, `alerts`, `watchlists`, `screener`, and more.
- **Token-lean output**: compact text by default; `--json`, `--raw`, `--fields`, `--limit` on request.
- **Safe**: writes need `--yes` (exit 4); live orders require an explicit scope opt-in (`--allow-orders`) and `AUTONOMOUS` flow, are previewed interactively, and are never retried after an ambiguous outcome (exit 5).
- **Guided login**: `ibkr connect` (OAuth with PKCE in your browser; credentials stored in the OS keyring or a `0600` file).

## Install the CLI

### With Go

Requires Go 1.24 or newer:

```bash
go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`. To install a specific release, replace `@latest` with `@v0.1.0`.

### From GitHub Releases

Download an archive for your OS and architecture (`linux`, `darwin`, `windows`; `amd64`, `arm64`) from [Releases](https://github.com/NunoMotaRicardo/ibkr-cli/releases), verify it against `checksums.txt`, unpack it, and put `ibkr` on your `PATH`.

### Verify

```bash
ibkr version
ibkr --help
```

## Quick start (CLI)

```bash
ibkr connect                    # guided OAuth login in your browser (run it yourself)
ibkr status                     # connection state, scopes, expiry, flow type
ibkr search AAPL                # find contract_id
ibkr quote 265598               # price snapshot
ibkr history 265598 --period 3m --step 1d
ibkr positions                  # open positions
ibkr tools                      # list everything the connector offers
ibkr describe get_price_snapshot
ibkr call get_price_snapshot contract_id=265598
```

Exit codes: `0` ok, `1` error, `2` usage, `3` not connected or expired (run `ibkr connect`), `4` confirmation required (`--yes`), `5` ambiguous order outcome (never retry; check `ibkr orders` and `ibkr trades`).

## Install and use the Agent Skill

The Agent Skill lives in [`skills/ibkr/`](skills/ibkr/) and is published as the `ibkr-cli` plugin through the [nmr-plugins](https://github.com/NunoMotaRicardo/nmr-plugins) marketplace.

### 1. Install the skill

**Claude Code**

```bash
claude plugin marketplace add NunoMotaRicardo/nmr-plugins
claude plugin install ibkr-cli@nmr-plugins
```

**Codex**

```bash
codex plugin marketplace add NunoMotaRicardo/nmr-plugins
codex plugin add ibkr-cli@nmr-plugins
```

**GitHub Copilot CLI**

```bash
copilot plugin marketplace add NunoMotaRicardo/nmr-plugins
copilot plugin install ibkr-cli@nmr-plugins
```

**Cursor** — add `NunoMotaRicardo/nmr-plugins` as a plugin marketplace and install `ibkr-cli`.

**Gemini CLI**

```bash
gemini extensions install https://github.com/NunoMotaRicardo/ibkr-cli
```

**Antigravity / other Agent Skills tools** — install from `https://github.com/NunoMotaRicardo/ibkr-cli`, or copy `skills/ibkr/` into your agent's skills directory.

### 2. Connect and use with an agent

1. Install the `ibkr` binary (above) so it is on the agent's `PATH`.
2. Run `ibkr connect` yourself in a terminal (it opens a browser for IBKR OAuth login; if an agent hits exit 3, it will ask you to run `ibkr connect`).
3. Ask your agent normally — for example: *"Show my IBKR positions and cash balances"*, *"Look up AAPL option expirations"*, or *"Create an alert for AAPL above 250"*. The skill instructs the agent to use compact commands, confirm before `--yes`, and surface `Review link:` URLs verbatim.

## Docs

- User guide: [wiki](wiki/Home.md) ([GitHub Wiki](https://github.com/NunoMotaRicardo/ibkr-cli/wiki))
- Technical contracts: [specs/](specs/)
- Design, ADRs and plans: [docs/](docs/Home.md)
- Changes: [CHANGELOG.md](CHANGELOG.md)

Not affiliated with Interactive Brokers. This is not investment advice; you are responsible for every order.

[MIT](LICENSE) licensed.
