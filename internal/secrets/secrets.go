// Package secrets stores OAuth client registration and tokens: OS keyring first,
// a 0600 JSON file as fallback, and IBKR_ACCESS_TOKEN as an override for CI.
package secrets

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

const keyringUser = "default"

// Source says where credentials came from.
type Source string

const (
	SourceNone    Source = "none"
	SourceKeyring Source = "keyring"
	SourceFile    Source = "file"
	SourceEnv     Source = "env"
)

// Credentials is everything persisted by `ibkr connect`.
type Credentials struct {
	Server string `json:"server"`
	Issuer string `json:"issuer,omitempty"`

	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string `json:"token_endpoint,omitempty"`
	RevocationEndpoint    string `json:"revocation_endpoint,omitempty"`
	RegistrationEndpoint  string `json:"registration_endpoint,omitempty"`

	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`

	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	ConnectedAt  time.Time `json:"connected_at,omitempty"`
}

// HasToken reports whether an access or refresh token is present.
func (c *Credentials) HasToken() bool {
	return c != nil && (c.AccessToken != "" || c.RefreshToken != "")
}

// Store persists Credentials.
type Store struct {
	Service   string // keyring service name
	Dir       string // directory for the fallback file
	NoKeyring bool   // skip the OS keyring (also IBKR_NO_KEYRING=1)
}

func (s *Store) useKeyring() bool {
	return !s.NoKeyring && os.Getenv("IBKR_NO_KEYRING") == ""
}

func (s *Store) file() string { return filepath.Join(s.Dir, "credentials.json") }

// Path returns the fallback file path.
func (s *Store) Path() string { return s.file() }

// Load returns stored credentials, applying the IBKR_ACCESS_TOKEN override.
// It returns (nil, SourceNone, nil) when nothing is stored and no env token is set.
func (s *Store) Load() (*Credentials, Source, error) {
	creds, src, err := s.loadStored()
	if err != nil {
		return nil, SourceNone, err
	}
	if tok := os.Getenv("IBKR_ACCESS_TOKEN"); tok != "" {
		if creds == nil {
			creds = &Credentials{}
		}
		c := *creds
		c.AccessToken = tok
		c.RefreshToken = ""
		c.Expiry = time.Time{}
		return &c, SourceEnv, nil
	}
	return creds, src, nil
}

func (s *Store) loadStored() (*Credentials, Source, error) {
	if s.useKeyring() {
		v, err := keyring.Get(s.Service, keyringUser)
		if err == nil {
			var c Credentials
			if jerr := json.Unmarshal([]byte(v), &c); jerr != nil {
				return nil, SourceNone, jerr
			}
			return &c, SourceKeyring, nil
		}
	}
	b, err := os.ReadFile(s.file())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, SourceNone, nil
	}
	if err != nil {
		return nil, SourceNone, err
	}
	var c Credentials
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, SourceNone, err
	}
	return &c, SourceFile, nil
}

// Save persists credentials and reports where they went.
func (s *Store) Save(c *Credentials) (Source, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return SourceNone, err
	}
	if s.useKeyring() {
		if err := keyring.Set(s.Service, keyringUser, string(b)); err == nil {
			_ = os.Remove(s.file())
			return SourceKeyring, nil
		}
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return SourceNone, err
	}
	tmp := s.file() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return SourceNone, err
	}
	if err := os.Rename(tmp, s.file()); err != nil {
		return SourceNone, err
	}
	return SourceFile, nil
}

// Delete removes credentials from every backend.
func (s *Store) Delete() error {
	if s.useKeyring() {
		_ = keyring.Delete(s.Service, keyringUser)
	}
	if err := os.Remove(s.file()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
