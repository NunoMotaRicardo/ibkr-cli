// Package cli wires cobra commands to the MCP client and renders output.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/auth"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/config"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/mcp"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
)

// Exit codes.
const (
	ExitOK        = 0
	ExitError     = 1
	ExitUsage     = 2
	ExitAuth      = 3
	ExitConfirm   = 4
	ExitAmbiguous = 5
)

// ExitErr carries an exit code and a message for stderr.
type ExitErr struct {
	Code int
	Msg  string
}

func (e *ExitErr) Error() string { return e.Msg }

func usageErr(format string, a ...any) error { return &ExitErr{ExitUsage, fmt.Sprintf(format, a...)} }
func failErr(format string, a ...any) error  { return &ExitErr{ExitError, fmt.Sprintf(format, a...)} }

type options struct {
	json    bool
	raw     bool
	fields  string
	limit   int
	yes     bool
	server  string
	profile string
	timeout time.Duration
}

// App holds the dependencies of every command so tests can inject fakes.
type App struct {
	Out, Err    io.Writer
	In          io.Reader
	InTTY       bool
	OutTTY      bool
	Store       *secrets.Store
	HC          *http.Client // base client; nil = default
	CacheDir    string
	OpenBrowser func(url string) error
	ReleaseURL  string // GitHub "latest release" API URL for `version --check`
	Now         func() time.Time

	opts   options
	reader *bufio.Reader
	cl     *mcp.Client
	cache  *mcp.ToolCache
}

// NewDefault builds an App for real use.
func NewDefault() (*App, error) {
	cfg, err := config.Dir()
	if err != nil {
		return nil, err
	}
	cache, err := config.CacheDir()
	if err != nil {
		return nil, err
	}
	return &App{
		Out: os.Stdout, Err: os.Stderr, In: os.Stdin,
		InTTY:       term.IsTerminal(int(os.Stdin.Fd())),
		OutTTY:      term.IsTerminal(int(os.Stdout.Fd())),
		Store:       &secrets.Store{Service: config.AppName, Dir: cfg},
		CacheDir:    cache,
		OpenBrowser: openBrowser,
		ReleaseURL:  "https://api.github.com/repos/NunoMotaRicardo/ibkr-cli/releases/latest",
	}, nil
}

func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *App) lineReader() *bufio.Reader {
	if a.reader == nil {
		a.reader = bufio.NewReader(a.In)
	}
	return a.reader
}

func (a *App) baseHC() *http.Client {
	if a.HC != nil {
		return a.HC
	}
	return http.DefaultClient
}

func (a *App) serverURL() string { return config.ServerURL(a.opts.server) }

func (a *App) mcpClient() *mcp.Client {
	if a.cl == nil {
		hc := *a.baseHC()
		hc.Timeout = a.opts.timeout
		a.cl = &mcp.Client{
			URL:       a.serverURL(),
			HC:        &hc,
			Tokens:    &auth.Manager{Store: a.Store, HC: a.baseHC()},
			UserAgent: "ibkr-cli",
		}
	}
	return a.cl
}

