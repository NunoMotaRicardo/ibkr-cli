# ADR 0003: Hybrid command surface (dynamic + curated)

Status: accepted (2026-11-12)

## Context

The provider ships features often. A fully curated CLI is always behind; a fully generic one (call tool with JSON) is token-heavy and error-prone for agents.

## Decision

Two layers on one client. (a) Dynamic, always current: `tools`, `describe`, `call` - the tool list and schemas are cached (24 h) and refreshed on an unknown tool; `call` coerces `key=value` by the JSON schema. (b) Curated short commands for the common jobs (`positions`, `quote`, `history`, ...) that only map flags to tool arguments and render chosen fields. Curated commands look parameter names up in the live schema and accept `--arg`/`--args-json`, so a renamed or new parameter does not break them. Rendering prefers known fields and falls back to generic columns, so unknown fields never crash it.

## Consequences

- New provider tools are usable the day they ship via `ibkr call`; curated commands are added later (provider-sync plan).
- Output field names in curated views are guesses until verified against real responses (see Troubleshooting); `--raw` and `--fields` are the escape hatches.
