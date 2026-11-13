# Using with agents

## Setup

1. Install `ibkr` ([Install](Install.md)) so it is on the agent's PATH.
2. Run `ibkr connect` yourself in a terminal.
3. Install the skill: as a plugin through the nmr-plugins marketplace, or copy `skills/ibkr/` into your agent's skills folder.

## What the agent does

The skill tells the agent to prefer curated commands (`positions`, `quote`, ...), use `ibkr tools` / `describe` / `call` for anything else, show instruction links to you, and stop for your confirmation before any state change. When a command exits 3 the agent asks you to run `ibkr connect` (it opens a browser, so the agent cannot do it).

## Tokens

The skill is short and output is trimmed. Use `--json` only when the agent must parse values. If output looks outdated, `ibkr version --check` shows whether a newer release exists.

## Tool-specific notes

The skill uses only the shell and is tool-neutral. Permission prompts and sandboxing differ per agent: allow `ibkr` and its network access to `api.ibkr.com`.
