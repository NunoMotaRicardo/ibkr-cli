package mcp

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/auth"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
	"github.com/NunoMotaRicardo/ibkr-cli/internal/testfake"
)

func bp(b bool) *bool { return &b }

func TestClassify(t *testing.T) {
	cases := []struct {
		t    Tool
		want Kind
	}{
		{Tool{Name: "get_account_summary"}, KindRead},
		{Tool{Name: "search_contracts"}, KindRead},
		{Tool{Name: "list_saved_screeners"}, KindRead},
		{Tool{Name: "preview_order"}, KindRead},
		{Tool{Name: "whats_new"}, KindRead},
		{Tool{Name: "create_alert"}, KindWrite},
		{Tool{Name: "delete_watchlist"}, KindWrite},
		{Tool{Name: "provide_customer_feedback"}, KindWrite},
		{Tool{Name: "create_order_instruction"}, KindWrite},
		{Tool{Name: "delete_order_instruction"}, KindWrite},
		{Tool{Name: "submit_order"}, KindOrder},
		{Tool{Name: "modify_order"}, KindOrder},
		{Tool{Name: "cancel_order"}, KindOrder},
		{Tool{Name: "cancel_all"}, KindOrder},
		{Tool{Name: "mystery_tool"}, KindWrite},
		{Tool{Name: "mystery_tool", Annotations: &Annotations{ReadOnlyHint: bp(true)}}, KindRead},
		{Tool{Name: "get_thing", Annotations: &Annotations{ReadOnlyHint: bp(false)}}, KindWrite},
		{Tool{Name: "get_thing", Annotations: &Annotations{DestructiveHint: bp(true)}}, KindWrite},
		{Tool{Name: "submit_order", Annotations: &Annotations{ReadOnlyHint: bp(true)}}, KindOrder},
	}
	for _, c := range cases {
		if got := Classify(c.t); got != c.want {
			t.Errorf("%s: got %v want %v", c.t.Name, got, c.want)
		}
	}
}

var priceSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"contract_id":       map[string]any{"type": "integer"},
		"contract_id_ex":    map[string]any{"type": "string"},
		"outside_rth":       map[string]any{"type": "boolean"},
		"min_strike":        map[string]any{"type": "number"},
		"step":              map[string]any{"type": "string", "enum": []any{"ONE_DAY", "ONE_HOUR"}},
		"market_data_names": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"last", "bid_ask"}}},
		"ids":               map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
	},
	"required": []any{"contract_id"},
}

