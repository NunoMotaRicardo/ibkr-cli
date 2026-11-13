# Install

## With Go

```bash
go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@latest
```

Requires Go 1.24 or newer; the binary lands in `$(go env GOPATH)/bin`, which must be on your PATH.

## Release binaries

Download the archive for your OS and architecture from the GitHub releases page, check it against `checksums.txt`, unpack it and put `ibkr` on your PATH. Archives also contain the skill (`skills/ibkr`).

## Verify

```bash
ibkr version
ibkr --help
```

Next: [Connect](Connect.md). See [Releases](Releases.md) for upgrades and specific versions.
