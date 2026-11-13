# Install

## Install the CLI

### With Go

Requires Go 1.24 or newer:

```bash
go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@latest
```

The binary lands in `$(go env GOPATH)/bin`, which must be on your `PATH`. To pin a specific version, replace `@latest` with a tag such as `@v0.1.0`.

### From GitHub Releases

Download the archive for your OS and architecture (`linux`, `darwin`, `windows`; `amd64`, `arm64`) from the [GitHub Releases](https://github.com/NunoMotaRicardo/ibkr-cli/releases) page, check it against `checksums.txt`, unpack it and put `ibkr` on your `PATH`. Archives also contain the skill (`skills/ibkr`).

### Verify

```bash
ibkr version
ibkr --help
```

Next: [Connect](Connect.md). See [Releases](Releases.md) for upgrades and specific versions.

## Install the Agent Skill

The skill in `skills/ibkr/` teaches AI agents to use `ibkr` instead of loading 49+ MCP tool schemas into context. Installing the plugin ships the skill; install the `ibkr` binary separately (above) so the agent can run it on `PATH`.

- **Claude Code**:
  ```bash
  claude plugin marketplace add NunoMotaRicardo/nmr-plugins
  claude plugin install ibkr-cli@nmr-plugins
  ```
- **Codex**:
  ```bash
  codex plugin marketplace add NunoMotaRicardo/nmr-plugins
  codex plugin add ibkr-cli@nmr-plugins
  ```
- **GitHub Copilot CLI**:
  ```bash
  copilot plugin marketplace add NunoMotaRicardo/nmr-plugins
  copilot plugin install ibkr-cli@nmr-plugins
  ```
- **Cursor**: add `NunoMotaRicardo/nmr-plugins` as a plugin marketplace and install `ibkr-cli`.
- **Gemini CLI**:
  ```bash
  gemini extensions install https://github.com/NunoMotaRicardo/ibkr-cli
  ```
- **Antigravity / other Agent Skills tools**: install from `https://github.com/NunoMotaRicardo/ibkr-cli`, or copy `skills/ibkr/` into your agent's skills directory.

Next: [Using with agents](Using-with-agents.md).