func TestBuildArgs(t *testing.T) {
	got, err := BuildArgs(priceSchema, []string{"contract_id=265598", "contract_id_ex=8314", "outside_rth=true", "min_strike=12.5", "step=one_day", "market_data_names=last,bid_ask", "ids=1,2", "ids=3"}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"contract_id": int64(265598), "contract_id_ex": "8314", "outside_rth": true, "min_strike": 12.5, "step": "ONE_DAY",
		"market_data_names": []any{"last", "bid_ask"}, "ids": []any{int64(1), int64(2), int64(3)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
	for _, bad := range [][]string{{"contract_id=abc"}, {"contract_id=1", "bogus=1"}, {"contract_id=1", "step=weekly"}, {"contract_id=1", "outside_rth=maybe"}, {"noequals"}, {"outside_rth=true"}} {
		if _, err := BuildArgs(priceSchema, bad, ""); err == nil {
			t.Errorf("expected error for %v", bad)
		}
	}
	got, err = BuildArgs(priceSchema, []string{"contract_id=5"}, `{"outside_rth":false,"contract_id":9}`)
	if err != nil || got["contract_id"] != int64(5) || got["outside_rth"] != false {
		t.Fatalf("json merge: %v %v", got, err)
	}
	// no schema: infer, but *_ex stays string
	got, _ = BuildArgs(nil, []string{"a=1", "b=true", "c_ex=123@CME", "d=x", "e_ex=5"}, "")
	if got["a"] != int64(1) || got["b"] != true || got["c_ex"] != "123@CME" || got["d"] != "x" || got["e_ex"] != "5" {
		t.Fatalf("%v", got)
	}
}

func TestParamsAndPick(t *testing.T) {
	ps := Params(priceSchema)
	if ps[0].Name != "contract_id" || !ps[0].Required {
		t.Fatalf("%+v", ps[0])
	}
	if PickParam(priceSchema, "watchlist_id", "contract_id") != "contract_id" || PickParam(nil, "a", "b") != "a" {
		t.Fatal("PickParam")
	}
}

func newClient(f *testfake.Fake, tok string, st *secrets.Store) *Client {
	return &Client{URL: f.MCPURL(), HC: http.DefaultClient, Tokens: &auth.Manager{Store: st, HC: http.DefaultClient}}
}

func storeWith(t *testing.T, f *testfake.Fake, access string) *secrets.Store {
	t.Setenv("IBKR_ACCESS_TOKEN", "")
	st := &secrets.Store{Service: "x", Dir: t.TempDir(), NoKeyring: true}
	_, _ = st.Save(&secrets.Credentials{Server: f.MCPURL(), TokenEndpoint: f.URL + "/token", ClientID: "c", AccessToken: access, RefreshToken: "r", Expiry: time.Now().Add(time.Hour)})
	return st
}

func fakeWithTools(n int) *testfake.Fake {
	f := testfake.New()
	for i := 0; i < n; i++ {
		f.Tools = append(f.Tools, testfake.Tool("get_t"+string(rune('a'+i)), "desc", nil, nil))
	}
	return f
}

func TestClientListPaginatedAndCallJSONAndSSE(t *testing.T) {
	f := fakeWithTools(5)
	defer f.Close()
	f.PageSize = 2
	f.AllowToken("tok")
	f.Handlers["get_ta"] = func(a map[string]any) (string, bool) { return `{"x":1}`, false }
	c := newClient(f, "tok", storeWith(t, f, "tok"))
	tools, err := c.ListTools(context.Background())
	if err != nil || len(tools) != 5 {
		t.Fatalf("%d %v", len(tools), err)
	}
	for _, sse := range []bool{false, true} {
		f.SSE = sse
		res, err := c.CallTool(context.Background(), "get_ta", map[string]any{"a": 1})
		if err != nil {
			t.Fatal(err)
		}
		m, ok := res.Data().(map[string]any)
		if !ok || m["x"] == nil {
			t.Fatalf("sse=%v data=%#v", sse, res.Data())
		}
	}
	if f.Initializes != 1 {
		t.Fatalf("initializes %d", f.Initializes)
	}
}

func TestRefreshOn401(t *testing.T) {
	f := fakeWithTools(1)
	defer f.Close()
	// stored token is not accepted by the server -> 401 -> refresh -> retry
	c := newClient(f, "", storeWith(t, f, "revoked-token"))
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.RefreshCount != 1 || f.Unauthorized < 1 {
		t.Fatalf("refresh=%d unauthorized=%d", f.RefreshCount, f.Unauthorized)
	}
}

func TestAuthExpiredWhenRefreshFails(t *testing.T) {
	f := fakeWithTools(1)
	defer f.Close()
	f.RefreshFails = true
	c := newClient(f, "", storeWith(t, f, "bad"))
	_, err := c.ListTools(context.Background())
	if !errors.Is(err, auth.ErrAuthExpired) {
		t.Fatalf("%v", err)
	}
}

func TestTransportErrorOnTimeout(t *testing.T) {
	f := fakeWithTools(1)
	defer f.Close()
	f.AllowToken("tok")
	f.Hang["get_ta"] = true
	hc := &http.Client{Timeout: 150 * time.Millisecond}
	c := newClient(f, "tok", storeWith(t, f, "tok"))
	c.HC = hc
	_, err := c.CallTool(context.Background(), "get_ta", nil)
	var te *TransportError
	if !errors.As(err, &te) {
		t.Fatalf("want TransportError, got %v", err)
	}
}

func TestCache(t *testing.T) {
	dir := t.TempDir()
	if c, _ := LoadCache(dir); c != nil {
		t.Fatal("expected nil")
	}
	now := time.Now()
	c := &ToolCache{Server: "s", FetchedAt: now, Tools: []Tool{{Name: "b"}, {Name: "a"}}}
	if err := SaveCache(dir, c); err != nil {
		t.Fatal(err)
	}
	l, _ := LoadCache(dir)
	if !l.Fresh("s", time.Hour, now.Add(time.Minute)) || l.Fresh("s", time.Hour, now.Add(2*time.Hour)) || l.Fresh("other", time.Hour, now) {
		t.Fatal("freshness")
	}
	if _, ok := l.Find("a"); !ok || strings.Join(l.Names(), ",") != "a,b" {
		t.Fatal("find/names")
	}
}

func TestSnapshotDiff(t *testing.T) {
	mk := func(props map[string]any, req ...any) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": req}
	}
	snap := NewSnapshot("s", []Tool{
		{Name: "keep", Description: "Keeps things. More.", InputSchema: mk(map[string]any{"a": map[string]any{"type": "string", "description": "x"}})},
		{Name: "gone"},
		{Name: "chg", InputSchema: mk(map[string]any{"a": map[string]any{"type": "string"}}, "a")},
		{Name: "namesonly"},
	}, time.Now())
	live := []Tool{
		{Name: "keep", InputSchema: mk(map[string]any{"a": map[string]any{"type": "string", "description": "reworded"}})},
		{Name: "chg", InputSchema: mk(map[string]any{"a": map[string]any{"type": "integer"}, "b": map[string]any{"type": "string"}}, "a")},
		{Name: "namesonly", InputSchema: mk(map[string]any{"z": map[string]any{"type": "string"}})},
		{Name: "fresh"},
	}
	d := DiffSnapshot(snap, live)
	if !reflect.DeepEqual(d.Added, []string{"fresh"}) || !reflect.DeepEqual(d.Removed, []string{"gone"}) {
		t.Fatalf("%+v", d)
	}
	if len(d.Changed) != 1 || !strings.Contains(d.Changed[0], "+param b") || !strings.Contains(d.Changed[0], "~param a") {
		t.Fatalf("%+v", d.Changed)
	}
	if DiffSnapshot(snap, []Tool{{Name: "keep", InputSchema: live[0].InputSchema}, {Name: "gone"}, {Name: "chg", InputSchema: snap.Tools[0].InputSchema}}).Empty() {
		t.Log("ok")
	}
}

func TestCommittedSnapshotLoads(t *testing.T) {
	s, err := ReadSnapshot("../../specs/provider/tools-snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.ToolCount != 49 || len(s.Tools) != 49 {
		t.Fatalf("tool count %d/%d", s.ToolCount, len(s.Tools))
	}
	// the seed signatures must match what Signature() produces from a live-like schema
	live := []Tool{{Name: "get_account_orders", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"show_completed": map[string]any{"type": "boolean", "description": "x"}}, "required": []any{}}}}
	d := DiffSnapshot(s, live)
	for _, c := range d.Changed {
		t.Errorf("unexpected change: %s", c)
	}
}
