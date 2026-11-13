# ADR 0002: Hand-rolled MCP client over streamable HTTP

Status: accepted (2026-11-12)

## Context

The CLI is an MCP client of IBKR's remote connector. The official SDK (github.com/modelcontextprotocol/go-sdk) is an option. Needs: inject a bearer token with refresh-on-401, tolerate both JSON and text/event-stream responses, track `Mcp-Session-Id` and `MCP-Protocol-Version`, paginate `tools/list`, and tell apart 'no response arrived' from 'server said no' (essential for the never-retry-orders rule).

## Decision

Write a small client (`internal/mcp`, ~350 lines): `initialize` + `notifications/initialized`, `tools/list` with cursors, `tools/call`. It POSTs JSON-RPC with `Accept: application/json, text/event-stream`, parses either response type (SSE: the event whose id matches), re-initializes once on a 404 with a session id, and refreshes the token once on 401. Transport failures surface as a distinct `TransportError`.

The SDK was not adopted because: it pulls a large dependency tree (jsonschema, oauth2, ...) for the three methods we need; it types tool arguments through Go structs, while this CLI needs raw schemas and dynamic arguments; and ambiguity of outcome on timeout must be observable at the request level, which a hand-rolled client makes trivial.

## Consequences

- Less to depend on and audit; behaviour on edge cases is ours to test (httptest fakes cover JSON, SSE, pagination, 401 refresh, timeout).
- We own protocol drift: if the connector moves to a newer transport revision, `internal/mcp` must follow. Revisit the SDK if requirements grow (server-initiated requests, resumable streams).
