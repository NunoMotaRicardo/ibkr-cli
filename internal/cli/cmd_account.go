package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/auth"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/mcp"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/version"
)

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return d.String()
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func (a *App) statusCmd() *cobra.Command {
	var offline bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show connection state, scopes, token expiry, flow type and storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.status(cmd.Context(), offline)
		},
	}
	c.Flags().BoolVar(&offline, "offline", false, "do not contact the server")
	return c
}

func (a *App) status(ctx context.Context, offline bool) error {
	creds, src, err := a.Store.Load()
	if err != nil {
		return failErr("cannot read stored credentials: %v", err)
	}
	kv := [][2]string{{"server", a.serverURL()}}
	connected := creds != nil && creds.HasToken()
	if !connected {
		kv = append(kv, [2]string{"connected", "no"}, [2]string{"hint", "run `ibkr connect` in a terminal"})
		a.printKV(kv)
		return &ExitErr{Code: ExitAuth}
	}
	kv = append(kv, [2]string{"connected", "yes"}, [2]string{"storage", a.storageDesc(src)})
	if creds.ClientID != "" {
		kv = append(kv, [2]string{"client_registered", "yes"})
	}
	if len(creds.Scopes) > 0 {
		kv = append(kv, [2]string{"scopes", strings.Join(creds.Scopes, " ")})
	} else if src == secrets.SourceEnv {
		kv = append(kv, [2]string{"scopes", "unknown (token from IBKR_ACCESS_TOKEN)"})
	}
	switch {
	case src == secrets.SourceEnv:
		kv = append(kv, [2]string{"token_expiry", "unknown (env token)"})
	case creds.Expiry.IsZero():
		kv = append(kv, [2]string{"token_expiry", "unknown"})
	case creds.Expiry.After(a.now()):
		kv = append(kv, [2]string{"token_expiry", "in " + humanDuration(creds.Expiry.Sub(a.now()))})
	default:
		note := "refreshes automatically on next use"
		if creds.RefreshToken == "" {
			note = "no refresh token: run `ibkr connect`"
		}
		kv = append(kv, [2]string{"token_expiry", "expired " + humanDuration(a.now().Sub(creds.Expiry)) + " ago (" + note + ")"})
	}
	kv = append(kv, [2]string{"refresh_token", yesNo(creds.RefreshToken != "")})
	if scopesHave(creds.Scopes, "mcp.orders.submit") {
		kv = append(kv, [2]string{"orders_scope", "granted (autonomous trading possible)"})
	} else {
		kv = append(kv, [2]string{"orders_scope", "not granted (live order tools unavailable)"})
	}
	if c, _ := mcp_loadCache(a); c != nil {
		kv = append(kv, [2]string{"tool_cache", fmt.Sprintf("%d tools, fetched %s ago", len(c.Tools), humanDuration(a.now().Sub(c.FetchedAt)))})
	}
	var retErr error
	if !offline {
		res, err := a.call(ctx, "get_flow_type", nil, callOpts{})
		switch {
		case err == nil:
			kv = append(kv, [2]string{"flow_type", flowOf(res)})
		case errors.Is(err, auth.ErrAuthExpired) || errors.Is(err, auth.ErrNotConnected):
			retErr = err
			kv = append(kv, [2]string{"flow_type", "unavailable: authorization problem"})
		default:
			kv = append(kv, [2]string{"flow_type", "unavailable: " + truncate(err.Error(), 100)})
		}
	}
	if a.opts.json {
		m := map[string]string{}
		for _, p := range kv {
			m[p[0]] = p[1]
		}
		b, _ := json.Marshal(m)
		fmt.Fprintln(a.Out, string(b))
	} else {
		a.printKV(kv)
	}
	return retErr
}

func mcp_loadCache(a *App) (*mcp.ToolCache, error) { return mcp.LoadCache(a.CacheDir) }

func (a *App) printKV(kv [][2]string) {
	for _, p := range kv {
		fmt.Fprintf(a.Out, "%s: %s\n", p[0], p[1])
	}
}

