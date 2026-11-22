package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/auth"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/mcp"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
)

const totalSteps = 8

type connectFlags struct {
	allowOrders bool
	noBrowser   bool
	manual      bool
	status      bool
	wait        time.Duration
	port        int
}

func (a *App) connectCmd() *cobra.Command {
	var f connectFlags
	c := &cobra.Command{
		Use:   "connect",
		Short: "Guided login: authorize this CLI with your IBKR account (opens a browser)",
		Long: "Walks you through connecting ibkr to your Interactive Brokers account using the connector's OAuth login.\n" +
			"Run it yourself in a terminal: it opens your browser for the IBKR login and consent screen.\n" +
			"Default scopes: mcp.read mcp.write. --allow-orders adds mcp.orders.submit (autonomous trading).\n" +
			"Headless / SSH: use --manual and paste the redirected URL.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.status {
				return a.status(cmd.Context(), false)
			}
			return a.runConnect(cmd.Context(), f)
		},
	}
	c.Flags().BoolVar(&f.allowOrders, "allow-orders", false, "also request mcp.orders.submit (autonomous live orders)")
	c.Flags().BoolVar(&f.noBrowser, "no-browser", false, "do not open a browser; print the URL")
	c.Flags().BoolVar(&f.manual, "manual", false, "headless mode: print the URL, then paste the redirected URL")
	c.Flags().BoolVar(&f.status, "status", false, "same as ibkr status")
	c.Flags().DurationVar(&f.wait, "wait", 5*time.Minute, "how long to wait for the browser login")
	c.Flags().IntVar(&f.port, "port", 0, "fixed loopback port for the redirect (default: random)")
	return c
}

func (a *App) step(n int, title string) {
	fmt.Fprintf(a.Out, "\n[%d/%d] %s\n", n, totalSteps, title)
}

func (a *App) stepFail(cause, fix string) error {
	return failErr("  FAILED: %s\n  Fix: %s", cause, fix)
}

