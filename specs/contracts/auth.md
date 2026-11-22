# Contract: authentication, storage, refresh

## Provider facts (verified 2026-11-12)

- MCP server: `https://api.ibkr.com/v1/api/mcp-public` (streamable HTTP). Unauthenticated POST -> `401` with `WWW-Authenticate: Bearer resource_metadata="<url>"`.
- Protected resource metadata lists `authorization_servers: ["https://api.ibkr.com"]` and scopes `mcp.read`, `mcp.write`, `mcp.orders.submit`.
- Authorization server metadata (`/.well-known/oauth-authorization-server`): authorize `/oauth2/authorize`, token `/oauth2/api/v1/token`, revoke `/oauth2/api/v1/token/revoke`, register `/oauth2/register` (RFC 7591), PKCE `S256`, grant types `authorization_code`, `refresh_token`, `iss` response parameter supported.
- Not verified against the real server (needs a login): that dynamic registration accepts loopback `http://127.0.0.1:<port>/callback` redirect URIs.

## Discovery

1. POST to the MCP URL; on 401 read `resource_metadata` from `WWW-Authenticate`.
2. Fetch protected resource metadata (header URL, then `<mcp>/.well-known/oauth-protected-resource`, then the RFC 9728 path-inserted form). Take the first `authorization_servers` entry as issuer.
3. Fetch authorization server metadata (`<issuer>/.well-known/oauth-authorization-server`, path-inserted variant, then `openid-configuration`).
4. If discovery fails for the default IBKR server, built-in default endpoints (the values above) are used and a note is shown. For any other server, discovery failure is an error.

## Registration (RFC 7591)

Body: `client_name: "ibkr-cli"` (`"ibkr-cli (<profile>)"` for a named profile, so consents of different logins are distinct in Manage Third-Party Consents; accounts of the same login still replace each other), `redirect_uris: ["http://127.0.0.1:<port>/callback"]`, `token_endpoint_auth_method: "none"`, `grant_types: ["authorization_code","refresh_token"]`, `response_types: ["code"]`, `scope`. A stored registration is reused only if server, registration endpoint and redirect URI (same port) still match; the wizard tries to rebind the previous port first. A rejected registration prints status and response body.

## Authorization

Authorization code + PKCE (`S256`, 256-bit verifier) + random `state` + `resource` (RFC 8707, the MCP server URL). Redirect: loopback listener on `127.0.0.1`, random port (or `--port`), with a timeout (`--wait`, default 5 min). `--manual` accepts a pasted redirect URL or bare code; the listener also stays active. The callback is rejected on: `error` parameter, missing code, `state` mismatch, `iss` different from the discovered issuer (RFC 9207), or `iss` missing when the server advertises support. Token exchange sends `code_verifier`, `redirect_uri`, `client_id` and `resource`.

## Scopes

Default `mcp.read mcp.write`. `--allow-orders` adds `mcp.orders.submit` after the user types `yes` in an interactive terminal (non-interactive: refused, exit 1 with explanation).

## Storage

One JSON document (`secrets.Credentials`): server, discovered endpoints, client registration, access token, refresh token, expiry, granted scopes.

1. OS keyring, service `ibkr-cli`, user `default` (named profile: `profile/<name>`).
2. Fallback: `<UserConfigDir>/ibkr-cli/credentials.json`, mode 0600 (directory 0700), written atomically (named profile: `<UserConfigDir>/ibkr-cli/profiles/<name>/credentials.json`).
2a. Profile registry `<UserConfigDir>/ibkr-cli/profiles.json` (`{"profiles":[...]}`, 0600): written by `connect`, pruned by `logout`. Needed because keyrings cannot be enumerated. A pre-profile login counts as profile `default` without a registry entry.
2b. The tool cache lives in `<UserCacheDir>/ibkr-cli/` (named profile: `.../profiles/<name>/`).
3. `IBKR_ACCESS_TOKEN` overrides the stored access token for the process (no refresh, no expiry tracking); a 401 with it exits 3.
4. `IBKR_CONFIG_DIR`, `IBKR_CACHE_DIR`, `IBKR_NO_KEYRING=1` relocate or disable backends.

Secret values are never printed. `status` shows presence, source and expiry only.

## Refresh

- Before each request: if the access token expires within 60 s, refresh (`grant_type=refresh_token`, with `client_id` and `resource`); persist the new tokens (keeping the old refresh token if none is returned).
- On a `401` from the MCP server: refresh once and retry the request once.
- If refresh fails with 400/401/403 (`invalid_grant`, revoked) or no refresh token exists: exit 3 with "run `ibkr connect`". Network errors during refresh are plain errors (exit 1).

## Logout

Per profile: revoke the refresh token and the access token at the revocation endpoint (best effort, warnings on failure), delete local credentials from keyring and file, remind about Client Portal > Settings > Manage Third-Party Consents.