// Execute runs the CLI and returns the process exit code.
func (a *App) Execute(args []string) int {
	root := a.Root()
	root.SetArgs(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	return a.report(err)
}

func (a *App) report(err error) int {
	var ee *ExitErr
	switch {
	case errors.As(err, &ee):
		if ee.Msg != "" {
			fmt.Fprintln(a.Err, ee.Msg)
		}
		return ee.Code
	case errors.Is(err, auth.ErrNotConnected):
		fmt.Fprintf(a.Err, "Not connected to IBKR%s. Run `%s` in a terminal (it opens your browser for login).\n", a.profileSuffix(), a.connectHint())
		return ExitAuth
	case errors.Is(err, auth.ErrAuthExpired):
		fmt.Fprintf(a.Err, "IBKR authorization expired or was revoked (%s). Run `%s` again.\n", strings.TrimPrefix(err.Error(), auth.ErrAuthExpired.Error()+": "), a.connectHint())
		return ExitAuth
	}
	msg := err.Error()
	for _, p := range []string{"unknown command", "required flag", "invalid argument", "accepts ", "requires at least", "unknown flag", "unknown shorthand", "flag needs an argument", "invalid "} {
		if strings.HasPrefix(msg, p) {
			fmt.Fprintln(a.Err, "error: "+msg+"\nRun `ibkr --help` for usage.")
			return ExitUsage
		}
	}
	fmt.Fprintln(a.Err, "error: "+msg)
	return ExitError
}

// Root builds the command tree.
func (a *App) Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "ibkr",
		Short:         "Token-lean command line for Interactive Brokers' MCP connector",
		Long:          "ibkr talks to IBKR's public remote MCP connector and exposes it as short shell commands.\nStart with `ibkr connect`, then try `ibkr account`, `ibkr positions`, `ibkr quote <contract_id>`.\nAny tool the connector adds later works immediately via `ibkr tools`, `ibkr describe <tool>`, `ibkr call <tool> k=v`.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error { return a.selectProfile(cmd) }
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return usageErr("error: %v", err) })
	f := root.PersistentFlags()
	f.BoolVar(&a.opts.json, "json", false, "emit compact JSON (the trimmed object)")
	f.BoolVar(&a.opts.raw, "raw", false, "emit the provider's full response")
	f.StringVar(&a.opts.fields, "fields", "", "comma-separated output fields/columns")
	f.IntVar(&a.opts.limit, "limit", 0, "max rows for list output")
	f.BoolVarP(&a.opts.yes, "yes", "y", false, "confirm state-changing actions")
	f.StringVar(&a.opts.server, "server", "", "MCP server URL (default: IBKR public connector; env IBKR_MCP_URL)")
	f.StringVar(&a.opts.profile, "profile", "", "credential profile (env IBKR_PROFILE); required once more than one profile exists")
	f.DurationVar(&a.opts.timeout, "timeout", 60*time.Second, "per-request timeout")
	_ = f.MarkHidden("server")

	root.AddCommand(
		a.connectCmd(), a.statusCmd(), a.logoutCmd(), a.versionCmd(), a.profilesCmd(),
		a.toolsCmd(), a.describeCmd(), a.callCmd(),
	)
	a.addCurated(root)
	return root
}

// ---- tool access ------------------------------------------------------

// toolCache returns the tool list, from cache when fresh, else live.
func (a *App) toolCache(ctx context.Context, force bool) (*mcp.ToolCache, error) {
	server := a.serverURL()
	if a.cache != nil && !force {
		return a.cache, nil
	}
	cached, _ := mcp.LoadCache(a.cacheDir())
	if !force && cached.Fresh(server, mcp.DefaultTTL, a.now()) {
		a.cache = cached
		return cached, nil
	}
	tools, err := a.mcpClient().ListTools(ctx)
	if err != nil {
		if cached != nil && cached.Server == server && !errors.Is(err, auth.ErrAuthExpired) && !errors.Is(err, auth.ErrNotConnected) {
			fmt.Fprintf(a.Err, "warning: could not refresh tool list (%v); using stale cache\n", err)
			a.cache = cached
			return cached, nil
		}
		return nil, err
	}
	c := &mcp.ToolCache{Server: server, FetchedAt: a.now(), Tools: tools}
	_ = mcp.SaveCache(a.cacheDir(), c)
	a.cache = c
	return c, nil
}

// lookupTool finds a tool, refreshing the cache once when unknown.
func (a *App) lookupTool(ctx context.Context, name string) (mcp.Tool, error) {
	c, err := a.toolCache(ctx, false)
	if err != nil {
		return mcp.Tool{}, err
	}
	if t, ok := c.Find(name); ok {
		return t, nil
	}
	if c, err = a.toolCache(ctx, true); err != nil {
		return mcp.Tool{}, err
	}
	if t, ok := c.Find(name); ok {
		return t, nil
	}
	return mcp.Tool{}, usageErr("unknown tool %q.%s", name, suggest(c.Names(), name))
}

func suggest(names []string, q string) string {
	q = strings.ToLower(q)
	var m []string
	for _, n := range names {
		if strings.Contains(n, q) || strings.Contains(q, n) {
			m = append(m, n)
		}
	}
	if len(m) == 0 {
		return " Run `ibkr tools` to list tools."
	}
	if len(m) > 6 {
		m = m[:6]
	}
	return " Did you mean: " + strings.Join(m, ", ") + "?"
}

var urlRe = regexp.MustCompile(`https?://[^\s"'<>\)\]\\]+`)

