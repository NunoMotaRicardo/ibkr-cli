package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/mcp"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/testfake"
)

type env struct {
	app      *App
	out, err *bytes.Buffer
	f        *testfake.Fake
}

func obj(props map[string]any, req ...any) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": req}
}

func typ(t string) map[string]any { return map[string]any{"type": t} }

func fakeIBKR() *testfake.Fake {
	f := testfake.New()
	ro := true
	f.Tools = []map[string]any{
		testfake.Tool("get_flow_type", "Returns the order flow type.", nil, &ro),
		testfake.Tool("get_account_summary", "Account summary.", nil, &ro),
		testfake.Tool("get_account_positions", "Open positions.", nil, &ro),
		testfake.Tool("get_account_orders", "Orders.", obj(map[string]any{"show_completed": typ("boolean")}), &ro),
		testfake.Tool("search_contracts", "Search contracts by symbol.", obj(map[string]any{"query": typ("string")}, "query"), &ro),
		testfake.Tool("get_price_snapshot", "Price snapshot.", obj(map[string]any{"contract_id": typ("integer"), "exchange": typ("string"), "market_data_names": map[string]any{"type": "array", "items": typ("string")}}, "contract_id"), &ro),
		testfake.Tool("get_price_history", "Bars.", obj(map[string]any{
			"contract_id": typ("integer"), "security_type": map[string]any{"type": "string", "enum": []any{"STK", "OPT"}},
			"step": map[string]any{"type": "string", "enum": []any{"ONE_HOUR", "ONE_DAY"}}, "outside_rth": typ("boolean"),
			"period": map[string]any{"type": "string", "enum": []any{"ONE_MONTH", "ONE_WEEK"}}, "step_count": typ("integer")},
			"contract_id", "security_type", "step", "outside_rth"), &ro),
		testfake.Tool("create_alert", "Creates an alert.", obj(map[string]any{"contract_id": typ("integer"), "price": typ("number")}, "contract_id"), nil),
		testfake.Tool("create_order_instruction", "Creates an order instruction.", obj(map[string]any{"contract_id_ex": typ("string"), "side": typ("string"), "quantity": typ("number")}, "contract_id_ex"), nil),
		testfake.Tool("submit_order", "Submits a live order.", obj(map[string]any{"contract_id": typ("integer"), "side": typ("string"), "quantity": typ("number")}), nil),
		testfake.Tool("preview_order", "Previews an order.", obj(map[string]any{"contract_id": typ("integer"), "side": typ("string"), "quantity": typ("number")}), nil),
	}
	f.Handlers["get_flow_type"] = func(map[string]any) (string, bool) { return `{"flow_type":"REVIEW_AND_APPROVE"}`, false }
	f.Handlers["get_account_summary"] = func(map[string]any) (string, bool) {
		return `{"net_liquidation":1000.5,"currency":"USD","buying_power":2000}`, false
	}
	f.Handlers["get_account_positions"] = func(map[string]any) (string, bool) {
		return `{"positions":[{"symbol":"AAPL","contract_id":265598,"position":10,"market_value":1900.25,"contract_id_ex":"265598@SMART","junk":{"a":1}},{"symbol":"MSFT","contract_id":272093,"position":5,"market_value":2000,"new_field":"x"}]}`, false
	}
	f.Handlers["search_contracts"] = func(a map[string]any) (string, bool) {
		return `[{"symbol":"AAPL","contract_id":265598,"description":"APPLE INC","exchange":"NASDAQ"}]`, false
	}
	f.Handlers["get_price_snapshot"] = func(a map[string]any) (string, bool) {
		return `{"last":{"price":190.5,"ts":1},"bid-ask":{"bid":190.4,"ask":190.6}}`, false
	}
	f.Handlers["get_price_history"] = func(a map[string]any) (string, bool) {
		return `{"bars":[{"t":1,"o":1,"h":2,"l":0.5,"c":1.5,"v":100}]}`, false
	}
	f.Handlers["create_alert"] = func(a map[string]any) (string, bool) { return `{"alert_id":"A1"}`, false }
	f.Handlers["create_order_instruction"] = func(a map[string]any) (string, bool) {
		return `{"status":"created","url":"https://www.ibkr.example/review/abc123"}`, false
	}
	f.Handlers["submit_order"] = func(a map[string]any) (string, bool) { return `{"order_id":"9"}`, false }
	f.Handlers["preview_order"] = func(a map[string]any) (string, bool) { return `{"est_commission":1.0}`, false }
	return f
}

