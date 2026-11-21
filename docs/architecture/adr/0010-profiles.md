# ADR 0010: Credential profiles

Status: accepted (2026-11-21)

## Context
One IBKR authorization covers one account, and `ibkr` stored a single credential slot, so a user with a live and a paper login (two IBKR users) could not use both. A test showed that a second `ibkr connect` under the same OAuth client name `ibkr-cli` revoked the first connection: IBKR keeps one consent per app name.

## Decision
- A profile is one stored login. `default` is the pre-existing login with unchanged storage; named profiles get their own keyring user (`profile/<name>`), fallback file and tool cache.
- Each profile registers with IBKR as `ibkr-cli (<name>)`, so consents are distinct.
- Selection by `--profile` or `IBKR_PROFILE` only; no stored "active profile". Once more than one profile exists, every credential-using command must name one (exit 2 otherwise), so an agent cannot hit the wrong account.
- A small registry file (`profiles.json`) lists profiles because keyrings cannot be enumerated.

## Consequences
- Existing users see no change until they add a second profile.
- A distinct client name per profile is believed, not proven, to avoid replacement for logins that are the same user; two separate logins (live + paper) are the supported case.
- Not supported: one command fanning out over several profiles.
