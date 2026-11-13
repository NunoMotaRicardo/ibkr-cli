# Provider compatibility

Single source of truth for the provider version this CLI was last verified against. `internal/version/version.go` mirrors these values and `ibkr version` prints them.

| Field | Value |
|---|---|
| Provider | IBKR MCP connector (public remote MCP, `https://api.ibkr.com/v1/api/mcp-public`) |
| Connector version last verified | 1.2.1 (released 2026-10-27) |
| Verified on | 2026-11-12 |
| Verified how | Tool list from the connector (49 tools, read-only schemas for 10 of them), OAuth metadata documents, release notes. `ibkr connect` and live tool calls were NOT exercised by the build agent (they need a browser login). |
| Tool snapshot | `specs/provider/tools-snapshot.json` |

Not covered by the snapshot: live-order tools (`submit_order`, `modify_order`, `preview_order`, `get_order_rules`, cancel), which only appear with the `mcp.orders.submit` scope and the AUTONOMOUS flow. They are handled dynamically.

Update this file (and `internal/version/version.go`) whenever the provider-sync routine is run (`docs/plans/provider-sync.md`), even when nothing changed: a fresh date means "still compatible".
