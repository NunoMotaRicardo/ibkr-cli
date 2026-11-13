package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/mcp"
)

// extra holds the pass-through flags every curated command accepts, so a
// provider parameter this CLI does not know yet is still reachable.
type extra struct {
	args []string
	json string
}

func (e *extra) register(c *cobra.Command) {
	c.Flags().StringArrayVar(&e.args, "arg", nil, "extra tool argument key=value (repeatable; see `ibkr describe <tool>`)")
	c.Flags().StringVar(&e.json, "args-json", "", "extra tool arguments as a JSON object")
}

func pk(t mcp.Tool, cands ...string) string { return mcp.PickParam(t.InputSchema, cands...) }

func kvAdd(kv *[]string, key, val string) {
	if val != "" {
		*kv = append(*kv, key+"="+val)
	}
}

// exec is the shared body of curated commands: resolve the tool, build typed
// arguments from key=value pairs against its live schema, call, render.
func (a *App) exec(cmd *cobra.Command, tool string, ex *extra, view View, o callOpts, kv func(t mcp.Tool) []string) error {
	ctx := cmd.Context()
	t, err := a.lookupTool(ctx, tool)
	if err != nil {
		if o.missingHint != "" {
			if ee, ok := err.(*ExitErr); ok && ee.Code == ExitUsage {
				return failErr("tool %s is not available. %s", tool, o.missingHint)
			}
		}
		return err
	}
	var pairs []string
	if kv != nil {
		pairs = kv(t)
	}
	var exArgs []string
	var exJSON string
	if ex != nil {
		exArgs, exJSON = ex.args, ex.json
	}
	args, err := mcp.BuildArgs(t.InputSchema, append(pairs, exArgs...), exJSON)
	if err != nil {
		return usageErr("%v\nRun `ibkr describe %s` to see its parameters.", err, tool)
	}
	res, err := a.callTool(ctx, t, args, o)
	if err != nil {
		return err
	}
	if err := a.emit(res, view); err != nil {
		return err
	}
	if o.links != "" {
		a.printLinks(res, o.links)
	}
	return nil
}

func exactArgs(n int) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) != n {
			return usageErr("error: %s takes %d argument(s), got %d\nUsage: %s", c.CommandPath(), n, len(args), c.UseLine())
		}
		return nil
	}
}

func minArgs(n int) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) < n {
			return usageErr("error: %s needs at least %d argument(s)\nUsage: %s", c.CommandPath(), n, c.UseLine())
		}
		return nil
	}
}

func noArgs(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usageErr("error: %s takes no arguments\nUsage: %s", c.CommandPath(), c.UseLine())
	}
	return nil
}

// simple builds a no-argument read command.
func (a *App) simple(use, short, tool string, view View) *cobra.Command {
	var ex extra
	c := &cobra.Command{Use: use, Short: short, Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, tool, &ex, view, callOpts{}, nil)
		}}
	ex.register(c)
	return c
}

var (
	rowKeys = []string{"data", "items", "results", "rows"}
	idAlias = []string{"contract_id", "conid", "id"}
)

func lk(keys ...string) []string { return append(append([]string{}, keys...), rowKeys...) }

func (a *App) addCurated(root *cobra.Command) {
	root.AddCommand(
		a.searchCmd(), a.quoteCmd(), a.historyCmd(),
		a.simple("account", "Account summary (net liquidation, buying power, margin)", "get_account_summary", View{}),
		a.simple("balances", "Cash balances by currency", "get_account_balances", View{ListKeys: lk("balances", "currencies"), Cols: []string{"currency", "cash", "cash_balance", "balance", "market_value", "total", "settled_cash"}}),
		a.simple("positions", "Open positions", "get_account_positions", View{ListKeys: lk("positions"), Cols: []string{"symbol", "description", "contract_id", "position", "quantity", "avg_price", "average_price", "avg_cost", "market_price", "last_price", "market_value", "unrealized_pnl", "realized_pnl", "currency"}}),
		a.ordersCmd(), a.tradesCmd(),
		a.watchlistsCmd(), a.watchlistCmd(), a.alertsCmd(), a.alertCmd(),
		a.optionsCmd(), a.futuresCmd(), a.screenerCmd(), a.earningsCmd(), a.themesCmd(), a.topicsCmd(),
		a.perfCmd(),
		a.simple("flow", "Show the order flow type (REVIEW_AND_APPROVE or AUTONOMOUS)", "get_flow_type", View{}),
		a.simple("whats-new", "Show the connector's recent changes", "whats_new", View{}),
		a.instructionCmd(), a.orderCmd(),
	)
}

