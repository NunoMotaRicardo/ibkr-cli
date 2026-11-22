package cli

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
)

// addProfile stores working credentials for a named profile in the env's store dir.
func addProfile(t *testing.T, e *env, name string) {
	t.Helper()
	st := &secrets.Store{Service: "t", Dir: e.app.Store.Dir, NoKeyring: true, Profile: name}
	if _, err := st.Save(&secrets.Credentials{Server: e.f.MCPURL(), TokenEndpoint: e.f.URL + "/token", RevocationEndpoint: e.f.URL + "/revoke", ClientID: "c",
		AccessToken: "tok", RefreshToken: "r", Expiry: time.Now().Add(time.Hour), Scopes: []string{"mcp.read"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Register(); err != nil {
		t.Fatal(err)
	}
}

func TestSingleProfileNeedsNoFlag(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true) // legacy default login only
	if c := e.run("account"); c != 0 {
		t.Fatalf("code %d err %q", c, e.err.String())
	}
}

func TestSecondProfileForcesExplicitSelection(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	addProfile(t, e, "paper")

	c := e.run("account")
	if c != 2 || !strings.Contains(e.err.String(), "--profile") || !strings.Contains(e.err.String(), "paper") {
		t.Fatalf("want usage error naming profiles, got %d %q", c, e.err.String())
	}
	if c := e.run("--profile", "paper", "account"); c != 0 {
		t.Fatalf("--profile paper: %d %q", c, e.err.String())
	}
	if c := e.run("--profile", "default", "account"); c != 0 {
		t.Fatalf("--profile default: %d %q", c, e.err.String())
	}
	t.Setenv("IBKR_PROFILE", "paper")
	if c := e.run("account"); c != 0 {
		t.Fatalf("IBKR_PROFILE: %d %q", c, e.err.String())
	}
}

func TestExplicitForOrdersAndWrites(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	addProfile(t, e, "paper")
	if c := e.run("alert", "create", "--yes"); c != 2 {
		t.Fatalf("state-changing command without profile must be refused, got %d", c)
	}
	if len(f.CallsTo("create_alert")) != 0 {
		t.Fatal("tool was called despite missing profile")
	}
}

func TestExemptCommandsNeedNoProfile(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	addProfile(t, e, "paper")
	for _, args := range [][]string{{"profiles"}, {"version"}, {"--help"}} {
		if c := e.run(args...); c != 0 {
			t.Fatalf("%v: code %d err %q", args, c, e.err.String())
		}
	}
}

func TestProfilesListAndInvalidName(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	addProfile(t, e, "paper")
	e.run("profiles")
	out := e.out.String()
	if !strings.Contains(out, "default: connected") || !strings.Contains(out, "paper: connected") {
		t.Fatalf("profiles output %q", out)
	}
	if c := e.run("--profile", "Bad/Name", "account"); c != 2 {
		t.Fatalf("invalid name code %d", c)
	}
}

func TestStatusShowsProfileAndHint(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	if c := e.run("--profile", "paper", "status"); c != 3 {
		t.Fatalf("code %d", c)
	}
	out := e.out.String()
	if !strings.Contains(out, "profile: paper") || !strings.Contains(out, "ibkr --profile paper connect") {
		t.Fatalf("status %q", out)
	}
	if c := e.run("--profile", "paper", "positions"); c != 3 || !strings.Contains(e.err.String(), "ibkr --profile paper connect") {
		t.Fatalf("code %d err %q", c, e.err.String())
	}
}

func TestConnectProfileRegistersDistinctClientName(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	e.app.OpenBrowser = func(u string) error {
		go func() {
			if r, err := http.Get(u); err == nil {
				r.Body.Close()
			}
		}()
		return nil
	}
	if c := e.run("--profile", "paper", "connect", "--wait", "10s"); c != 0 {
		t.Fatalf("code %d\nout:%s\nerr:%s", c, e.out.String(), e.err.String())
	}
	if len(f.Registered) != 1 || f.Registered[0]["client_name"] != "ibkr-cli (paper)" {
		t.Fatalf("registered %v", f.Registered)
	}
	st := &secrets.Store{Service: "t", Dir: e.app.Store.Dir, NoKeyring: true, Profile: "paper"}
	if c, _, _ := st.Load(); c == nil || !c.HasToken() {
		t.Fatal("paper credentials not stored")
	}
	if names, _ := st.Profiles(); len(names) != 1 || names[0] != "paper" {
		t.Fatalf("registry %v", names)
	}
	if def := (&secrets.Store{Service: "t", Dir: e.app.Store.Dir, NoKeyring: true}); func() bool { c, _, _ := def.Load(); return c != nil }() {
		t.Fatal("connect --profile paper wrote the default profile")
	}
}

func TestLogoutOneProfileLeavesOther(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, true)
	addProfile(t, e, "paper")
	if c := e.run("--profile", "paper", "logout"); c != 0 {
		t.Fatalf("code %d err %q", c, e.err.String())
	}
	if c := e.run("account"); c != 0 { // only default remains: no flag needed
		t.Fatalf("default after paper logout: %d %q", c, e.err.String())
	}
}

func TestConnectWarnsAboutSameLoginWhenOtherProfilesExist(t *testing.T) {
	f := fakeIBKR()
	defer f.Close()
	e := newEnv(t, f, false)
	e.app.OpenBrowser = func(u string) error {
		go func() {
			if r, err := http.Get(u); err == nil {
				r.Body.Close()
			}
		}()
		return nil
	}
	const warn = "one consent per login"
	if c := e.run("--profile", "a", "connect", "--wait", "10s"); c != 0 {
		t.Fatalf("code %d\nerr:%s", c, e.err.String())
	}
	if strings.Contains(e.out.String(), warn) {
		t.Fatal("warned on the first profile")
	}
	e.out.Reset()
	if c := e.run("--profile", "b", "connect", "--wait", "10s"); c != 0 {
		t.Fatalf("code %d\nerr:%s", c, e.err.String())
	}
	if !strings.Contains(e.out.String(), warn) {
		t.Fatalf("no same-login warning on the second profile:\n%s", e.out.String())
	}
}
