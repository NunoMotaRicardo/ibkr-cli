# ADR 0004: MCP authorization flow with dynamic client registration

Status: accepted (2026-11-12)

## Context

IBKR's connector uses OAuth 2.1 as described by the MCP authorization spec: 401 + protected resource metadata, an authorization server with RFC 7591 registration, PKCE S256, scopes `mcp.read mcp.write mcp.orders.submit`, and the `iss` response parameter.

## Decision

Implement the flow generically (`internal/auth`): discover endpoints at run time (IBKR's current endpoints are only fallback defaults), register a public client (`token_endpoint_auth_method: none`, redirect `http://127.0.0.1:<port>/callback`), authorization code + PKCE S256 + `state` + `resource` (RFC 8707), verify `iss`, loopback listener with timeout, `--manual` paste mode for headless machines. Tokens refresh proactively and on a 401 (once); a failed refresh exits 3. `logout` revokes at the revocation endpoint. The connect command is a numbered, verified wizard with a cause and a fix for every failure.

## Consequences

- No client secret exists, so nothing secret ships in the binary.
- Unverified: whether IBKR accepts loopback redirect URIs for dynamic clients. The wizard prints status and body on rejection and Troubleshooting documents this as unverified.
- Users revoke access in Client Portal > Settings > Manage Third-Party Consents.
