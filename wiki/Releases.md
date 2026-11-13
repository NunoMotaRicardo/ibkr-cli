# Releases

Versions follow SemVer (tags `vX.Y.Z`); one version covers the binary and the plugin manifests. Changes are in the [CHANGELOG](https://github.com/NunoMotaRicardo/ibkr-cli/blob/main/CHANGELOG.md).

## Check what you have

```bash
ibkr version            # version, commit, date, provider compatibility
ibkr version --check    # asks GitHub whether a newer release exists
```

`version` also prints the IBKR connector version this build was last verified against.

## Install or upgrade

```bash
go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@latest     # latest
go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@v0.1.0     # a specific version
```

Or download the archive for that tag from the releases page and verify `checksums.txt`.

## When to upgrade

When IBKR ships a connector update (new tools work immediately through `ibkr call`, but curated commands and the skill are updated per release), when `ibkr version --check` reports a newer release, or when output looks outdated.
