# ADR 0010: Credential profiles

Status: accepted (2026-11-21)

## Context
One IBKR authorization covers one account, and `ibkr` stored a single credential slot, so a user with a live and a paper login (two IBKR users) could not use both. A test showed that a second `ibkr connect` under the same OAuth client name `ibkr-cli` revoked the first connection: IBKR replaces the consent when the same app connects again (verified for the same client name; see the update under Consequences for same-login accounts).

## Decision
- A profile is one stored login. `default` is the pre-existing login with unchanged storage; named profiles get their own keyring user (`profile/<name>`), fallback file and tool cache.
- Each profile registers with IBKR as `ibkr-cli (<name>)`, so consents are distinct in Manage Third-Party Consents (this separates logins, not accounts of one login).
- Selection by `--profile` or `IBKR_PROFILE` only; no stored "active profile". Once more than one profile exists, every credential-using command must name one (exit 2 otherwise), so an agent cannot hit the wrong account.
- A small registry file (`profiles.json`) lists profiles because keyrings cannot be enumerated.

## Consequences
- Existing users see no change until they add a second profile.
- Update 2026-11-22: observed in use, a distinct client name does **not** prevent replacement when two profiles use the same IBKR login (two accounts under one login). The older profile then fails with `invalid_grant`. IBKR appears to keep one consent per login; separate logins (live + paper) coexist. Supported: one profile per IBKR login. Not supported: several accounts of one login as simultaneous profiles; reconnect and pick the other account instead.
- Not supported: one command fanning out over several profiles.