func newEnv(t *testing.T, f *testfake.Fake, connected bool) *env {
	t.Helper()
	t.Setenv("IBKR_ACCESS_TOKEN", "")
	t.Setenv("IBKR_MCP_URL", "")
	t.Setenv("IBKR_PROFILE", "")
	t.Setenv("IBKR_CONFIG_DIR", t.TempDir())
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	st := &secrets.Store{Service: "t", Dir: t.TempDir(), NoKeyring: true}
	if connected {
		f.AllowToken("tok")
		_, _ = st.Save(&secrets.Credentials{Server: f.MCPURL(), TokenEndpoint: f.URL + "/token", RevocationEndpoint: f.URL + "/revoke", ClientID: "c",
			AccessToken: "tok", RefreshToken: "r", Expiry: time.Now().Add(time.Hour), Scopes: []string{"mcp.read", "mcp.write"}})
	}
	a := &App{Out: out, Err: errb, In: strings.NewReader(""), Store: st, CacheDir: t.TempDir(), HC: http.DefaultClient}
	a.opts.server = f.MCPURL()
	return &env{a, out, errb, f}
}

// run executes a command line and returns the exit code. Output buffers are reset first.
func (e *env) run(args ...string) int {
	e.out.Reset()
	e.err.Reset()
	e.app.opts = options{server: e.f.MCPURL(), timeout: 5 * time.Second}
	e.app.cl, e.app.cache = nil, nil
	return e.app.Execute(append([]string{"--server", e.f.MCPURL()}, args...))
}

func TestNotConnectedExit3(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	if c := e.run("positions"); c != 3 || !strings.Contains(e.err.String(), "ibkr connect") {
		t.Fatalf("code %d err %q", c, e.err.String())
	}
	if c := e.run("tools"); c != 3 {
		t.Fatalf("tools code %d", c)
	}
}

