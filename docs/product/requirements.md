# Requirements

## Goal

Let AI agents (and people) use Interactive Brokers through IBKR's public remote MCP connector with far fewer tokens than the raw MCP tool schemas cost, safely, and without the CLI falling behind the provider.

## Users

- An IBKR account holder who wants an agent to read the account, research instruments and manage alerts, watchlists and order instructions.
- Agents in Claude Code, Codex, Copilot CLI, Gemini CLI, Antigravity or Cursor that load the Agent Skill and run `ibkr` through a shell.
- Maintainers who must keep up with connector releases.

## Why a CLI saves tokens

An MCP client puts every tool definition (names, long descriptions, JSON schemas) in the model's context, in every session: the connector exposes 49 tools, with multi-paragraph descriptions and enums. With the CLI the agent loads a skill of under 100 lines, runs `ibkr <cmd>` and receives a trimmed table instead of the full JSON. Unknown or rare tools cost tokens only when used (`describe` prints a compact parameter list, far shorter than the raw schema).

## Functional requirements

1. Connect to the connector with the standard MCP OAuth flow; guide the user through it (`connect`), show state (`status`), revoke (`logout`).
2. Expose every connector tool dynamically (`tools`, `describe`, `call`) with arguments typed from the live schema.
3. Provide curated commands for: contract search, quotes, history, account, balances, positions, orders, trades, watchlists, alerts, options, futures, screeners, earnings, themes, performance, flow type, order instructions, live orders, what's new.
4. Trim output to useful fields; offer `--json`, `--raw`, `--fields`, `--limit`.
5. Classify tools as read/write/order and gate every state change behind `--yes`; never retry an ambiguous order call; always show instruction deep links.
6. Detect provider drift (`tools --diff`, snapshot) and expose the compatibility version (`version`).
7. Ship releases for Linux, macOS and Windows (amd64/arm64) with one version across binary and plugin manifests.
8. Ship an Agent Skill in the tool-neutral Agent Skills format.

## Non-functional requirements

- **Safety**: no state change without `--yes`; autonomous trading only by an explicit interactive opt-in; secrets never printed or committed.
- **Agent-safe**: never prompt when stdin is not a TTY; stable exit codes (0-5).
- **Resilience**: provider field renames or additions must not crash rendering; unknown tools stay reachable.
- **Portability**: one static binary, OS keyring with a file fallback, environment token for CI.
- **Testability**: all tests use httptest fakes of the MCP and OAuth servers; no network.
- **Small footprint**: three third-party dependencies.

## Out of scope

Running a local MCP server, storing the user's IBKR password, bypassing IBKR's review flows, giving investment advice.

## Known gaps (2026-11-12)

The login flow and live calls were not exercised against the real server by the build agent; field names in curated views are provisional until verified (see wiki Troubleshooting).
