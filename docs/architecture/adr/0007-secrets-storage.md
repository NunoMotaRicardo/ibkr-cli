# ADR 0007: Secret storage

Status: accepted (2026-11-12)

## Context

OAuth tokens and the client registration are credentials. CI and agents need a non-interactive way to supply a token.

## Decision

Store one JSON document in the OS keyring (service `ibkr-cli`); fall back to `credentials.json` (0600, directory 0700) in the user config dir when no keyring is available; `IBKR_ACCESS_TOKEN` overrides both for a process. Secret values are never printed; `status` shows presence and source.

## Consequences

- Works on desktops and headless servers.
- An env token cannot be refreshed; a 401 with it exits 3.
