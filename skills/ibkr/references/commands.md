# ibkr command reference

Global flags: `--json` (compact JSON), `--raw` (full provider response), `--fields a,b`, `--limit N`, `-y/--yes`, `--timeout D`.
Every curated command also takes `--arg key=value` (repeatable) and `--args-json '{...}'` for parameters it does not know.

Exit codes: 0 ok, 1 error, 2 usage, 3 not connected/expired (user runs `ibkr connect`), 4 needs `--yes`, 5 ambiguous order outcome (never retry; check `ibkr orders`).

## Session
- `connect [--allow-orders] [--no-browser] [--manual]` (user only), `status [--offline]`, `logout`, `version [--check]`.

## Discover and call anything
- `tools [--search q] [--refresh] [--full]`: name, kind (read/write/order), summary. `tools --diff`, `tools --snapshot FILE` are maintainer commands.
- `describe <tool>`: parameters (name, type, required, enum).
- `call <tool> key=value ... [--args-json '{}']`: values typed from the schema (ints, numbers, booleans, arrays as `a,b,c`; `contract_id_ex` stays a string; enums match case-insensitively). Write tools need `--yes`.

## Read commands
- `search <query>`: contracts. Use the row whose `symbol` equals the ticker; for options use its `underlying_contract_id`.
- `quote <contract_id>... [--exchange X] [--data last,bid_ask,change,prior_close,volume,option_greeks,...]`
- `history <contract_id> [--period 1d|2d|3d|1w|2w|1m|3m|6m|1y|2y|5y | --bars N] [--step 30s|1min|5min|15min|30min|1h|4h|1d|1w|1mo] [--type STK|OPT|FUT|FOP|IND|...] [--exchange X] [--outside-rth]` (period and bars are exclusive)
- `account`, `balances`, `positions`, `orders [--completed]`, `trades [--period today|7d|30d|60d|90d|mtd|ytd|lq]`, `perf [--allocation]`
- `options params <underlying_id> [--type OPT|FOP]`; `options chain <expiration_id> [--min S] [--max S]` or `options chain --underlying <id> --expiry YYYYMMDD [--trading-class C]`. Bound strikes around the spot price (get it with `quote`). Quote an option with `quote <call_contract_id|put_contract_id>` (futures options need `--exchange`).
- `futures <query>`, `earnings <query>|--id ID`, `themes [--id ID|--company CONTRACT]`, `topics <query>`
- `screener templates|capabilities|saved|preview [k=v]|run <id>`
- `watchlists`, `watchlist <id>`, `alerts`, `alert <id>`, `flow`, `whats-new`, `instruction list`

## Write commands (need `--yes` after user approval)
- `watchlist create <name> [--contracts ids]`, `watchlist edit <id> [--name N] [--add ids] [--remove ids]`, `watchlist delete <id>`
- `alert create k=v...`, `alert update k=v...`, `alert delete <id>`, `alert status <id> on|off` (alerts without an email are in-app only, visible in IBKR Desktop)
- `instruction create --contract <id_ex> --side BUY|SELL --qty N [--type LMT --price P --tif DAY]`: reviewable, NOT live; only when `ibkr flow` = REVIEW_AND_APPROVE; prints `Review link:` which you must show the user. `instruction delete <id>`.
- `call provide_customer_feedback ...` is a write.

## Live orders (opt-in, real money)
`order preview|submit|modify|cancel` with `--contract --side --qty --type --price --tif --order-id`. Exist only with the orders scope and AUTONOMOUS flow; otherwise they exit 1 with an explanation. Preview first, get an explicit user yes for the exact order, run once with `--yes`. On exit 5 do not retry.

## Presenting results
Show symbol, description, expiry date, strike, quantities and prices. Do not show internal ids (`*_ex`, expiration ids). Link formatting: print review links verbatim.
