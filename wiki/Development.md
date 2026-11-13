# Development

```bash
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
go build ./...
go run ./cmd/ibkr --help
```

Tests use httptest fakes (`internal/testfake`) for the MCP and OAuth servers; they never touch the network and never need a login. Never run `ibkr connect` against the real server from tests or CI.

## Layout

See `AGENTS.md`, `specs/layering-rules.md` and `docs/architecture/code/overview.md`.

## Updating to new provider features

1. `ibkr connect`, then `ibkr whats-new` and `ibkr tools --refresh --diff`.
2. New tools work at once via `ibkr call`; add a curated command in `internal/cli/curated.go` only for frequent, stable tools (map flags to `key=value`, add a view of preferred columns, add a test using the fake).
3. Fix commands whose tools changed; check `--raw` output against the views.
4. `ibkr tools --snapshot specs/provider/tools-snapshot.json`; update `specs/provider/compatibility.md` and `internal/version/version.go`; update the skill, `specs/contracts/cli.md`, `wiki/Commands.md`.
5. Add CHANGELOG entries; release with `scripts/release.sh X.Y.Z`.

Full routine: `docs/plans/provider-sync.md`. Release procedure and bump rules: `docs/plans/release-process.md`.
