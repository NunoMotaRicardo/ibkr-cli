# ADR 0008: Staying current with provider changes

Status: accepted (2026-11-12)

## Context

IBKR updates the connector often (v1.1.5 custom connector, v1.2.1 autonomous trading, screeners, earnings insights). The CLI must make catching up cheap and visible.

## Decision

Commit a snapshot of the provider's tool list and input-schema signatures (`specs/provider/tools-snapshot.json`). `ibkr tools --diff` compares it with the live list and prints added/removed/changed tools; `ibkr tools --snapshot` regenerates it. A written routine (`docs/plans/provider-sync.md`): run `whats-new` and `tools --diff`, adjust curated commands, update skill references, record the verified connector version in `specs/provider/compatibility.md`, bump the version and release. The skill tells agents that unknown tools are reachable through `ibkr call`.

## Consequences

- Drift is detected by a command, not by reading release notes.
- The seed snapshot has schemas for 10 read-only tools and names for the rest; running `--snapshot` once after connecting fills it in.
