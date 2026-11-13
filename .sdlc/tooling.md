# Tooling — ibkr-cli

## Setup (run in every fresh worktree)

```bash
go mod download
```

## Running things

- Run the CLI: `go run ./cmd/ibkr --help`
- Without a login every data command exits 3 (`not connected`); that is the expected sanity check. `ibkr connect` needs a human browser login and must not be run by agents or CI.
- Required tools: Go 1.24+ (`go` on PATH), bash for `scripts/*.sh`, git.

## GitHub

- Board / project: none.
- Auth: `gh auth status`.

## Secrets & environment

- `IBKR_ACCESS_TOKEN`: bearer token override (CI/agents). Never commit values.
- `IBKR_MCP_URL`: override the MCP server URL (tests, staging).
- `IBKR_CONFIG_DIR`, `IBKR_CACHE_DIR`, `IBKR_NO_KEYRING=1`: relocate state or skip the OS keyring (useful in tests).
