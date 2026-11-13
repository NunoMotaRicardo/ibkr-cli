# Layering rules

```
cmd/ibkr  ->  internal/cli  ->  internal/mcp  ->  internal/auth  ->  internal/secrets
                                                                 ->  internal/config
                                  internal/version (leaf, used by cli)
```

1. A package imports only packages to its right/below. `internal/cli` is the only package that touches cobra, stdout/stderr, prompts, or exit codes.
2. `internal/mcp`, `internal/auth`, `internal/secrets`, `internal/config` never import `internal/cli` and never print.
3. `internal/mcp` knows the MCP protocol and schemas, not IBKR field names. IBKR-specific field preferences live only in `internal/cli` views.
4. `internal/auth` returns errors `ErrNotConnected` / `ErrAuthExpired`; only `internal/cli` maps them to exit code 3.
5. `internal/testfake` is imported only from `_test.go` files.
6. Rendering is tolerant: unknown or missing provider fields must never cause an error or panic.
7. Secrets are only read and written through `internal/secrets`; secret values are never logged or printed.