func (a *App) readLine(prompt string) (string, error) {
	fmt.Fprint(a.Out, prompt)
	line, err := a.lineReader().ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (a *App) runConnect(ctx context.Context, f connectFlags) error {
	// 1. explain
	a.step(1, "What will happen")
	if !a.isDefaultProfile() {
		fmt.Fprintf(a.Out, "  Profile: %s (registered with IBKR as %q). Log in with the IBKR login for this profile.\n", a.profile(), a.clientName())
	}
	fmt.Fprintln(a.Out, "  ibkr-cli will ask Interactive Brokers for permission to act on your account through IBKR's")
	fmt.Fprintln(a.Out, "  MCP connector. You log in on IBKR's own page (2FA via the IBKR mobile app may be required);")
	fmt.Fprintln(a.Out, "  this CLI never sees your IBKR password. You need an IBKR account with API/connector access.")
	fmt.Fprintln(a.Out, "  You can revoke access any time: `ibkr logout`, or Client Portal > Settings > Manage Third-Party Consents.")
	if others, _ := a.Store.Profiles(); len(others) > 0 && !(len(others) == 1 && others[0] == a.profile()) {
		fmt.Fprintln(a.Out, "  Note: IBKR keeps one consent per login. If another profile uses this same IBKR login, connecting here")
		fmt.Fprintln(a.Out, "  replaces its consent and it will fail with invalid_grant. Different logins (live, paper) are unaffected.")
	}
	existing, _, _ := a.Store.Load()
	if existing != nil && existing.HasToken() {
		fmt.Fprintln(a.Out, "  Note: you are already connected; this replaces the stored tokens"+a.profileSuffix()+".")
	}

	// 2. scopes
	a.step(2, "Choose permissions")
	scopes := []string{"mcp.read", "mcp.write"}
	fmt.Fprintln(a.Out, "  mcp.read   read account, positions, market data")
	fmt.Fprintln(a.Out, "  mcp.write  manage alerts, watchlists, order instructions (you approve instructions in IBKR)")
	if f.allowOrders {
		if !a.InTTY {
			return a.stepFail("--allow-orders needs an interactive terminal to confirm.", "run `ibkr connect --allow-orders` yourself in a terminal.")
		}
		fmt.Fprintln(a.Out, "\n  WARNING: mcp.orders.submit enables AUTONOMOUS TRADING. With IBKR's Autonomous Trading flow")
		fmt.Fprintln(a.Out, "  an agent using this CLI can submit, modify and cancel LIVE orders without per-order confirmation")
		fmt.Fprintln(a.Out, "  in IBKR. Real money is at risk. ibkr still requires --yes per order, but that is not a guarantee.")
		ans, err := a.readLine("  Type \"yes\" to request this scope, anything else to skip it: ")
		if err != nil {
			return a.stepFail("could not read your answer.", "run in an interactive terminal.")
		}
		if strings.EqualFold(ans, "yes") {
			scopes = append(scopes, "mcp.orders.submit")
		} else {
			fmt.Fprintln(a.Out, "  Skipping mcp.orders.submit.")
		}
	} else {
		fmt.Fprintln(a.Out, "  (live-order scope is off; add it with `ibkr connect --allow-orders`)")
	}
	fmt.Fprintf(a.Out, "  Requesting: %s\n", strings.Join(scopes, " "))

	hc := auth.DefaultHTTPClient()
	if a.HC != nil {
		hc = a.HC
	}
	server := a.serverURL()

	// 3. discovery
	a.step(3, "Discover the IBKR authorization endpoints")
	ep, err := auth.Discover(ctx, hc, server)
	if err != nil {
		return a.stepFail(err.Error(), "check your internet connection / proxy and that "+server+" is reachable, then retry.")
	}
	fmt.Fprintf(a.Out, "  server:        %s\n  authorize:     %s\n  token:         %s\n  registration:  %s\n", ep.Resource, ep.Authorization, ep.Token, ep.Registration)
	for _, w := range ep.Warnings {
		fmt.Fprintf(a.Out, "  note: %s (used built-in defaults where needed)\n", w)
	}
	if len(ep.ScopesSupported) > 0 {
		for _, s := range scopes {
			if !scopesHave(ep.ScopesSupported, s) {
				fmt.Fprintf(a.Out, "  warning: server does not advertise scope %s\n", s)
			}
		}
	}

	// 4. register client + listener
	a.step(4, "Register this CLI as an OAuth client")
	preferred := f.port
	if preferred == 0 && existing != nil && existing.RedirectURI != "" {
		if u, err := url.Parse(existing.RedirectURI); err == nil {
			preferred, _ = strconv.Atoi(u.Port())
		}
	}
	ln, err := auth.Listen(preferred)
	if err != nil {
		return a.stepFail("could not open a loopback listener on 127.0.0.1: "+err.Error(), "free a port or pass --port, or use --manual.")
	}
	defer ln.Close()
	redirect := ln.RedirectURI()
	var client *auth.Client
	if existing != nil && existing.ClientID != "" && existing.RedirectURI == redirect && existing.Server == server && existing.RegistrationEndpoint == ep.Registration {
		client = &auth.Client{ID: existing.ClientID, Secret: existing.ClientSecret, RedirectURI: redirect}
		fmt.Fprintln(a.Out, "  reusing the stored client registration")
	} else {
		client, err = auth.RegisterNamed(ctx, hc, ep, redirect, scopes, a.clientName())
		if err != nil {
			var he *auth.HTTPError
			detail := err.Error()
			if asHTTP(err, &he) {
				detail = fmt.Sprintf("HTTP %d from %s\n  response body: %s", he.Status, he.URL, strings.TrimSpace(he.Body))
			}
			return a.stepFail("client registration was rejected.\n  "+detail,
				"this may mean IBKR rejects loopback redirect URIs for dynamic clients (unverified). Retry once; if it persists, open an issue at https://github.com/NunoMotaRicardo/ibkr-cli/issues with the text above (no secrets are in it).")
		}
		fmt.Fprintf(a.Out, "  registered client (redirect %s)\n", redirect)
	}

	// 5. browser
	a.step(5, "Log in at IBKR and approve access")
	pk, err := auth.NewPKCE()
	if err != nil {
		return failErr("%v", err)
	}
	state, err := auth.NewState()
	if err != nil {
		return failErr("%v", err)
	}
	authURL := auth.AuthURL(ep, client, scopes, state, pk.Challenge)
	opened := false
	if !f.noBrowser && !f.manual && a.OpenBrowser != nil {
		if err := a.OpenBrowser(authURL); err == nil {
			opened = true
			fmt.Fprintln(a.Out, "  Opened your browser. Log in, complete 2FA (IBKR mobile app) if asked, then approve.")
		} else {
			fmt.Fprintf(a.Out, "  %v\n", err)
		}
	}
	if !opened {
		fmt.Fprintln(a.Out, "  Open this URL in a browser and approve access:")
	} else {
		fmt.Fprintln(a.Out, "  If nothing opened, use this URL:")
	}
	fmt.Fprintf(a.Out, "\n  %s\n\n", authURL)

	type outcome struct {
		cb  auth.Callback
		err error
	}
	ch := make(chan outcome, 2)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		cb, err := ln.Wait(wctx, f.wait)
		ch <- outcome{cb, err}
	}()
	if f.manual {
		fmt.Fprintln(a.Out, "  Manual mode: after approving, your browser is sent to a 127.0.0.1 address that may fail to load.")
		fmt.Fprintln(a.Out, "  Copy the full URL from the address bar (or just the code) and paste it here.")
		go func() {
			line, err := a.readLine("  Paste redirected URL: ")
			if err != nil {
				return
			}
			cb, perr := auth.ParseCallbackURL(line)
			ch <- outcome{cb, perr}
		}()
	}
	stopSpin := a.spinner(wctx, "Waiting for the browser login", f.wait)
	out := <-ch
	stopSpin()
	if out.err != nil {
		return a.stepFail(out.err.Error(), "run `"+a.connectHint()+"` again (use --manual on a headless machine, --wait to allow more time).")
	}
	if err := auth.ValidateCallback(out.cb, state, ep, f.manual); err != nil {
		return a.stepFail(err.Error(), "run `"+a.connectHint()+"` again and use only the newest browser tab.")
	}
	fmt.Fprintln(a.Out, "  Authorization received.")

	// 6. exchange + store
	a.step(6, "Exchange the code for tokens and store them")
	tok, err := auth.Exchange(ctx, hc, ep, client, out.cb.Code, pk.Verifier)
	if err != nil {
		return a.stepFail("token exchange failed: "+err.Error(), "run `"+a.connectHint()+"` again; codes are single-use and expire quickly.")
	}
	granted := tok.Scopes
	if len(granted) == 0 {
		granted = scopes
	}
	creds := &secrets.Credentials{
		Server: server, Issuer: ep.Issuer,
		AuthorizationEndpoint: ep.Authorization, TokenEndpoint: ep.Token, RevocationEndpoint: ep.Revocation, RegistrationEndpoint: ep.Registration,
		ClientID: client.ID, ClientSecret: client.Secret, RedirectURI: client.RedirectURI,
		AccessToken: tok.AccessToken, RefreshToken: tok.RefreshToken, TokenType: tok.TokenType, Expiry: tok.Expiry,
		Scopes: granted, ConnectedAt: a.now(),
	}
	src, err := a.Store.Save(creds)
	if err == nil {
		if rerr := a.Store.Register(); rerr != nil {
			fmt.Fprintf(a.Err, "warning: could not record the profile: %v\n", rerr)
		}
	}
	if err != nil {
		return a.stepFail("could not store credentials: "+err.Error(), "check permissions on "+a.Store.Dir+", or set IBKR_ACCESS_TOKEN for a session.")
	}
	fmt.Fprintf(a.Out, "  Stored in: %s\n  Granted scopes: %s\n", a.storageDesc(src), strings.Join(granted, " "))
	if scopesHave(scopes, "mcp.orders.submit") && !scopesHave(granted, "mcp.orders.submit") {
		fmt.Fprintln(a.Out, "  note: the live-order scope was not granted.")
	}
	if len(tok.RefreshToken) == 0 {
		fmt.Fprintln(a.Out, "  note: no refresh token issued; you will need to run `"+a.connectHint()+"` again when the token expires.")
	}

	// 7. verify
	a.step(7, "Verify the connection")
	a.cl, a.cache = nil, nil
	cl := a.mcpClient()
	info, err := cl.Initialize(ctx)
	if err != nil {
		return a.stepFail("MCP initialize failed: "+err.Error(), "run `ibkr status`; if the problem persists, `ibkr logout` then `ibkr connect`.")
	}
	fmt.Fprintf(a.Out, "  MCP session ok (%s %s, protocol %s)\n", info.Name, info.Version, info.ProtocolVersion)
	tc, err := a.toolCache(ctx, true)
	if err != nil {
		return a.stepFail("could not list tools: "+err.Error(), "retry `ibkr tools --refresh`.")
	}
	fmt.Fprintf(a.Out, "  %d tools available\n", len(tc.Tools))
	if res, err := a.call(ctx, "get_flow_type", nil, callOpts{}); err == nil {
		fmt.Fprintf(a.Out, "  flow type: %s\n", flowOf(res))
	} else {
		fmt.Fprintf(a.Out, "  flow type: unavailable (%v)\n", truncate(err.Error(), 100))
	}
	if res, err := a.call(ctx, "get_account_summary", nil, callOpts{}); err == nil {
		r := extract(res.Data(), View{Scalars: []string{"net_liquidation", "net_liquidation_value", "netliquidation", "available_funds", "buying_power", "currency"}}, nil, 0)
		var parts []string
		if len(r.Rows) > 0 {
			for _, c := range r.Cols {
				if len(parts) >= 3 {
					break
				}
				parts = append(parts, fmt.Sprintf("%s=%s", c, fmtValue(r.Rows[0][c])))
			}
		}
		fmt.Fprintf(a.Out, "  account: %s\n", strings.Join(parts, " "))
	} else {
		fmt.Fprintf(a.Out, "  account summary: unavailable (%v)\n", truncate(err.Error(), 100))
	}

	// 8. next steps
	a.step(8, "Done. Next steps")
	fmt.Fprintln(a.Out, "  ibkr status            connection, scopes, expiry")
	fmt.Fprintln(a.Out, "  ibkr positions         your open positions")
	fmt.Fprintln(a.Out, "  ibkr search AAPL       find a contract id, then `ibkr quote <id>`")
	fmt.Fprintln(a.Out, "  ibkr tools             everything the connector offers")
	fmt.Fprintln(a.Out, "  Agents: install the skill in skills/ibkr (see the README); it tells them to use this CLI.")
	return nil
}

func asHTTP(err error, target **auth.HTTPError) bool {
	for err != nil {
		if he, ok := err.(*auth.HTTPError); ok {
			*target = he
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// spinner shows progress on a TTY; otherwise it prints one line. The returned
// func stops it.
func (a *App) spinner(ctx context.Context, msg string, total time.Duration) func() {
	if !a.OutTTY {
		fmt.Fprintf(a.Out, "  %s (up to %s)...\n", msg, total)
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		frames := []string{"|", "/", "-", "\\"}
		start := time.Now()
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				fmt.Fprint(a.Out, "\r\033[K")
				return
			case <-ctx.Done():
				return
			case <-t.C:
				left := total - time.Since(start)
				fmt.Fprintf(a.Out, "\r  %s %s (%s left)", frames[i%len(frames)], msg, left.Round(time.Second))
			}
		}
	}()
	return func() { close(done); <-stopped }
}

var _ = mcp.ProtocolVersion
