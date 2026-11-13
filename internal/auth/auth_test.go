package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/testfake"
)

func TestPKCEVector(t *testing.T) {
	got := Challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("challenge %s", got)
	}
	p, _ := NewPKCE()
	if len(p.Verifier) < 43 || Challenge(p.Verifier) != p.Challenge {
		t.Fatal("bad pkce")
	}
}

func TestDiscoverRegisterAndAuthURL(t *testing.T) {
	f := testfake.New()
	defer f.Close()
	ep, err := Discover(context.Background(), http.DefaultClient, f.MCPURL())
	if err != nil {
		t.Fatal(err)
	}
	if ep.Token != f.URL+"/token" || ep.Issuer != f.URL || !ep.IssParam || ep.Resource != f.MCPURL() {
		t.Fatalf("%+v", ep)
	}
	c, err := Register(context.Background(), http.DefaultClient, ep, "http://127.0.0.1:1234/callback", []string{"mcp.read"})
	if err != nil || c.ID != "client-1" {
		t.Fatal(err, c)
	}
	reg := f.Registered[0]
	if reg["token_endpoint_auth_method"] != "none" || reg["client_name"] != "ibkr-cli" {
		t.Fatalf("%v", reg)
	}
	u, _ := url.Parse(AuthURL(ep, c, []string{"mcp.read", "mcp.write"}, "st", "ch"))
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("resource") != f.MCPURL() || q.Get("state") != "st" || q.Get("scope") != "mcp.read mcp.write" {
		t.Fatalf("%v", q)
	}
}

func TestRegisterRejectedGivesHTTPError(t *testing.T) {
	f := testfake.New()
	defer f.Close()
	f.RejectDCR = true
	ep, _ := Discover(context.Background(), http.DefaultClient, f.MCPURL())
	_, err := Register(context.Background(), http.DefaultClient, ep, "http://127.0.0.1:1/callback", nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 400 || !strings.Contains(he.Body, "loopback") {
		t.Fatalf("%v", err)
	}
}

func TestValidateCallback(t *testing.T) {
	ep := &Endpoints{Issuer: "https://as", IssParam: true}
	cases := []struct {
		name string
		cb   Callback
		ok   bool
	}{
		{"good", Callback{Code: "c", State: "s", Iss: "https://as"}, true},
		{"bad state", Callback{Code: "c", State: "x", Iss: "https://as"}, false},
		{"bad iss", Callback{Code: "c", State: "s", Iss: "https://evil"}, false},
		{"missing iss", Callback{Code: "c", State: "s"}, false},
		{"error", Callback{Error: "access_denied"}, false},
		{"no code", Callback{State: "s", Iss: "https://as"}, false},
	}
	for _, c := range cases {
		if err := ValidateCallback(c.cb, "s", ep, false); (err == nil) != c.ok {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

func TestParseCallbackURL(t *testing.T) {
	cb, _ := ParseCallbackURL("http://127.0.0.1:9/callback?code=a&state=b&iss=https%3A%2F%2Fx")
	if cb.Code != "a" || cb.State != "b" || cb.Iss != "https://x" {
		t.Fatalf("%+v", cb)
	}
	if cb, _ := ParseCallbackURL("justcode"); cb.Code != "justcode" {
		t.Fatal("bare code")
	}
}

func TestListenerEndToEnd(t *testing.T) {
	f := testfake.New()
	defer f.Close()
	ctx := context.Background()
	ep, _ := Discover(ctx, http.DefaultClient, f.MCPURL())
	l, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := Register(ctx, http.DefaultClient, ep, l.RedirectURI(), nil)
	pk, _ := NewPKCE()
	go http.Get(AuthURL(ep, c, []string{"mcp.read"}, "state1", pk.Challenge)) // follows redirect to the listener
	cb, err := l.Wait(ctx, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCallback(cb, "state1", ep, false); err != nil {
		t.Fatal(err)
	}
	tok, err := Exchange(ctx, http.DefaultClient, ep, c, cb.Code, pk.Verifier)
	if err != nil || tok.AccessToken != "access-1" || tok.RefreshToken != "refresh-1" || len(tok.Scopes) != 2 {
		t.Fatalf("%+v %v", tok, err)
	}
}

func TestManagerRefresh(t *testing.T) {
	f := testfake.New()
	defer f.Close()
	t.Setenv("IBKR_ACCESS_TOKEN", "")
	st := &secrets.Store{Service: "x", Dir: t.TempDir(), NoKeyring: true}
	now := time.Now()
	_, _ = st.Save(&secrets.Credentials{Server: f.MCPURL(), TokenEndpoint: f.URL + "/token", ClientID: "c", AccessToken: "old", RefreshToken: "r", Expiry: now.Add(10 * time.Second)})
	m := &Manager{Store: st, HC: http.DefaultClient}
	tok, err := m.AccessToken(context.Background()) // within leeway -> proactive refresh
	if err != nil || tok != "access-r1" || f.RefreshCount != 1 {
		t.Fatalf("%q %v %d", tok, err, f.RefreshCount)
	}
	if c, _, _ := st.Load(); c.AccessToken != "access-r1" || c.RefreshToken != "r" {
		t.Fatalf("not persisted: %+v", c)
	}
	// still fresh now
	if tok2, _ := m.AccessToken(context.Background()); tok2 != "access-r1" || f.RefreshCount != 1 {
		t.Fatal("refreshed again")
	}
	f.RefreshFails = true
	if _, err := m.Refresh(context.Background()); !errors.Is(err, ErrAuthExpired) {
		t.Fatalf("want ErrAuthExpired, got %v", err)
	}
}

func TestManagerNotConnectedAndEnv(t *testing.T) {
	t.Setenv("IBKR_ACCESS_TOKEN", "")
	st := &secrets.Store{Service: "x", Dir: t.TempDir(), NoKeyring: true}
	m := &Manager{Store: st}
	if _, err := m.AccessToken(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatal(err)
	}
	t.Setenv("IBKR_ACCESS_TOKEN", "ci")
	if tok, err := m.AccessToken(context.Background()); err != nil || tok != "ci" {
		t.Fatal(tok, err)
	}
	if _, err := m.Refresh(context.Background()); !errors.Is(err, ErrAuthExpired) {
		t.Fatal(err)
	}
}
