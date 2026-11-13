# Commands

Run `ibkr <command> --help` for flags. Output is compact text; add `--json` to parse, `--raw` for the provider's full response, `--fields a,b` to choose columns, `--limit N` to cap rows.

## Always-current commands

| Command | What it does |
|---|---|
| `ibkr tools [--search q] [--refresh]` | every tool: name, kind (read/write/order), summary |
| `ibkr describe <tool>` | compact parameter list: name, type, required, enum |
| `ibkr call <tool> key=value ... [--args-json '{...}']` | call any tool; values are typed from the schema |

New connector tools work through `call` immediately, before this CLI gets a curated command for them.

## Market data and research

| Command | What it does |
|---|---|
| `search <query>` | find contracts and their contract ids |
| `quote <id>... [--data last,bid_ask,change]` | price snapshot (futures need `--exchange`) |
| `history <id> [--period 1m \| --bars 30] [--step 1d] [--type STK]` | OHLCV bars |
| `options params <underlying_id>` | expirations |
| `options chain <expiration_id> --min S --max S` | or `--underlying <id> --expiry YYYYMMDD` |
| `futures <query>` | futures contracts |
| `screener templates\|capabilities\|preview\|saved\|run` | screeners |
| `earnings <query>`, `themes`, `topics <query>` | earnings insights, themes, investment topics |

## Account

`account`, `balances`, `positions`, `orders [--completed]`, `trades [--period 7d]`, `perf [--allocation]`, `flow`, `whats-new`.

## Managing things (need `--yes`)

| Command | What it does |
|---|---|
| `watchlists`, `watchlist <id>` | read |
| `watchlist create <name> [--contracts ids]`, `watchlist edit <id> --add/--remove/--name`, `watchlist delete <id>` | change |
| `alerts`, `alert <id>` | read (alerts are visible in IBKR Desktop only) |
| `alert create k=v...`, `alert update k=v...`, `alert delete <id>`, `alert status <id> on\|off` | change |
| `instruction create --contract ID --side BUY --qty 1 ...` | create a reviewable order instruction (not a live order) and print its review link |
| `instruction list`, `instruction delete <id>` | list / delete |

## Live orders (opt-in)

`order preview|submit|modify|cancel`: only available with the orders scope and the AUTONOMOUS flow. See [Safety](Safety.md).

## Meta

`connect`, `status`, `logout`, `version [--check]`, `completion <shell>`.

## Exit codes

0 ok, 1 error, 2 usage, 3 not connected or expired (run `ibkr connect`), 4 confirmation required (add `--yes` after you agree), 5 ambiguous order outcome (check `ibkr orders`, do not retry).

Every curated command also accepts `--arg key=value` and `--args-json` for provider parameters it does not know yet.
