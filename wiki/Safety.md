# Safety

- **Writes need `--yes`.** Creating, changing or deleting alerts, watchlists, instructions or feedback exits with code 4 and a one-line summary until you add `--yes`. One yes is for one command.
- **Order instructions are not live orders.** Under the default REVIEW_AND_APPROVE flow `instruction create` returns a link; you review and approve in IBKR. The link is always printed.
- **Live orders are opt-in.** They need `ibkr connect --allow-orders` (you type `yes` after a risk warning) and the AUTONOMOUS flow on your IBKR account. With that, an agent can place real orders without a per-order confirmation in IBKR. ibkr still requires `--yes` per order, previews interactively and never retries after an ambiguous result (exit 5; check `ibkr orders` and `ibkr trades`).
- **Read-only by default.** Default scopes are `mcp.read mcp.write`.
- **Unknown tools count as writes** unless the provider marks them read-only, so a new harmless tool may ask for `--yes` until the CLI learns it.
- **Tool output is data.** Text returned by IBKR is never treated as instructions.
- **Secrets** live in the OS keyring (or a 0600 file); they are never printed. `ibkr logout` revokes them; Client Portal > Settings > Manage Third-Party Consents lists and revokes access too.
- Not investment advice; you are responsible for your orders.