// ---- market data -------------------------------------------------------

func (a *App) searchCmd() *cobra.Command {
	var ex extra
	c := &cobra.Command{Use: "search <query>", Short: "Find contracts (stocks, ETFs, futures...) and their contract ids", Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "search_contracts", &ex, View{ListKeys: lk("contracts"), Cols: []string{"contract_id", "symbol", "description", "exchange", "sections", "country_code", "underlying_contract_id"}}, callOpts{},
				func(t mcp.Tool) []string { return []string{pk(t, "query") + "=" + strings.Join(args, " ")} })
		}}
	ex.register(c)
	return c
}

func (a *App) quoteCmd() *cobra.Command {
	var exchange, data string
	var ex extra
	c := &cobra.Command{Use: "quote <contract_id>...", Short: "Price snapshot (last, bid/ask) for one or more contract ids", Args: minArgs(1),
		Long: "Price snapshot. --data selects market_data_names, e.g. --data last,bid_ask,change,volume,option_greeks.\nFutures and futures options need --exchange (e.g. CME).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			t, err := a.lookupTool(ctx, "get_price_snapshot")
			if err != nil {
				return err
			}
			var rows []map[string]any
			var colOrder []string
			seen := map[string]bool{}
			for _, id := range args {
				kv := []string{pk(t, "contract_id") + "=" + id}
				kvAdd(&kv, "exchange", exchange)
				kvAdd(&kv, "market_data_names", data)
				targs, err := mcp.BuildArgs(t.InputSchema, append(kv, ex.args...), ex.json)
				if err != nil {
					return usageErr("%v\nRun `ibkr describe get_price_snapshot`.", err)
				}
				res, err := a.callTool(ctx, t, targs, callOpts{})
				if err != nil {
					return err
				}
				if a.opts.raw {
					if err := a.emit(res, View{}); err != nil {
						return err
					}
					continue
				}
				row := map[string]any{"contract_id": id}
				switch d := res.Data().(type) {
				case map[string]any:
					for k, v := range flatten(d) {
						row[k] = v
					}
				default:
					row["value"] = d
				}
				var keys []string
				for k := range row {
					if !seen[k] && k != "contract_id" && !hidden(k, View{}) {
						keys = append(keys, k)
					}
				}
				sort.Strings(keys)
				for _, k := range keys {
					seen[k] = true
					colOrder = append(colOrder, k)
				}
				rows = append(rows, row)
			}
			if a.opts.raw {
				return nil
			}
			cols := append([]string{"contract_id"}, colOrder...)
			if f := parseFields(a.opts.fields); len(f) > 0 {
				cols = f
			}
			if len(cols) > 14 && len(parseFields(a.opts.fields)) == 0 {
				cols = cols[:14]
			}
			return a.writeResult(&Result{List: len(rows) > 1, Cols: cols, Rows: rows})
		}}
	c.Flags().StringVar(&exchange, "exchange", "", "exchange (required for futures / futures options)")
	c.Flags().StringVar(&data, "data", "", "market data fields, comma-separated (default last,bid_ask)")
	ex.register(c)
	return c
}

var periodAlias = map[string]string{"1d": "ONE_DAY", "2d": "TWO_DAYS", "3d": "THREE_DAYS", "1w": "ONE_WEEK", "2w": "TWO_WEEKS", "1m": "ONE_MONTH", "3m": "THREE_MONTHS", "6m": "SIX_MONTHS", "1y": "ONE_YEAR", "2y": "TWO_YEARS", "5y": "FIVE_YEARS"}
var stepAlias = map[string]string{"30s": "THIRTY_SECS", "1min": "ONE_MIN", "2min": "TWO_MINS", "5min": "FIVE_MINS", "10min": "TEN_MINS", "15min": "FIFTEEN_MINS", "30min": "THIRTY_MINS", "1h": "ONE_HOUR", "2h": "TWO_HOURS", "4h": "FOUR_HOURS", "1d": "ONE_DAY", "1w": "ONE_WEEK", "1mo": "ONE_MONTH"}

