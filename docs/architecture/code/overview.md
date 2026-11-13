# Code overview

```
cmd/ibkr/main.go          builds cli.App, exits with its code
internal/cli/             cobra commands, gates, rendering
  app.go                  App, exit codes, tool cache access, call() with safety gates, emit()
  render.go               View/Result: trimmed, tolerant text and JSON output
  curated.go, orders.go   curated commands (thin: flags -> key=value -> schema-typed args)
  cmd_tools.go            tools, describe, call
  cmd_connect.go          the 8-step connect wizard
  cmd_account.go          status, logout, version
internal/mcp/             client.go (JSON-RPC over streamable HTTP, JSON + SSE), safety.go (classification),
                          args.go (schema-typed coercion), cache.go (tool cache), snapshot.go (diff)
internal/auth/            discovery.go, flow.go (DCR, PKCE, listener, exchange, refresh, revoke), manager.go (token source)
internal/secrets/         keyring + 0600 file + env override
internal/config/          paths, server URL resolution
internal/version/         ldflags metadata + provider compatibility
internal/testfake/        httptest fake of the MCP server and the OAuth server
```

## Request path (a curated command)

1. `exec()` resolves the tool from the cache (live `tools/list` when stale or unknown).
2. Flags become `key=value`, using parameter names looked up in the tool's schema; `BuildArgs` types and validates them (enum normalization, required check).
3. `callTool()` classifies the tool, applies the `--yes` gate, previews orders when interactive, calls via `mcp.Client` (token refreshed proactively; once on 401), maps transport errors on orders to exit 5.
4. `emit()` renders through a `View` (preferred columns, hidden `*_ex`, generic fallback).

## Testing

`internal/testfake` serves `/mcp` (401 then OK, initialize, paginated `tools/list`, `tools/call` as JSON or SSE, session header) plus OAuth metadata, `/register`, `/authorize` (redirects to the redirect URI), `/token`, `/revoke`. The connect wizard test drives the full flow with a fake "browser".
