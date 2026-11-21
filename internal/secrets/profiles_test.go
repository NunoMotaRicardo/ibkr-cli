package secrets

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestProfileStorageIsolated(t *testing.T) {
	dir := t.TempDir()
	def := &Store{Service: "t", Dir: dir, NoKeyring: true}
	paper := &Store{Service: "t", Dir: dir, NoKeyring: true, Profile: "paper"}
	if _, err := def.Save(&Credentials{AccessToken: "live"}); err != nil {
		t.Fatal(err)
	}
	if _, err := paper.Save(&Credentials{AccessToken: "paper"}); err != nil {
		t.Fatal(err)
	}
	if got, want := def.Path(), filepath.Join(dir, "credentials.json"); got != want {
		t.Fatalf("default path %s want %s (must stay backward compatible)", got, want)
	}
	if got, want := paper.Path(), filepath.Join(dir, "profiles", "paper", "credentials.json"); got != want {
		t.Fatalf("paper path %s want %s", got, want)
	}
	c, _, _ := def.Load()
	p, _, _ := paper.Load()
	if c.AccessToken != "live" || p.AccessToken != "paper" {
		t.Fatalf("tokens mixed up: %q %q", c.AccessToken, p.AccessToken)
	}
	if err := paper.Delete(); err != nil {
		t.Fatal(err)
	}
	if c, _, _ := def.Load(); c == nil || c.AccessToken != "live" {
		t.Fatal("deleting paper touched default")
	}
}

func TestKeyringUserPerProfile(t *testing.T) {
	if (&Store{}).keyringUser() != "default" || (&Store{Profile: "default"}).keyringUser() != "default" {
		t.Fatal("default profile must keep keyring user \"default\"")
	}
	if got := (&Store{Profile: "paper"}).keyringUser(); got != "profile/paper" {
		t.Fatalf("got %q", got)
	}
}

func TestProfilesListIncludesLegacyDefault(t *testing.T) {
	dir := t.TempDir()
	def := &Store{Service: "t", Dir: dir, NoKeyring: true}
	if n, _ := def.Profiles(); len(n) != 0 {
		t.Fatalf("empty store: %v", n)
	}
	_, _ = def.Save(&Credentials{AccessToken: "x"}) // pre-profiles login: no registry file
	if n, _ := def.Profiles(); !reflect.DeepEqual(n, []string{"default"}) {
		t.Fatalf("legacy default not counted: %v", n)
	}
	paper := &Store{Service: "t", Dir: dir, NoKeyring: true, Profile: "paper"}
	_, _ = paper.Save(&Credentials{AccessToken: "y"})
	if err := paper.Register(); err != nil {
		t.Fatal(err)
	}
	_ = paper.Register() // idempotent
	if n, _ := def.Profiles(); !reflect.DeepEqual(n, []string{"default", "paper"}) {
		t.Fatalf("got %v", n)
	}
	if err := paper.Unregister(); err != nil {
		t.Fatal(err)
	}
	if n, _ := def.Profiles(); !reflect.DeepEqual(n, []string{"default"}) {
		t.Fatalf("after unregister: %v", n)
	}
}

func TestValidProfileName(t *testing.T) {
	for _, ok := range []string{"paper", "live-2", "a_b", "1x"} {
		if ValidProfileName(ok) != nil {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "Paper", "../x", "a b", "-x", "a/b"} {
		if ValidProfileName(bad) == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
