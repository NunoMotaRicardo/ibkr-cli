# ADR 0005: Scopes and the autonomous-trading opt-in

Status: accepted (2026-11-12)

## Context

Connector v1.2.1 added Autonomous Trading: with `mcp.orders.submit` (and the AUTONOMOUS flow) an agent can place live orders without per-order confirmation. The default flow, REVIEW_AND_APPROVE, only creates reviewable instructions with a link.

## Decision

Default scopes are `mcp.read mcp.write`. `ibkr connect --allow-orders` adds `mcp.orders.submit`, explains the risk and requires the user to type `yes` in an interactive terminal. `ibkr status` shows the granted scopes and the flow type. Live-order tools are never hardcoded as available: `order ...` commands work only if the tools exist and otherwise explain the scope and flow needed.

## Consequences

- An agent cannot enable live trading by itself: it needs a human at a terminal.
- Even with the scope, every order needs `--yes` and is previewed interactively.