func alias(m map[string]string, v string) string {
	if a, ok := m[strings.ToLower(v)]; ok {
		return a
	}
	return v
}

func (a *App) historyCmd() *cobra.Command {
	var period, step, typ, exchange string
	var bars int
	var rth bool
	var ex extra
	c := &cobra.Command{Use: "history <contract_id>", Short: "OHLCV price bars", Args: exactArgs(1),
		Long: "OHLCV bars. Use --period (1d 2d 3d 1w 2w 1m 3m 6m 1y 2y 5y or ONE_MONTH...) OR --bars N, not both.\n--step: 30s 1min 2min 5min 10min 15min 30min 1h 2h 4h 1d 1w 1mo (default 1d). --type: STK OPT FUT FOP IND ... (default STK).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if period != "" && bars > 0 {
				return usageErr("error: use --period or --bars, not both")
			}
			if period == "" && bars == 0 {
				period = "1m"
			}
			return a.exec(cmd, "get_price_history", &ex, View{ListKeys: lk("bars", "history", "points", "candles", "ohlc"),
				Cols: []string{"time", "t", "date", "datetime", "timestamp", "open", "o", "high", "h", "low", "l", "close", "c", "volume", "v"}}, callOpts{},
				func(t mcp.Tool) []string {
					kv := []string{pk(t, "contract_id") + "=" + args[0], "security_type=" + strings.ToUpper(typ), "step=" + alias(stepAlias, step), "outside_rth=" + strconv.FormatBool(rth)}
					if period != "" {
						kv = append(kv, "period="+alias(periodAlias, period))
					} else {
						kv = append(kv, "step_count="+strconv.Itoa(bars))
					}
					kvAdd(&kv, "exchange", exchange)
					return kv
				})
		}}
	c.Flags().StringVar(&period, "period", "", "time period (default 1m when --bars is not set)")
	c.Flags().IntVar(&bars, "bars", 0, "number of bars instead of a period")
	c.Flags().StringVar(&step, "step", "1d", "bar size")
	c.Flags().StringVar(&typ, "type", "STK", "security type")
	c.Flags().StringVar(&exchange, "exchange", "", "exchange (omit for SMART)")
	c.Flags().BoolVar(&rth, "outside-rth", false, "include data outside regular trading hours")
	ex.register(c)
	return c
}

// ---- account -------------------------------------------------------------

func (a *App) ordersCmd() *cobra.Command {
	var completed bool
	var ex extra
	c := &cobra.Command{Use: "orders", Short: "Working orders (--completed adds today's finished orders)", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "get_account_orders", &ex, View{ListKeys: lk("orders"), Cols: []string{"order_id", "symbol", "side", "order_type", "status", "quantity", "price", "filled", "filled_quantity", "avg_fill_price", "tif"}}, callOpts{},
				func(t mcp.Tool) []string {
					if completed {
						return []string{"show_completed=true"}
					}
					return nil
				})
		}}
	c.Flags().BoolVar(&completed, "completed", false, "include orders completed today")
	ex.register(c)
	return c
}

var tradePeriod = map[string]string{"today": "TODAY", "7d": "DAYS_7", "30d": "DAYS_30", "60d": "DAYS_60", "90d": "DAYS_90", "mtd": "MONTH_TO_DATE", "ytd": "YEAR_TO_DATE", "lq": "LAST_QUARTER"}

func (a *App) tradesCmd() *cobra.Command {
	var period string
	var ex extra
	c := &cobra.Command{Use: "trades", Short: "Executed trades (--period today|7d|30d|60d|90d|mtd|ytd|lq)", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "get_account_trades", &ex, View{ListKeys: lk("trades"), Cols: []string{"trade_time", "symbol", "side", "size", "quantity", "price", "commission", "trade_id", "exchange"}}, callOpts{},
				func(t mcp.Tool) []string {
					var kv []string
					kvAdd(&kv, "period", alias(tradePeriod, period))
					return kv
				})
		}}
	c.Flags().StringVar(&period, "period", "", "time window (default: today)")
	ex.register(c)
	return c
}

