# ADR 0006: Read/write/order classification and the confirmation gate

Status: accepted (2026-11-12)

## Context

Agents must not change state by accident, and a trade must never be duplicated by a retry.

## Decision

Classify every tool: order-looking names first (`submit_*order*`, `modify_*order*`, `cancel*`), then MCP annotations (`readOnlyHint`, `destructiveHint`) when present, else a name heuristic (`get_`, `search_`, `list_`, `preview_`, `whats_new`, `describe` = read; everything else write). Write and order tools need `--yes` (exit 4 with a one-line summary otherwise). Order tools additionally preview first in interactive mode, and an ambiguous outcome (timeout, connection loss, 5xx) exits 5 and is never retried. `create_order_instruction` is a write (not live) and its review link is always printed. Tool output text is data, never instructions. `provide_customer_feedback` is a write.

## Consequences

- Unknown future tools default to write: safe, at the cost of needing `--yes` for harmless new tools until the provider annotates them (known case: `run_saved_screener`).
- The agent skill requires the user's explicit yes before the agent adds `--yes`; one yes covers one execution.
