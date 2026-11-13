# Contract: output formats

Output is the token-saving core. Provider responses are trimmed to the useful fields; nothing else is printed.

## Modes

| Flag | Output |
|---|---|
| (default) | compact plain text: no colors, no decoration |
| `--json` | one line of compact JSON: the trimmed object |
| `--raw` | the provider's full response, compact JSON (or raw text if the tool returned plain text) |
| `--fields a,b,c` | choose columns/keys (dotted paths for nested values, e.g. `last.price`); overrides the default selection and can reveal hidden `*_ex` ids |
| `--limit N` | at most N rows for list output |

`--json` and `--raw` print only to stdout; diagnostics, previews and warnings go to stderr.

## Default text format

- **Object** (single record): one `key: value` line per field, in the view's preferred order.
- **List**: a header line then one row per item, columns separated by two spaces, aligned. Empty result: `(no results)`.
- **List with scalar siblings** (e.g. an option chain with `currency`, `exchange`): the siblings print first as `key: value`.
- Missing cell: `-`. Nested maps are flattened to dotted keys (depth 2); other composites become compact JSON. Cells longer than 70 characters are truncated with `…` in text mode (never in JSON).
- Numbers are never printed in scientific notation.

## Trimming rules

1. Each curated command declares a preferred column list (and list keys). Only preferred columns present in the data are shown.
2. If none of the preferred columns exist (the provider renamed fields), the first 8 keys (lists) or 40 keys (objects) sorted alphabetically are shown, so output never goes empty.
3. Keys ending in `_ex` (internal ids) are hidden by default. Expiration ids are shown by `options params` because the next command needs them; agents must not show them to end users.
4. A response that is not JSON is printed as trimmed text.

## JSON shapes (`--json`)

- Object -> `{"key": value, ...}` (dotted keys for flattened nested values).
- List -> `[{"col": value, ...}, ...]`.
- List with scalar siblings -> `{"rows": [...], "currency": "USD", ...}`.
- `tools` -> `[{"name","kind","summary"}]`; `describe` -> `{"rows":[{name,type,required,enum,description}],"tool":..., "kind":...}`.
- `status` -> flat string map.

## Deep links

`instruction create` and `instruction list` always print `Review link: <url>` for every URL in the response. In `--json`/`--raw` mode the line goes to stderr so stdout stays valid JSON.

## Stability

Column names follow the provider's field names. Curated commands may add or drop preferred columns in a minor release (noted in the CHANGELOG); `--raw` is the stable escape hatch.
