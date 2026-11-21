# Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Exit 3 "Not connected" | run `ibkr connect` in a terminal |
| Exit 2 "more than one profile exists" | pass `--profile <name>` or set `IBKR_PROFILE` (see [Profiles](Profiles.md)) |
| Exit 3 "authorization expired or revoked" | the refresh token was revoked or expired; run `ibkr connect` again (check Manage Third-Party Consents) |
| `connect` step 4 fails: registration rejected | the wizard prints the HTTP status and body. **Unverified:** IBKR may reject loopback redirect URIs (`http://127.0.0.1:<port>/callback`) for dynamically registered clients. Retry once; if it persists open an issue with the printed text |
| Browser did not open | use the printed URL, or `--no-browser`; on SSH/headless use `--manual` |
| Callback times out | finish 2FA in the IBKR mobile app; use `--wait 10m` |
| "state mismatch" / "issuer mismatch" | use only the newest browser tab; run `connect` again |
| Exit 4 | a write/order command needs `--yes`; confirm with the account owner first |
| Exit 5 | an order call got no answer: do not retry; check `ibkr orders` / `ibkr trades` |
| `order ...` says the tool is not available | needs `connect --allow-orders` and the AUTONOMOUS flow (`ibkr flow`) |
| `instruction create` rejected | only valid when `ibkr flow` shows REVIEW_AND_APPROVE |
| Unknown tool / stale tool list | `ibkr tools --refresh` |
| Columns look different or wrong | field names are provisional until verified against live responses; use `--raw` or `--fields` and report the shape |
| No keyring on Linux/headless | credentials fall back to a 0600 file (shown by `ibkr status`) |
| Futures quote is empty | pass `--exchange` (e.g. CME) |
| `alerts` missing in the app | alerts appear in IBKR Desktop only |