func findLinks(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, u := range urlRe.FindAllString(s, -1) {
		u = strings.TrimRight(u, ".,;")
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// callOpts tweaks a tool call.
type callOpts struct {
	summary     string // human description for the confirmation gate
	noPreview   bool
	links       string // label for deep links to always print (e.g. "Review link")
	missingHint string // explanation when the tool does not exist
}

// call invokes a tool with the safety gates: write and order tools need --yes
// (exit 4), orders are previewed in interactive mode and never retried on an
// ambiguous outcome (exit 5).
func (a *App) call(ctx context.Context, name string, args map[string]any, o callOpts) (*mcp.CallResult, error) {
	tool, err := a.lookupTool(ctx, name)
	if err != nil {
		return nil, err
	}
	return a.callTool(ctx, tool, args, o)
}

func (a *App) callTool(ctx context.Context, tool mcp.Tool, args map[string]any, o callOpts) (*mcp.CallResult, error) {
	kind := mcp.Classify(tool)
	if kind != mcp.KindRead && !a.opts.yes {
		sum := o.summary
		if sum == "" {
			sum = fmt.Sprintf("call %s %s", tool.Name, compactArgs(args))
		}
		warn := ""
		if kind == mcp.KindOrder {
			warn = " This can place, change or cancel a LIVE order."
		}
		return nil, &ExitErr{ExitConfirm, fmt.Sprintf("confirmation required (%s tool): would %s.%s\nRe-run with --yes to execute.", kind, sum, warn)}
	}
	if kind == mcp.KindOrder && a.InTTY && !o.noPreview && tool.Name != "preview_order" {
		if c, err := a.toolCache(ctx, false); err == nil {
			if _, ok := c.Find("preview_order"); ok {
				fmt.Fprintln(a.Err, "Previewing order first (preview_order):")
				pr, perr := a.mcpClient().CallTool(ctx, "preview_order", args)
				if perr != nil {
					return nil, failErr("preview_order failed, not submitting: %v", perr)
				}
				fmt.Fprintln(a.Err, strings.TrimSpace(pr.Text()))
				if pr.IsError {
					return nil, failErr("preview_order reported an error, not submitting")
				}
			}
		}
	}
	res, err := a.mcpClient().CallTool(ctx, tool.Name, args)
	if err != nil {
		var te *mcp.TransportError
		var he *mcp.HTTPStatusError
		if kind == mcp.KindOrder && (errors.As(err, &te) || (errors.As(err, &he) && he.Status >= 500)) {
			return nil, &ExitErr{ExitAmbiguous, fmt.Sprintf("outcome unknown: %v\nThe order may or may not have been placed. NOT retrying. Check `ibkr orders` and `ibkr trades` before doing anything else.", err)}
		}
		if errors.As(err, &te) && kind != mcp.KindRead {
			return nil, failErr("%v\n(this was a %s call; check the current state before retrying)", err, kind)
		}
		return nil, err
	}
	if res.IsError {
		return res, failErr("%s returned an error: %s", tool.Name, truncate(strings.TrimSpace(res.Text()), 600))
	}
	return res, nil
}

func compactArgs(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

// emit renders a result according to --json / --raw / --fields / --limit.
func (a *App) emit(res *mcp.CallResult, v View) error {
	return a.emitData(res.Data(), v, res)
}

func (a *App) emitData(data any, v View, res *mcp.CallResult) error {
	if a.opts.raw {
		if res != nil && res.StructuredContent == nil && !isJSONish(data) {
			fmt.Fprintln(a.Out, strings.TrimSpace(res.Text()))
			return nil
		}
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, string(b))
		return nil
	}
	r := extract(data, v, parseFields(a.opts.fields), a.opts.limit)
	return a.writeResult(r)
}

func isJSONish(d any) bool {
	switch d.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

func (a *App) writeResult(r *Result) error {
	if a.opts.json {
		return writeJSON(a.Out, r)
	}
	writeText(a.Out, r)
	return nil
}

// printLinks always surfaces deep links (e.g. order-instruction review URLs).
func (a *App) printLinks(res *mcp.CallResult, label string) {
	if res == nil {
		return
	}
	links := findLinks(res.Text())
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		links = append(links, findLinks(string(b))...)
	}
	seen := map[string]bool{}
	w := a.Out
	if a.opts.json || a.opts.raw {
		w = a.Err
	}
	for _, l := range links {
		if seen[l] {
			continue
		}
		seen[l] = true
		fmt.Fprintf(w, "%s: %s\n", label, l)
	}
}

func sortedStrings(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
