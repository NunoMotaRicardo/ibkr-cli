# Connect

`ibkr connect` is a guided login. Run it yourself in a terminal: it opens your browser for the IBKR login and consent page. Agents should ask you to run it (exit code 3 means "not connected").

## Requirements

An Interactive Brokers account with access to the IBKR MCP connector. You log in on IBKR's own page (2FA through the IBKR mobile app may be required). ibkr never sees your IBKR password.

## The eight steps

1. **What will happen**: summary and how to revoke access.
2. **Choose permissions**: `mcp.read mcp.write` by default. `--allow-orders` adds `mcp.orders.submit` (autonomous trading) after a warning; you must type `yes` interactively.
3. **Discover endpoints**: found from the server's OAuth metadata and shown.
4. **Register the client**: a one-time dynamic registration (`ibkr-cli`), reused later.
5. **Log in and approve**: the browser opens; the CLI waits (default 5 minutes, `--wait`).
6. **Exchange the code and store tokens**: in the OS keyring, or a 0600 file if no keyring exists. The location is shown.
7. **Verify**: opens an MCP session, counts tools, reads the flow type and a one-line account summary.
8. **Next steps**.

Every failure prints the cause and what to do.

## Options

| Flag | Use |
|---|---|
| `--allow-orders` | also request live-order access (read [Safety](Safety.md) first) |
| `--no-browser` | print the URL instead of opening a browser |
| `--manual` | headless/SSH: open the printed URL on any machine, then paste the URL your browser was redirected to (it may show a connection error; copy the address anyway) |
| `--wait 10m` | how long to wait for the login |
| `--port N` | fixed loopback port |
| `--status` | same as `ibkr status` |

## After connecting

`ibkr status` shows scopes, token expiry, flow type and where credentials are stored. Tokens refresh automatically; if IBKR revoked access you get exit code 3 and need to run `ibkr connect` again.

## Disconnect

`ibkr logout` revokes the tokens at IBKR and deletes local credentials. You can also revoke in Client Portal > Settings > Manage Third-Party Consents.

## CI and automation

Set `IBKR_ACCESS_TOKEN` to supply a bearer token without running `connect`. It is not refreshed.