func (a *App) storageDesc(src secrets.Source) string {
	switch src {
	case secrets.SourceKeyring:
		return "OS keyring"
	case secrets.SourceFile:
		return "file " + a.Store.Path() + " (0600)"
	case secrets.SourceEnv:
		return "environment (IBKR_ACCESS_TOKEN)"
	}
	return string(src)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func scopesHave(s []string, want string) bool {
	for _, x := range s {
		if x == want {
			return true
		}
	}
	return false
}

// flowOf extracts the flow type string from a get_flow_type result.
func flowOf(res *mcp.CallResult) string {
	switch d := res.Data().(type) {
	case map[string]any:
		for _, k := range []string{"flow_type", "flowType", "flow", "type"} {
			if v, ok := d[k]; ok {
				return fmtValue(v)
			}
		}
		return compactJSON(d)
	case string:
		return strings.Trim(strings.TrimSpace(d), `"`)
	}
	return fmtValue(res.Data())
}

func (a *App) logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the tokens at IBKR and delete local credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			creds, src, err := a.Store.Load()
			if err != nil {
				return failErr("cannot read stored credentials: %v", err)
			}
			if creds == nil || (!creds.HasToken() && creds.ClientID == "") {
				fmt.Fprintln(a.Out, "Nothing stored; already logged out.")
				return nil
			}
			if src != secrets.SourceEnv && creds.RevocationEndpoint != "" && creds.ClientID != "" {
				ep := &auth.Endpoints{Revocation: creds.RevocationEndpoint}
				cl := &auth.Client{ID: creds.ClientID, Secret: creds.ClientSecret}
				hc := a.baseHC()
				for _, t := range []struct{ tok, hint string }{{creds.RefreshToken, "refresh_token"}, {creds.AccessToken, "access_token"}} {
					if t.tok == "" {
						continue
					}
					if err := auth.Revoke(ctx, hc, ep, cl, t.tok, t.hint); err != nil {
						fmt.Fprintf(a.Err, "warning: could not revoke %s at IBKR: %v\n", t.hint, err)
					} else {
						fmt.Fprintf(a.Out, "revoked %s at IBKR\n", t.hint)
					}
				}
			}
			if err := a.Store.Delete(); err != nil {
				return failErr("could not delete local credentials: %v", err)
			}
			fmt.Fprintln(a.Out, "Deleted local credentials.")
			if os := envTokenSet(); os {
				fmt.Fprintln(a.Out, "Note: IBKR_ACCESS_TOKEN is still set in your environment.")
			}
			fmt.Fprintln(a.Out, "To be sure access is gone, also check Client Portal > Settings > Manage Third-Party Consents.")
			return nil
		},
	}
}

func (a *App) versionCmd() *cobra.Command {
	var check bool
	c := &cobra.Command{
		Use:   "version",
		Short: "Print version, provider compatibility; --check looks for a newer release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(a.Out, version.Line())
			fmt.Fprintln(a.Out, version.Compat())
			if !check {
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.ReleaseURL, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("User-Agent", "ibkr-cli")
			resp, err := a.baseHC().Do(req)
			if err != nil {
				return failErr("could not check for updates: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return failErr("could not check for updates: GitHub returned HTTP %d", resp.StatusCode)
			}
			var rel struct {
				Tag string `json:"tag_name"`
				URL string `json:"html_url"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil || rel.Tag == "" {
				return failErr("could not parse the latest release response")
			}
			latest := strings.TrimPrefix(rel.Tag, "v")
			switch cmp := compareSemver(latest, version.Version); {
			case version.Version == "dev":
				fmt.Fprintf(a.Out, "latest release: %s (this is a development build)\n", rel.Tag)
			case cmp > 0:
				fmt.Fprintf(a.Out, "update available: %s -> %s\n  go install github.com/NunoMotaRicardo/ibkr-cli/cmd/ibkr@%s\n  or download a binary: %s\n", version.Version, rel.Tag, rel.Tag, rel.URL)
			default:
				fmt.Fprintf(a.Out, "up to date (latest release %s)\n", rel.Tag)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&check, "check", false, "query GitHub for the latest release")
	return c
}

// compareSemver compares dotted numeric versions (a pre-release suffix is ignored).
func compareSemver(a, b string) int {
	pa, pb := semverParts(a), semverParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func semverParts(s string) [3]int {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	var out [3]int
	for i, p := range strings.Split(s, ".") {
		if i > 2 {
			break
		}
		out[i], _ = strconv.Atoi(p)
	}
	return out
}
