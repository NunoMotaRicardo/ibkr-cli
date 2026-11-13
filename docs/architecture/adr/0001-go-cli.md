# ADR 0001: Implement the tool as a Go CLI

Status: accepted (2026-11-12)

## Context

Agents that call IBKR's MCP connector directly pay for 49+ tool schemas in every context window, and each provider update changes them. The owner wants a command-line tool that agents drive through the shell.

## Decision

Build a single static Go binary, `ibkr`, plus an Agent Skill that teaches agents to use it. Go gives one dependency-free binary per OS/arch (goreleaser), fast startup, a good HTTP/OAuth standard library and easy cross-compilation. Dependencies are kept to cobra (commands), go-keyring (OS keyring) and golang.org/x/term (TTY detection).

## Consequences

- Install is `go install` or a release archive; no runtime to manage.
- The skill is short because the CLI itself carries the detail (`--help`, `describe`).
- The module targets Go 1.24 so it builds with current stable toolchains.