func (a *App) perfCmd() *cobra.Command {
	var alloc bool
	var ex extra
	c := &cobra.Command{Use: "perf", Short: "Portfolio analyst performance (all periods); --allocation for allocation", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := "get_pa_performance_all_periods"
			if alloc {
				tool = "get_pa_allocation"
			}
			return a.exec(cmd, tool, &ex, View{}, callOpts{}, nil)
		}}
	c.Flags().BoolVar(&alloc, "allocation", false, "show allocation instead of performance")
	ex.register(c)
	return c
}

// ---- research --------------------------------------------------------------

func (a *App) futuresCmd() *cobra.Command {
	var ex extra
	c := &cobra.Command{Use: "futures <query>", Short: "Search futures contracts", Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "search_futures", &ex, View{ListKeys: lk("futures", "contracts"), Cols: []string{"contract_id", "symbol", "description", "exchange", "expiration", "expiry", "multiplier", "currency"}}, callOpts{},
				func(t mcp.Tool) []string {
					return []string{pk(t, "query", "symbol", "search") + "=" + strings.Join(args, " ")}
				})
		}}
	ex.register(c)
	return c
}

func (a *App) earningsCmd() *cobra.Command {
	var id string
	var ex extra
	c := &cobra.Command{Use: "earnings [query]", Short: "Earnings insights: search by query, or --id for one insight", Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if id != "" {
				return a.exec(cmd, "get_earnings_insight", &ex, View{}, callOpts{}, func(t mcp.Tool) []string { return []string{pk(t, "id", "insight_id", "earnings_id") + "=" + id} })
			}
			if len(args) == 0 {
				return usageErr("error: give a query or --id\nUsage: %s", cmd.UseLine())
			}
			return a.exec(cmd, "search_earnings_insights", &ex, View{ListKeys: lk("insights", "earnings")}, callOpts{},
				func(t mcp.Tool) []string {
					return []string{pk(t, "query", "search", "symbol") + "=" + strings.Join(args, " ")}
				})
		}}
	c.Flags().StringVar(&id, "id", "", "fetch one earnings insight by id")
	ex.register(c)
	return c
}

func (a *App) themesCmd() *cobra.Command {
	var id, company string
	var ex extra
	c := &cobra.Command{Use: "themes", Short: "Investment themes: list, --id for details, --company <contract_id> for a company's themes", Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case id != "":
				return a.exec(cmd, "get_theme_details", &ex, View{}, callOpts{}, func(t mcp.Tool) []string { return []string{pk(t, "theme_id", "id") + "=" + id} })
			case company != "":
				return a.exec(cmd, "get_company_themes", &ex, View{ListKeys: lk("themes")}, callOpts{}, func(t mcp.Tool) []string { return []string{pk(t, "contract_id") + "=" + company} })
			}
			return a.exec(cmd, "get_screener_themes", &ex, View{ListKeys: lk("themes")}, callOpts{}, nil)
		}}
	c.Flags().StringVar(&id, "id", "", "theme id for details")
	c.Flags().StringVar(&company, "company", "", "contract id: themes of this company")
	ex.register(c)
	return c
}

func (a *App) topicsCmd() *cobra.Command {
	var ex extra
	c := &cobra.Command{Use: "topics <query>", Short: "Search investment topics", Args: minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "search_investment_topics", &ex, View{ListKeys: lk("topics")}, callOpts{},
				func(t mcp.Tool) []string { return []string{pk(t, "query", "search") + "=" + strings.Join(args, " ")} })
		}}
	ex.register(c)
	return c
}

// ---- options -----------------------------------------------------------------

