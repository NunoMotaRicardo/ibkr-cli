# Using with agents

## Setup

1. Install `ibkr` ([Install](Install.md)) so it is on the agent's `PATH`.
2. Run `ibkr connect` yourself in a terminal.
3. Install the skill:
   - **Claude Code**: `claude plugin marketplace add NunoMotaRicardo/nmr-plugins` then `claude plugin install ibkr-cli@nmr-plugins`
   - **Codex**: `codex plugin marketplace add NunoMotaRicardo/nmr-plugins` then `codex plugin add ibkr-cli@nmr-plugins`
   - **GitHub Copilot CLI**: `copilot plugin marketplace add NunoMotaRicardo/nmr-plugins` then `copilot plugin install ibkr-cli@nmr-plugins`
   - **Cursor**: add `NunoMotaRicardo/nmr-plugins` as a marketplace and install `ibkr-cli`
   - **Gemini CLI**: `gemini extensions install https://github.com/NunoMotaRicardo/ibkr-cli`
   - **Antigravity / manual**: install from `https://github.com/NunoMotaRicardo/ibkr-cli` or copy `skills/ibkr/` into your agent's skills folder.

## How to use it

Ask your agent in natural language, for example:
- *"Show my IBKR positions, cash balances, and open orders."*
- *"Get AAPL option expirations and calls near the current price."*
- *"Create an order instruction to buy 5 shares of AAPL at $180 limit."*

## What the agent does

The skill tells the agent to prefer curated commands (`positions`, `quote`, ...), use `ibkr tools` / `describe` / `call` for anything else, show `Review link:` URLs verbatim, and stop for your confirmation before any state change (`--yes`). When a command exits 3 the agent asks you to run `ibkr connect` (it opens a browser, so the agent cannot do it).

## Tokens

The skill is short and output is trimmed. Use `--json` only when the agent must parse values. If output looks outdated, `ibkr version --check` shows whether a newer release exists.

## Tool-specific notes

The skill uses only the shell and is tool-neutral. Permission prompts and sandboxing differ per agent: allow `ibkr` and its network access to `api.ibkr.com`.