func TestConnectWizardEndToEnd(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	e.app.OpenBrowser = func(u string) error {
		go func() {
			r, err := http.Get(u)
			if err == nil {
				r.Body.Close()
			}
		}()
		return nil
	}
	if c := e.run("connect", "--wait", "10s"); c != 0 {
		t.Fatalf("code %d\nout:%s\nerr:%s", c, e.out.String(), e.err.String())
	}
	out := e.out.String()
	for _, want := range []string{"[1/8]", "[8/8]", "Granted scopes: mcp.read mcp.write", "tools available", "flow type: REVIEW_AND_APPROVE", "account:"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	creds, _, _ := e.app.Store.Load()
	if creds == nil || creds.AccessToken != "access-1" || creds.RefreshToken != "refresh-1" || creds.ClientID != "client-1" {
		t.Fatalf("creds %+v", creds)
	}
	// resource + PKCE in the authorize request
	if !strings.Contains(f.LastAuthURL, "code_challenge_method=S256") || !strings.Contains(f.LastAuthURL, "resource=") {
		t.Fatalf("auth url %s", f.LastAuthURL)
	}
	// second connect reuses the registration
	if c := e.run("connect", "--wait", "10s"); c != 0 {
		t.Fatalf("second connect %d: %s", c, e.err.String())
	}
	if len(f.Registered) > 2 {
		t.Fatalf("registered %d times", len(f.Registered))
	}
}

func TestConnectDCRRejectedDiagnostic(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	f.RejectDCR = true
	e := newEnv(t, f, false)
	if c := e.run("connect", "--no-browser"); c != 1 {
		t.Fatalf("code %d", c)
	}
	if !strings.Contains(e.err.String(), "HTTP 400") || !strings.Contains(e.err.String(), "loopback not allowed") {
		t.Fatalf("diagnostic missing: %s", e.err.String())
	}
}

func TestConnectAllowOrdersNeedsTTY(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	if c := e.run("connect", "--allow-orders"); c != 1 || !strings.Contains(e.err.String(), "interactive") {
		t.Fatalf("code %d %s", c, e.err.String())
	}
}

func TestToolsDescribeAndCache(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	if c := e.run("tools"); c != 0 {
		t.Fatal(e.err.String())
	}
	if !strings.Contains(e.out.String(), "create_alert") || !strings.Contains(e.out.String(), "write") || !strings.Contains(e.out.String(), "get_flow_type") {
		t.Fatalf("%s", e.out.String())
	}
	before := f.Initializes
	e.run("tools", "--search", "alert")
	if f.Initializes != before {
		t.Fatal("cache not used")
	}
	if !strings.Contains(e.out.String(), "create_alert") || strings.Contains(e.out.String(), "get_flow_type") {
		t.Fatalf("search: %s", e.out.String())
	}
	if c := e.run("describe", "get_price_history"); c != 0 {
		t.Fatal(e.err.String())
	}
	d := e.out.String()
	if !strings.Contains(d, "contract_id  integer  required") || !strings.Contains(d, "enum: ONE_HOUR|ONE_DAY") || !strings.Contains(d, "[read]") {
		t.Fatalf("%s", d)
	}
	if c := e.run("describe", "get_price_hist"); c != 2 || !strings.Contains(e.err.String(), "unknown tool") {
		t.Fatalf("unknown tool: %d %s", c, e.err.String())
	}
}

func TestUnknownToolRefreshesCache(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	e.run("tools")
	f.Tools = append(f.Tools, testfake.Tool("get_brand_new", "New thing.", obj(map[string]any{"n": typ("integer")}, "n"), nil))
	f.Handlers["get_brand_new"] = func(a map[string]any) (string, bool) { return `{"n":` + toStr(a["n"]) + `}`, false }
	if c := e.run("call", "get_brand_new", "n=7"); c != 0 || !strings.Contains(e.out.String(), "n: 7") {
		t.Fatalf("%d out=%s err=%s", c, e.out.String(), e.err.String())
	}
}

func toStr(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestCallCoercionAndGate(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	// read tool: typed args
	if c := e.run("call", "get_price_snapshot", "contract_id=265598", "market_data_names=last,bid_ask", "--json"); c != 0 {
		t.Fatal(e.err.String())
	}
	calls := f.CallsTo("get_price_snapshot")
	if len(calls) != 1 || calls[0].Args["contract_id"] != float64(265598) {
		t.Fatalf("%+v", calls)
	}
	if arr, _ := calls[0].Args["market_data_names"].([]any); len(arr) != 2 {
		t.Fatalf("%+v", calls[0].Args)
	}
	// write tool without --yes: exit 4, nothing sent
	if c := e.run("call", "create_alert", "contract_id=1", "price=2.5"); c != 4 {
		t.Fatalf("code %d", c)
	}
	if len(f.CallsTo("create_alert")) != 0 || !strings.Contains(e.err.String(), "--yes") {
		t.Fatalf("gate failed: %s", e.err.String())
	}
	if c := e.run("call", "create_alert", "contract_id=1", "price=2.5", "--yes"); c != 0 {
		t.Fatal(e.err.String())
	}
	if got := f.CallsTo("create_alert"); len(got) != 1 || got[0].Args["price"] != 2.5 {
		t.Fatalf("%+v", got)
	}
	// bad arg => usage exit 2
	if c := e.run("call", "get_price_snapshot", "contract_id=abc"); c != 2 {
		t.Fatalf("code %d", c)
	}
}

func TestCuratedRendering(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	if c := e.run("positions"); c != 0 {
		t.Fatal(e.err.String())
	}
	out := e.out.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "symbol") || !strings.Contains(out, "AAPL") || strings.Contains(out, "junk") || strings.Contains(out, "contract_id_ex") || strings.Contains(out, "new_field") {
		t.Fatalf("text:\n%s", out)
	}
	e.run("positions", "--json", "--limit", "1")
	var rows []map[string]any
	if err := json.Unmarshal(e.out.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0]["symbol"] != "AAPL" || rows[0]["junk.a"] != nil {
		t.Fatalf("json: %s %v", e.out.String(), err)
	}
	e.run("positions", "--fields", "symbol,new_field,contract_id_ex")
	if !strings.Contains(e.out.String(), "265598@SMART") || !strings.Contains(e.out.String(), "x") {
		t.Fatalf("fields: %s", e.out.String())
	}
	e.run("positions", "--raw")
	if !strings.Contains(e.out.String(), `"junk"`) {
		t.Fatalf("raw: %s", e.out.String())
	}
	// object mode
	e.run("account")
	if !strings.Contains(e.out.String(), "net_liquidation: 1000.5") {
		t.Fatalf("account: %s", e.out.String())
	}
	// top-level array
	e.run("search", "AAPL")
	if !strings.Contains(e.out.String(), "APPLE INC") || !strings.Contains(e.out.String(), "contract_id") {
		t.Fatalf("search: %s", e.out.String())
	}
}

func TestQuoteAndHistoryArgs(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	if c := e.run("quote", "265598", "272093"); c != 0 {
		t.Fatal(e.err.String())
	}
	if !strings.Contains(e.out.String(), "last.price") || !strings.Contains(e.out.String(), "bid-ask.bid") || !strings.Contains(e.out.String(), "272093") {
		t.Fatalf("quote:\n%s", e.out.String())
	}
	if c := e.run("history", "265598", "--period", "1w", "--step", "1h"); c != 0 {
		t.Fatal(e.err.String())
	}
	h := f.CallsTo("get_price_history")[0].Args
	if h["period"] != "ONE_WEEK" || h["step"] != "ONE_HOUR" || h["security_type"] != "STK" || h["outside_rth"] != false || h["step_count"] != nil {
		t.Fatalf("%v", h)
	}
	if c := e.run("history", "265598", "--bars", "30"); c != 0 {
		t.Fatal(e.err.String())
	}
	h = f.CallsTo("get_price_history")[1].Args
	if h["step_count"] != float64(30) || h["period"] != nil {
		t.Fatalf("%v", h)
	}
	if c := e.run("history", "265598", "--bars", "3", "--period", "1w"); c != 2 {
		t.Fatalf("both -> %d", c)
	}
}

func TestInstructionAlwaysPrintsLink(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	if c := e.run("instruction", "create", "--contract", "8314", "--side", "buy", "--qty", "3"); c != 4 {
		t.Fatalf("code %d", c)
	}
	if c := e.run("instruction", "create", "--contract", "8314", "--side", "buy", "--qty", "3", "--yes"); c != 0 {
		t.Fatalf("code %d %s", c, e.err.String())
	}
	if !strings.Contains(e.out.String(), "Review link: https://www.ibkr.example/review/abc123") {
		t.Fatalf("%s", e.out.String())
	}
	a := f.CallsTo("create_order_instruction")[0].Args
	if a["contract_id_ex"] != "8314" || a["side"] != "BUY" || a["quantity"] != float64(3) {
		t.Fatalf("%v", a)
	}
	// link surfaces on stderr in json mode, stdout stays JSON
	e.run("instruction", "create", "--contract", "8314", "--yes", "--json")
	if !json.Valid(bytes.TrimSpace(e.out.Bytes())) || !strings.Contains(e.err.String(), "Review link:") {
		t.Fatalf("out=%s err=%s", e.out.String(), e.err.String())
	}
}

func TestOrderGatingPreviewAndAmbiguous(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	if c := e.run("order", "submit", "--contract", "1", "--side", "BUY", "--qty", "1"); c != 4 || !strings.Contains(e.err.String(), "LIVE") {
		t.Fatalf("code %d %s", c, e.err.String())
	}
	// interactive: preview first
	e.app.InTTY = true
	if c := e.run("order", "submit", "--contract", "1", "--side", "BUY", "--qty", "1", "--yes"); c != 0 {
		t.Fatalf("code %d %s", c, e.err.String())
	}
	if len(f.CallsTo("preview_order")) != 1 || len(f.CallsTo("submit_order")) != 1 || !strings.Contains(e.err.String(), "est_commission") {
		t.Fatalf("preview not run: %v", f.Calls)
	}
	e.app.InTTY = false
	// timeout -> exit 5, no retry
	f.Hang["submit_order"] = true
	e.app.HC = &http.Client{}
	start := time.Now()
	e.app.opts.timeout = 200 * time.Millisecond
	e.app.cl, e.app.cache = nil, nil
	e.out.Reset()
	e.err.Reset()
	code := e.app.Execute([]string{"--server", f.MCPURL(), "--timeout", "200ms", "order", "submit", "--contract", "1", "--side", "BUY", "--qty", "1", "--yes"})
	if code != 5 || !strings.Contains(e.err.String(), "ibkr orders") {
		t.Fatalf("code %d %s", code, e.err.String())
	}
	if n := len(f.CallsTo("submit_order")); n != 2 { // one earlier success + one hung attempt, no retry
		t.Fatalf("submit_order called %d times", n)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("too slow")
	}
}

func TestOrderToolsMissingExplains(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	f.Tools = f.Tools[:5]
	e := newEnv(t, f, true)
	if c := e.run("order", "submit", "--contract", "1", "--yes"); c != 1 || !strings.Contains(e.err.String(), "--allow-orders") {
		t.Fatalf("code %d %s", c, e.err.String())
	}
	if c := e.run("order", "cancel", "--order-id", "5", "--yes"); c != 1 || !strings.Contains(e.err.String(), "no cancel tool") {
		t.Fatalf("code %d %s", c, e.err.String())
	}
}

func TestRefreshOn401ThroughCLI(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	delete(f.ValidTokens, "tok") // server revoked the access token; refresh still works
	if c := e.run("positions"); c != 0 {
		t.Fatalf("code %d %s", c, e.err.String())
	}
	if f.RefreshCount != 1 {
		t.Fatalf("refresh count %d", f.RefreshCount)
	}
	f.RefreshFails = true
	delete(f.ValidTokens, "access-r1")
	if c := e.run("positions"); c != 3 {
		t.Fatalf("expired -> %d %s", c, e.err.String())
	}
	if !strings.Contains(e.err.String(), "ibkr connect") {
		t.Fatal(e.err.String())
	}
}

func TestSSEResponses(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	f.SSE = true
	e := newEnv(t, f, true)
	if c := e.run("account"); c != 0 || !strings.Contains(e.out.String(), "net_liquidation") {
		t.Fatalf("%d %s %s", c, e.out.String(), e.err.String())
	}
}

func TestToolsSnapshotAndDiff(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	path := filepath.Join(t.TempDir(), "snap.json")
	if c := e.run("tools", "--snapshot", path); c != 0 {
		t.Fatal(e.err.String())
	}
	if c := e.run("tools", "--diff", "--against", path); c != 0 || !strings.Contains(e.out.String(), "no differences") {
		t.Fatalf("%s %s", e.out.String(), e.err.String())
	}
	f.Tools = append(f.Tools[1:], testfake.Tool("get_new", "n", nil, nil))
	e.run("tools", "--diff", "--against", path)
	o := e.out.String()
	if !strings.Contains(o, "+ get_new") || !strings.Contains(o, "- get_flow_type") {
		t.Fatalf("%s", o)
	}
}

func TestStatusAndLogout(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	if c := e.run("status"); c != 0 {
		t.Fatalf("%d %s", c, e.err.String())
	}
	o := e.out.String()
	for _, w := range []string{"connected: yes", "scopes: mcp.read mcp.write", "flow_type: REVIEW_AND_APPROVE", "orders_scope: not granted", "token_expiry: in", "storage: file"} {
		if !strings.Contains(o, w) {
			t.Errorf("missing %q:\n%s", w, o)
		}
	}
	if strings.Contains(o, "tok") && strings.Contains(o, "access") {
		t.Fatal("secret leaked")
	}
	if c := e.run("logout"); c != 0 {
		t.Fatal(e.err.String())
	}
	if len(f.Revoked) != 2 || !strings.Contains(e.out.String(), "Manage Third-Party Consents") {
		t.Fatalf("revoked %v out %s", f.Revoked, e.out.String())
	}
	if c, _, _ := e.app.Store.Load(); c != nil {
		t.Fatal("creds not deleted")
	}
	if c := e.run("status"); c != 3 {
		t.Fatalf("status after logout %d", c)
	}
}

func TestVersionCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://example.test/rel"}`))
	}))
	defer srv.Close()
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	e.app.ReleaseURL = srv.URL
	if c := e.run("version", "--check"); c != 0 {
		t.Fatal(e.err.String())
	}
	if !strings.Contains(e.out.String(), "compatible with IBKR MCP connector 1.2.1") || !strings.Contains(e.out.String(), "latest release: v9.9.9") {
		t.Fatalf("%s", e.out.String())
	}
	if compareSemver("0.2.0", "0.1.9") != 1 || compareSemver("v1.0.0", "1.0.0") != 0 || compareSemver("0.1.0", "0.1.1") != -1 {
		t.Fatal("semver")
	}
}

func TestEnvTokenOverride(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	f.AllowToken("ci-token")
	t.Setenv("IBKR_ACCESS_TOKEN", "ci-token")
	if c := e.run("account"); c != 0 {
		t.Fatalf("%d %s", c, e.err.String())
	}
}

func TestExtractRobustness(t *testing.T) {
	for _, data := range []any{nil, "plain text", []any{}, []any{1, "x"}, map[string]any{}, map[string]any{"a": []any{map[string]any{"x": 1}}}, map[string]any{"positions": "oops"}} {
		r := extract(data, View{ListKeys: []string{"positions"}, Cols: []string{"symbol"}}, nil, 0)
		var b bytes.Buffer
		writeText(&b, r)
		if err := writeJSON(&b, r); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.Getenv
	_ = mcp.KindRead
}