func (a *App) optionsCmd() *cobra.Command {
	c := &cobra.Command{Use: "options", Short: "Option expirations and chains (params, chain)"}
	var typ, exchange string
	var pex extra
	params := &cobra.Command{Use: "params <underlying_contract_id>", Short: "Option expirations for an underlying (use the underlying_contract_id from `search`)", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "get_option_parameters", &pex, View{ListKeys: []string{"expirations"}, Cols: []string{"date", "regular", "trading_class", "id"}}, callOpts{},
				func(t mcp.Tool) []string {
					kv := []string{pk(t, "underlying_contract_id") + "=" + args[0], "option_sec_type=" + strings.ToUpper(typ)}
					kvAdd(&kv, "option_exchange", exchange)
					return kv
				})
		}}
	params.Flags().StringVar(&typ, "type", "OPT", "OPT or FOP")
	params.Flags().StringVar(&exchange, "exchange", "", "option exchange")
	pex.register(params)

	var minS, maxS, underlying, expiry, tclass string
	var cex extra
	chain := &cobra.Command{Use: "chain [expiration_id]", Short: "Option chain for one expiration (strikes with call/put contract ids)",
		Long: "Give an expiration id from `options params`, or --underlying <id> --expiry YYYYMMDD to resolve it automatically.\nAlways bound strikes with --min/--max around the spot price. Quote contracts with `ibkr quote <call_contract_id>`.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expID := ""
			if len(args) == 1 {
				expID = args[0]
			}
			if expID == "" {
				if underlying == "" || expiry == "" {
					return usageErr("error: give an expiration id, or --underlying and --expiry\nUsage: %s", cmd.UseLine())
				}
				id, err := a.resolveExpiration(cmd, underlying, expiry, tclass, typ, exchange)
				if err != nil {
					return err
				}
				expID = id
			}
			return a.exec(cmd, "get_option_data", &cex, View{ListKeys: lk("strikes", "chain", "options"), Cols: []string{"strike", "call_contract_id", "call_description", "put_contract_id", "put_description"}}, callOpts{},
				func(t mcp.Tool) []string {
					kv := []string{pk(t, "expiration_id") + "=" + expID}
					kvAdd(&kv, "min_strike", minS)
					kvAdd(&kv, "max_strike", maxS)
					return kv
				})
		}}
	chain.Flags().StringVar(&minS, "min", "", "minimum strike")
	chain.Flags().StringVar(&maxS, "max", "", "maximum strike")
	chain.Flags().StringVar(&underlying, "underlying", "", "underlying contract id (with --expiry)")
	chain.Flags().StringVar(&expiry, "expiry", "", "expiration date YYYYMMDD (with --underlying)")
	chain.Flags().StringVar(&tclass, "trading-class", "", "trading class when a date has several")
	chain.Flags().StringVar(&typ, "type", "OPT", "OPT or FOP (with --underlying)")
	chain.Flags().StringVar(&exchange, "exchange", "", "option exchange (with --underlying)")
	cex.register(chain)
	c.AddCommand(params, chain)
	return c
}

// resolveExpiration maps (underlying, date[, trading class]) to an expiration id.
func (a *App) resolveExpiration(cmd *cobra.Command, underlying, date, tclass, typ, exchange string) (string, error) {
	ctx := cmd.Context()
	t, err := a.lookupTool(ctx, "get_option_parameters")
	if err != nil {
		return "", err
	}
	kv := []string{pk(t, "underlying_contract_id") + "=" + underlying, "option_sec_type=" + strings.ToUpper(typ)}
	kvAdd(&kv, "option_exchange", exchange)
	args, err := mcp.BuildArgs(t.InputSchema, kv, "")
	if err != nil {
		return "", usageErr("%v", err)
	}
	res, err := a.callTool(ctx, t, args, callOpts{})
	if err != nil {
		return "", err
	}
	m, _ := res.Data().(map[string]any)
	exps, _ := m["expirations"].([]any)
	var match []map[string]any
	for _, e := range exps {
		em, ok := e.(map[string]any)
		if !ok || fmtValue(em["date"]) != date {
			continue
		}
		match = append(match, em)
	}
	if len(match) == 0 {
		return "", failErr("no expiration on %s for that underlying; run `ibkr options params %s`", date, underlying)
	}
	var pick map[string]any
	for _, em := range match {
		tc := fmtValue(em["trading_class"])
		if tclass != "" && tc == tclass {
			pick = em
		}
		if tclass == "" && (em["trading_class"] == nil || tc == "-") {
			pick = em
		}
	}
	if pick == nil && tclass == "" && len(match) == 1 {
		pick = match[0]
	}
	if pick == nil {
		var tcs []string
		for _, em := range match {
			tcs = append(tcs, fmtValue(em["trading_class"]))
		}
		return "", usageErr("several contracts expire on %s (trading classes: %s); choose one with --trading-class", date, strings.Join(tcs, ", "))
	}
	id := fmtValue(pick["id"])
	if id == "-" {
		return "", failErr("expiration entry had no id; use `ibkr options params` and pass the id")
	}
	return id, nil
}

