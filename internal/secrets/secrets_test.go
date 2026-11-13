package secrets

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestFileFallbackAndPerms(t *testing.T) {
	t.Setenv("IBKR_ACCESS_TOKEN", "")
	s := &Store{Service: "t", Dir: t.TempDir(), NoKeyring: true}
	if c, src, _ := s.Load(); c != nil || src != SourceNone {
		t.Fatal("expected empty")
	}
	src, err := s.Save(&Credentials{ClientID: "c", AccessToken: "a", Expiry: time.Now()})
	if err != nil || src != SourceFile {
		t.Fatalf("save: %v %v", src, err)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(s.Path())
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("perm %v", st.Mode().Perm())
		}
	}
	c, src, err := s.Load()
	if err != nil || src != SourceFile || c.ClientID != "c" {
		t.Fatalf("load: %+v %v %v", c, src, err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if c, _, _ := s.Load(); c != nil {
		t.Fatal("not deleted")
	}
}

func TestKeyringAndEnvOverride(t *testing.T) {
	keyring.MockInit()
	t.Setenv("IBKR_ACCESS_TOKEN", "")
	s := &Store{Service: "t", Dir: t.TempDir()}
	src, err := s.Save(&Credentials{ClientID: "c", AccessToken: "a", RefreshToken: "r"})
	if err != nil || src != SourceKeyring {
		t.Fatalf("save: %v %v", src, err)
	}
	if _, err := os.Stat(s.Path()); err == nil {
		t.Fatal("file should not exist when keyring works")
	}
	t.Setenv("IBKR_ACCESS_TOKEN", "envtok")
	c, src, _ := s.Load()
	if src != SourceEnv || c.AccessToken != "envtok" || c.RefreshToken != "" || c.ClientID != "c" {
		t.Fatalf("env override: %+v %v", c, src)
	}
}