// ---- screeners ---------------------------------------------------------------

func (a *App) screenerCmd() *cobra.Command {
	c := &cobra.Command{Use: "screener", Short: "Stock screeners (templates, capabilities, preview, saved, run)"}
	mk := func(use, short, tool string, view View) *cobra.Command {
		var ex extra
		cc := &cobra.Command{Use: use, Short: short, Args: noArgs,
			RunE: func(cmd *cobra.Command, args []string) error { return a.exec(cmd, tool, &ex, view, callOpts{}, nil) }}
		ex.register(cc)
		return cc
	}
	var pex extra
	preview := &cobra.Command{Use: "preview [key=value ...]", Short: "Preview a screener (arguments as key=value / --args-json; see `ibkr describe preview_screener`)", Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			pex.args = append(append([]string{}, args...), pex.args...)
			return a.exec(cmd, "preview_screener", &pex, View{ListKeys: lk("results", "rows", "matches")}, callOpts{}, nil)
		}}
	pex.register(preview)
	var rex extra
	run := &cobra.Command{Use: "run <saved_screener_id>", Short: "Run a saved screener", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "run_saved_screener", &rex, View{ListKeys: lk("results", "rows", "matches")}, callOpts{},
				func(t mcp.Tool) []string {
					return []string{pk(t, "screener_id", "saved_screener_id", "id") + "=" + args[0]}
				})
		}}
	rex.register(run)
	c.AddCommand(
		mk("templates", "List screener templates", "get_screener_templates", View{ListKeys: lk("templates")}),
		mk("capabilities", "List screener fields and operators", "get_screener_capabilities", View{}),
		preview,
		mk("saved", "List your saved screeners", "list_saved_screeners", View{ListKeys: lk("screeners", "saved_screeners")}),
		run,
	)
	return c
}

// ---- watchlists / alerts ---------------------------------------------------------

func (a *App) watchlistsCmd() *cobra.Command {
	return a.simple("watchlists", "List watchlists", "get_watchlists", View{ListKeys: lk("watchlists"), Cols: []string{"id", "name", "count", "instrument_count"}})
}

func (a *App) watchlistCmd() *cobra.Command {
	var gex extra
	c := &cobra.Command{Use: "watchlist <id>", Short: "Show a watchlist; subcommands: create, edit, delete", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "get_watchlist", &gex, View{ListKeys: lk("instruments", "contracts", "items"), Cols: []string{"contract_id", "symbol", "description", "name", "exchange"}}, callOpts{},
				func(t mcp.Tool) []string { return []string{pk(t, "watchlist_id", "id") + "=" + args[0]} })
		}}
	gex.register(c)

	var cids string
	var cex extra
	create := &cobra.Command{Use: "create <name>", Short: "Create a watchlist (--contracts id,id,...)", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "create_watchlist", &cex, View{}, callOpts{summary: "create watchlist " + strconv.Quote(args[0])},
				func(t mcp.Tool) []string {
					kv := []string{pk(t, "name", "watchlist_name") + "=" + args[0]}
					kvAdd(&kv, pk(t, "contract_ids", "contracts", "instruments", "conids"), cids)
					return kv
				})
		}}
	create.Flags().StringVar(&cids, "contracts", "", "contract ids, comma-separated")
	cex.register(create)

	var newName, add, remove string
	var eex extra
	edit := &cobra.Command{Use: "edit <id>", Short: "Edit a watchlist (--name, --add ids, --remove ids)", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "edit_watchlist", &eex, View{}, callOpts{summary: "edit watchlist " + args[0]},
				func(t mcp.Tool) []string {
					kv := []string{pk(t, "watchlist_id", "id") + "=" + args[0]}
					kvAdd(&kv, pk(t, "name", "watchlist_name"), newName)
					kvAdd(&kv, pk(t, "add_contract_ids", "add", "contracts_to_add", "add_contracts"), add)
					kvAdd(&kv, pk(t, "remove_contract_ids", "remove", "contracts_to_remove", "remove_contracts"), remove)
					return kv
				})
		}}
	edit.Flags().StringVar(&newName, "name", "", "new name")
	edit.Flags().StringVar(&add, "add", "", "contract ids to add")
	edit.Flags().StringVar(&remove, "remove", "", "contract ids to remove")
	eex.register(edit)

	var dex extra
	del := &cobra.Command{Use: "delete <id>", Short: "Delete a watchlist", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "delete_watchlist", &dex, View{}, callOpts{summary: "DELETE watchlist " + args[0]},
				func(t mcp.Tool) []string { return []string{pk(t, "watchlist_id", "id") + "=" + args[0]} })
		}}
	dex.register(del)
	c.AddCommand(create, edit, del)
	return c
}

func (a *App) alertsCmd() *cobra.Command {
	return a.simple("alerts", "List price alerts (visible in IBKR Desktop only)", "get_alerts", View{ListKeys: lk("alerts"), Cols: []string{"alert_id", "id", "name", "symbol", "condition", "status", "active", "created"}})
}

func (a *App) alertCmd() *cobra.Command {
	var gex extra
	c := &cobra.Command{Use: "alert <id>", Short: "Show an alert; subcommands: create, update, delete, status", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "get_alert", &gex, View{}, callOpts{}, func(t mcp.Tool) []string { return []string{pk(t, "alert_id", "id") + "=" + args[0]} })
		}}
	gex.register(c)

	pass := func(use, short, tool, sum string) *cobra.Command {
		var ex extra
		cc := &cobra.Command{Use: use, Short: short, Args: cobra.ArbitraryArgs,
			Long: short + "\nArguments are key=value pairs (typed from the tool schema) or --args-json. Run `ibkr describe " + tool + "` first.\nAlerts without an email are in-app only and visible in IBKR Desktop.",
			RunE: func(cmd *cobra.Command, args []string) error {
				ex.args = append(append([]string{}, args...), ex.args...)
				return a.exec(cmd, tool, &ex, View{}, callOpts{summary: sum + " " + strings.Join(args, " ")}, nil)
			}}
		ex.register(cc)
		return cc
	}
	create := pass("create [key=value ...]", "Create a price alert", "create_alert", "create alert")
	update := pass("update [key=value ...]", "Update an alert", "update_alert", "update alert")
	var dex extra
	del := &cobra.Command{Use: "delete <id>", Short: "Delete an alert", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.exec(cmd, "delete_alert", &dex, View{}, callOpts{summary: "DELETE alert " + args[0]}, func(t mcp.Tool) []string { return []string{pk(t, "alert_id", "id") + "=" + args[0]} })
		}}
	dex.register(del)
	var sex extra
	status := &cobra.Command{Use: "status <id> <on|off>", Short: "Activate or deactivate an alert", Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			on, ok := map[string]bool{"on": true, "active": true, "true": true, "off": false, "inactive": false, "false": false}[strings.ToLower(args[1])]
			if !ok {
				return usageErr("error: state must be on or off")
			}
			return a.exec(cmd, "set_alert_status", &sex, View{}, callOpts{summary: fmt.Sprintf("set alert %s %s", args[0], args[1])},
				func(t mcp.Tool) []string {
					p := pk(t, "active", "enabled", "status", "is_active")
					val := strconv.FormatBool(on)
					if props, _ := t.InputSchema["properties"].(map[string]any); props != nil {
						if pm, ok := props[p].(map[string]any); ok && mcp.Params(map[string]any{"properties": map[string]any{p: pm}})[0].Type != "boolean" {
							val = map[bool]string{true: "ACTIVE", false: "INACTIVE"}[on]
						}
					}
					return []string{pk(t, "alert_id", "id") + "=" + args[0], p + "=" + val}
				})
		}}
	sex.register(status)
	c.AddCommand(create, update, del, status)
	return c
}
